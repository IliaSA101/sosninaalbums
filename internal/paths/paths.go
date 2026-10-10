package paths

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// MaxPathLen — предел длины пути Windows (UTF-16 знаков); байтов UTF-8 не меньше, так что
// проверка по байтам строже, а настоящие пути на порядки короче.
const MaxPathLen = 32_767

// CheckLocalPath пропускает только абсолютный локальный путь. На Windows это
// белый список `X:\…` / `X:/…` (буква диска, двоеточие, разделитель): UNC,
// `\??\UNC\…`, `\\?\`, `\\.\`, `\x` и `C:x` открыли бы сетевую шару, устройство
// или зависели бы от текущей папки процесса. Двоеточие дальше второй позиции —
// альтернативный поток NTFS (`C:.jpg:поток`). На остальных ОС — абсолютный без `//`.
func CheckLocalPath(path string) error {
	if len(path) > MaxPathLen {
		return fmt.Errorf("слишком длинный путь")
	}
	if path == "" || strings.ContainsRune(path, 0) {
		return fmt.Errorf("пустой или недопустимый путь")
	}
	if runtime.GOOS == "windows" {
		ok := len(path) >= 3 && path[1] == ':' && IsSep(path[2]) &&
			(path[0] >= 'A' && path[0] <= 'Z' || path[0] >= 'a' && path[0] <= 'z') &&
			!strings.ContainsRune(path[2:], ':')
		if !ok {
			return fmt.Errorf("поддерживаются только локальные пути вида C:\\папка: %s", path)
		}
		return nil
	}
	if !filepath.IsAbs(path) || strings.HasPrefix(path, "//") {
		return fmt.Errorf("путь должен быть абсолютным и локальным: %s", path)
	}
	return nil
}

// IsSep — разделитель пути любой ОС: путь мог быть записан на другой.
func IsSep(c byte) bool { return c == '\\' || c == '/' }

// SamePath — один и тот же путь после Clean; на Windows без учёта регистра.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
