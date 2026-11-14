package osx

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

// Известные папки: «Документы», «Рабочий стол», «Загрузки».
var knownFolderIDs = []syscall.GUID{
	{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}},
	{Data1: 0xB4BFCC3A, Data2: 0xDB2C, Data3: 0x424C, Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}},
	{Data1: 0x374DE290, Data2: 0x123F, Data3: 0x4565, Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}},
}

// maxKnownPath — предел длины пути от WinAPI в знаках UTF-16 (32 767 — предел самой Windows).
const maxKnownPath = 32768

// knownDirs — адреса трёх папок по токену процесса (SHGetKnownFolderPath): они бывают на другом
// диске или в OneDrive, имя «Documents» под домашней тут не годится. Не нашлась — пропуск.
func knownDirs() []string {
	var out []string
	for i := range knownFolderIDs {
		var p *uint16
		hr, _, _ := procKnownFolder.Call(uintptr(unsafe.Pointer(&knownFolderIDs[i])), 0, 0, uintptr(unsafe.Pointer(&p))) //nolint:gosec // G103: аргументы WinAPI
		if p == nil {
			continue
		}
		if hr == 0 {
			n := 0
			for n < maxKnownPath && *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 { //nolint:gosec // G103: строка, оканчивающаяся нулём, от WinAPI
				n++
			}
			if d := syscall.UTF16ToString(unsafe.Slice(p, n)); filepath.IsAbs(d) { //nolint:gosec // G103: см. выше
				out = append(out, d)
			}
		}
		_, _, _ = procCoTaskFree.Call(uintptr(unsafe.Pointer(p))) //nolint:gosec // G103: аргумент WinAPI
	}
	return out
}
