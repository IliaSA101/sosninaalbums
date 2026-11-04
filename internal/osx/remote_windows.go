package osx

// remoteOS (Windows): UNC-путь — сетевой; буква диска — GetDriveTypeW: локальны DRIVE_FIXED и
// DRIVE_REMOVABLE, сетевой диск (DRIVE_REMOTE), CD-ROM, RAM-диск и неизвестный — нет.

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var procGetDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

// Типы GetDriveTypeW.
const (
	driveRemovable = 2
	driveFixed     = 3
)

func remoteOS(path string) (bool, error) {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return true, nil // UNC и \\?\: сетевой путь (и \\?\C:\ тоже не берём)
	}
	vol := filepath.VolumeName(path)
	if len(vol) != 2 || vol[1] != ':' {
		return true, errors.New("у пути нет буквы диска")
	}
	root, err := syscall.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return true, err
	}
	// Указатель живёт до возврата Call: так WinAPI зовут через syscall (x/sys в osx запрещён).
	t, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(root))) //nolint:gosec // G103: аргумент WinAPI
	return t != driveFixed && t != driveRemovable, nil
}
