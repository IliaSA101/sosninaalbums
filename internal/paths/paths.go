package paths

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// MaxPathLen — предел длины пути Windows (UTF-16 знаков); байтов UTF-8 не меньше, так что
// проверка по байтам строже, а настоящие пути на порядки короче.
const MaxPathLen = 32_767

// ErrNotLocal — путь не локальный или недопустимый. Текст без пути: его пишут в log.txt
// (NoPath); тост получает ошибку с путём, errors.Is(err, ErrNotLocal) её узнаёт.
var ErrNotLocal = errors.New("путь не локальный или недопустимый")

// notLocalError — ErrNotLocal с подробным текстом для тоста (путь в нём); dev — отказ по
// имени устройства (узнаётся и как ErrDevice).
type notLocalError struct {
	msg string
	dev bool
}

func (e *notLocalError) Error() string { return e.msg }
func (e *notLocalError) Unwrap() error { return ErrNotLocal }

// Is — отказ по устройству узнаётся и как ErrDevice.
func (e *notLocalError) Is(target error) bool { return e.dev && target == ErrDevice }

func notLocal(format string, a ...any) error {
	return &notLocalError{msg: fmt.Sprintf(format, a...)}
}

// CheckLocalPath пропускает только абсолютный локальный путь. На Windows это
// белый список `X:\…` / `X:/…` (буква диска, двоеточие, разделитель): UNC,
// `\??\UNC\…`, `\\?\`, `\\.\`, `\x` и `C:x` открыли бы сетевую шару, устройство
// или зависели бы от текущей папки процесса. Двоеточие дальше второй позиции —
// альтернативный поток NTFS (`C:.jpg:поток`). Устройство в любом элементе (DeviceElem: «CON»
// без расширения — всегда, «CON.jpg» — как судит ОС) — отказ, узнаётся и как ErrDevice:
// открыть такой путь — значит обратиться к устройству (073-10). На остальных ОС — абсолютный
// без `//`.
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
			return notLocal("поддерживаются только локальные пути вида C:\\папка: %s", path)
		}
		for _, el := range strings.FieldsFunc(path[3:], IsSepRune) {
			if DeviceElem(el) {
				return &notLocalError{msg: fmt.Sprintf("имя «%s» зарезервировано Windows", el), dev: true}
			}
		}
		return nil
	}
	if !filepath.IsAbs(path) || strings.HasPrefix(path, "//") {
		return notLocal("путь должен быть абсолютным и локальным: %s", path)
	}
	return nil
}

// IsSep — разделитель пути любой ОС: путь мог быть записан на другой.
func IsSep(c byte) bool { return c == '\\' || c == '/' }

// IsSepRune — IsSep для руны (strings.FieldsFunc).
func IsSepRune(r rune) bool { return r == '\\' || r == '/' }

// SamePath — один и тот же путь после Clean (FoldEqual: на Windows и macOS без учёта регистра).
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return FoldEqual(filepath.Clean(a), filepath.Clean(b))
}

// Fold — ключ пути для «один ли это путь» (101, п. 4): на Windows и macOS — нижний регистр,
// на Linux — как есть (там `/x/Весна` и `/x/ВЕСНА` — разные папки, а `DSC_1.JPG` в них — разные
// фото). Расширения, имена файлов и порядок сортировки сводят регистр сами, не через Fold.
func Fold(s string) string { return foldWith(foldCase, s) }

// foldWith — Fold с явным режимом: тесты проверяют оба на любой ОС.
func foldWith(ci bool, s string) string {
	if ci {
		return strings.ToLower(s)
	}
	return s
}

// FoldEqual — a и b — один путь по правилу Fold, без Clean. Без регистра — strings.EqualFold,
// как сравнивали на Windows до 0.23.
func FoldEqual(a, b string) bool { return foldEqualWith(foldCase, a, b) }

func foldEqualWith(ci bool, a, b string) bool {
	if ci {
		return strings.EqualFold(a, b)
	}
	return a == b
}
