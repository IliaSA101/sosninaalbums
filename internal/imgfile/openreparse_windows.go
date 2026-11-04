//go:build windows

package imgfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procGetFileInformationByHandleEx = kernel32.NewProc("GetFileInformationByHandleEx")
	procReOpenFile                   = kernel32.NewProc("ReOpenFile")
)

// fileAttributeTagInfo — класс FileAttributeTagInfo в GetFileInformationByHandleEx.
const fileAttributeTagInfo = 9

// reparseTag — тег точки повторной обработки открытого файла (по дескриптору, а не по пути); ok = false —
// файл не точка повторной обработки или тег прочитать не удалось.
func reparseTag(h syscall.Handle) (tag uint32, ok bool) {
	var info struct{ Attributes, Tag uint32 }
	r, _, _ := procGetFileInformationByHandleEx.Call(uintptr(h), fileAttributeTagInfo,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)) //nolint:gosec // G103: аргумент WinAPI
	if r == 0 || info.Attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return 0, false
	}
	return info.Tag, true
}

// isCloudTag — тег облачной заглушки Cloud Files API (OneDrive и другие поставщики): IO_REPARSE_TAG_CLOUD
// 0x9000001A и CLOUD_1…CLOUD_F (0x9000X01A, X — номер поставщика). Это не «поверхностный» тег имени:
// ссылкой по файловой системе он не ведёт, данные скачивает фильтр cldflt при чтении.
func isCloudTag(tag uint32) bool { return tag&^0xF000 == 0x9000001A }

// cloudStub — открытый файл f (сведения st) — облачная заглушка. Go с 1.23 называет любую точку повторной
// обработки, кроме ссылки, ModeIrregular; тег читаем по дескриптору, подмена пути между проверкой и чтением
// на него не влияет.
func cloudStub(f *os.File, st fs.FileInfo) bool {
	if st.Mode().Type() != fs.ModeIrregular {
		return false
	}
	tag, ok := reparseTag(syscall.Handle(f.Fd()))
	return ok && isCloudTag(tag)
}

// regularOpened — открытый файл обычный или облачная заглушка (OpenRegular, openNoReparse).
func regularOpened(f *os.File, st fs.FileInfo) bool {
	return st.Mode().IsRegular() || cloudStub(f, st)
}

// reopenData — тот же файл (ReOpenFile от дескриптора, без обращения к пути и потому без гонки подмены) без
// FILE_FLAG_OPEN_REPARSE_POINT: открытие с флагом может отдать заглушку как есть, без скачивания данных.
// Не вышло — остаётся исходный дескриптор: тот же файл, защита от ссылки прежняя.
func reopenData(f *os.File, path string) *os.File {
	r, _, _ := procReOpenFile.Call(f.Fd(), syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, 0)
	if r == uintptr(syscall.InvalidHandle) || r == 0 {
		return f
	}
	h := syscall.Handle(r)
	if tag, ok := reparseTag(h); ok && !isCloudTag(tag) { // нельзя: между открытиями это уже не заглушка
		_ = syscall.CloseHandle(h)
		return f
	}
	_ = f.Close()
	return os.NewFile(uintptr(h), path)
}

// openNoReparse — OpenRegular, но Windows-ссылку в последнем звене не разворачивает: файл открывается с
// FILE_FLAG_OPEN_REPARSE_POINT, и подставленная между проверкой и открытием ссылка (в том числе на \\host, откуда
// ушёл бы запрос SMB с NTLM-хешем) открывается как сама ссылка и отклоняется по дескриптору: она не обычный файл.
// Вызывающий проверяет цель заранее (LinkTarget) и сверяет открытое с проверенным (os.SameFile). Другие точки повторной
// обработки отклоняются там же, кроме облачных заглушек (isCloudTag): их Go называет ModeIrregular, но это файлы
// OneDrive «только в сети» — тег читается по дескриптору, данные читает переоткрытый без флага дескриптор (reopenData).
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
	st, err := f.Stat()
	switch {
	case err != nil || !regularOpened(f, st):
		_ = f.Close()
		return nil, errors.New("не обычный файл")
	case !st.Mode().IsRegular(): // облачная заглушка
		return reopenData(f, path), nil
	}
	return f, nil
}
