// Package library — фотографии из папок: список, размеры, миниатюры.
package library

import (
	"crypto/sha1" //nolint:gosec // sha1 — только ключ кэша, не защита
	"encoding/hex"
	"image"
	"image/color"
	_ "image/jpeg" // регистрация декодеров для image.DecodeConfig
	_ "image/png"  // то же
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// Photo — одна фотография. W и H уже с учётом поворота из EXIF.
type Photo struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Rotate int    `json:"-"` // EXIF Orientation
	// Taken — время съёмки (EXIF), секунды Unix, местное время камеры как UTC; 0 — нет. Для серий автосборки.
	Taken int64 `json:"-"`
	// Size — размер файла в байтах (из Stat при чтении).
	Size int64 `json:"size,omitempty"`
	// Color и Profile — класс и имя цветового профиля: от них зависят значки в интерфейсе.
	Color   imgfile.Color `json:"color,omitempty"`
	Profile string        `json:"profile,omitempty"`
	// Xmp — отметки из файла (XMP в JPEG и сайдкар); nil — нет, чтобы не раздувать событие library.
	Xmp *Marks `json:"xmp,omitempty"`
}

func (p Photo) Aspect() float64 {
	if p.H == 0 {
		return 1
	}
	return float64(p.W) / float64(p.H)
}

// IDFor — стабильный идентификатор фото по пути к файлу.
func IDFor(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	sum := sha1.Sum([]byte(strings.ToLower(filepath.Clean(path)))) //nolint:gosec // ключ кэша, не криптография
	return hex.EncodeToString(sum[:8])
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// ReadInfo читает заголовок фото: размеры с учётом поворота EXIF, размер файла, цвет.
// Нужен проекту, когда файл на диске сменился (пересохранили, повернули); отметки XMP не читает.
func ReadInfo(path string) (Photo, error) { return readInfo(path) }

func readInfo(path string) (Photo, error) {
	ph, _, err := readHead(path)
	return ph, err
}

// readHead — readInfo и сырой пакет XMP из заголовка JPEG (разбирает его только сканер).
func readHead(path string) (Photo, []byte, error) {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return Photo{}, nil, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	st, err := f.Stat()
	if err != nil {
		return Photo{}, nil, err
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return Photo{}, nil, err
	}
	if err := imgfile.CheckDims(cfg.Width, cfg.Height); err != nil {
		return Photo{}, nil, err
	}
	head := imgfile.Head{Orient: 1}
	if _, err := f.Seek(0, 0); err == nil {
		head = imgfile.Header(f)
	}
	col, prof := head.Color, head.Profile
	if col == imgfile.ColorNone && cfg.ColorModel == color.CMYKModel {
		col, prof = imgfile.ColorOther, "CMYK" // CMYK без ICC: пересчитать нечем, только предупредить
	}
	w, h := cfg.Width, cfg.Height
	if head.Orient >= 5 {
		w, h = h, w
	}
	return Photo{
		ID: IDFor(path), Path: path, Name: filepath.Base(path), W: w, H: h,
		Rotate: head.Orient, Taken: head.Taken, Size: st.Size(), Color: col, Profile: prof,
	}, head.XMP, nil
}

// Under сообщает, лежит ли файл path внутри папки folder (на любой глубине).
// Сравнение по префиксу с разделителем, без учёта регистра: «C:\Фото2» не внутри «C:\Фото».
func Under(path, folder string) bool {
	p := strings.ToLower(filepath.Clean(path))
	f := strings.ToLower(filepath.Clean(folder))
	if !strings.HasSuffix(f, string(filepath.Separator)) {
		f += string(filepath.Separator)
	}
	return strings.HasPrefix(p, f)
}

// naturalLess сравнивает пути так, чтобы img-9 шло раньше img-10. Пути — уже в нижнем регистре:
// сканер приводит их один раз, а не на каждом сравнении сортировки.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		ca, cb := a[0], b[0]
		if isDigit(ca) && isDigit(cb) {
			na, ra := splitNum(a)
			nb, rb := splitNum(b)
			ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			a, b = ra, rb
			continue
		}
		if ca != cb {
			return ca < cb
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func splitNum(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}
