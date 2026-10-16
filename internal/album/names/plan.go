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
// Места — фото листа: у обычного — по порядку Photos, у виньетки — места сетки с фото по
// порядку Grid; групповое имени не получает (строка с No = 0). Имена: в режиме «Имена файлов» —
// из названия фото (PersonFromFileLoose); со столбцом «Файл» — по имени файла (match.go);
// иначе по порядку, а на виньетке с учителями в списке — учителя на места учителей,
// остальные — на места учеников.
func Plan(p *model.Project, spreadID string, r Roster) (model.NamesPlan, error) {
	s := p.Sheet(spreadID) // и обложка (032 §1)
	if s == nil {
		return model.NamesPlan{}, errNoSpread
	}
	vig := s.IsVignette()
	plan := model.NamesPlan{Rows: seatRows(s, vig), Left: []model.NamesLeft{}, Cols: r.Cols, Ignored: r.Ignored,
		ByFile: r.ByFile, Order: r.Order, NeedCaption: !vig && (s.Caps == nil || s.Caps.Text == "")}
	if plan.Ignored == nil {
		plan.Ignored = []string{}
	}
	if vig {
		// Rows — только места с фото; окну для подсказки нужны и пустые места.
		for _, pl := range s.Grid {
			switch model.RoleOf(pl) {
			case model.RoleTeacher:
				plan.TeacherSeats++
			case model.RolePupil:
				plan.PupilSeats++
			}
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
	for _, row := range plan.Rows {
		ref, ok := p.Photos[row.Photo]
		if !ok || row.No == 0 {
			continue // групповое и фото не из альбома имени не получают
		}
		plan.Seats++
		if row.Who != nil || named(ref) {
			plan.Named++
		}
	}
	plan.Missing = plan.Seats - plan.Named
	// Учителя и ученики раскладываются двумя очередями: строки без фото — снова по порядку поля.
	slices.SortStableFunc(all, func(a, b model.NamesLeft) int { return a.Line - b.Line })
	plan.Extra = len(all)
	plan.Left = append(plan.Left, all[:min(len(all), maxLeft)]...)
	return plan, nil
}

// Подписи мест виньетки — как в окне до 043 (slotsOf в TS).
const (
	tagTeacher = "учитель"
	tagGroup   = "групповое"
)

// seatRows — строки предпросмотра по местам листа, без имён.
func seatRows(s *model.Spread, vig bool) []model.NamesRow {
	rows := []model.NamesRow{}
	if !vig {
		for i, id := range s.Photos {
			rows = append(rows, model.NamesRow{Photo: id, No: i + 1})
		}
		return rows
	}
	no, pupil := 0, 0
	for _, pl := range s.Grid {
		if pl.Photo == "" {
			continue
		}
		row := model.NamesRow{Photo: pl.Photo}
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

// named — у фото есть имя (своё или из названия файла); предмет без имени — не имя (049).
func named(r model.PhotoRef) bool {
	who, _ := r.Name()
	return who != nil && who.Named()
}

// put — строка e ложится на место row.
func put(row *model.NamesRow, e Entry) {
	who := e.Who
	row.Who, row.Line = &who, e.Line
}

// Apply — вставка по плану Plan того же проекта: имена в фото и, если просили и листу нужно
// (NeedCaption), стиль подписей листа с шаблоном NameCaption — иначе вставленных имён не видно.
// Фото не из альбома пропускаются. Тем же шагом отмены, что и Plan (Editor.PasteNames).
func Apply(p *model.Project, spreadID string, plan model.NamesPlan, caption bool) {
	for _, row := range plan.Rows {
		if r, ok := p.Photos[row.Photo]; ok && row.Who != nil {
			r.Person = model.ClampPerson(*row.Who)
			p.Photos[row.Photo] = r
		}
	}
	s := p.Sheet(spreadID)
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
