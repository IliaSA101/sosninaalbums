// Package osx — вызовы ОС: файловый менеджер, папка данных WebView2, доступность тома и
// свободное место на нём (FreeBytes, free_*.go).
package osx

import (
	"log"
	"os"
	"os/exec"
	goruntime "runtime"
)

// startCmd — запуск внешнего процесса; подменяется в тестах (проверка «не запущен»).
var startCmd = func(cmd *exec.Cmd) error { return cmd.Start() }

// quiet — стенд QA не открывает окна ОС на рабочем столе владельца (случай 20.10: QA из
// браузера вызвал ShowDataFolder и вылезло «Расположение недоступно»). Пишет в журнал
// действие без путей и сообщает, что запуск нужно пропустить. Только в сборке dev (standEnabled).
func quiet(action string) bool {
	if !standOn() {
		return false
	}
	log.Printf("стенд: окно не открыто (%s)", action)
	return true
}

// standOn — стенд QA включён; подменяется в тестах (тег dev в обычном прогоне не задан).
var standOn = func() bool { return standActive(standEnabled, os.Getenv) }

// standActive — задан ли стенд QA; имя не разбираем: неверное тоже значит «не открывать».
func standActive(enabled bool, getenv func(string) string) bool {
	return enabled && getenv(StandEnv) != ""
}

// OpenURL — открыть адрес в браузере через open; у стенда QA окно не открывается.
func OpenURL(url string, open func(url string)) {
	if quiet("браузер") {
		return
	}
	open(url)
}

// Reveal — запуск файлового менеджера; file — выделить файл в его папке, иначе открыть
// папку. Ждать завершения не нужно. У стенда QA ничего не запускается (quiet).
func Reveal(path string, file bool) error {
	if quiet("проводник") {
		return nil
	}
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
	return startCmd(cmd)
}

// OpenDir открывает папку в файловом менеджере; существование dir вызывающий уже проверил.
func OpenDir(dir string) error { return Reveal(dir, false) }
