package osx

import (
	"syscall"
	"unsafe"
)

// tokenElevation — TokenElevation (класс сведений токена): структура из одного uint32.
const tokenElevation = 20

// Elevated — процесс запущен от администратора (повышенный токен): правки агентов такой процесс не
// принимает (149, 0.33-149-12). Токен не открылся или не ответил — false: сбой запроса не должен
// закрывать правки всем, а свой токен процесс открывает всегда.
func Elevated() bool {
	var tok syscall.Token
	h, err := syscall.GetCurrentProcess()
	if err != nil || syscall.OpenProcessToken(h, syscall.TOKEN_QUERY, &tok) != nil {
		return false
	}
	defer tok.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	var v, n uint32
	if syscall.GetTokenInformation(tok, tokenElevation, (*byte)(unsafe.Pointer(&v)), uint32(unsafe.Sizeof(v)), &n) != nil { //nolint:gosec // G103: аргумент WinAPI
		return false
	}
	return v != 0
}
