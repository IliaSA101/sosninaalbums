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
// Фото на месте с табличкой (071) — правится табличка (её карточка и показывает), nil
// снимает её: снова видно имя фото или из файла. Табличка только с предметом (071-16): имя
// в карточке — фото (своё или из файла), поэтому части имени идут в фото, предмет — в
// табличку; иначе имя из файла застряло бы на месте и после обмена подписало бы чужое фото.
// Фото не в альбоме — холостая правка.
func SetPerson(p *model.Project, photoID string, who *model.Person) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	if _, pl := p.PlaceOf(photoID); pl != nil && pl.Person != (model.Person{}) {
		if !pl.Person.Named() {
			setSubjectSeat(p, photoID, pl, who)
			return
		}
		pl.Person = model.Person{}
		if who != nil {
			pl.Person = *who
		}
		return
	}
	r.Person = who
	p.Photos[photoID] = r
}

// setSubjectSeat — правка имени фото на месте с табличкой только предмета (071-16): части имени
// — фото, предмет — табличке (пустой снимает её). Свой предмет фото снимается: он скрыт под
// табличкой и всплыл бы после снятия предмета (как dropOwnName, 071-15).
func setSubjectSeat(p *model.Project, photoID string, pl *model.Place, who *model.Person) {
	var name, seat model.Person
	if who != nil {
		name = model.Person{Last: who.Last, First: who.First, Middle: who.Middle}
		seat.Subject = who.Subject
	}
	r := p.Photos[photoID]
	r.Person = nil
	if name.Named() {
		r.Person = &name
	}
	p.Photos[photoID] = r
	pl.Person = seat
}

// SetPlacePerson — табличка места placeID листа spreadID (и обложки); t уже зажат
// (ClampSeat), ноль — снять. Нет листа или места — холостая правка.
func SetPlacePerson(p *model.Project, spreadID, placeID string, t model.Person) {
	if pl := seat(p, spreadID, placeID); pl != nil {
		pl.Person = t
	}
}

// PlacePerson — табличка места для карточки и окошка «Имя» (071); Teacher — место учителя
// виньетки (поле «Предмет»), Place — ID места, если табличка есть. ok = false — места нет.
func PlacePerson(p *model.Project, spreadID, placeID string) (pv model.PersonView, ok bool) {
	pl := seat(p, spreadID, placeID)
	if pl == nil {
		return model.PersonView{}, false
	}
	t := pl.Person
	pv = model.PersonView{Last: t.Last, First: t.First, Middle: t.Middle, Subject: t.Subject,
		Teacher: p.Sheet(spreadID).IsVignette() && pl.Role == model.RoleTeacher}
	if t != (model.Person{}) {
		pv.Place, pv.SeatNamed = pl.ID, t.Named()
	}
	return pv, true
}

// seat — место placeID листа spreadID; nil — нет такого.
func seat(p *model.Project, spreadID, placeID string) *model.Place {
	s := p.Sheet(spreadID)
	if s == nil || placeID == "" {
		return nil
	}
	for _, pl := range s.Grid {
		if pl != nil && pl.ID == placeID {
			return pl
		}
	}
	return nil
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
