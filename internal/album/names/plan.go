package names

import (
	"slices"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// maxLeft — строк без фото в плане: окно показывает первые 5, остальное — числом (Extra).
const maxLeft = 50

// NameCaption — шаблон «Подписать фото» на листе без своего шаблона (043 сц. 8а, решение 16).
const NameCaption = "{Фамилия} {Имя}"

// Причины строки без фото (NamesLeft.Reason).
const (
	leftExtra  = "extra"  // мест не хватило
	leftNoFile = "nofile" // по столбцу «Файл» фото нет
	leftMany   = "many"   // по столбцу «Файл» подходит несколько фото
	leftDup    = "dup"    // фото уже занято строкой выше
	leftNoSeat = "noseat" // учителю не хватило места учителя
)

// Plan — кому какое имя ляжет: предпросмотр окна и план вставки — одна функция, поэтому
// показанное и вставленное не расходятся. Проект не меняется.
//
// Места: у листа «Авто» — фото по порядку Photos, у листа с местами (виньетка, ручная сетка) —
// все места по порядку Grid, и пустые (071: имя ляжет табличкой и дождётся фото); групповое
// имени не получает (строка с No = 0). Имена: в режиме «Имена файлов» — из названия фото
// (PersonFromFileLoose); со столбцом «Файл» — по имени файла (match.go), пустые места не
// участвуют; иначе по порядку, а на виньетке с учителями в списке — учителя на места учителей,
// остальные — на места учеников. «Авто» без фото — строк нет, весь список в Left.
func Plan(p *model.Project, spreadID string, r Roster) (model.NamesPlan, error) {
	s := p.Sheet(spreadID) // и обложка (032 §1)
	if s == nil {
		return model.NamesPlan{}, errNoSpread
	}
	vig := s.IsVignette()
	plan := model.NamesPlan{Rows: seatRows(s, vig), Left: []model.NamesLeft{}, Cols: r.Cols, Ignored: r.Ignored,
		ByFile: r.ByFile, Order: r.Order}
	// Подписывать нечего — и шаблон не нужен: вставка на лист без мест и фото холостая (071-11).
	plan.NeedCaption = !vig && len(plan.Rows) > 0 && (s.Caps == nil || s.Caps.Text == "")
	if plan.Ignored == nil {
		plan.Ignored = []string{}
	}
	// Имя ляжет табличкой места (071-4); из названия файла — свойство фото (071-5).
	for i := range plan.Rows {
		plan.Rows[i].Seat = plan.Rows[i].Place != "" && !r.Files && !r.ByFile
	}
	if vig {
		// Роли мест — для подсказки окна о нехватке мест учителя или учеников.
		for _, pl := range s.Grid {
			plan.TeacherSeats += b2i(model.RoleOf(pl) == model.RoleTeacher)
			plan.PupilSeats += b2i(model.RoleOf(pl) == model.RolePupil)
		}
	}
	for _, e := range r.People {
		if e.Teacher {
			plan.Teachers++
		} else {
			plan.Pupils++
		}
	}
	var all []model.NamesLeft
	left := func(e Entry, reason string) {
		all = append(all, model.NamesLeft{Line: e.Line, Who: e.Who, Reason: reason, File: e.File})
	}
	switch {
	case r.Files:
		for i, row := range plan.Rows {
			if ref, ok := p.Photos[row.Photo]; ok && row.No > 0 {
				plan.Rows[i].Who = model.PersonFromFileLoose(ref.Path, r.FirstFirst)
			}
		}
	case r.ByFile:
		byFile(p, plan.Rows, r.People, left)
	case vig && plan.Teachers > 0:
		fill(plan.Rows, r.People, func(row model.NamesRow) bool { return row.Tag == tagTeacher },
			func(e Entry) bool { return e.Teacher }, leftNoSeat, left)
		fill(plan.Rows, r.People, func(row model.NamesRow) bool { return row.No > 0 && row.Tag != tagTeacher },
			func(e Entry) bool { return !e.Teacher }, leftExtra, left)
	default:
		fill(plan.Rows, r.People, func(row model.NamesRow) bool { return row.No > 0 },
			func(Entry) bool { return true }, leftExtra, left)
	}
	countSeats(p, s, &plan)
	// Учителя и ученики раскладываются двумя очередями: строки без фото — снова по порядку поля.
	slices.SortStableFunc(all, func(a, b model.NamesLeft) int { return a.Line - b.Line })
	plan.Extra = len(all)
	plan.Left = append(plan.Left, all[:min(len(all), maxLeft)]...)
	return plan, nil
}

// countSeats — итог плана: мест, получающих имя (групповое не в счёт, фото не из альбома —
// тоже), из них с именем после вставки — строка списка или видимое имя (табличка пустого
// места, имя фото на месте, 071), и без имени.
func countSeats(p *model.Project, s *model.Spread, plan *model.NamesPlan) {
	for _, row := range plan.Rows {
		if row.No == 0 {
			continue
		}
		var pl *model.Place
		if row.Place != "" {
			_, pl = s.FindPlace(row.Place)
		}
		ref, ok := p.Photos[row.Photo]
		switch {
		case row.Photo == "" && pl != nil:
			plan.Seats++
			plan.Named += b2i(row.Who != nil || pl.Person.Named())
		case ok:
			plan.Seats++
			plan.Named += b2i(row.Who != nil || named(ref, pl))
		}
	}
	plan.Missing = plan.Seats - plan.Named
}

// b2i — 1 за true: счётчики плана.
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Подписи мест виньетки — как в окне до 043 (slotsOf в TS).
const (
	tagTeacher = "учитель"
	tagGroup   = "групповое"
)

// seatRows — строки предпросмотра по местам листа, без имён: «Авто» — по фото, лист с местами —
// по местам, и пустым.
func seatRows(s *model.Spread, vig bool) []model.NamesRow {
	rows := []model.NamesRow{}
	if len(s.Grid) == 0 {
		for i, id := range s.Photos {
			rows = append(rows, model.NamesRow{Photo: id, No: i + 1})
		}
		return rows
	}
	no, pupil := 0, 0
	for _, pl := range s.Grid {
		if pl == nil {
			continue
		}
		row := model.NamesRow{Place: pl.ID, Photo: pl.Photo}
		if !vig {
			no++ // ручная сетка: ролей нет, места по порядку
			row.No = no
			rows = append(rows, row)
			continue
		}
		switch model.RoleOf(pl) {
		case model.RoleGroup:
			row.Tag = tagGroup
		case model.RoleTeacher:
			no++
			row.No, row.Tag = no, tagTeacher
		default:
			no++
			pupil++
			row.No, row.Tag = no, "ученик "+strconv.Itoa(pupil)
		}
		rows = append(rows, row)
	}
	return rows
}

// fill — строки people, подходящие who, по порядку на места rows, подходящие seat;
// не хватило мест — в left с причиной reason.
func fill(rows []model.NamesRow, people []Entry, seat func(model.NamesRow) bool, who func(Entry) bool,
	reason string, left func(Entry, string)) {
	i := 0
	for _, e := range people {
		if !who(e) {
			continue
		}
		for i < len(rows) && !seat(rows[i]) {
			i++
		}
		if i == len(rows) {
			left(e, reason)
			continue
		}
		put(&rows[i], e)
		i++
	}
}

// named — у фото r на месте pl видно имя (табличка 071, своё или из названия файла); предмет
// без имени — не имя (049).
func named(r model.PhotoRef, pl *model.Place) bool {
	who, _, _ := model.NameAt(r, pl)
	return who != nil && who.Named()
}

// put — строка e ложится на место row.
func put(row *model.NamesRow, e Entry) {
	who := e.Who
	row.Who, row.Line = &who, e.Line
}

// Apply — вставка по плану Plan того же проекта: имена в таблички мест (Seat) или в фото и, если
// просили и листу нужно (NeedCaption), стиль подписей листа с шаблоном NameCaption — иначе
// вставленных имён не видно. Табличка снимает своё имя фото на этом месте (071-4,
// dropOwnName); имя в фото снимает табличку его места, иначе его не видно (071-5). Фото не из
// альбома пропускаются. Тем же шагом отмены, что и Plan (Editor.PasteNames).
func Apply(p *model.Project, spreadID string, plan model.NamesPlan, caption bool) {
	s := p.Sheet(spreadID)
	for _, row := range plan.Rows {
		if row.Who == nil {
			continue
		}
		var pl *model.Place
		if s != nil && row.Place != "" {
			_, pl = s.FindPlace(row.Place)
		}
		if row.Seat {
			if pl != nil {
				pl.Person = model.ClampSeat(*row.Who)
				dropOwnName(p, pl.Photo)
			}
			continue
		}
		if r, ok := p.Photos[row.Photo]; ok {
			r.Person = model.ClampPerson(*row.Who)
			p.Photos[row.Photo] = r
			if pl != nil {
				pl.Person = model.Person{}
			}
		}
	}
	if !caption || !plan.NeedCaption || s == nil {
		return
	}
	c := model.SheetCaps{Style: model.NewCaptionStyle(p.Settings.PageH), Pad: model.DefaultPad}
	if s.Caps != nil {
		c = *s.Caps // стиль листа без шаблона остаётся, добавляется шаблон
	}
	c.Text = NameCaption
	SetSheetCaps(p, spreadID, true, model.ClampSheetCaps(c))
}

// dropOwnName — своё имя фото id снимается целиком, с предметом: под табличкой его не видно, а
// «всплыло» бы оно, когда фото уйдёт с места (071-4); предмет без столбца «Предмет» список
// снимает, как в 043 (0.12 №175) — свой предмет фото проступил бы под табличкой без предмета.
func dropOwnName(p *model.Project, id string) {
	if r, ok := p.Photos[id]; ok && r.Person != nil {
		r.Person = nil
		p.Photos[id] = r
	}
}
