//go:build !windows

package agents

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// secureDir готовит папку сокета mcp: каталог, не ссылка, владелец — текущий пользователь,
// права 0700. Chmod молча не действует на exFAT, ntfs-3g и общих разделах, а ссылка увела бы
// Chmod по ней — поэтому ссылку смотрим до Chmod, а итог проверяем после него.
func secureDir(dir string) error {
	fi, err := os.Lstat(dir)
	switch {
	case err == nil:
		if fi.Mode()&os.ModeSymlink != 0 {
			return errNoPerm
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		return err
	}
	if fi, err = os.Lstat(dir); err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0o077 != 0 ||
		!ok || int(st.Uid) != os.Getuid() {
		return errNoPerm
	}
	return nil
}

// secureSock ставит сокету 0600 и проверяет итог: защищает папка, это — второй рубеж.
func secureSock(path string) error {
	if err := os.Chmod(path, sockMode); err != nil {
		return err
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 || fi.Mode().Perm()&0o077 != 0 {
		return errNoPerm
	}
	return nil
}
