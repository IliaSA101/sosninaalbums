package imgfile

import (
	"fmt"
	"os"

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
