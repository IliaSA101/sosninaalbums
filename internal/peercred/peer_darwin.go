package peercred

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// xucredVersion — XUCRED_VERSION из sys/ucred.h (в x/sys константы нет).
const xucredVersion = 0

// sameFD: uid собеседника из LOCAL_PEERCRED; XNU копирует учётную запись слушателя клиенту
// при connect, серверу отдаёт учётную запись подключившегося.
func sameFD(fd uintptr, _ time.Time) (bool, error) {
	x, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return false, err
	}
	if x.Version != xucredVersion {
		return false, errors.New("неизвестная версия xucred")
	}
	return int(x.Uid) == os.Getuid(), nil
}
