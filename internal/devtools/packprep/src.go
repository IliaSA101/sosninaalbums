package packprep

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/sheetcut"
	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
)

// srcFile — байты исходника рецепта и его расширение.
type srcFile struct {
	Data []byte
	Ext  string
}

// readSrc читает исходник rel строго внутри каталога исходников: «..», абсолютный путь и выход по
// ссылке — отказ (svgcut.ResolveSrc); обычный файл; размер — до чтения (sheetcut.MaxInputBytes).
// Корень -src CLI уже прогнал через paths.CheckLocalPath (devtools internal не импортирует).
func readSrc(root, rel string) (*srcFile, error) {
	full, err := svgcut.ResolveSrc(root, rel)
	if err != nil {
		return nil, errors.New(strings.TrimPrefix(err.Error(), "svgcut: "))
	}
	f, err := os.Open(full) //nolint:gosec // путь прошёл ResolveSrc: внутри -src, без выхода по ссылке
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%q не обычный файл", rel)
	}
	if st.Size() > sheetcut.MaxInputBytes {
		return nil, fmt.Errorf("%q: %d МБ больше предела %d МБ", rel, st.Size()>>20, sheetcut.MaxInputBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(f, sheetcut.MaxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > sheetcut.MaxInputBytes {
		return nil, fmt.Errorf("%q больше предела %d МБ", rel, sheetcut.MaxInputBytes>>20)
	}
	return &srcFile{Data: data, Ext: strings.ToLower(path.Ext(rel))}, nil
}

// decode — картинка исходника: размер и пиксели проверяются по заголовку до декодирования.
func (s *srcFile) decode() (image.Image, error) {
	return sheetcut.DecodeSheet(bytes.NewReader(s.Data), int64(len(s.Data)))
}
