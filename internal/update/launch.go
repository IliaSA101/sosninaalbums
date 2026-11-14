package update

import (
	"errors"
	"strings"
)

// ErrPath — путь установщика или папки не годится для командной строки.
var ErrPath = errors.New("путь к установщику содержит недопустимые знаки")

// CmdLine — командная строка установщика (design §4). install: «"<setup>" /S /relaunch
// /D=<папка запущенной копии>» — тихо, на месте и с повторным запуском. /D= идёт последним и без
// кавычек (требование NSIS), поэтому строка собирается сырой, а не через аргументы Go (он
// кавычит всё с пробелом). setup (переносная копия) и page: только «"<setup>"» — обычный мастер.
// Кавычка или управляющий знак в пути — ErrPath: Windows их в путях не допускает, а в строке
// они открыли бы чужие аргументы.
func CmdLine(setup string, mode Mode, dir string) (string, error) {
	if !plainPath(setup) || mode == ModeInstall && !plainPath(dir) {
		return "", ErrPath
	}
	if mode != ModeInstall {
		return `"` + setup + `"`, nil
	}
	return `"` + setup + `" /S /relaunch /D=` + dir, nil
}

// plainPath — непустой путь без кавычек, управляющих знаков и перевода строки.
func plainPath(p string) bool {
	return p != "" && !strings.ContainsFunc(p, func(r rune) bool { return r == '"' || r < 0x20 || r == 0x7f })
}
