//go:build darwin

package osx

import "golang.org/x/sys/unix"

// osVersion — kern.osproductversion без cgo. На macOS 11 программе, собранной со старым SDK,
// система в режиме совместимости отдаёт «10.16», на macOS 26 — «16.0» вместо 26.x; «10.16»
// отсекает update.SystemOf, а «16.0» при пороге minOS 13 безвредна (16 ≥ 13). Порог ≥ 16
// потребует отдельной проверки.
func osVersion() (string, bool) {
	v, err := unix.Sysctl("kern.osproductversion")
	return v, err == nil && v != ""
}
