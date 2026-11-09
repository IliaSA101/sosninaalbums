package library

import "io/fs"

type tb interface {
	Helper()
	Cleanup(func())
}

// UseDehydratedForTest подменяет признак «файл в облаке» (шов dehydrated) на время теста: сквозные проверки
// пакетов выше (app) на любой ОС. Не для t.Parallel: шов общий на процесс.
func UseDehydratedForTest(t tb, f func(fs.FileInfo) bool) {
	t.Helper()
	prev := dehydrated
	dehydrated = f
	t.Cleanup(func() { dehydrated = prev })
}
