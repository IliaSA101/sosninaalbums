package peercred

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// sameFD: uid собеседника из SO_PEERCRED. Ядро помнит учётную запись подключившегося (на
// стороне сервера) и слушателя на момент listen (на стороне клиента) — гонки с PID нет.
func sameFD(fd uintptr, _ time.Time) (bool, error) {
	u, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return false, err
	}
	return int(u.Uid) == os.Getuid(), nil
}
