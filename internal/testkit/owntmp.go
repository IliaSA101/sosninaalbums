package testkit

import (
	"regexp"
	"strings"
)

// randTmpRe — остаток library.WriteFileAtomic*: os.CreateTemp(<имя>.*.tmp) даёт десятичное число.
var randTmpRe = regexp.MustCompile(`^.+\.\d+\.tmp$`)

// OwnTmp — имя файла (без пути) похоже на временный файл нашего кода. Шаблоны — точный список:
// import-*.tmp (импорт decor/fontlib), .dl-<id>.tmp (скачивание набора), *.sacopy.tmp (сбор
// альбома), <имя>.<число>.tmp (WriteFileAtomic, CreateTemp). Прочие *.tmp — чужие: на Windows
// антивирус переименовывает удаляемый файл в 8.3-имя + .tmp (ABC.PNG.tmp) и держит его открытым;
// такой «призрак» не должен валить тест на остатки.
func OwnTmp(name string) bool {
	n := strings.ToLower(name)
	switch {
	case !strings.HasSuffix(n, ".tmp"):
		return false
	case strings.HasPrefix(n, "import-"), strings.HasPrefix(n, ".dl-"):
		return true
	case strings.HasSuffix(n, ".sacopy.tmp"):
		return len(n) > len(".sacopy.tmp")
	}
	return randTmpRe.MatchString(n)
}
