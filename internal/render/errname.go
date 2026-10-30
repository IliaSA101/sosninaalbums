package render

import (
	"errors"
	"io/fs"
	"strings"
)

// baseName — имя файла без каталога. Путь может прийти из .salbum с другой ОС, поэтому
// режем по обоим разделителям: filepath.Base на Unix не знает «\».
func baseName(path string) string {
	return path[strings.LastIndexAny(path, `/\`)+1:]
}

// photoError — ошибка по фото: в тексте только имя файла, без каталога (в пути имя
// пользователя и названия альбомов, а текст уходит в log.txt и тост). Причина доступна
// через Unwrap, errors.Is работает (fs.ErrNotExist и т. п.).
type photoError struct {
	name string
	msg  string
	err  error
}

func (e *photoError) Error() string { return e.name + ": " + e.msg }
func (e *photoError) Unwrap() error { return e.err }

// photoErr оборачивает ошибку чтения или обработки фото path. Из *fs.PathError берём
// операцию и причину (путь в нём — личные данные), в прочих текстах путь заменяем на имя.
// PathError в цепочку не отдаём: иначе paths.NoPath в логе схлопнул бы текст до «open: …».
func photoErr(path string, err error) error {
	if err == nil {
		return nil
	}
	name := baseName(path)
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return &photoError{name: name, msg: pe.Op + ": " + pe.Err.Error(), err: pe.Err}
	}
	return &photoError{name: name, msg: strings.ReplaceAll(err.Error(), path, name), err: err}
}
