//go:build !windows

package imgfile

import "os"

// openNoReparse — вне Windows то же, что OpenRegular: ссылку там отсекают O_NOFOLLOW у вызывающего
// (тихое открытие альбома) и проверка цели LinkTarget; точек повторной обработки NTFS нет.
func openNoReparse(path string) (*os.File, error) { return OpenRegular(path) }
