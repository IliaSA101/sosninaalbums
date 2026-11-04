//go:build windows

package imgfile

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// openNoReparse — OpenRegular, но Windows-ссылку в последнем звене не разворачивает: файл открывается с
// FILE_FLAG_OPEN_REPARSE_POINT, и подставленная между проверкой и открытием ссылка (в том числе на \\host, откуда
// ушёл бы запрос SMB с NTLM-хешем) открывается как сама ссылка и отклоняется по дескриптору: она не обычный файл.
// Вызывающий проверяет цель заранее (LinkTarget) и сверяет открытое с проверенным (os.SameFile). Другие точки повторной
// обработки (облачная заглушка) и раньше не были обычным файлом для Go и отклоняются там же.
func openNoReparse(path string) (*os.File, error) {
	// Abs чистит путь: с префиксом `\\?\` Windows сама его не нормализует. Пустое имя Abs превращает в текущую папку
	// (не в ошибку) — её отклонит проверка «обычный файл»; ошибка Abs (нет текущей папки) даёт пустую строку,
	// на которой откажет CreateFile. Локальность пути проверил OpenNoReparse.
	abs, _ := filepath.Abs(path)
	if len(abs) >= 248 && len(abs) > 1 && abs[1] == ':' { // длинный путь без префикса не откроется (MAX_PATH)
		abs = `\\?\` + abs
	}
	p16, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p16, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		_ = f.Close()
		return nil, errors.New("не обычный файл")
	}
	return f, nil
}
