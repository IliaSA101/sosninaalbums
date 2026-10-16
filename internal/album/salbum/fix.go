package salbum

import (
	"maps"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// dedupe: фото стоит в альбоме один раз (остаётся первое вхождение по Sheets: обложка
// первой), ID без записи в photos убирается, повтор ID разворота получает новый; ID
// обложки зарезервирован всегда — разворот файла с ID CoverID получает новый, даже если
// обложки нет (иначе его нашёл бы Sheet, а потом он столкнулся бы с созданной обложкой).
// Место сетки с убранным фото (или вторым вхождением) пустеет, но остаётся. Разворот,
// опустевший от этого и пустой (Spread.empty), удаляется; обложка остаётся.
func dedupe(p *model.Project) {
	seenPhoto, seenSpread := map[string]bool{}, map[string]bool{model.CoverID: true}
	keep := func(s *model.Spread) bool {
		n := len(s.Photos)
		s.Photos = slices.DeleteFunc(s.Photos, func(id string) bool {
			_, ok := p.Photos[id]
			if !ok || seenPhoto[id] {
				return true
			}
			seenPhoto[id] = true
			return false
		})
		model.EmptyGridDupes(s)
		if len(s.Photos) == n {
			return true
		}
		s.Layout, s.Variant = "", 0
		return !s.Empty()
	}
	if p.Cover != nil {
		keep(p.Cover.Sheet)
	}
	out := p.Spreads[:0]
	for _, s := range p.Spreads {
		if s.ID == "" || seenSpread[s.ID] {
			s.ID = model.NewID()
		}
		seenSpread[s.ID] = true
		if keep(s) {
			out = append(out, s)
		}
	}
	clear(p.Spreads[len(out):])
	p.Spreads = out
}

// Rekey приводит ID фото к idFor(путь) (library.IDFor), своего из библиотеки — к
// model.LibPhotoID(ключ): альбом переехал — иначе
// лента не узнала бы фото как использованные; а ID, не равный пути, показывал бы
// на экране одно фото, а в печать отдавал другое. Записи с одним путём сливаются:
// остаётся первая в порядке ленты (с её кадром и подписью), ссылки в разворотах
// переводятся на новый ID (и в местах сетки — renameRefs), повтор фото в альбоме —
// первое вхождение (dedupe; слившийся повтор в месте — пустое место).
func Rekey(p *model.Project, idFor func(string) string) {
	order := make([]string, 0, len(p.Photos))
	seen := make(map[string]bool, len(p.Photos))
	for _, s := range p.Sheets() {
		for _, id := range s.PhotoIDs() {
			if _, ok := p.Photos[id]; ok && !seen[id] {
				seen[id] = true
				order = append(order, id)
			}
		}
	}
	// Записи вне лент в альбоме не нужны (pruneRefs), но порядок держим стабильным.
	for _, id := range slices.Sorted(maps.Keys(p.Photos)) {
		if !seen[id] {
			order = append(order, id)
		}
	}
	photos := make(map[string]model.PhotoRef, len(p.Photos))
	moved := map[string]string{}
	merged := false
	for _, id := range order {
		ref := p.Photos[id]
		var nid string
		if ref.Lib != "" {
			nid = model.LibPhotoID(ref.Lib) // своё: путь к библиотеке на каждом компьютере свой
		} else {
			nid = idFor(ref.Path)
		}
		if _, taken := photos[nid]; taken {
			merged = true
		} else {
			photos[nid] = ref
		}
		if nid != id {
			moved[id] = nid
		}
	}
	p.Photos = photos
	p.RenameRefs(moved)
	if merged {
		dedupe(p)
	}
}

// migrate доводит прочитанный проект до текущей версии. v1 → v2: новые поля
// нулевые, папки-строки разобраны в decode; номер варианта (файлы до ключей)
// переводится в ключ той же раскладки. v2 → v3, v3 → v4, v4 → v5, v5 → v6, v6 → v7,
// v7 → v8: только номер версии (у старых файлов нет мест, зеркала, прозрачности фона, своих
// отметок, больше 20 текстов на развороте, своих шаблонов виньетки и наклона фото фона —
// прежний frame.r фона зажим и так обнулял).
// До v6 (0.10) ничьи раскладок решал порядок перебора по точным битам, с 0.10 — tieQ и
// ключ (решение 95): «Авто» с Layout "" и номер варианта берутся из ряда в прежнем
// порядке (layout.LegacyLayouts), и ключ, отличный от нынешней лучшей, записывается —
// старый альбом выглядит как раньше на той же машине. Номер вне ряда (в том числе
// отрицательный) зажимается к краю ряда.
func migrate(p *model.Project) {
	legacy := p.From() < tiesFrom
	var todo []*model.Spread
	for _, s := range p.Spreads {
		auto := legacy && len(s.Grid) == 0 && len(s.Photos) > 0 && len(s.Photos) < layout.ManyFrom
		if s.Layout == "" && (s.Variant > 0 || auto) {
			todo = append(todo, s)
		}
	}
	p.WarmLayouts(todo)
	for i, r := range migRows(p, todo, legacy) {
		if len(r.old) > 0 && len(r.cur) > 0 {
			s := todo[i]
			s.Layout = r.old[min(max(s.Variant, 0), len(r.old)-1)].Key
			if s.Layout == r.cur[0].Key {
				s.Layout = ""
			}
		}
	}
	for _, s := range p.Spreads {
		s.Variant = 0
	}
	p.Version = model.FormatVersion
}

// tiesFrom — с этой версии файла (0.10) «Авто» с Layout "" — первая в порядке tieQ и ключа.
const tiesFrom = 6

// migRow — ряды разворота для миграции: cur — нынешний (по нему «Авто» = cur[0]), old — из
// которого берётся номер варианта: прежний порядок ничьих у файла до v6, иначе тот же cur.
type migRow struct{ cur, old []layout.Result }

// migRowImpl — счёт рядов разворота; в тестах подменяется, чтобы имитировать панику.
var migRowImpl = func(asp []float64, prm layout.Params, legacy bool) migRow {
	r := migRow{cur: layout.Cached(asp, prm, model.VariantLimit)}
	r.old = r.cur
	if legacy {
		r.old = layout.LegacyLayouts(asp, prm, model.VariantLimit)
	}
	return r
}

// migRows считает ряды разворотов параллельно, как WarmLayouts. Паника в раскладке — в лог,
// ряды разворота пустые (Layout остаётся ""): один сбойный разворот не мешает открыть альбом.
func migRows(p *model.Project, spreads []*model.Spread, legacy bool) []migRow {
	rows := make([]migRow, len(spreads))
	prm := p.Params()
	var pl model.PanicLog
	one := func(i int) {
		s := spreads[i]
		pl.Safely(s.ID, func() { rows[i] = migRowImpl(p.Aspects(s.Photos), prm, legacy) })
	}
	if model.ParallelLayouts() {
		model.Each(len(spreads), one)
	} else {
		for i := range spreads {
			one(i)
		}
	}
	pl.Summary()
	return rows
}

// dropNilPlaces — первый шаг decode: `"grid":[null]` даёт nil в []*Place без
// ошибки разбора, а дальше всё, что трогает места, упало бы.
func dropNilPlaces(p *model.Project) {
	for _, s := range p.Sheets() {
		s.Grid = slices.DeleteFunc(s.Grid, func(pl *model.Place) bool { return pl == nil })
	}
}

// unionGrid — истина при чтении и места, и Photos: фото, стоящее только в месте,
// дописывается в Photos (иначе выпало бы); фото только в Photos получит место
// при починке (syncGrid(true)). До dedupe: повторы она разбирает уже с местами.
func unionGrid(p *model.Project) {
	for _, s := range p.Sheets() {
		if len(s.Grid) == 0 {
			continue
		}
		in := make(map[string]bool, len(s.Photos))
		for _, id := range s.Photos {
			in[id] = true
		}
		for _, pl := range s.Grid {
			if pl.Photo != "" && !in[pl.Photo] {
				in[pl.Photo] = true
				s.Photos = append(s.Photos, pl.Photo)
			}
		}
	}
}

// albumVars — поля альбома из файлов dev-сборок v6 (030, до решения 123): только чтение.
type albumVars struct {
	School  string `json:"school,omitempty"`
	Class   string `json:"class,omitempty"`
	Year    string `json:"year,omitempty"`
	Teacher string `json:"teacher,omitempty"`
}

// moveAlbumVars — поля альбома old в листы (решение 125, номер формата прежний: v6 не
// выпущен): в каждую виньетку без своих полей; виньеток нет — в каждый лист (и обложку), где
// в надписях, шаблонах подписей или своих подписях фото есть поле шапки. Показать значения
// больше негде — остальное отбрасывается. Зажим — один раз до раздачи: сырое поле размером с
// файл в каждом из тысячи листов чистилось бы тысячу раз (минуты на открытие). Виньетка — по
// Kind: FixVignettes ещё впереди (так же считает VarsOf).
func moveAlbumVars(p *model.Project, old *albumVars) {
	if old == nil {
		return
	}
	v := model.ClampVars(model.Vars{Title: old.School, Subtitle: old.Class, Year: old.Year, Lead: old.Teacher})
	if v == nil {
		return
	}
	vig := slices.ContainsFunc(p.Spreads, func(s *model.Spread) bool { return s.Kind == model.KindVignette })
	for _, s := range p.Sheets() {
		if s.Vars != nil || vig && s.Kind != model.KindVignette || !vig && !usesHeadFields(p, s) {
			continue
		}
		c := *v
		s.Vars = &c
	}
}

// usesHeadFields — в текстах листа s есть поле шапки ({Заголовок}… или прежнее имя).
func usesHeadFields(p *model.Project, s *model.Spread) bool {
	for _, t := range s.Texts {
		if t != nil && model.HasHeadFields(t.Text) {
			return true
		}
	}
	if s.Caps != nil && model.HasHeadFields(s.Caps.Text) || s.Vig != nil && model.HasHeadFields(s.Vig.TText) {
		return true
	}
	return slices.ContainsFunc(s.Photos, func(id string) bool {
		c := p.Photos[id].Caption
		return c != nil && model.HasHeadFields(c.Text)
	})
}
