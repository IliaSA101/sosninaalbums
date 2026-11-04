//go:build !unix

package app

import (
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// openAlbumFile: на Windows ссылку вместо файла не открываем — FILE_FLAG_OPEN_REPARSE_POINT (М1): подставленная
// после Lstat ссылка (в том числе на \\host) не разворачивается, дескриптор — не обычный файл. Дальше os.SameFile.
func openAlbumFile(path string) (*os.File, error) { return imgfile.OpenNoReparse(path) }
