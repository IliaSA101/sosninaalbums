//go:build !windows

package mcpbridge

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// checkSock — проверка перед dial (design §4.1): папка сокета — каталог, не ссылка, владелец —
// мы, права 0700 (та же проверка, что у программы в agents/perm_unix.go); файл — сокет того же
// владельца. present=false — папки или файла нет (программа закрыта). errUnsafe — dial не делаем.
// Проверка здесь своя, а не из agents: agents тянет go-sdk (посреднику он не нужен), а пакеты
// с проверкой прав под правилами depguard не импортируются.
func checkSock(sock string) (present bool, err error) {
	fi, err := os.Lstat(filepath.Dir(sock))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errUnsafe
	}
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0o077 != 0 || !ownedByMe(fi) {
		return false, errUnsafe
	}
	fi, err = os.Lstat(sock)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil || fi.Mode()&os.ModeSocket == 0 || !ownedByMe(fi) {
		return false, errUnsafe
	}
	return true, nil
}

func ownedByMe(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
