package paths

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
)

// NoPath — ошибка для лога без путей: в них имя пользователя и названия альбомов
// (log.txt фотограф отправляет разработчику). У ошибок файловой системы остаются
// операция и причина; прочие ошибки возвращаются как есть.
func NoPath(err error) error {
	if err == nil {
		return nil
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return errors.New(pe.Op + ": " + pe.Err.Error())
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return errors.New(le.Op + ": " + le.Err.Error())
	}
	return err
}

// LogRunes — длина строки из файла в логе и тексте ошибки (LogQ).
const LogRunes = 40

// LogQ — строка из файла (вид, тип изделия, ID, относительный путь) для лога и текста ошибки:
// в кавычках, как %q, не длиннее LogRunes знаков, обрезанная — с «…». Чужой файл не пишет
// в log.txt мегабайтную строку.
func LogQ(s string) string {
	n := 0
	for i := range s {
		if n == LogRunes {
			return strconv.Quote(s[:i]) + "…"
		}
		n++
	}
	return strconv.Quote(s)
}
