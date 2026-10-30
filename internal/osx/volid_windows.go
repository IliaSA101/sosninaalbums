package osx

// volumeOS (Windows): GetVolumePathNameW → тип диска (только FIXED и REMOVABLE) →
// GetVolumeInformationW: серийный номер, метка, ФС. Пустой слот картридера отвечает
// ERROR_NOT_READY без окна «Вставьте диск»: Go при старте ставит SEM_FAILCRITICALERRORS
// (TestVolErrorMode проверяет, что режим не снят). WinAPI зовём через syscall: x/sys в osx запрещён.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procVolPathName   = kernel32.NewProc("GetVolumePathNameW")
	procVolInfo       = kernel32.NewProc("GetVolumeInformationW")
	procLogicalDrives = kernel32.NewProc("GetLogicalDrives")
	procVolDriveType  = kernel32.NewProc("GetDriveTypeW")

	errVolNotLocal = errors.New("том не локальный (сетевой, CD-ROM или неизвестный)")
	errVolUNC      = errors.New("сетевой путь")
	errVolNoRoot   = errors.New("не удалось определить корень тома")
)

// Типы GetDriveTypeW, годные для привязки.
const (
	volDriveRemovable = 2
	volDriveFixed     = 3
)

func isAbsVol(p string) bool { return filepath.IsAbs(p) }

// volRoot — корень тома пути («E:\» или папка-точка монтирования с «\» в конце).
func volRoot(p string) (string, error) {
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return "", errVolUNC // UNC и \\?\: сетевой путь — привязки нет
	}
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, len(p)+4)
	// Указатели живут до возврата Call: так WinAPI зовут через syscall.
	r, _, e := procVolPathName.Call(uintptr(unsafe.Pointer(in)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))) //nolint:gosec // G103: аргументы WinAPI
	if r == 0 {
		return "", fmt.Errorf("GetVolumePathName: %w", e)
	}
	root := syscall.UTF16ToString(buf)
	if root == "" {
		return "", errVolNoRoot
	}
	if !strings.HasSuffix(root, `\`) {
		root += `\`
	}
	return root, nil
}

// volInfo — серийный номер, метка и ФС тома по корню; сперва тип диска.
func volInfo(root string) (serial uint32, label, fsName string, err error) {
	rp, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, "", "", err
	}
	t, _, _ := procVolDriveType.Call(uintptr(unsafe.Pointer(rp))) //nolint:gosec // G103: аргумент WinAPI
	if t != volDriveFixed && t != volDriveRemovable {
		return 0, "", "", errVolNotLocal
	}
	var nameBuf, fsBuf [261]uint16
	// Указатели живут до возврата Call; G103 — аргументы WinAPI.
	r, _, e := procVolInfo.Call(
		uintptr(unsafe.Pointer(rp)),                                 //nolint:gosec // G103
		uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(len(nameBuf)), //nolint:gosec // G103
		uintptr(unsafe.Pointer(&serial)), 0, 0, //nolint:gosec // G103
		uintptr(unsafe.Pointer(&fsBuf[0])), uintptr(len(fsBuf)), //nolint:gosec // G103
	)
	if r == 0 {
		return 0, "", "", fmt.Errorf("GetVolumeInformation: %w", e)
	}
	return serial, syscall.UTF16ToString(nameBuf[:]), syscall.UTF16ToString(fsBuf[:]), nil
}

// winVolID — «win:<серийный номер hex8>:<ФС>». Серийный номер 0 — тома не различить.
func winVolID(serial uint32, fsName string) (string, error) {
	if serial == 0 || fsName == "" {
		return "", errNoVolID
	}
	return fmt.Sprintf("win:%08x:%s", serial, fsName), nil
}

func volumeOS(p string) (Vol, error) {
	root, err := volRoot(p)
	if err != nil {
		return Vol{}, err
	}
	serial, label, fsName, err := volInfo(root)
	if err != nil {
		return Vol{}, err
	}
	id, err := winVolID(serial, fsName)
	if err != nil {
		return Vol{}, err
	}
	rest, ok := cutFold(filepath.Clean(p), filepath.Clean(root))
	if !ok {
		return Vol{}, errVolNoRoot // путь не под корнем (короткие имена 8.3, subst): Rel не определить
	}
	return Vol{Root: root, ID: id, Label: cleanLabel(label), Rel: strings.Trim(filepath.ToSlash(rest), "/")}, nil
}

// cutFold отрезает префикс без учёта регистра (пути Windows нечувствительны к нему).
func cutFold(p, prefix string) (string, bool) {
	if len(p) < len(prefix) || !strings.EqualFold(p[:len(prefix)], prefix) {
		return "", false
	}
	return p[len(prefix):], true
}

// volumePathOS — перебор букв GetLogicalDrives: только FIXED и REMOVABLE (сетевые диски и
// пустые CD-ROM не трогаем — они могут висеть). Тома без буквы (папка-точка) не ищем.
func volumePathOS(id, rel string) (string, string, error) {
	mask, _, _ := procLogicalDrives.Call()
	for i := range 26 {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		serial, label, fsName, err := volInfo(root)
		if err != nil {
			continue // пустой слот (ERROR_NOT_READY), сетевой, CD-ROM
		}
		if got, err := winVolID(serial, fsName); err != nil || got != id {
			continue
		}
		return root + filepath.FromSlash(rel), cleanLabel(label), nil
	}
	return "", "", ErrVolGone
}
