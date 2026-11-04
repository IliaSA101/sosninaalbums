package imgfile

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// OpenNoReparse — открыть обычный файл, не разворачивая ссылку в последнем звене (на Windows; детали — openNoReparse
// в openreparse_windows.go). Сначала paths.CheckLocalPath, защита в глубину: пустое, относительное и нелокальное
// (UNC, устройство) имя отказывает здесь, а не в ОС — вызывающий мог забыть проверку.
func OpenNoReparse(path string) (*os.File, error) {
	if err := paths.CheckLocalPath(path); err != nil {
		return nil, fmt.Errorf("открытие файла: %w", err)
	}
	return openNoReparse(path)
}

// MaybeRegular — по сведениям о пути запись может оказаться обычным файлом. Кроме обычного, на Windows это
// ModeIrregular: так Go называет облачную заглушку (OneDrive «только в сети»), но так же и точку соединения,
// и другие точки повторной обработки — различает их только дескриптор (OpenNoReparse, OpenRegular), там и решение.
// Папка и ссылка сюда не попадают (у них свои биты режима).
func MaybeRegular(m fs.FileMode) bool {
	return m.IsRegular() || runtime.GOOS == "windows" && m.Type() == fs.ModeIrregular
}
