//go:build !windows

package osx

import "syscall"

// FreeBytes — сколько байт на томе папки dir доступно для записи без прав root (Bavail):
// сбор проекта в папку (069) проверяет место до копирования. Папки нет — ошибка.
func FreeBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:gosec,unconvert // типы полей Statfs разные на Linux и macOS
}
