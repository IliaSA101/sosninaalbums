package printshop

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ourRe — имена файлов экспорта программы с префиксом типографии (NN.jpg, NN-1.jpg, 00.jpg;
// 1-NN.jpg у Фабрики); свои ли — по метке.
func ourRe(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `\d{2,}(?:-[12])?\.jpg$`)
}

// BookDir — папка книги name в выбранной папке parent (047 §4.3, решение 56); логика общая со
// сбором в папку (069) — paths.BookDir. Создаёт вызывающий os.Mkdir после всех вопросов.
func BookDir(parent, name string) (dir string, exists bool, err error) {
	return paths.BookDir(parent, name, "книги")
}

// Extra — посторонние файлы папки книги dir: обычные файлы не из written (регистр не важен).
// Не считаются служебные (desktop.ini, Thumbs.db, скрытые с «.») и свои файлы прежней выгрузки
// (метка программы и префикс prefix типографии: лишние удалит экспорт; 01.jpg обычного экспорта
// в папке Фабрики — посторонний). Остаток записи (05.jpg.<число>.tmp) — посторонний:
// параллельной записи нет (флаг exporting, один экземпляр), а загрузчик его возьмёт. first —
// первый по имени. Нечитаемая папка — 0: запись сама скажет об ошибке.
func Extra(dir string, written []string, prefix string) (n int, first string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	re := ourRe(prefix)
	for _, e := range ents {
		name := e.Name()
		if !e.Type().IsRegular() || service(name) || in(written, name) {
			continue
		}
		if re.MatchString(name) && export.IsOurs(filepath.Join(dir, name)) {
			continue
		}
		if n++; n == 1 {
			first = name
		}
	}
	return n, first
}

// OldCover — свой файл обложки name (00.jpg, 1-00.jpg) прежней выгрузки в папке книги dir (метка
// программы): выгрузка без обложки удаляет его после записи (stale), иначе загрузчик возьмёт
// старую обложку. Чужой — посторонний (Extra), его не трогаем.
func OldCover(dir, name string) []string {
	path := filepath.Join(dir, name)
	if st, err := os.Lstat(path); err == nil && st.Mode().IsRegular() && export.IsOurs(path) {
		return []string{name}
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
