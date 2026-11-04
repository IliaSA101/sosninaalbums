//go:build !windows

package imgfile

import (
	"io/fs"
	"os"
)

// openNoReparse — вне Windows то же, что OpenRegular: ссылку там отсекают O_NOFOLLOW у вызывающего
// (тихое открытие альбома) и проверка цели LinkTarget; точек повторной обработки NTFS нет.
func openNoReparse(path string) (*os.File, error) { return OpenRegular(path) }

// regularOpened — вне Windows облачных заглушек Go не называет irregular: только обычный файл.
func regularOpened(_ *os.File, st fs.FileInfo) bool { return st.Mode().IsRegular() }
