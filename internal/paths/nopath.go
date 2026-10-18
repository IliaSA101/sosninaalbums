package paths

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NoPath — ошибка для лога без путей: в них имя пользователя и названия альбомов
// (log.txt фотограф отправляет разработчику). У отказа CheckLocalPath текст с путём
// заменяется на ErrNotLocal, приставка обёртки («разметка разворотов в проекте: …») остаётся;
// у ошибок файловой системы остаются операция и причина; прочие ошибки возвращаются как есть.
func NoPath(err error) error {
	if err == nil {
		return nil
	}
	var ne *notLocalError
	if errors.As(err, &ne) {
		msg, inner := err.Error(), ne.Error()
		if i := strings.LastIndex(msg, inner); i >= 0 && i+len(inner) == len(msg) {
			// Приставки обёрток — тексты программы; путь несёт только сам отказ в конце.
			return &notLocalError{msg: msg[:i] + ErrNotLocal.Error(), dev: ne.dev}
		}
		return &notLocalError{msg: ErrNotLocal.Error(), dev: ne.dev}
	}
	if errors.Is(err, ErrNotLocal) {
		return ErrNotLocal
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

// LogExt — вид расширения файла для лога вместо имени (073-7): «.heic» в нижнем регистре,
// "" — без расширения, «прочее» — не 1–8 латинских букв или цифр (в чужом «расширении» бывает
// имя: «IMG.Иванов»).
func LogExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" || ext == "." {
		return ""
	}
	if len(ext) > 9 {
		return "прочее"
	}
	for _, c := range []byte(ext[1:]) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return "прочее"
		}
	}
	return ext
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
