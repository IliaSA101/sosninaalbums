package mcpbridge

import (
	"errors"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// Чистая часть запуска (без exec): окружение ребёнка и признаки песочницы. Сам запуск — launch_*.go.

// errBreakaway — запуск с отделением от задания процессов клиента отклонён ОС; без отделения
// программа погибла бы вместе с клиентом, поэтому это отказ, а не повтор (design §5).
var errBreakaway = errors.New("breakaway denied")

// Белый список окружения ребёнка (аудит В6): окружение клиента не должно открыть окно отладчику
// (WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port, WEBKIT_INSPECTOR_*) или подгрузить
// чужую библиотеку (LD_*, DYLD_*). Остальное — только по списку.
var (
	envUnix = []string{"PATH", "HOME", "USER", "LOGNAME", "LANG", "DISPLAY", "WAYLAND_DISPLAY",
		"XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"}
	envWindows = []string{"PATH", "SystemRoot", "windir", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
		"TEMP", "TMP", "ProgramData"}
)

// childEnv — окружение ребёнка из environ (формат os.Environ). Результат непустой срез:
// nil означал бы «унаследовать всё». Windows сравнивает имена без учёта регистра.
func childEnv(environ []string, goos string) []string {
	out := []string{}
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || name == "" || !envAllowed(name, goos) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// envAllowed — имя из белого списка: точное, а для Unix ещё LC_* и XDG_*_HOME; SA_STAND — только
// у стенда QA.
func envAllowed(name, goos string) bool {
	// Стенд QA (dev/qa): ребёнок без SA_STAND открыл бы настоящие настройки и замок вместо стенда.
	// В выпуске StandOn всегда false — переменная дальше не идёт.
	if osx.StandOn() && (name == osx.StandEnv || goos == "windows" && strings.EqualFold(name, osx.StandEnv)) {
		return true
	}
	if goos == "windows" {
		for _, n := range envWindows {
			if strings.EqualFold(n, name) {
				return true
			}
		}
		return false
	}
	for _, n := range envUnix {
		if n == name {
			return true
		}
	}
	if rest, ok := strings.CutPrefix(name, "LC_"); ok {
		return rest != ""
	}
	if mid, ok := strings.CutPrefix(name, "XDG_"); ok {
		mid, ok = strings.CutSuffix(mid, "_HOME")
		return ok && mid != ""
	}
	return false
}

// envGet — значение переменной из среза os.Environ ("" — нет). Имя Unix — с учётом регистра.
func envGet(environ []string, name string) string {
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v
		}
	}
	return ""
}

// sandboxed — Linux: агент в Flatpak или Snap. Домашняя папка и сокеты там внутри контейнера,
// программа создала бы config.json и черновик не там, где их ждёт настоящая.
func (d *startDeps) sandboxed() bool {
	if d.goos != "linux" {
		return false
	}
	env := d.environ()
	return envGet(env, "FLATPAK_ID") != "" || envGet(env, "SNAP") != "" || d.exists("/.flatpak-info")
}

// noDisplay — Linux: ни DISPLAY, ни WAYLAND_DISPLAY; окно не откроется.
func (d *startDeps) noDisplay() bool {
	env := d.environ()
	return envGet(env, "DISPLAY") == "" && envGet(env, "WAYLAND_DISPLAY") == ""
}

// appBundle — Mac: путь пакета .app, если exe лежит в <имя>.app/Contents/MacOS/; иначе "".
func appBundle(exe string) string {
	const marker = ".app/Contents/MacOS/"
	i := strings.LastIndex(exe, marker)
	if i <= 0 || strings.Contains(exe[i+len(marker):], "/") {
		return ""
	}
	return exe[:i+len(".app")]
}
