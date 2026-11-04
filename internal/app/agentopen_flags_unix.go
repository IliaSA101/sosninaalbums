//go:build unix

package app

import (
	"os"
	"syscall"
)

// openAlbumFile: ссылка вместо файла не откроется (O_NOFOLLOW), FIFO не повесит открытие (O_NONBLOCK).
func openAlbumFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: путь из «Последних» после recentCheck и CheckLocalPath
}
