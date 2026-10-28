package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// appDataDir — Roaming AppData по токену процесса (известная папка), а не по переменной среды: у процесса,
// запущенного через Start-Process -Credential, APPDATA может остаться от родителя. Среда выводится в журнал.
func appDataDir() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0); err == nil && p != "" {
		return p
	}
	d, _ := os.UserConfigDir()
	return d
}
