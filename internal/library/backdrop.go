package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ErrExt — расширение файла не из разрешённых (PlainFile).
var ErrExt = errors.New("недопустимое расширение файла")

// PlainFile — общая проверка чужого файла: локальный путь без «..», расширение из exts
// (строчные, с точкой), обычный файл; ссылка — только с локальной целью и читается по ней.
// Возвращает путь, по которому файл читать, и размер.
func PlainFile(path string, exts ...string) (string, int64, error) {
	if err := paths.CheckLocalPath(path); err != nil {
		return "", 0, err
	}
	if slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..") {
		return "", 0, fmt.Errorf("недопустимый путь к файлу")
	}
	path = filepath.Clean(path)
	if !slices.Contains(exts, strings.ToLower(filepath.Ext(path))) {
		return "", 0, ErrExt
	}
	st, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		// Ссылку читаем по её цели: каждое звено цепочки должно быть локальным (LinkTarget).
		if path, st, err = LinkTarget(path); err != nil {
			return "", 0, err
		}
	}
	if !st.Mode().IsRegular() {
		return "", 0, fmt.Errorf("это не файл с фото")
	}
	return path, st.Size(), nil
}

// BackdropFile — проверки своего файла (design-backdrop §7): локальный путь без «..»,
// JPEG/PNG по расширению и заголовку, обычный файл (ссылка — только с локальной целью),
// размер не больше imgfile.MaxPixels. ID — IDFor пути; заполнены ID, Path, W, H, Size.
func BackdropFile(path string) (Photo, error) {
	var zero Photo
	path, _, err := PlainFile(path, ".jpg", ".jpeg", ".png")
	if errors.Is(err, ErrExt) {
		return zero, fmt.Errorf("для фона подходят только JPEG и PNG")
	}
	if err != nil {
		return zero, err
	}
	if err := imgfile.CheckSize(path); err != nil {
		return zero, err
	}
	info, err := ReadInfo(path)
	if err != nil {
		return zero, err
	}
	return Photo{ID: IDFor(path), Path: path, W: info.W, H: info.H, Size: info.Size}, nil
}
