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
		// Ссылку читаем по её цели: она тоже должна быть локальной. Сырые цели — до EvalSymlinks:
		// он сам идёт по цели, и ссылка на UNC дала бы запрос SMB до проверки.
		if !linkLocal(path) {
			return "", 0, fmt.Errorf("ссылка ведёт за пределы локального диска")
		}
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", 0, err
		}
		if err := paths.CheckLocalPath(target); err != nil {
			return "", 0, fmt.Errorf("ссылка ведёт за пределы локального диска")
		}
		path = target
		if st, err = os.Stat(path); err != nil {
			return "", 0, err
		}
	}
	if !st.Mode().IsRegular() {
		return "", 0, fmt.Errorf("это не файл с фото")
	}
	return path, st.Size(), nil
}

// readLink — чтение сырой цели ссылки; подменяется в тестах (на Windows без прав ссылку не создать).
var readLink = os.Readlink

// maxLinkHops — длиннее цепочка ссылок не бывает у своих файлов; дальше — отказ.
const maxLinkHops = 8

// linkLocal — у ссылки p каждая цель цепочки локальна (localTarget, как при обходе папок).
// Цель проверяется по записи в ссылке, на диск по ней не ходим, пока она не проверена.
func linkLocal(p string) bool {
	for range maxLinkHops {
		raw, err := readLink(p)
		if err != nil {
			return true // не ссылка: конец цепочки, дальше решают EvalSymlinks и Stat
		}
		if !localTarget(filepath.Dir(p), raw) {
			return false
		}
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(filepath.Dir(p), raw)
		}
		p = raw
	}
	return false
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
