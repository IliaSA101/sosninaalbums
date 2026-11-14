package osx

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// startMarkMax — предел файла отметки: в нём одно число; подсунутый большой файл не читаем.
const startMarkMax = 64

// ClaimStart — «занять» запуск программы посредником (149 §5): true без ошибки, если последняя отметка в
// папке dir старше window, и тогда сразу пишет новую. Отметка лежит в кэше, а не в памяти: каждый
// клиент агента запускает своего посредника, и у них общая только папка. Отметка из будущего
// (часы переведены назад или файл подложен) устаревшей: иначе запуск был бы закрыт навсегда.
// Ошибка записи (нет папки, диск, права) — отказ с error: запуск без отметки частоты не разрешаем.
// Гонка двух посредников в одну миллисекунду допустима: второй экземпляр программы не запустится
// (замок экземпляра), а окно всплывёт один раз.
func ClaimStart(dir string, now time.Time, window time.Duration) (bool, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return false, errMarkDir
	}
	path := filepath.Join(dir, "start.mark")
	if d := now.Sub(readMark(path)); d >= 0 && d < window {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(strconv.FormatInt(now.UnixNano(), 10)), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

var errMarkDir = errors.New("нет абсолютной папки для отметки запуска")

// readMark — время отметки; нулевое время (давным-давно), если файла нет или он негоден.
func readMark(path string) time.Time {
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() { // канал или устройство читать нельзя
		return time.Time{}
	}
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, startMarkMax+1))
	if err != nil || len(b) > startMarkMax {
		return time.Time{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}
