package osx

import (
	"syscall"
	"unsafe"
)

var procFreeSpace = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// FreeBytes — сколько байт на томе папки dir доступно для записи этому пользователю (с учётом
// квот): сбор проекта в папку (069) проверяет место до копирования. Папки нет — ошибка.
func FreeBytes(dir string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var avail uint64
	// Указатели живут до возврата Call: так WinAPI зовут через syscall (x/sys в osx запрещён).
	r, _, err := procFreeSpace.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&avail)), 0, 0) //nolint:gosec // G103: аргументы WinAPI
	if r == 0 {
		return 0, err
	}
	return avail, nil
}
