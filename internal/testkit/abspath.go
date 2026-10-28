package testkit

import (
	"runtime"
	"strings"
)

// AbsPath возвращает абсолютный путь для данных-фикстур проекта: на Windows — `C:\` + имя с обратными
// косыми (как раньше писали в тестах), на других ОС — `/` + имя с прямыми. Разделители в name
// можно писать любые: приводятся к принятым на текущей ОС. Нужен, потому что CheckLocalPath на Linux
// и macOS не считает `C:\a.jpg` абсолютным путём.
func AbsPath(name string) string {
	if runtime.GOOS == "windows" {
		return `C:\` + strings.ReplaceAll(name, "/", `\`)
	}
	return "/" + strings.ReplaceAll(name, `\`, "/")
}
