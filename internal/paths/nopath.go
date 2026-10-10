package paths

import (
	"errors"
	"io/fs"
	"os"
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
