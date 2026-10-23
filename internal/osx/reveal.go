// Package osx — вызовы ОС: файловый менеджер, папка данных WebView2, доступность тома и
// свободное место на нём (FreeBytes, free_*.go).
package osx

import (
	"log"
	"os"
	"os/exec"
	"path"
	goruntime "runtime"
	"strings"
)

// startCmd — запуск внешнего процесса; подменяется в тестах (проверка «не запущен»).
// После удачного запуска процесс подбирается в фоне: без Wait завершённый остаётся зомби.
var startCmd = func(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

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
// папку. Ждать завершения не нужно (кроме Linux: ответ dbus нужен для запасного пути).
// У стенда QA ничего не запускается (quiet).
func Reveal(path string, file bool) error {
	if quiet("проводник") {
		return nil
	}
	switch goruntime.GOOS {
	case "linux":
		return revealLinux(path, file)
	case "darwin":
		return startCmd(argvCmd(revealArgv("darwin", path, file)))
	}
	var cmd *exec.Cmd
	if file {
		cmd = exec.Command("explorer") //nolint:gosec // path — из проекта, проверен paths.CheckLocalPath и Stat
		cmd.SysProcAttr = explorerSelectAttr(path)
	} else {
		cmd = exec.Command("explorer", path) //nolint:gosec // см. выше
		cmd.SysProcAttr = explorerOpenAttr(path)
	}
	return startCmd(cmd)
}

// OpenDir открывает папку в файловом менеджере; существование dir вызывающий уже проверил.
func OpenDir(dir string) error { return Reveal(dir, false) }

// runCmd — запуск с ожиданием; подменяется в тестах.
var runCmd = func(cmd *exec.Cmd) error { return cmd.Run() }

// argvCmd — процесс из готового argv: без оболочки, путь — отдельным аргументом.
func argvCmd(argv []string) *exec.Cmd {
	return exec.Command(argv[0], argv[1:]...) //nolint:gosec // путь — из проекта, проверен paths.CheckLocalPath и Stat; оболочки нет
}

// revealLinux: файл — dbus-send ShowItems (выделяет файл в Nautilus, Dolphin, Nemo, Caja);
// нет службы или ошибка — xdg-open папки файла. Папка — сразу xdg-open.
func revealLinux(target string, file bool) error {
	if file {
		if err := runCmd(argvCmd(revealArgv("linux", target, true))); err == nil {
			return nil
		}
		// path, а не filepath: путь Linux с «/» и на Windows (тест запасного пути идёт на всех ОС).
		target = path.Dir(target)
	}
	return startCmd(argvCmd(revealArgv("linux", target, false)))
}

// revealArgv — команда по ОС (чистая функция, для Linux и Mac). Windows — nil: проводнику
// командную строку собирает Reveal через SysProcAttr.
func revealArgv(goos, path string, file bool) []string {
	switch goos {
	case "darwin":
		if file {
			return []string{"open", "-R", path}
		}
		return []string{"open", path}
	case "linux":
		if file {
			return []string{
				"dbus-send", "--session", "--print-reply", "--reply-timeout=5000",
				"--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
				"org.freedesktop.FileManager1.ShowItems", "array:string:" + fileURI(path), "string:",
			}
		}
		return []string{"xdg-open", dashSafe(path)}
	}
	return nil
}

// dashSafe — относительный путь с «-» впереди приняли бы за флаг; абсолютные путь не задевает.
func dashSafe(path string) string {
	if strings.HasPrefix(path, "-") {
		return "./" + path
	}
	return path
}

// fileURI — file:// с процент-кодированием всего, кроме безопасных знаков и «/»: запятая тоже,
// dbus-send делит «array:string:» по запятым.
func fileURI(path string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.WriteString("file://")
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}
