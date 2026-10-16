package fsfera

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// ourRe — имена файлов экспорта программы (NN.jpg, NN-1.jpg, 00.jpg); свои ли — по метке.
var ourRe = regexp.MustCompile(`(?i)^\d{2,}(?:-[12])?\.jpg$`)

// BookDir — папка книги name в выбранной папке parent (047 §4.3). Выбрана сама папка книги
// (имя совпало без учёта регистра) — она же, exists (решение 56). Иначе parent/name: нет —
// exists=false (создаёт вызывающий os.Mkdir после всех вопросов), обычная папка — exists.
// Файл, ссылка или junction на её месте — отказ: через ссылку не пишем (Lstat, не Stat).
func BookDir(parent, name string) (dir string, exists bool, err error) {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		return "", false, fmt.Errorf("недопустимое имя папки книги %q", name)
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
	return "", false, fmt.Errorf("на месте папки книги «%s» уже есть файл или ссылка — выберите другую папку", name)
}

// Extra — посторонние файлы папки книги dir: обычные файлы не из written (регистр не важен).
// Не считаются служебные (desktop.ini, Thumbs.db, скрытые с «.») и свои файлы прежней выгрузки
// (метка программы: лишние удалит экспорт). Остаток записи (05.jpg.<число>.tmp) — посторонний:
// параллельной записи нет (флаг exporting, один экземпляр), а загрузчик его возьмёт. first —
// первый по имени. Нечитаемая папка — 0: запись сама скажет об ошибке.
func Extra(dir string, written []string) (n int, first string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	for _, e := range ents {
		name := e.Name()
		if !e.Type().IsRegular() || service(name) || in(written, name) {
			continue
		}
		if ourRe.MatchString(name) && export.IsOurs(filepath.Join(dir, name)) {
			continue
		}
		if n++; n == 1 {
			first = name
		}
	}
	return n, first
}

// OldCover — свой 00.jpg прежней выгрузки в папке книги dir (метка программы): выгрузка без
// обложки удаляет его после записи (stale), иначе загрузчик возьмёт старую обложку. Чужой
// 00.jpg — посторонний (Extra), его не трогаем.
func OldCover(dir string) []string {
	path := filepath.Join(dir, export.CoverName)
	if st, err := os.Lstat(path); err == nil && st.Mode().IsRegular() && export.IsOurs(path) {
		return []string{export.CoverName}
	}
	return nil
}

// service — служебный файл Windows и macOS: его загрузчик не возьмёт.
func service(name string) bool {
	return strings.HasPrefix(name, ".") || strings.EqualFold(name, "desktop.ini") || strings.EqualFold(name, "Thumbs.db")
}

// in — имя есть в списке без учёта регистра (Windows: 01.JPG и 01.jpg — один файл).
func in(list []string, name string) bool {
	for _, w := range list {
		if strings.EqualFold(w, name) {
			return true
		}
	}
	return false
}
