package update

import (
	"os"
	"runtime"
	"strings"
)

// System — система, на которой идёт проверка: GOOS (после подмены стенда) и версия; "" — версия
// не определилась, правило minOS не применяется (171-3).
type System struct {
	GOOS, Version string
	// StandBad — SA_UPDATE_OS задана, но не разобрана (подмены нет): служба пишет строку в log.txt.
	StandBad bool
}

// SystemOf — система этой машины: osver — сырая строка ОС (osx.OSVersion; цикл импорта не даёт
// звать ParseOS из osx, поэтому проверка здесь). В сборке dev/qa SA_UPDATE_OS=darwin:12.6
// подменяет GOOS и версию; в выпуске переменная не читается.
func SystemOf(osver func() (string, bool)) System {
	return systemFrom(standEnabled, runtime.GOOS, osver, os.Getenv)
}

// systemFrom — тело SystemOf с подставляемыми enabled, goos и getenv (обе ветки — без тега dev).
func systemFrom(enabled bool, goos string, osver func() (string, bool), getenv func(string) string) System {
	sys := System{GOOS: goos}
	if enabled {
		if v := getenv("SA_UPDATE_OS"); v != "" {
			g, ver, ok := strings.Cut(v, ":")
			if ok && knownGOOS(g) && validOS(ver) {
				return System{GOOS: g, Version: cleanOS(g, ver)}
			}
			sys.StandBad = true
		}
	}
	if osver != nil {
		if raw, ok := osver(); ok {
			sys.Version = cleanOS(goos, raw)
		}
	}
	return sys
}

func knownGOOS(g string) bool {
	for _, k := range knownOS {
		if k == g {
			return true
		}
	}
	return false
}

// cleanOS — строка ОС годна, иначе "". macOS 11+ в режиме совместимости отдаёт «10.16»: это не
// настоящая версия, правило не применяется.
func cleanOS(goos, raw string) string {
	v, ok := ParseOS(raw)
	if !ok || goos == "darwin" && v[0] == 10 && v[1] == 16 {
		return ""
	}
	return raw
}
