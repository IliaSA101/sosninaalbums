//go:build !windows

package agents

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// secureDir готовит папку сокета mcp: родитель не открыт на запись другим, каталог не ссылка,
// владелец — текущий пользователь, права 0700. Chmod молча не действует на exFAT, ntfs-3g и общих
// разделах, а ссылка увела бы Chmod по ней — поэтому ссылку смотрим до Chmod, а итог проверяем
// после него. release пустой: держать нечего (Windows держит дескриптор).
func secureDir(dir string) (release func(), err error) {
	release = func() {}
	if err = checkParent(filepath.Dir(dir)); err != nil {
		return release, err
	}
	fi, err := os.Lstat(dir)
	switch {
	case err == nil:
		if fi.Mode()&os.ModeSymlink != 0 {
			return release, errNoPerm
		}
	case !errors.Is(err, fs.ErrNotExist):
		return release, err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return release, err
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		return release, chmodErr(err)
	}
	if fi, err = os.Lstat(dir); err != nil {
		return release, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0o077 != 0 ||
		!ok || int(st.Uid) != os.Getuid() {
		return release, errNoPerm
	}
	return release, nil
}

// checkParent — родитель папки сокета (<config>/<имя>) не должен позволять другим переименовать
// mcp и подложить свою (0.32-148-5): владелец — свой или root, группе и всем писать нельзя.
// Смотрим цель, не ссылку (StatLocal): ссылка на папку настроек — перенос на другой диск. Родителя
// нет — его создаст MkdirAll с нашими правами. Выше по пути программа не отвечает: после сверки
// собеседника подмена в предке даёт только отказ в обслуживании.
func checkParent(parent string) error {
	fi, err := library.StatLocal(parent)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || !ok {
		return errNoPerm
	}
	if uid := int(st.Uid); uid != os.Getuid() && uid != 0 || fi.Mode().Perm()&0o022 != 0 {
		return errParentOpen
	}
	return nil
}

// secureSock ставит сокету 0600 и проверяет итог: защищает папка, это — второй рубеж.
func secureSock(path string) error {
	if err := os.Chmod(path, sockMode); err != nil {
		return chmodErr(err)
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

// chmodErr: ФС без прав (exFAT, ntfs-3g, общий раздел) отвечает на Chmod отказом — для окна это
// «папка данных на диске без прав доступа», а не общий сбой.
func chmodErr(err error) error {
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) {
		return errNoPerm
	}
	return err
}
