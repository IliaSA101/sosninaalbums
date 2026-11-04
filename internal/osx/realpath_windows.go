//go:build windows

package osx

import (
	"errors"
	"syscall"
	"unsafe"
)

// finalPathMax — предел длины настоящего пути в UTF-16 единицах (32 767 — предел самой Windows).
const finalPathMax = 1 << 15

var procFinalPath = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// realPath — GetFinalPathNameByHandle по дескриптору папки: ссылки, junction и подставные диски
// раскрыты. BACKUP_SEMANTICS нужен, чтобы открыть каталог; доступ 0 — читать нечего.
func realPath(p string) (string, error) {
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	h, err := syscall.CreateFile(u, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	buf := make([]uint16, 512)
	for {
		// Флаги 0 = FILE_NAME_NORMALIZED | VOLUME_NAME_DOS. Указатель живёт до возврата Call.
		r, _, callErr := procFinalPath.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0) //nolint:gosec // G103: аргументы WinAPI
		n := int(r)
		switch {
		case n == 0:
			return "", callErr
		case n > finalPathMax:
			return "", errors.New("путь длиннее предела Windows")
		case n > len(buf): // не влезло: n — нужный размер с нулём
			buf = make([]uint16, n)
		default:
			return trimFinalPrefix(syscall.UTF16ToString(buf[:n])), nil
		}
	}
}
