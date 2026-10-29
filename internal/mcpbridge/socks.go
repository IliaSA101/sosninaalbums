package mcpbridge

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// errUnsafe — папка или файл сокета не прошли проверку; наружу идёт только textUnsafe.
var errUnsafe = errors.New("unsafe")

// Want — запуск как посредник: ровно `--mcp` первым аргументом. Флаг, а не подкоманда `mcp`:
// файл с таким именем в аргументе открытия альбома не спутается.
func Want(args []string) bool {
	return len(args) > 1 && args[1] == "--mcp"
}

// userHome — подмена в тестах (иначе «пустое окружение» нашло бы настоящий дом).
var userHome = func() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}

// SockPaths — пути сокета, которые пробует посредник (design §4.1 «Где сокет»), только
// абсолютные и без повторов. Окружение посредника бывает беднее окружения программы (TS SDK
// передаёт на Unix лишь HOME, LOGNAME, PATH, SHELL, TERM, USER — XDG_CONFIG_HOME теряется), поэтому:
//  1. osx.MCPSock — как у программы (на Windows — известная папка по токену, не APPDATA);
//  2. Linux и Mac — по домашней папке: $HOME, если пуст — из os/user (на Windows os/user не
//     зовётся: в домене вне сети это запрос к контроллеру).
//
// Пустой список — папки данных нет; пути от текущей папки не бывает.
func SockPaths(dataName string) []string {
	var out []string
	add := func(p string) { out = appendSock(out, p) }
	add(osx.MCPSock(dataName))
	switch runtime.GOOS {
	case "linux":
		if home := homeDir(); home != "" && dataName != "" {
			add(filepath.Join(home, ".config", dataName, "mcp", "sock"))
		}
	case "darwin":
		if home := homeDir(); home != "" && dataName != "" {
			add(filepath.Join(home, "Library", "Application Support", dataName, "mcp", "sock"))
		}
	}
	return out
}

// appendSock добавляет путь в список: только абсолютный, локальный и без повтора. Как у программы
// (agents/channel.go) UNC и `\\?\` не берём: проба по SMB висела бы в главном цикле, а путь из
// APPDATA окружения клиента отправил бы NTLM на чужой хост.
func appendSock(out []string, p string) []string {
	if p == "" || !filepath.IsAbs(p) || paths.CheckLocalPath(p) != nil {
		return out
	}
	for _, q := range out {
		if q == p {
			return out
		}
	}
	return append(out, p)
}

func homeDir() string {
	if h := os.Getenv("HOME"); filepath.IsAbs(h) {
		return h
	}
	if h := userHome(); filepath.IsAbs(h) {
		return h
	}
	return ""
}
