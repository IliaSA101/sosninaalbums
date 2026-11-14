//go:build darwin

package app

import (
	"io/fs"
	"syscall"
)

// sfDataless — SF_DATALESS в st_flags: содержимое в iCloud.
const sfDataless = 0x40000000

// entryCloud — фото без содержимого на диске; не прочитать — тоже заглушка.
func entryCloud(e fs.DirEntry) bool {
	info, err := e.Info()
	if err != nil {
		return true
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return !ok || st.Flags&sfDataless != 0
}
