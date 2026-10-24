package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/wailsapp/wails/v2/pkg/options"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Открытие альбома из аргумента запуска и вторым запуском (спека 112, design §4): двойной щелчок
// в проводнике, «Открыть в 0.X» из другой версии, Mac OnFileOpen. Аргументы — чужие данные.

// afterPIDFlag — флаг «Открыть в 0.X»: новая версия ждёт выхода старой (по pid) и открывает файл.
// Контракт между версиями (0.25 запускает 0.26+): менять нельзя, сторож — TestAfterPidContract.
const afterPIDFlag = "--after-pid"

// afterPIDWait — сколько новая версия ждёт выхода старой: замок единственного экземпляра занят
// до её выхода. Вышел предел — идём дальше (файл тогда уйдёт старой, и она снова покажет окно).
const afterPIDWait = 20 * time.Second

// launchArgs разбирает аргументы запуска: `--after-pid N` (и `--after-pid=N`, N > 0) и первый
// аргумент на .salbum (без учёта регистра; относительный — от cwd). Файл вне CheckLocalPath
// отбрасывается (причина — в лог без пути). Прочее (флаги wails dev, мусор) пропускается.
func launchArgs(args []string, cwd string) (file string, afterPID int) {
	return launchArgsLog(args, cwd, log.Printf)
}

// launchArgsLog — launchArgs с приёмником строк лога: до startup log.txt ещё не открыт.
func launchArgsLog(args []string, cwd string, logf func(format string, v ...any)) (file string, afterPID int) {
	var first string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == afterPIDFlag && i+1 < len(args):
			if n, ok := parsePID(args[i+1]); ok {
				afterPID = n
				i++
			}
		case strings.HasPrefix(arg, afterPIDFlag+"="):
			if n, ok := parsePID(arg[len(afterPIDFlag)+1:]); ok {
				afterPID = n
			}
		case first == "" && hasProjectExt(arg):
			first = arg // первый .salbum и есть запрошенный: отказ ниже не ищет следующий
		}
	}
	if first != "" {
		file = salbumArgLog(first, cwd, logf)
	}
	return file, afterPID
}

// parsePID — положительное число; иначе ok=false.
func parsePID(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n > 0
}

// hasProjectExt — имя кончается на .salbum без учёта регистра.
func hasProjectExt(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), projectExt)
}

// salbumArg — путь альбома из аргумента: абсолютный, локальный, .salbum; иначе "". Относительный
// путь — от cwd (так его шлёт замок второго экземпляра), без cwd отказ придёт из CheckLocalPath.
func salbumArg(arg, cwd string) string { return salbumArgLog(arg, cwd, log.Printf) }

func salbumArgLog(arg, cwd string, logf func(format string, v ...any)) string {
	if !filepath.IsAbs(arg) {
		arg = filepath.Join(cwd, arg)
	}
	if err := checkAlbumPath(arg); err != nil {
		logf("аргумент запуска отклонён: %v", paths.NoPath(err))
		return ""
	}
	return filepath.Clean(arg)
}

// checkAlbumPath — граница открытия извне: локальный путь и расширение .salbum.
func checkAlbumPath(path string) error {
	if err := paths.CheckLocalPath(path); err != nil {
		return err
	}
	if !hasProjectExt(path) {
		return fmt.Errorf("это не альбом (%s)", projectExt)
	}
	return nil
}

// initLaunch разбирает os.Args до окна: ждёт выхода старой версии (--after-pid), запоминает файл
// для startup и отдаёт право на передний план (второй процесс от проводника передаёт его первому).
// log.txt ещё не открыт (setupLog — в startup), поэтому строки лога копятся и уходят туда в
// flushLaunchLog: иначе они попали бы в stderr GUI-exe.
func (a *App) initLaunch() {
	cwd, _ := os.Getwd() // ошибка — пусто: относительный путь тогда отклонит CheckLocalPath
	a.initLaunchFrom(os.Args[1:], cwd, osx.WaitExit)
	osx.AllowForeground()
}

// initLaunchFrom — тело initLaunch с подставляемыми аргументами и ожиданием (проверяется в тестах).
func (a *App) initLaunchFrom(args []string, cwd string, wait func(pid int, d time.Duration) bool) {
	var lines []string
	logf := func(format string, v ...any) { lines = append(lines, fmt.Sprintf(format, v...)) }
	file, pid := launchArgsLog(args, cwd, logf)
	if pid > 0 && !wait(pid, afterPIDWait) {
		logf("прежняя версия не закрылась за %v", afterPIDWait)
	}
	a.mu.Lock()
	a.argFile, a.launchLog = file, lines
	a.mu.Unlock()
}

// flushLaunchLog пишет накопленные initLaunch строки в лог (зовётся в startup после setupLog).
func (a *App) flushLaunchLog() {
	a.mu.Lock()
	lines := a.launchLog
	a.launchLog = nil
	a.mu.Unlock()
	for _, l := range lines {
		log.Print(l)
	}
}

// takeArg отдаёт файл запуска startup'у один раз; позже macOpen и второй запуск идут в очередь.
// Файл в папке копий не открывается при старте (loadStartup читает его как последний проект, без
// «восстановления»): он уходит в очередь внешнего открытия — openPath знает openBackup.
func (a *App) takeArg() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	file := a.argFile
	a.argFile, a.argTaken = "", true
	if file != "" && a.backups.Inside(file) {
		a.ext = file
		return ""
	}
	return file
}

// openedArg — альбом из аргумента запуска открылся: путь редактора после loadStartup тот же
// (SamePath — регистр и слэши Windows). Нет аргумента, файл не найден, открылся прежний или
// черновик (путь пуст) — false.
func openedArg(arg, path string) bool {
	return arg != "" && path != "" && paths.SamePath(arg, path)
}

// secondLaunch — второй запуск (замок единственного экземпляра): окно вперёд и, если среди
// аргументов альбом, он встаёт в очередь внешнего открытия. Зовётся из горутины Wails.
func (a *App) secondLaunch(data options.SecondInstanceData) {
	defer guard("secondLaunch")
	if a.ctx != nil {
		wr.WindowUnminimise(a.ctx)
		wr.WindowShow(a.ctx)
	}
	if file, _ := launchArgs(data.Args, data.WorkingDirectory); file != "" {
		a.queueExternal(file)
	}
}

// macOpen — Mac OnFileOpen: файл пришёл либо до startup (тогда он как аргумент запуска), либо потом.
func (a *App) macOpen(path string) {
	defer guard("macOpen")
	file := salbumArg(path, "")
	if file == "" {
		return
	}
	a.mu.Lock()
	if !a.argTaken {
		a.argFile = file
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.queueExternal(file)
}

// queueExternal ставит альбом в очередь внешнего открытия (последний побеждает) и зовёт окно:
// событие `open:external` с именем файла. Открывает его фронт через OpenExternal, когда нет
// окон (тост «откроется, когда закроете окно») — путь от фронта не принимается.
func (a *App) queueExternal(path string) {
	a.mu.Lock()
	a.ext = path
	a.mu.Unlock()
	if ed := a.editor(); ed != nil && paths.SamePath(ed.Path(), path) {
		log.Print(logArgAlready)
	} else {
		log.Print(logArgQueued)
	}
	a.emit("open:external", filepath.Base(path))
}

// Строки log.txt про альбом из командной строки (спека 118, Б1–Б5 и Д6): без имени и пути — в
// журнале, который фотограф шлёт разработчику, названия альбомов не нужны.
const (
	logArgOpened  = "альбом из командной строки: открыт"
	logArgQueued  = "альбом из командной строки: отложен до закрытия окна" // в очереди; откроет окно
	logArgAlready = "альбом из командной строки: уже открыт"
	logArgFailed  = "альбом из командной строки: не открылся"
)

// logLaunchArg — итог открытия альбома из аргумента запуска (зовётся в startup после loadStartup):
// открылся, ушёл в очередь (файл из папки копий открывает OpenExternal) или не открылся.
// Аргумента нет — молчит.
func (a *App) logLaunchArg(arg, path string) {
	switch {
	case arg == "":
	case openedArg(arg, path):
		log.Print(logArgOpened)
	default:
		a.mu.Lock()
		queued := a.ext != "" && paths.SamePath(a.ext, arg)
		a.mu.Unlock()
		if queued {
			log.Print(logArgQueued)
		} else {
			log.Print(logArgFailed)
		}
	}
}

// pendingName — имя файла в очереди внешнего открытия для State.Pending; под a.mu.
func (a *App) pendingName() string {
	if a.ext == "" {
		return ""
	}
	return filepath.Base(a.ext)
}

// OpenExternal открывает альбом из очереди внешнего открытия (двойной щелчок, второй запуск).
// Пусто или этот альбом уже открыт — Cancelled; не .salbum или не локальный путь — ошибка.
// Дальше как «Открыть» и «Последние»: openPath, прежний альбом пишется сам (0.25-q112-2);
// альбом новее программы приходит в ответе как у них же (State.Newer, спека 112 §5).
func (a *App) OpenExternal() (State, error) {
	a.mu.Lock()
	path := a.ext
	a.ext = ""
	a.mu.Unlock()
	if path == "" || paths.SamePath(a.editor().Path(), path) {
		st := a.GetState()
		st.Cancelled = true
		return st, nil
	}
	if err := checkAlbumPath(path); err != nil {
		log.Print(logArgFailed)
		return a.GetState(), err
	}
	st, err := a.openPath(path)
	if err != nil || st.Cancelled {
		log.Print(logArgFailed) // и альбом новее программы: окно «из новой версии», не открытие
	} else {
		log.Print(logArgOpened)
	}
	return st, err
}
