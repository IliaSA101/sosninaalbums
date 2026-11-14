//go:build !windows

package agentplaces

import (
	"os"
	"syscall"
)

// DevOf — номер устройства папки (st_dev): у корня тома он другой, чем у родителя. Lstat: путь
// уже без ссылок. Не узнали — ok = false (место не берём).
func DevOf(p string) (dev uint64, ok bool) {
	st, err := os.Lstat(p)
	if err != nil {
		return 0, false
	}
	sys, isStat := st.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, false
	}
	return uint64(sys.Dev), true //nolint:unconvert,gosec // G115: Dev — int32 на macOS, uint64 на Linux
}
