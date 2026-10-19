package library

import (
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// sigPath — файл подписи рядом с миниатюрой: тот же ключ, значит смена файла даёт новую подпись.
func (c *Cache) sigPath(key string) string {
	return filepath.Join(c.dir, key[:2], key+"_s.bin")
}

// Sig возвращает подпись кадра (спека 078): из файла <ключ>_s.bin, а если его нет или он повреждён —
// считает по миниатюре 400 px (при нужде сначала делает её) и записывает файл. Подпись всегда из
// декода миниатюры, а не из оригинала: не зависит от того, как миниатюра появилась (решение 078-2).
func (c *Cache) Sig(p Photo) (imgsig.Sig, error) {
	key, err := cacheKey(p.Path, p.Color)
	if err != nil {
		return imgsig.Sig{}, err
	}
	sp := c.sigPath(key)
	if s, ok := readSig(sp); ok {
		return s, nil
	}
	f, err := c.File(p, false)
	if err != nil {
		return imgsig.Sig{}, err
	}
	img, err := decodeThumb(f)
	if err != nil {
		return imgsig.Sig{}, err
	}
	s := imgsig.Of(img)
	if !s.Ok {
		return imgsig.Sig{}, errors.New("пустая миниатюра, подписи нет")
	}
	// Не записалось (нет места, антивирус) — подпись верна, её просто посчитают снова при следующем окне.
	_ = WriteFileAtomic(sp, s.Bytes())
	return s, nil
}

// errAlphaSig — у картинки с прозрачностью подписи для отбора похожих нет.
var errAlphaSig = errors.New("картинка с прозрачностью — не кадр съёмки")

// ShotSig — Sig для отбора похожих кадров (078 §5): у картинки с прозрачностью (её миниатюра —
// PNG) подписи нет — это оформление, а не кадр съёмки, и в серию оно не входит.
func (c *Cache) ShotSig(p Photo) (imgsig.Sig, error) {
	s, err := c.Sig(p)
	if err != nil {
		return s, err
	}
	f, err := c.File(p, false) // миниатюра уже есть: только поиск файла
	if err != nil {
		return imgsig.Sig{}, err
	}
	if isPNGPath(f) {
		return imgsig.Sig{}, errAlphaSig
	}
	return s, nil
}

// readSig читает файл подписи; чужой размер, версия или мусор — «нет подписи».
func readSig(path string) (imgsig.Sig, bool) {
	st, err := os.Stat(path)
	if err != nil || st.Size() != imgsig.FileSize {
		return imgsig.Sig{}, false // размер сверяем до чтения: подмена большим файлом не ест память
	}
	b, err := os.ReadFile(path) //nolint:gosec // путь собран из ключа кэша
	if err != nil {
		return imgsig.Sig{}, false
	}
	return imgsig.Parse(b)
}

// decodeThumb читает миниатюру кэша (JPEG, а у картинок с прозрачностью — PNG).
func decodeThumb(path string) (image.Image, error) {
	f, err := os.Open(path) //nolint:gosec // файл кэша, путь из File
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return png.Decode(f)
	}
	return jpeg.Decode(f)
}
