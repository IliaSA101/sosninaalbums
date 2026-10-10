package library

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// BackdropFile — проверки своего файла (design-backdrop §7): локальный путь без «..»,
// JPEG/PNG по расширению и заголовку, обычный файл (ссылка — только с локальной целью),
// размер не больше imgfile.MaxPixels. ID — IDFor пути; заполнены ID, Path, W, H, Size.
func BackdropFile(path string) (Photo, error) {
	var zero Photo
	if err := paths.CheckLocalPath(path); err != nil {
		return zero, err
	}
	if slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..") {
		return zero, fmt.Errorf("недопустимый путь к файлу")
	}
	path = filepath.Clean(path)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
	default:
		return zero, fmt.Errorf("для фона подходят только JPEG и PNG")
	}
	st, err := os.Lstat(path)
	if err != nil {
		return zero, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		// Ссылку читаем по её цели: она тоже должна быть локальной.
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return zero, err
		}
		if err := paths.CheckLocalPath(target); err != nil {
			return zero, fmt.Errorf("ссылка ведёт за пределы локального диска")
		}
		path = target
		if st, err = os.Stat(path); err != nil {
			return zero, err
		}
	}
	if !st.Mode().IsRegular() {
		return zero, fmt.Errorf("это не файл с фото")
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
