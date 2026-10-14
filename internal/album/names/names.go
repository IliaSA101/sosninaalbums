// Package names — правки полей подписей (030): имя на фото, вставка
// списка имён по местам листа, общий стиль подписей листа. Тела правок над *model.Project;
// обёртки Editor — ops_design.go ядра (аргументы там уже зажаты). Разбор списка — parse.go.
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

// Paste — имена из списка people по порядку фото листа (места сетки по порядку или лента
// «Авто»): i-е имя — i-му фото. extra — строк больше, чем фото; missing — фото листа без
// имени после вставки (не хватило строк и имени из файла нет). Групповое фото виньетки
// (031) пропускается: имя одного человека ему не подходит.
func Paste(p *model.Project, spreadID string, people []model.Person) (extra, missing int, err error) {
	s := p.Sheet(spreadID) // и обложка (032 §1)
	if s == nil {
		return 0, 0, errNoSpread
	}
	ids := s.Photos
	if s.IsVignette() {
		ids = nil
		for _, pl := range s.Grid {
			if pl.Photo != "" && pl.Role != model.RoleGroup {
				ids = append(ids, pl.Photo)
			}
		}
	}
	for i, id := range ids {
		r, ok := p.Photos[id]
		if !ok {
			continue
		}
		if i < len(people) {
			r.Person = model.ClampPerson(people[i])
			p.Photos[id] = r
		}
		if who, _ := r.Name(); who == nil {
			missing++
		}
	}
	return max(0, len(people)-len(ids)), missing, nil
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
