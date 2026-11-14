//go:build !windows

package osx

import (
	"errors"
	"io/fs"
	"os"
)

// RenameNoReplace переименовывает файл, только если имя to свободно; занято — ошибка с
// fs.ErrExist. rename(2) заменяет файл, а link(2) — нет: жёсткая ссылка под новым именем и
// удаление старого. Без жёстких ссылок (FAT, exFAT флешек) — прежние Lstat и Rename: окно
// гонки остаётся, без renameat2/renamex_np (их нет в syscall) его там не закрыть.
func RenameNoReplace(from, to string) error {
	err := os.Link(from, to)
	switch {
	case err == nil:
		_ = os.Remove(from) // имя to уже наше; не удалился — временный уберёт следующий сбор
		return nil
	case errors.Is(err, fs.ErrExist):
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: fs.ErrExist}
	}
	return os.Rename(from, to)
}
