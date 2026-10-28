package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// sockPath — путь сокета: --sock, SPIKE_SOCK или <AppData>\SosninaAlbums-stand-t0\mcp\sock (design §4.3 С2).
func sockPath(o opts) string {
	if p := o.str("sock", os.Getenv("SPIKE_SOCK")); p != "" {
		return p
	}
	return filepath.Join(appDataDir(), "SosninaAlbums-stand-t0", "mcp", "sock")
}

// listenSock занимает путь: папка 0700; файл от упавшего запуска (никто не отвечает) заменяется,
// живой сервер — ошибка. clean=false — не трогать чужой файл (проба «без замены»).
func listenSock(path string, clean bool) (ln net.Listener, replaced bool, err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("папка сокета: %w", err)
	}
	if _, serr := os.Lstat(path); serr == nil && clean {
		c, derr := net.DialTimeout("unix", path, 700*time.Millisecond)
		if derr == nil {
			_ = c.Close()
			return nil, false, errors.New("сокет занят живой программой")
		}
		if err = os.Remove(path); err != nil {
			return nil, false, fmt.Errorf("удаление старого сокета: %w", err)
		}
		replaced = true
	}
	ln, err = net.Listen("unix", path)
	return ln, replaced, err
}
