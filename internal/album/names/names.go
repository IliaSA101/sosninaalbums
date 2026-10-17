// Package names — правки полей подписей (030): имя на фото, вставка
// списка имён по местам листа, общий стиль подписей листа. Тела правок над *model.Project;
// обёртки Editor — ops_design.go ядра (аргументы там уже зажаты). Окно «Имена» (043): разбор
// поля — parse.go (Read) и header.go (заголовки таблицы), кому какое имя — plan.go (Plan,
// Apply), по столбцу «Файл» — match.go. Имя из названия файла при броске (060) — auto.go
// (WithAuto, AutoFromFiles, ForgetFileNames).
package names

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// errNoSpread — разворота нет (удалили, пока было открыто окно «Имена»).
var errNoSpread = errors.New("разворот не найден")

// SetPerson — имя на фото; who уже зажат (ClampPerson), nil — снова из имени файла.
// Фото не в альбоме — холостая правка.
func SetPerson(p *model.Project, photoID string, who *model.Person) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	r.Person = who
	p.Photos[photoID] = r
}

// SetSheetCaps — общий стиль подписей листа: on — c (уже зажат ClampSheetCaps), иначе
// снять (свои стили подписей снова действуют). Caps заменяется целиком, а не правится на
// месте: копии разворота (SplitBig) делят указатель.
func SetSheetCaps(p *model.Project, spreadID string, on bool, c model.SheetCaps) {
	s := p.Sheet(spreadID) // и обложка (032 §1)
	if s == nil {
		return
	}
	if !on {
		s.Caps = nil
		return
	}
	s.Caps = &c
}
