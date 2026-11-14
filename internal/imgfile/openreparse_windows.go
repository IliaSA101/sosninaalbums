//go:build windows

package imgfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
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

// ErrCloudUnavailable — облачный файл не открылся для чтения (поставщик не отвечает, нет сети, доступ закрыт).
var ErrCloudUnavailable = errors.New("облачный файл недоступен")

// fileID — том и индекс файла по дескриптору (GetFileInformationByHandle): один и тот же файл у двух дескрипторов.
func fileID(h syscall.Handle) (vol uint32, idx uint64, ok bool) {
	var fi syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(h, &fi) != nil {
		return 0, 0, false
	}
	return fi.VolumeSerialNumber, uint64(fi.FileIndexHigh)<<32 | uint64(fi.FileIndexLow), true
}

// reopenData — тот же файл (ReOpenFile от дескриптора, без обращения к пути и потому без гонки подмены) без
// FILE_FLAG_OPEN_REPARSE_POINT: открытие с флагом отдаёт заглушку как есть, без скачивания данных. Переоткрытый
// дескриптор сверяется с исходным: том и индекс файла те же, тег по-прежнему облачный — иначе это уже другой
// объект (ErrChanged). Отказ ReOpenFile — ErrCloudUnavailable: читать исходный дескриптор «как есть» значило бы
// читать заглушку, а не фото. В любом исходе f закрыт или заменён.
func reopenData(f *os.File, path string) (*os.File, error) {
	r, _, _ := procReOpenFile.Call(f.Fd(), syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, 0)
	if r == uintptr(syscall.InvalidHandle) || r == 0 {
		_ = f.Close()
		return nil, &os.PathError{Op: "open", Path: path, Err: ErrCloudUnavailable}
	}
	h := syscall.Handle(r)
	v0, i0, ok0 := fileID(syscall.Handle(f.Fd()))
	v1, i1, ok1 := fileID(h)
	tag, isReparse := reparseTag(h)
	if !ok0 || !ok1 || v0 != v1 || i0 != i1 || !isReparse || !isCloudTag(tag) {
		_ = syscall.CloseHandle(h)
		_ = f.Close()
		return nil, ErrChanged
	}
	_ = f.Close()
	return os.NewFile(uintptr(h), path), nil
}

// evalSymlinks — шов для теста: разворот ссылок не должен вызываться для сетевого пути (запрос SMB).
var evalSymlinks = filepath.EvalSymlinks

// cloudStubPath — по пути лежит облачная заглушка: путь разворачивается до цели (ссылка на заглушку), тег читается
// по дескриптору, открытому только на атрибуты и с FILE_FLAG_OPEN_REPARSE_POINT: данные не скачиваются. Цепочка
// ссылок проверяется по сырым записям (paths.LinkChainLocal) до EvalSymlinks: тот пошёл бы по подставленной
// ссылке на \\host и отправил бы запрос SMB с NTLM-хешем ещё до CheckLocalPath итога.
func cloudStubPath(path string) bool {
	if paths.CheckLocalPath(path) != nil || !paths.LinkChainLocal(path, os.Readlink) {
		return false
	}
	target, err := evalSymlinks(path)
	if err != nil || paths.CheckLocalPath(target) != nil {
		return false
	}
	p16, err := syscall.UTF16PtrFromString(longPath(target))
	if err != nil {
		return false
	}
	h, err := syscall.CreateFile(p16, fileReadAttributes, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // чтение атрибутов: ошибка закрытия ничего не теряет
	tag, ok := reparseTag(h)
	return ok && isCloudTag(tag)
}

// fileReadAttributes — FILE_READ_ATTRIBUTES: доступ без чтения данных.
const fileReadAttributes = 0x80

// longPath — абсолютный путь для CreateFile; длинный (от 248 знаков) с диском — с префиксом \\?\.
// Abs чистит путь: с префиксом Windows сама его не нормализует. Пустое имя Abs превращает в текущую папку
// (не в ошибку); ошибка Abs (нет текущей папки) даёт пустую строку, на которой откажет CreateFile.
func longPath(path string) string {
	abs, _ := filepath.Abs(path)
	if len(abs) >= 248 && len(abs) > 1 && abs[1] == ':' { // длинный путь без префикса не откроется (MAX_PATH)
		abs = `\\?\` + abs
	}
	return abs
}

// fileAttrs — атрибуты Windows из сведений о файле (Go кладёт их в Sys()).
func fileAttrs(st fs.FileInfo) uint32 {
	if d, ok := st.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes
	}
	return 0
}

// openNoReparse — OpenRegular, но Windows-ссылку в последнем звене не разворачивает: файл открывается с
// FILE_FLAG_OPEN_REPARSE_POINT, и подставленная между проверкой и открытием ссылка (в том числе на \\host, откуда
// ушёл бы запрос SMB с NTLM-хешем) открывается как сама ссылка и отклоняется по дескриптору: она не обычный файл.
// Вызывающий проверяет цель заранее (LinkTarget) и сверяет открытое с проверенным (os.SameFile). Другие точки повторной
// обработки отклоняются там же, кроме облачных заглушек (isCloudTag): их Go называет ModeIrregular, но это файлы
// OneDrive «только в сети» — тег читается по дескриптору, данные читает переоткрытый без флага дескриптор (reopenData;
// отказ переоткрытия — ErrCloudUnavailable, подмена — ErrChanged).
func openNoReparse(path string) (*os.File, error) {
	// Локальность пути проверил OpenNoReparse; пустое имя longPath превращает в текущую папку — её отклонит
	// проверка «обычный файл».
	abs := longPath(path)
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
		return reopenData(f, path)
	}
	return f, nil
}
