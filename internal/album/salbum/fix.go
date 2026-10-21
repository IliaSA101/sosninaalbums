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
// v7 → v8, v8 → v9, v9 → v10: только номер версии (у старых файлов нет мест, зеркала,
// прозрачности фона, своих отметок, больше 20 текстов на развороте, своих шаблонов виньетки,
// наклона фото фона — прежний frame.r фона зажим и так обнулял, — ключей форм 051: старые ключи
// строятся как раньше, — фото корешка 061: прежняя «Лицевая + задняя» и есть «Задняя + корешок
// + лицевая» с пустым корешком). v10 → v11 — шаги 0.15 (migrateV11), v11 → v12 — 0.16 (migrateV12),
// v12 → v13 — 0.18 (migrateV13), v13 → v14 — 0.20 (migrateV14), v14 → v15 — 0.21 (migrateV15).
// До v6 (0.10) ничьи раскладок решал порядок перебора по точным битам, с 0.10 — tieQ и
// ключ (решение 95): «Авто» с Layout "" и номер варианта берутся из ряда в прежнем
// порядке (layout.LegacyLayouts), и ключ, отличный от нынешней лучшей, записывается —
// старый альбом выглядит как раньше на той же машине. Номер вне ряда (в том числе
// отрицательный) зажимается к краю ряда. Сначала — ключи длиннее предела (dropLongKeys).
// В migrateV11 у файла с layflat последний шаг — pinFoldAuto (064).
func migrate(p *model.Project) {
	dropLongKeys(p)
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
	if p.From() < v11From {
		migrateV11(p)
	}
	if p.From() < v12From {
		migrateV12(p)
	}
	if p.From() < v13From {
		migrateV13(p)
	}
	if p.From() < v14From {
		migrateV14(p)
	}
	if p.From() < v15From {
		migrateV15(p)
	}
	p.Version = model.FormatVersion
}

// migrateV15 — шаги v14 → v15 (общий формат 0.21, номер общий у 082 и 083). Каждая спека 0.21 с
// полями, которым нужна починка старого файла, добавляет сюда свой шаг. 083 (подборки bins,
// членство marks[].bins) — только номер: у старых файлов подборок нет, nil верен.
func migrateV15(*model.Project) {}

// v15From — общий формат 0.21: файлы раньше проходят migrateV15.
const v15From = 15

// migrateV14 — шаги v13 → v14 (общий формат 0.20, номер общий у 078 и 079). Каждая спека 0.20 с
// полями, которым нужна починка старого файла, добавляет сюда свой шаг. 079 (рамочки мест
// grid[].border, прозрачность фото photos.*.fade) — только номер: у старых файлов их нет, нули верны.
func migrateV14(*model.Project) {}

// v14From — общий формат 0.20: файлы раньше проходят migrateV14.
const v14From = 14

// migrateV13 — шаги v12 → v13 (общий формат 0.18). Каждая спека 0.18 с полями, которым нужна
// починка старого файла, добавляет сюда свой шаг. 071 (табличка места grid[].person) — только
// номер: у старых файлов табличек нет, ноль верен.
func migrateV13(*model.Project) {}

// v13From — общий формат 0.18: файлы раньше проходят migrateV13.
const v13From = 13

// migrateV12 — шаги v11 → v12 (общий формат 0.16). Каждая спека 0.16 с полями, которым нужна
// починка старого файла, добавляет сюда свой шаг. 068 (тон фото photos.*.adj) — только номер:
// у старых файлов тона нет, nil верен.
func migrateV12(*model.Project) {}

// v12From — общий формат 0.16: файлы раньше проходят migrateV12.
const v12From = 12

// migrateV11 — шаги v10 → v11 (общий формат 0.15). Каждая спека 0.15 с полями, которым нужна
// починка старого файла, добавляет сюда свой шаг. 066 (обрез settings.bleed, разметка
// разворотов spreadGuide) — только номер: у старых файлов обреза и разметки нет, нули верны.
// 064 (правило сгиба settings.foldAny) — у того же файла, что читал foldLegacy, pinFoldAuto;
// он последний среди шагов раскладок: migRows выше считал «Авто» по-прежнему. Условие —
// foldWas, а не FoldAny: ClampSettings между ними тоже трогает FoldAny.
func migrateV11(p *model.Project) {
	if foldWas(p.From(), p.Settings) {
		pinFoldAuto(p)
	}
}

// dropLongKeys — ключ раскладки длиннее layout.MaxKeyLen не строится никогда (Build
// отказывает до разбора), а строку копирует каждый снимок отмены: при чтении — «Авто».
// Прочие негодные ключи остаются: годность зависит от пропорций и настроек.
func dropLongKeys(p *model.Project) {
	for _, s := range p.Sheets() {
		if len(s.Layout) > layout.MaxKeyLen {
			s.Layout = ""
		}
	}
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
	eachSpread(spreads, func(i int, s *model.Spread) { rows[i] = migRowImpl(p.Aspects(s.Photos), prm, legacy) })
	return rows
}

// eachSpread зовёт f по разворотам параллельно, как WarmLayouts (без параллельности — по
// очереди); паника f — в лог, остальные развороты считаются.
func eachSpread(spreads []*model.Spread, f func(i int, s *model.Spread)) {
	var pl model.PanicLog
	one := func(i int) {
		s := spreads[i]
		pl.Safely(s.ID, func() { f(i, s) })
	}
	if model.ParallelLayouts() {
		model.Each(len(spreads), one)
	} else {
		for i := range spreads {
			one(i)
		}
	}
	pl.Summary()
}

// v11From — общий формат 0.15: с него у layflat-альбома действует правило сгиба (064), файлы
// раньше считали «Авто» как нынешнее «Любые фото» и проходят migrateV11.
const v11From = 11

// foldWas — файл версии from с настройками s считал «Авто» как «Любые фото»: до v11, layflat,
// не лист (у листа шва нет). Одно условие для foldLegacy и migrateV11.
func foldWas(from int, s model.Settings) bool { return from < v11From && s.Layflat && !s.Sheet }

// foldLegacy — настройки файла версии from: у foldWas на время чтения FoldAny = true, чтобы
// migrate и ничьи до v6 считали «Авто» по прежнему поведению; иначе ветка legacy сравнивала бы
// прежний ряд с новой первой (decisions 0.15 №15). Снимает pinFoldAuto.
func foldLegacy(from int, s model.Settings) model.Settings {
	if foldWas(from, s) {
		s.FoldAny = true
	}
	return s
}

// pinFoldAuto — последний шаг раскладок при чтении файла до v11 (decisions 0.15 №2, №15):
// альбом переходит на «Только широкие», а «Авто», у которого первая по правилу меняется,
// закрепляется прежним ключом — свёрстанное не трогается (владелец 2026-10-17). Сетки и ключи,
// выбранные руками, не трогаются; ряды — параллельно, как migRows. Ключ строит Build (он не
// фильтрует), SetLayout пускает его как стоящий.
func pinFoldAuto(p *model.Project) {
	p.Settings.FoldAny = true // прежнее поведение — явно, не по следу foldLegacy
	was := p.Params()
	p.Settings.FoldAny = false
	now := p.Params()
	var todo []*model.Spread
	for _, s := range p.Spreads {
		if len(s.Grid) == 0 && len(s.Photos) > 0 {
			todo = append(todo, s)
		}
	}
	keys := make([]string, len(todo))
	eachSpread(todo, func(i int, s *model.Spread) { keys[i] = pinKey(p.Aspects(s.Photos), s.Layout, was, now) })
	for i, k := range keys {
		if k != "" {
			todo[i].Layout = k
		}
	}
}

// pinKey — ключ, которым закрепить разворот с раскладкой key: прежняя первая (was), если
// нынешняя (now) другая; "" — закреплять нечего. Негодный ключ показывался как «Авто» —
// закрепляется так же.
func pinKey(asp []float64, key string, was, now layout.Params) string {
	if key != "" {
		if _, ok := layout.Build(key, asp, now); ok {
			return ""
		}
	}
	n := model.RowLen(len(asp))
	old, cur := layout.Cached(asp, was, n), layout.Cached(asp, now, n)
	if len(old) == 0 || len(cur) > 0 && cur[0].Key == old[0].Key {
		return ""
	}
	return old[0].Key
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
