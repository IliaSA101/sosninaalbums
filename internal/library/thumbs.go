package library

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // sha1 — только ключ кэша, не защита
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/disintegration/imaging"
)

// Размеры уменьшенных копий по длинной стороне.
const (
	ThumbSize   = 400  // лента фото
	PreviewSize = 2048 // фото на развороте
)

// Cache делает и хранит на диске уменьшенные копии фото.
// Из одного чтения оригинала получаются сразу обе копии.
type Cache struct {
	dir      string
	sem      chan struct{}
	mu       sync.Mutex
	inflight map[string]chan struct{}
	trimMu   sync.Mutex
	bg       sync.WaitGroup // фоновые Trim; Close ждёт их
}

func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	n := max(1, runtime.NumCPU()/2) // imaging и так параллелит ресайз внутри
	c := &Cache{dir: dir, sem: make(chan struct{}, n), inflight: map[string]chan struct{}{}}
	c.bg.Go(func() { c.Trim(CacheMax, CacheTarget) }) // в фоне: старт приложения не ждёт обхода кэша
	return c, nil
}

// Close ждёт фоновую чистку кэша: после неё в папке кэша никто не пишет и не держит её открытой
// (тесты удаляют папку сразу после Close). Новый Warm после Close не вызывать.
func (c *Cache) Close() { c.bg.Wait() }

// File возвращает путь к копии нужного размера, создавая её при необходимости.
// preview=false — миниатюра для ленты. Копия — JPEG, а для PNG с прозрачностью — PNG:
// JPEG альфу не хранит, и прозрачные места стали бы чёрными.
func (c *Cache) File(p Photo, preview bool) (string, error) {
	key, err := cacheKey(p.Path, p.Color)
	if err != nil {
		return "", err
	}
	isPNG := isPNGPath(p.Path)
	if f, ok := c.find(key, preview, isPNG); ok {
		return f, nil
	}

	c.mu.Lock()
	if ch, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-ch
		if f, ok := c.find(key, preview, isPNG); ok {
			return f, nil
		}
		return "", fmt.Errorf("не удалось сделать превью (файл %q)", paths.LogExt(p.Name))
	}
	ch := make(chan struct{})
	c.inflight[key] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.inflight, key)
		c.mu.Unlock()
		close(ch)
	}()

	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	if err := generate(p.Path, c.base(key, false), c.base(key, true)); err != nil {
		return "", err
	}
	if f, ok := c.find(key, preview, isPNG); ok {
		return f, nil
	}
	return "", fmt.Errorf("не удалось сделать превью (файл %q)", paths.LogExt(p.Name))
}

// Warm заранее готовит миниатюры, чтобы лента не ждала.
func (c *Cache) Warm(ctx context.Context, photos []Photo, progress func(done, total int)) {
	defer func() { c.bg.Go(func() { c.Trim(CacheMax, CacheTarget) }) }()
	var done atomic.Int64
	jobs := make(chan Photo)
	var wg sync.WaitGroup
	for range cap(c.sem) {
		wg.Go(func() {
			for p := range jobs {
				warmOne(c, p)
				progress(int(done.Add(1)), len(photos))
			}
		})
	}
	for _, p := range photos {
		select {
		case jobs <- p:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}

// warmOne — один прогрев; паника декодера (битый файл) гасится, воркер и процесс живы.
func warmOne(c *Cache, p Photo) {
	defer func() {
		if r := recover(); r != nil {
			// Без имени файла: в log.txt, который отправляют разработчику, имена клиентов — лишнее (073-7).
			log.Printf("ПАНИКА при прогреве (файл %q): %v\n%s", paths.LogExt(p.Name), r, debug.Stack())
		}
	}()
	_, _ = c.File(p, false) // прогрев: ошибка проявится при реальном запросе
}

// base — путь копии без расширения: его ставит generate по тому, есть ли в картинке прозрачность.
func (c *Cache) base(key string, preview bool) string {
	suffix := "_t"
	if preview {
		suffix = "_p"
	}
	return filepath.Join(c.dir, key[:2], key+suffix)
}

// find ищет готовую копию. Расширение до генерации неизвестно (зависит от альфы), поэтому
// пробуем оба; у PNG сначала .png, у остальных .png и не бывает, но проверка дёшева.
func (c *Cache) find(key string, preview, isPNG bool) (string, bool) {
	exts := [2]string{".jpg", ".png"}
	if isPNG {
		exts = [2]string{".png", ".jpg"}
	}
	for _, e := range exts {
		if f := c.base(key, preview) + e; exists(f) {
			return f, true
		}
	}
	return "", false
}

func isPNGPath(path string) bool { return strings.EqualFold(filepath.Ext(path), ".png") }

// cacheKey меняется, если файл перезаписали (другой размер или время). У Adobe-фото
// к ключу добавлена версия пересчёта в sRGB: старые блёклые копии не подхватятся;
// так же у .png — версия «PNG с альфой»: прежние копии прозрачных картинок были чёрными
// JPEG. У JPEG ключ прежний, их кэш сохраняется.
func cacheKey(path string, col imgfile.Color) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	s := fmt.Sprintf("%s|%d|%d", path, st.Size(), st.ModTime().UnixNano())
	if col == imgfile.ColorAdobe {
		s += "|srgb1"
	}
	if isPNGPath(path) {
		s += "|png1"
	}
	sum := sha1.Sum([]byte(s)) //nolint:gosec // ключ кэша, не криптография
	return hex.EncodeToString(sum[:10]), nil
}

// generate делает обе копии; thumbBase и prevBase — пути без расширения. Картинка с прозрачностью
// пишется PNG (альфа сохраняется), остальные — JPEG.
func generate(src, thumbBase, prevBase string) error {
	img, release, err := imgfile.OpenOriented(src)
	if err != nil {
		return err
	}
	defer release()
	prev := fit(img, PreviewSize, imaging.Box)
	thumb := fit(prev, ThumbSize, imaging.Linear)
	if err := os.MkdirAll(filepath.Dir(thumbBase), 0o755); err != nil {
		return err
	}
	if !isOpaque(img) {
		if err := savePNG(prev, prevBase+".png"); err != nil {
			return err
		}
		return savePNG(thumb, thumbBase+".png")
	}
	if err := saveJPEG(prev, prevBase+".jpg", 88); err != nil {
		return err
	}
	return saveJPEG(thumb, thumbBase+".jpg", 82)
}

// isOpaque — нет ли в картинке прозрачных пикселей. Тип без Opaque (редкий) считаем прозрачным:
// PNG безопасен, чёрный фон нет.
func isOpaque(img image.Image) bool {
	o, ok := img.(interface{ Opaque() bool })
	return ok && o.Opaque()
}

func fit(img image.Image, size int, f imaging.ResampleFilter) image.Image {
	b := img.Bounds()
	if b.Dx() <= size && b.Dy() <= size {
		return img
	}
	return imaging.Fit(img, size, size, f)
}

func saveJPEG(img image.Image, path string, q int) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return err
	}
	return WriteFileAtomic(path, buf.Bytes())
}

func savePNG(img image.Image, path string) error {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed} // таких картинок единицы, но ждёт их лента
	if err := enc.Encode(&buf, img); err != nil {
		return err
	}
	return WriteFileAtomic(path, buf.Bytes())
}

// WriteFileAtomic пишет через временный path.<случайное>.tmp: Sync и Rename, при любой ошибке
// .tmp удаляется. Так оборванная запись (выход, нехватка места) не оставляет битый файл под
// настоящим именем. Имя случайное: антивирус при удалении 02.jpg переименовывает его в
// 02.JPG.tmp и держит открытым, а постоянное 02.jpg.tmp с ним совпало бы («used by another process»).
func WriteFileAtomic(path string, data []byte) error {
	return WriteFileAtomicMode(path, data, 0o644) // CreateTemp даёт 0600, а файл экспорта и настроек — обычный
}

// WriteFileAtomicMode — WriteFileAtomic с правами mode (резервные копии — 0600).
func WriteFileAtomicMode(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	err = f.Chmod(mode)
	if err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp) // уже есть ошибка записи; если и это не вышло — в кэше остаток уберёт Trim, в папке экспорта — RunPlan
	}
	return err
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
