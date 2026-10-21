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
	dir   string
	sched *sched
	open  func(path string) (image.Image, func(), error) // декод оригинала; тесты считают вызовы
	mu    sync.Mutex
	// inflight — генерация копий фото идёт (ключ кэша); ждущие ждут канал, а не декодируют второй раз.
	inflight map[string]chan struct{}
	// tails — превью, которые кодируются в фоне после ответа миниатюрой (090 design §3, «inflight key_p»);
	// queue — не начатые из них по порядку, tailMax — предел очереди (память: ≈ 11 МБ на хвост).
	tails   map[string]*tail
	queue   []*tail
	tailMax int
	tailRun bool // горутина хвостов запущена
	closed  bool
	life    context.Context // снимается в Close: горутина хвостов не ждёт место после выхода
	stop    context.CancelFunc
	trimMu  sync.Mutex
	bg      sync.WaitGroup // фоновые Trim и горутина хвостов; Close ждёт их
}

func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cpu := runtime.NumCPU()
	// Запросам окна — места placesFor, прогреву — половина ядер, как раньше: imaging и так параллелит
	// ресайз внутри, остальное ЦП — окну и ленте (090-D2).
	life, stop := context.WithCancel(context.Background())
	c := &Cache{dir: dir, sched: newSched(placesFor(cpu), cpu/2), open: imgfile.OpenOriented,
		inflight: map[string]chan struct{}{}, tails: map[string]*tail{}, tailMax: tailsFor(cpu), life: life, stop: stop}
	c.bg.Go(func() { c.Trim(CacheMax, CacheTarget) }) // в фоне: старт приложения не ждёт обхода кэша
	return c, nil
}

// placesFor — мест генерации на cpu ядер: две трети, не меньше двух. Все ядра (090 G1) давали окну
// минус 30 %, но 12 декодов по ≈ 80 МБ — пик +520 МБ, и правке не оставалось свободных P (отклик
// 576 мс); треть ядер — правке и View, порядок мест и ответ до превью 2048 сохраняются (090M-D1).
// Не больше восьми: предел памяти (memLimit в app) постоянный, 21 место на 32 ядрах — ≈ 1,7 ГБ (090M-D4).
func placesFor(cpu int) int {
	return min(max(2, cpu*2/3), 8)
}

// tailsFor — предел очереди хвостов: два на ядро, не больше 24 (≈ 11 МБ на хвост): предел памяти постоянный (090M-D4).
func tailsFor(cpu int) int {
	return min(2*cpu, 24)
}

// Close ждёт фоновую чистку кэша и начатое превью в фоне: после них в папке кэша никто не пишет и не
// держит её открытой (тесты удаляют папку сразу после Close). Не начатые превью выбрасываются — их
// сделает следующий прогрев; запись атомарная, недописанных файлов нет. Новый Warm после Close не вызывать.
func (c *Cache) Close() {
	c.mu.Lock()
	c.closed = true
	for _, t := range c.queue {
		c.endTail(t)
	}
	c.queue = nil
	c.mu.Unlock()
	c.stop()
	c.bg.Wait()
}

// File — FileCtx без запроса (плитки шаблонов, тесты).
func (c *Cache) File(p Photo, preview bool) (string, error) {
	return c.FileCtx(context.Background(), p, preview)
}

// FileCtx возвращает путь к копии нужного размера, создавая её при необходимости; места запросов
// окна — раньше прогрева. Снятый ctx, пока запрос ждёт место или чужую генерацию, — ctx.Err() без
// обёртки, генерации нет; начатая генерация доделывается и остаётся в кэше (090-D4).
// preview=false — миниатюра для ленты: ответ сразу после её записи, превью того же декода
// кодируется в фоне. Копия — JPEG, а для PNG с прозрачностью — PNG: JPEG альфу не хранит,
// и прозрачные места стали бы чёрными.
func (c *Cache) FileCtx(ctx context.Context, p Photo, preview bool) (string, error) {
	return c.get(ctx, p, preview, now)
}

// get — копия фото для класса cl. Цикл: готовый файл; чужая генерация — ждать её; превью в фоне —
// забрать и докодировать самому (холст не ждёт конца всплеска ленты); иначе — своя генерация.
func (c *Cache) get(ctx context.Context, p Photo, preview bool, cl prio) (string, error) {
	key, err := cacheKey(p.Path, p.Color)
	if err != nil {
		return "", err
	}
	isPNG := isPNGPath(p.Path)
	for {
		if f, ok := c.find(key, preview, isPNG); ok {
			return f, nil
		}
		c.mu.Lock()
		ch, gen := c.inflight[key]
		t := c.tails[key]
		c.mu.Unlock()
		switch {
		case gen:
			if f, done, err := c.awaitGen(ctx, ch, key, p, preview); done || err != nil {
				return f, err
			}
		case preview && t != nil:
			if err := c.takeTail(ctx, t, cl); err != nil {
				return "", err
			}
		case t != nil:
			// Миниатюру убрал Trim, а превью того же декода ещё в очереди: ждать его, иначе своя
			// генерация снова и снова занимала бы место и уходила в retry.
			if err := wait(ctx, t.done); err != nil {
				return "", err
			}
		default:
			retry, err := c.generateOwn(ctx, key, p.Path, preview, cl)
			if err != nil {
				return "", err
			}
			if retry {
				continue
			}
			if f, ok := c.find(key, preview, isPNG); ok {
				return f, nil
			}
			return "", noCopyErr(p)
		}
	}
}

// noCopyErr — ошибка «копии нет»; в тексте имя файла без пути (073-7).
func noCopyErr(p Photo) error {
	return fmt.Errorf("не удалось сделать превью (файл %q)", paths.LogExt(p.Name))
}

// awaitGen ждёт чужую генерацию. done — ответ готов (файл или ошибка); иначе get продолжает цикл:
// превью того же декода ещё в очереди хвоста.
func (c *Cache) awaitGen(ctx context.Context, ch <-chan struct{}, key string, p Photo, preview bool) (f string, done bool, err error) {
	if err := wait(ctx, ch); err != nil {
		return "", true, err
	}
	if f, ok := c.find(key, preview, isPNGPath(p.Path)); ok {
		return f, true, nil
	}
	if !preview || !c.hasTail(key) {
		return "", true, noCopyErr(p) // генерация не удалась: второй раз тот же файл не читаем
	}
	return "", false, nil
}

// generateOwn — своя генерация на месте класса cl. retry — пока ждали место, фото взял другой
// (генерация или превью в фоне): вернуться в цикл get и ждать его.
func (c *Cache) generateOwn(ctx context.Context, key, src string, preview bool, cl prio) (retry bool, err error) {
	release, err := c.sched.acquire(ctx, cl)
	if err != nil {
		return false, err
	}
	defer release() // и при панике декодера: место не теряется
	c.mu.Lock()
	if _, busy := c.inflight[key]; busy || c.tails[key] != nil {
		c.mu.Unlock()
		return true, nil
	}
	ch := make(chan struct{})
	c.inflight[key] = ch
	split := cl == now && !preview && !c.closed
	c.mu.Unlock()
	defer func() { // после постановки хвоста: запрос превью видит генерацию, хвост или файл
		c.mu.Lock()
		delete(c.inflight, key)
		c.mu.Unlock()
		close(ch)
	}()
	// Копию могли дописать между поиском и записью в inflight: второй раз не декодируем.
	if _, ok := c.find(key, preview, isPNGPath(src)); ok {
		return false, nil
	}
	t, err := c.generate(src, key, split)
	if err != nil {
		return false, err
	}
	if t != nil && !c.queueTail(t) {
		c.runTail(t) // очередь полна или кэш закрыт: превью — здесь же, до ответа, как раньше
	}
	return false, nil
}

// wait ждёт канал или снятие ctx.
func wait(ctx context.Context, ch <-chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Warm заранее готовит миниатюры, чтобы лента не ждала.
func (c *Cache) Warm(ctx context.Context, photos []Photo, progress func(done, total int)) {
	defer func() { c.bg.Go(func() { c.Trim(CacheMax, CacheTarget) }) }()
	var done atomic.Int64
	jobs := make(chan Photo)
	var wg sync.WaitGroup
	for range c.sched.bgMax {
		wg.Go(func() {
			for p := range jobs {
				warmOne(ctx, c, p)
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

// warmOne — один прогрев: миниатюра, превью (обычно его пишет та же генерация; нет — хвост ещё в очереди
// или выброшен при выходе) и подпись. Паника декодера (битый файл) гасится, воркер и процесс живы.
func warmOne(ctx context.Context, c *Cache, p Photo) {
	defer func() {
		if r := recover(); r != nil {
			// Без имени файла: в log.txt, который отправляют разработчику, имена клиентов — лишнее (073-7).
			log.Printf("ПАНИКА при прогреве (файл %q): %v\n%s", paths.LogExt(p.Name), r, debug.Stack())
		}
	}()
	if _, err := c.get(ctx, p, false, bg); err != nil {
		return // прогрев: ошибка проявится при реальном запросе
	}
	if _, err := c.get(ctx, p, true, bg); err != nil {
		return
	}
	_, _ = c.Sig(p) // подпись из готовой миниатюры (~7 мс, оригинал второй раз не читается)
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

// generate декодирует оригинал и пишет копии; картинка с прозрачностью пишется PNG (альфа
// сохраняется), остальные — JPEG. split — только миниатюра: оригинал отпускается, превью тех же
// пикселей возвращается хвостом для кодирования позже (без второго декода, 090-D3).
func (c *Cache) generate(src, key string, split bool) (*tail, error) {
	img, release, err := c.open(src)
	if err != nil {
		return nil, err
	}
	release = sync.OnceFunc(release)
	defer release()
	// alpha — до fit: после него img больше не нужен, и декод (до 96 МБ) уходит сборщику во время
	// вертикального прохода и кодирования, а не в конце generate (090M).
	alpha := !isOpaque(img)
	prev := fit(img, PreviewSize, imaging.Box)
	thumb := fit(prev, ThumbSize, imaging.Linear)
	thumbBase := c.base(key, false)
	if err := os.MkdirAll(filepath.Dir(thumbBase), 0o755); err != nil {
		return nil, err
	}
	if split {
		if err := writeCopy(thumb, thumbBase, alpha, false); err != nil {
			return nil, err
		}
		release() // бюджет декода — следующему: хвост держит только свои 2048 px
		return &tail{key: key, prev: prev, alpha: alpha, done: make(chan struct{})}, nil
	}
	if err := writeCopy(prev, c.base(key, true), alpha, true); err != nil {
		return nil, err
	}
	return nil, writeCopy(thumb, thumbBase, alpha, false)
}

// writeCopy пишет копию base+".png" (с прозрачностью) или base+".jpg": превью q88, миниатюра q82.
func writeCopy(img image.Image, base string, alpha, preview bool) error {
	if alpha {
		return savePNG(img, base+".png")
	}
	q := 82
	if preview {
		q = 88
	}
	return saveJPEG(img, base+".jpg", q)
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
