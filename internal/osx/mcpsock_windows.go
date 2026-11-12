package osx

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

// x/sys в osx запрещён (depguard): известная папка — через shell32 напрямую.
var (
	procKnownFolder = syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath")
	procCoTaskFree  = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
)

// folderRoamingAppData — FOLDERID_RoamingAppData.
var folderRoamingAppData = syscall.GUID{
	Data1: 0x3EB685DB, Data2: 0x65F9, Data3: 0x4CF6,
	Data4: [8]byte{0xA0, 0x3A, 0xE3, 0xEF, 0x65, 0x72, 0x9F, 0x3D},
}

// roamingDir — Roaming AppData по токену процесса (известная папка), а не по переменной APPDATA:
// у процесса, запущенного от другого пользователя, переменная бывает от родителя (спайк Т0).
// Известная папка не нашлась — ConfigDir.
func roamingDir() string {
	var p *uint16
	hr, _, _ := procKnownFolder.Call(uintptr(unsafe.Pointer(&folderRoamingAppData)), 0, 0, uintptr(unsafe.Pointer(&p))) //nolint:gosec // G103: аргументы WinAPI
	if p == nil {
		return ConfigDir()
	}
	defer procCoTaskFree.Call(uintptr(unsafe.Pointer(p))) //nolint:gosec,errcheck // G103: аргументы WinAPI; освобождение — ошибке нечего делать
	if hr != 0 {
		return ConfigDir()
	}
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 { //nolint:gosec // G103: строка, оканчивающаяся нулём, от WinAPI
		n++
	}
	if d := syscall.UTF16ToString(unsafe.Slice(p, n)); filepath.IsAbs(d) { //nolint:gosec // G103: см. выше
		return d
	}
	return ConfigDir()
}
