package peercred

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sioAfUnixGetPeerPID = _WSAIOR(IOC_VENDOR, 256) из afunix.h: в x/sys/windows константы нет.
const sioAfUnixGetPeerPID = 0x58000100

// ioctlPeerPID — запрос PID собеседника; тест подменяет его (ответ «не поддерживается»).
var ioctlPeerPID = func(fd uintptr) (uint32, error) {
	var pid, n uint32
	err := windows.WSAIoctl(windows.Handle(fd), sioAfUnixGetPeerPID, nil, 0,
		(*byte)(unsafe.Pointer(&pid)), uint32(unsafe.Sizeof(pid)), &n, nil, 0) //nolint:gosec // G103: WSAIoctl пишет ULONG в буфер размером с него
	return pid, err
}

// sameFD: PID собеседника → токен процесса → SID пользователя. start — начало проверки:
// процесс, созданный позже, — повторный PID после выхода настоящего собеседника (148-7).
// Время создания и start — часы с тактом до 15,6 мс: окно подмены не «микросекунды», а до такта;
// ложный отказ при переводе часов назад принят.
func sameFD(fd uintptr, start time.Time) (bool, error) {
	pid, err := ioctlPeerPID(fd)
	if errors.Is(err, windows.WSAEOPNOTSUPP) || errors.Is(err, windows.WSAEINVAL) {
		return false, ErrUnsupported
	}
	if err != nil {
		return false, err
	}
	if pid == 0 || pid == 4 { // Idle и System — не пользователь
		return false, nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil // чужая учётная запись или процесса уже нет
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var created, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exit, &kernel, &user); err != nil {
		return false, err
	}
	if time.Unix(0, created.Nanoseconds()).After(start) {
		return false, nil
	}
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return false, ErrTokenDenied
		}
		return false, err
	}
	defer func() { _ = tok.Close() }()
	peer, err := tok.GetTokenUser()
	if err != nil {
		return false, err
	}
	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false, err
	}
	return peer.User.Sid.Equals(me.User.Sid), nil
}
