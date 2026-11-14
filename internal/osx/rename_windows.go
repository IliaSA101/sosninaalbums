//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// RenameNoReplace переименовывает файл, только если имя to свободно; занято — ошибка с
// fs.ErrExist. os.Rename на Windows заменяет файл (MOVEFILE_REPLACE_EXISTING), а между
// проверкой имени и Rename его мог занять чужой файл. MoveFileW без флагов не заменяет.
func RenameNoReplace(from, to string) error {
	f, err := syscall.UTF16PtrFromString(extPath(from))
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	t, err := syscall.UTF16PtrFromString(extPath(to))
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	if err := syscall.MoveFile(f, t); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

// extPath — длинный абсолютный путь с префиксом \\?\, как fixLongPath пакета os: без него
// MoveFileW не примет путь от 260 знаков, если длинные пути в Windows не включены. Порог 248 — тот
// же, что у os (папке нужен запас под имя 8.3). Префикс отключает разбор «.» и «..» — путь
// сначала чистится; UNC — \\?\UNC\сервер\ресурс.
func extPath(p string) string {
	if len(p) < 248 || strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\\.\`) || !filepath.IsAbs(p) {
		return p
	}
	p = filepath.Clean(p)
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}
