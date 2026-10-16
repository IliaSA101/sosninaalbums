// Package osx — вызовы ОС: файловый менеджер и папка данных WebView2.
package osx

import (
	"os/exec"
	goruntime "runtime"
)

// Reveal — запуск файлового менеджера; file — выделить файл в его папке, иначе открыть
// папку. Ждать завершения не нужно.
func Reveal(path string, file bool) error {
	var cmd *exec.Cmd
	switch {
	case goruntime.GOOS == "darwin" && file:
		cmd = exec.Command("open", "-R", path) //nolint:gosec // path — из проекта, проверен paths.CheckLocalPath и Stat
	case goruntime.GOOS == "darwin":
		cmd = exec.Command("open", path) //nolint:gosec // см. выше
	case file:
		cmd = exec.Command("explorer") //nolint:gosec // см. выше
		cmd.SysProcAttr = explorerSelectAttr(path)
	default:
		cmd = exec.Command("explorer", path) //nolint:gosec // см. выше
		cmd.SysProcAttr = explorerOpenAttr(path)
	}
	return cmd.Start()
}

// OpenDir открывает папку в файловом менеджере; существование dir вызывающий уже проверил.
func OpenDir(dir string) error { return Reveal(dir, false) }
