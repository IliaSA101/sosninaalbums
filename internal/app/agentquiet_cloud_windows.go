//go:build windows

package app

import (
	"io/fs"
	"syscall"
)

// Атрибуты облачной заглушки: данные не на диске (OneDrive и подобные).
const cloudAttrs = 0x400000 | 0x40000 | 0x1000 // RECALL_ON_DATA_ACCESS | RECALL_ON_OPEN | OFFLINE

// entryCloud — запись ReadDir (атрибуты уже в ней) — заглушка; не прочитать — тоже заглушка.
func entryCloud(e fs.DirEntry) bool {
	info, err := e.Info()
	if err != nil {
		return true
	}
	d, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || d.FileAttributes&cloudAttrs != 0
}
