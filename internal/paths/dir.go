package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BookDir — папка name в выбранной пользователем папке parent: книга Фотосферы (047 §4.3),
// собранный альбом (069). Выбрана сама эта папка (имя совпало без учёта регистра) — она же,
// exists (решение 56). Иначе parent/name: нет — exists=false (создаёт вызывающий после всех
// вопросов), обычная папка — exists. Файл, ссылка или junction на её месте — отказ: через
// ссылку не пишем (Lstat, не Stat). what — чья папка в тексте ошибки («книги», «альбома»).
func BookDir(parent, name, what string) (dir string, exists bool, err error) {
	// Base — как раньше (на Windows отсекает и том «C:x»); плюс разделитель любой ОС (IsSep), а
	// не только своей: на Linux «..\x» — одно имя, но папку выгрузки откроют на Windows
	// (типография, сам фотограф), и там это выход из parent.
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsFunc(name, IsSepRune) {
		return "", false, fmt.Errorf("недопустимое имя папки %s %q", what, name)
	}
	if strings.EqualFold(filepath.Base(parent), name) {
		return parent, true, nil
	}
	dir = filepath.Join(parent, name)
	st, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, false, nil
	case err != nil:
		return "", false, err
	case st.Mode().Type() == fs.ModeDir:
		return dir, true, nil
	}
	return "", false, fmt.Errorf("на месте папки %s «%s» уже есть файл или ссылка — выберите другую папку", what, name)
}
