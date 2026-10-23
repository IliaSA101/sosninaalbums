package names

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Имя из названия файла при броске (060, сценарий А): бросок, «Поставить на места» и
// «Заполнить» в том же шаге правки дают новым фото имя model.LooksLikeName; тост
// «Убрать имена» снимает его ForgetFileNames.

// WithAuto — тело броска body и автоимена новых фото одним шагом правки (зовёт ядро внутри
// Editor.try): новые — фото из photos, чьего файла до body не было ни в одном месте листа (перенос
// между листами и копии имён не берут). Возвращает ID фото, получивших имя; ошибка body — nil и она.
func WithAuto(p *model.Project, photos []model.PhotoInfo, body func() error) ([]string, error) {
	var fresh []string
	files := p.PlacedFiles()
	for _, ph := range photos {
		// Копия (098) имя наследует; файл, стоящий в альбоме другой записью, — не новое фото.
		if !model.IsCopyReq(ph.ID) && !files[model.BaseID(ph.ID)] {
			fresh = append(fresh, ph.ID)
		}
	}
	if err := body(); err != nil {
		return nil, err
	}
	return AutoFromFiles(p, fresh), nil
}

// AutoFromFiles — имя из названия файла (LooksLikeName) фото ids, которые стоят на листе, где
// шаблон подписи их места содержит поля имени; у фото нет своей подписи, своего имени и
// строгого имени из файла (его и так видно), а место без таблички с именем (071: её и видно —
// автоимя скрытым всплыло бы при переносе). Групповое фото виньетки имени не получает (как в
// окне «Имена»). Возвращает ID получивших имя по порядку ids; никому — nil (в JSON [] ставит App).
func AutoFromFiles(p *model.Project, ids []string) (named []string) {
	if len(ids) == 0 {
		return nil
	}
	tpl := autoTemplates(p)
	for _, id := range ids {
		r, ok := p.Photos[id]
		t, on := tpl[id]
		if !ok || !on || r.Caption != nil || r.Person != nil && r.Person.Named() || model.PersonFromFile(r.Path) != nil {
			continue
		}
		if !hasNameField(t) {
			continue
		}
		if _, pl := p.PlaceOf(id); pl != nil && pl.Person.Named() {
			continue
		}
		who := model.LooksLikeName(r.Path)
		if who == nil {
			continue
		}
		if r.Person != nil {
			who.Subject = r.Person.Subject // свой предмет без имени (049) остаётся
		}
		r.Person = who
		p.Photos[id] = r
		named = append(named, id)
	}
	return named
}

// autoTemplates — шаблон подписи места каждого фото листов (с обложкой): у учителя виньетки —
// свой (Vig.TText), иначе шаблон листа; групповое и лист без шаблона — нет записи.
func autoTemplates(p *model.Project) map[string]string {
	out := map[string]string{}
	for _, s := range p.Sheets() {
		if s.Caps == nil {
			continue
		}
		vig := s.IsVignette()
		roles := map[string]string{}
		for _, pl := range s.Grid {
			if pl.Photo != "" {
				roles[pl.Photo] = pl.Role
			}
		}
		for _, id := range s.Photos {
			role := roles[id]
			switch {
			case vig && role == model.RoleGroup:
				continue
			case vig && role == model.RoleTeacher && s.Vig.TText != "":
				out[id] = s.Vig.TText
			default:
				out[id] = s.Caps.Text
			}
		}
	}
	return out
}

// hasNameField — в шаблоне есть фамилия, имя или отчество (один «Предмет» имени не покажет).
func hasNameField(tpl string) bool {
	for _, f := range model.NameFieldsOf(tpl) {
		if f != "subject" {
			return true
		}
	}
	return false
}

// ForgetFileNames — «Убрать имена» (060): у фото ids снимается имя, если оно всё ещё равно
// автоимени LooksLikeName (своё, набранное после броска, остаётся). Свой предмет (049)
// остаётся без имени. Неизвестные ID и фото без автоимени — холостая.
func ForgetFileNames(p *model.Project, ids []string) {
	for _, id := range ids {
		r, ok := p.Photos[id]
		if !ok || r.Person == nil {
			continue
		}
		auto := model.LooksLikeName(r.Path)
		if auto == nil {
			continue
		}
		auto.Subject = r.Person.Subject
		if *auto != *r.Person {
			continue
		}
		r.Person = nil
		if auto.Subject != "" {
			r.Person = &model.Person{Subject: auto.Subject}
		}
		p.Photos[id] = r
	}
}
