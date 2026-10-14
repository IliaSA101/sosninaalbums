//go:build windows

package osx

import "syscall"

// longPath — длинное имя вместо 8.3 (%TEMP% бывает «C:\Users\ABCDEF~1\…»); при любой ошибке
// (пути нет, не хватило буфера) возвращает p как есть.
func longPath(p string) string {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	size := uint32(syscall.MAX_PATH)
	for range 2 { // второй проход — если буфер был мал: тогда n — нужный размер
		buf := make([]uint16, size)
		n, err := syscall.GetLongPathName(in, &buf[0], size)
		if err != nil || n == 0 {
			return p
		}
		if n <= size {
			return syscall.UTF16ToString(buf[:n])
		}
		size = n
	}
	return p
}
