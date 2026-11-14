// Package namesfile — чтение таблицы с именами для окна «Имена» (043): файл → текст поля списка
// и строка сведений; ошибки — словами для пользователя, подробности — в журнале без путей.
// Диалог выбора, проверка тома и запоминание папки — в App.PickNamesFile.
package namesfile

import (
	"errors"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/table"
)

// File — прочитанная таблица: Text кладётся в поле списка, остальное — строка сведений
// «Класс.xlsx, лист «Класс» (листов: 2), строк: 26».
type File struct {
	Text   string
	Name   string // имя файла без пути
	Sheet  string // лист xlsx; "" — csv/txt
	Sheets int    // листов в книге; 0 — csv/txt
	Rows   int    // непустых строк, с заголовком
}

var (
	ErrText   = errors.New("в таблице слишком много текста: не больше 256 КБ")
	ErrGone   = errors.New("файл таблицы не найден")
	ErrDenied = errors.New("нет доступа к файлу таблицы")
	ErrRead   = errors.New("не удалось прочитать файл таблицы: закройте его в других программах и попробуйте снова")
)

// Read читает таблицу (.xlsx, CSV, «Текст Юникод») как текст поля списка.
func Read(path string) (File, error) {
	s, err := table.ReadFile(path)
	if err != nil {
		return File{}, userErr(err)
	}
	// Предел ячеек (1001×64×256 знаков) допускает текст в десятки МБ, а поле списка — 256 КБ:
	// больший текст names.Read всё равно отклонит, так что до окна (и в IPC) он не доходит.
	text := s.Text()
	if len(text) > model.MaxNamesBytes {
		return File{}, ErrText
	}
	return File{Text: text, Name: filepath.Base(path), Sheet: s.Name, Sheets: s.Sheets, Rows: len(s.Rows)}, nil
}

// userErr — ошибка чтения таблицы для окна: ошибки table уже по-русски, ошибки системы
// (с путём и по-английски) — общими словами, подробности в журнале.
func userErr(err error) error {
	for _, known := range []error{table.ErrOld, table.ErrTooBig, table.ErrBroken, table.ErrNoSheet, table.ErrEmpty} {
		if errors.Is(err, known) {
			return known
		}
	}
	log.Printf("таблица с именами: %v", paths.NoPath(err)) // в ошибке файла — путь с именем пользователя
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ErrGone
	case errors.Is(err, fs.ErrPermission):
		return ErrDenied
	}
	return ErrRead
}
