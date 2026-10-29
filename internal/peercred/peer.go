// Package peercred — проверка, что собеседник соединения AF_UNIX работает под той же учётной
// записью ОС, что и процесс. Граница безопасности канала агентов (148): только stdlib и x/sys.
package peercred

import (
	"errors"
	"net"
	"time"
)

// ErrUnsupported — ОС не умеет назвать собеседника (только Windows без запроса PID сокета).
var ErrUnsupported = errors.New("проверка собеседника недоступна")

// ErrTokenDenied — процесс собеседника открылся, а токен нет: так выглядит процесс «от
// администратора» для обычного. Возвращается только на Windows; посредник сверяет её через
// errors.Is без функций по ОС и отвечает отдельным текстом.
var ErrTokenDenied = errors.New("токен собеседника недоступен")

// errNotUnix — соединение не AF_UNIX: net.Pipe и TCP в рабочем коде не должны попасть сюда.
var errNotUnix = errors.New("соединение не AF_UNIX: учётную запись собеседника не определить")

// Same — собеседник соединения AF_UNIX той же учётной записи ОС, что и процесс.
// false, nil — чужой (в том числе root/SYSTEM и процесс, к которому нет доступа);
// ошибка — ОС не ответила; ErrUnsupported — только Windows без запроса PID.
func Same(c net.Conn) (bool, error) {
	start := time.Now()
	uc, ok := c.(*net.UnixConn)
	if !ok || uc == nil { // типизированный nil тоже не соединение
		return false, errNotUnix
	}
	rc, err := uc.SyscallConn()
	if err != nil {
		return false, err
	}
	var same bool
	var ierr error
	if err := rc.Control(func(fd uintptr) { same, ierr = sameFD(fd, start) }); err != nil {
		return false, err
	}
	return same && ierr == nil, ierr
}
