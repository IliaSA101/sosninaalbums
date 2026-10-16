package decor

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/decor/svgcheck"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Импорт стикеров (design §5.3, Ф2): JPEG, PNG и SVG. Растр длиннее 4096 px уменьшается в PNG;
// SVG проверяется в Go (decor/svgcheck), а рисует его окно — запись появляется в AddPNG.

// Пределы стикера.
const (
	MaxStickerBytes = 30 << 20 // исходник растра
	MaxStickerSide  = 4096     // длинная сторона копии в библиотеке
)

// importSticker — один файл стикера. job != nil — SVG чист и ждёт PNG от окна (записи пока нет).
// changed — реестр изменился (возврат скрытого).
func (s *Store) importSticker(theme, name, src string) (Item, *SvgJob, bool, error) {
	if paths.CheckLocalPath(src) != nil {
		return Item{}, nil, false, errors.New("файл не на локальном диске")
	}
	path, size, err := library.PlainFile(src, ".jpg", ".jpeg", ".png", ".svg")
	if errors.Is(err, library.ErrExt) {
		return Item{}, nil, false, errors.New("для стикера подходят JPEG, PNG и SVG")
	}
	if err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		return s.importSVG(theme, name, path, size)
	}
	if size > MaxStickerBytes {
		return Item{}, nil, false, fmt.Errorf("файл больше %d МБ", MaxStickerBytes>>20)
	}
	if err := imgfile.CheckSize(path); err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	dir := filepath.Join(s.root, kindDirs[KindSticker])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	tmp, key, err := copyHashed(path, dir, MaxStickerBytes)
	if err != nil {
		return Item{}, nil, false, err
	}
	defer func() { _ = os.Remove(tmp) }() // tmp меняется при уменьшении; после rename файла уже нет
	if s.alreadyHave(KindSticker, key) {
		return Item{}, nil, false, errHave
	}
	info, ext, err := checkCopy(tmp)
	if err != nil {
		return Item{}, nil, false, err
	}
	if max(info.W, info.H) > MaxStickerSide {
		small, sinfo, err := shrinkSticker(tmp, dir)
		if err != nil {
			return Item{}, nil, false, err
		}
		_ = os.Remove(tmp)
		tmp, info, ext = small, sinfo, "png"
	}
	it, changed, err := s.commit(KindSticker, theme, name, key, tmp, ext, info)
	return it, nil, changed, err
}

// shrinkSticker уменьшает копию до MaxStickerSide по длинной стороне (Lanczos) и пишет PNG
// (альфа сохраняется) во временный файл dir. Декод — через imgfile: лимиты, ориентация EXIF.
func shrinkSticker(tmp, dir string) (string, library.Photo, error) {
	var zero library.Photo
	img, release, err := imgfile.OpenOriented(tmp)
	if err != nil {
		return "", zero, errors.New("файл не читается как JPEG или PNG")
	}
	b := img.Bounds()
	k := float64(MaxStickerSide) / float64(max(b.Dx(), b.Dy()))
	w, h := max(1, int(math.Round(float64(b.Dx())*k))), max(1, int(math.Round(float64(b.Dy())*k)))
	small := imaging.Resize(img, w, h, imaging.Lanczos)
	release()
	out, err := os.CreateTemp(dir, tmpPrefix+"*.tmp")
	if err != nil {
		return "", zero, paths.NoPath(err)
	}
	path := out.Name()
	err = png.Encode(out, small)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	st, serr := os.Stat(path)
	if err == nil {
		err = serr
	}
	if err != nil {
		_ = os.Remove(path)
		return "", zero, paths.NoPath(err)
	}
	return path, library.Photo{W: w, H: h, Size: st.Size()}, nil
}

// importSVG проверяет SVG и выдаёт окну задание; тот же SVG, уже стоящий в библиотеке, —
// «уже есть», скрытый — возвращается.
func (s *Store) importSVG(theme, name, path string, size int64) (Item, *SvgJob, bool, error) {
	if size > svgcheck.MaxBytes {
		return Item{}, nil, false, fmt.Errorf("SVG больше %d МБ", svgcheck.MaxBytes>>20)
	}
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(f, svgcheck.MaxBytes+1))
	if err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	sz, err := svgcheck.Inspect(data) // и размер, и «больше предела»: файл мог вырасти после Stat
	if err != nil {
		return Item{}, nil, false, err
	}
	sum := sha256.Sum256(data)
	key := libPrefix + hex.EncodeToString(sum[:8])
	s.mu.RLock()
	e := s.entryOf(KindSticker, key)
	have, onDisk := s.files[KindSticker][key]
	n, _ := s.usage(KindSticker)
	s.mu.RUnlock()
	if onDisk {
		_, serr := os.Stat(have)
		onDisk = serr == nil
	}
	switch {
	case e != nil && onDisk && !e.Hidden:
		return Item{}, nil, false, errHave
	case e != nil && onDisk:
		s.mu.Lock()
		e.Hidden = false
		s.mu.Unlock()
		return e.item(), nil, true, nil
	case e == nil && n >= MaxOwn:
		return Item{}, nil, false, fmt.Errorf("своих уже %d — больше не помещается", MaxOwn)
	}
	tok, err := s.svg.issue(pendingSVG{kind: KindSticker, theme: theme, key: key, name: name, w: sz.W, h: sz.H})
	if err != nil {
		return Item{}, nil, false, errors.New("не удалось подготовить SVG")
	}
	return Item{}, &SvgJob{Token: tok, Name: name, Data: base64.StdEncoding.EncodeToString(data), W: sz.W, H: sz.H}, false, nil
}

// alreadyHave — видимый элемент вида с этим ключом есть и его файл на месте.
func (s *Store) alreadyHave(kind, key string) bool {
	s.mu.RLock()
	e := s.entryOf(kind, key)
	p, ok := s.files[kind][key]
	s.mu.RUnlock()
	if e == nil || e.Hidden || !ok {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}
