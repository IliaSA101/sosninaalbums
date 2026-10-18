package vigsheet_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Виньетка (031 G5): «Применить», пересборка при смене подписей и формата, сортировка,
// шапка, превью. Генератор мест — пакет vignette (там геометрия шаблонов).

// classPhotos — n фото 3:4 с именем в имени файла: «Фама Имяа Отча.jpg»… (с отчеством:
// у подписи учителя оно в шаблоне, пустое поле на холсте — «[Отчество]»).
func classPhotos(n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, n)
	for i := range n {
		s := string(rune('а' + i%32))
		out[i] = model.PhotoInfo{ID: fmt.Sprintf("k%02d", i), Path: edtest.Abs("Фам" + s + " Имя" + s + " Отч" + s + ".jpg"), W: 300, H: 400}
	}
	return out
}

// vigSheet — лист 204×304 с виньеткой по умолчанию; фото заполняют места по порядку.
func vigSheet(t *testing.T, photos int) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	if photos > 0 {
		e.FillPlaces(classPhotos(photos), sid)
	}
	return e, sid
}

// rolePhotos — фото мест роли role по порядку сетки.
func rolePhotos(e *album.Editor, i int, role string) []string {
	var out []string
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		if model.RoleOf(pl) == role && pl.Photo != "" {
			out = append(out, pl.Photo)
		}
	}
	return out
}

func vigOf(e *album.Editor, i int) model.Vignette { return *edtest.P(e).Spreads[i].Vig }

// noOverflow — ни у одной подписи вида нет «!».
func noOverflow(t *testing.T, v model.View, what string) {
	t.Helper()
	n := 0
	for _, sv := range v.Spreads {
		for _, c := range sv.Captions {
			n++
			if c.Overflow > 0 {
				t.Fatalf("%s: подпись %q не помещается (%v мм)", what, c.Text, c.Overflow)
			}
		}
	}
	if n == 0 {
		t.Fatalf("%s: подписей нет", what)
	}
}

func TestVignetteApplyKeepsOrder(t *testing.T) {
	e := edtest.Open(t, model.NewSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	ph := classPhotos(25)
	edtest.AddPhotos(e, sid, ph, -1)
	want := make([]string, len(ph))
	for i, p := range ph {
		want[i] = p.ID
	}
	// «Сделать виньеткой»: фото «Авто» по порядку — в места учеников.
	_, left, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 25}, nil)
	if err != nil || left != 0 || !slices.Equal(rolePhotos(e, 0, model.RolePupil), want) {
		t.Fatalf("сделать виньеткой: %v, left %d, ученики %v", err, left, rolePhotos(e, 0, model.RolePupil))
	}
	s := edtest.P(e).Spreads[0]
	if s.Kind != model.KindVignette || len(s.Grid) != 26 || s.Caps == nil || s.Caps.Text != model.PupilCaption ||
		s.Vig.TText != model.TeacherCaption {
		t.Fatalf("виньетка %q, мест %d, подписи %+v, %+v", s.Kind, len(s.Grid), s.Caps, s.Vig)
	}
	ids := edtest.PlaceIDsOf(e)
	_, left, err = e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 20}, nil)
	if err != nil || left != 5 || !slices.Equal(rolePhotos(e, 0, model.RolePupil), want[:20]) ||
		!slices.Equal(edtest.P(e).Spreads[0].Photos, want[:20]) {
		t.Fatalf("25 → 20: %v, left %d, ученики %v", err, left, rolePhotos(e, 0, model.RolePupil))
	}
	// ID мест прежние по ролям: 20 первых учеников и учитель.
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got[:20], ids[:20]) || got[20] != ids[25] {
		t.Fatalf("ID мест %v, были %v", got, ids)
	}
}

func TestVignetteUndoOneStep(t *testing.T) {
	e, sid := vigSheet(t, 25)
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	// Было 24 ученика и учитель: 4 ученика уходят, учитель остаётся, групповое пустое.
	if _, left, err := e.SetVignette(sid, model.Vignette{Tpl: "g-bottom", Teachers: 2, Pupils: 20}, nil); err != nil || left != 4 {
		t.Fatalf("применить: %v, left %d", err, left)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, было %d", edtest.UndoLen(e), undo)
	}
	// Повтор с теми же параметрами — холостая правка: ни шага, ни rev.
	rev := edtest.Rev(e)
	if _, left, err := e.SetVignette(sid, vigOf(e, 0), nil); err != nil || left != 0 {
		t.Fatalf("повтор: %v, left %d", err, left)
	}
	edtest.Same(t, e, rev, undo+1, "повтор «Применить»")
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена не вернула виньетку как была")
	}
}

func TestVignetteLimit(t *testing.T) {
	e, sid := vigSheet(t, 3)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, v := range []model.Vignette{
		{Tpl: "grid", Teachers: 18, Pupils: 64},
		{Tpl: "g-top", Teachers: 0, Pupils: 64}, // 64 + групповое
		{Tpl: "t-left", Teachers: 1, Pupils: 64},
	} {
		if _, _, err := e.SetVignette(sid, v, nil); err == nil || !strings.Contains(err.Error(), "не больше 64 мест") {
			t.Fatalf("%+v: ошибка %v", v, err)
		}
		edtest.Same(t, e, rev, undo, "больше 64 мест")
	}
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "grid", Pupils: 64}, nil); err != nil || len(edtest.P(e).Spreads[0].Grid) != 64 {
		t.Fatalf("ровно 64: %v", err)
	}
}

func TestVignetteRelayOnCaps(t *testing.T) {
	e, sid := vigSheet(t, 25)
	caps := *edtest.P(e).Spreads[0].Caps
	caps.Style.Size = 8
	e.SetSheetCaps(sid, true, caps)
	caps.Style.Size = 12
	undo := edtest.UndoLen(e)
	v := e.SetSheetCaps(sid, true, caps)
	noOverflow(t, v, "кегль 12")
	// Подпись учителя — по своему шаблону (с отчеством), ученика — по шаблону листа.
	for _, c := range v.Spreads[0].Captions {
		if teacher := c.Photo == "k24"; teacher != strings.Contains(c.Text, "Отч") {
			t.Fatalf("подпись %s: %q", c.Photo, c.Text)
		}
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatal("пересборка — не тем же шагом отмены")
	}
	// Места = план нового стиля: «Применить» с теми же параметрами — холостая.
	rev := edtest.Rev(e)
	e.SetVignette(sid, vigOf(e, 0), nil)
	edtest.Same(t, e, rev, undo+1, "места после смены кегля")

	// Сдвинули место руками — пересборки нет, места как были.
	pl := *edtest.P(e).Spreads[0].Grid[3]
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 2, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	was := slices.Clone(edtest.P(e).Spreads[0].Grid)
	keep := make([]model.Place, len(was))
	for i, p := range was {
		keep[i] = *p
	}
	caps.Style.Size = 16
	e.SetSheetCaps(sid, true, caps)
	for i, p := range edtest.P(e).Spreads[0].Grid {
		if *p != keep[i] {
			t.Fatalf("тронутая виньетка пересобрана: место %d %+v, было %+v", i, *p, keep[i])
		}
	}
}

func TestVignetteRelayOnFormat(t *testing.T) {
	e, sid := vigSheet(t, 25)
	ids := edtest.PlaceIDsOf(e)
	w0 := edtest.P(e).Spreads[0].Grid[0].W
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	undo := edtest.UndoLen(e)
	noOverflow(t, e.SetSettings(st), "лист 304×404")
	if edtest.UndoLen(e) != undo+1 || !slices.Equal(edtest.PlaceIDsOf(e), ids) {
		t.Fatalf("шагов %d, ID мест %v → %v", edtest.UndoLen(e)-undo, ids, edtest.PlaceIDsOf(e))
	}
	// План нового формата, без двойного масштаба: повтор «Применить» — холостая.
	rev := edtest.Rev(e)
	e.SetVignette(sid, vigOf(e, 0), nil)
	edtest.Same(t, e, rev, undo+1, "места после смены формата")
	if w := edtest.P(e).Spreads[0].Grid[0].W; !(w > w0) {
		t.Fatalf("портрет %v → %v: лист вырос, а портрет нет", w0, w)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Пересборка нетронутой виньетки (смена формата, стиля подписей) удалённую шапку не
// возвращает: её создаёт только «Применить».
func TestVignetteRelayKeepsDeletedHead(t *testing.T) {
	e, sid := vigSheet(t, 25)
	s := edtest.P(e).Spreads[0]
	e.DeleteText(s.Vig.Head)
	if s := edtest.P(e).Spreads[0]; s.Vig.Head != "" || len(s.Texts) != 0 {
		t.Fatalf("шапка не удалена: %q, надписей %d", s.Vig.Head, len(s.Texts))
	}
	w0 := edtest.P(e).Spreads[0].Grid[0].W
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	e.SetSettings(st)
	caps := *edtest.P(e).Spreads[0].Caps
	caps.Style.Size++
	e.SetSheetCaps(sid, true, caps)
	s = edtest.P(e).Spreads[0]
	if s.Vig.Head != "" || len(s.Texts) != 0 {
		t.Fatalf("пересборка вернула шапку: %q, надписей %d", s.Vig.Head, len(s.Texts))
	}
	if !(s.Grid[0].W > w0) {
		t.Fatalf("пересборки не было: портрет %v → %v", w0, s.Grid[0].W)
	}
}

func TestVignettePadOverGap(t *testing.T) {
	e, sid := vigSheet(t, 0)
	caps := *edtest.P(e).Spreads[0].Caps
	caps.Pad = 4 // промежуток листа — 3 мм
	e.SetSheetCaps(sid, true, caps)
	for _, c := range e.VignetteChoices(sid, 2, 25, "3:4", nil) {
		if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: c.ID, Teachers: 2, Pupils: 25}, nil); err != nil {
			t.Fatal(err)
		}
		e.FillPlaces(classPhotos(28), sid)
		noOverflow(t, e.View(), c.ID)
		e.RemovePhotos(edtest.P(e).Spreads[0].Photos)
	}
}

func TestVignetteSort(t *testing.T) {
	e, sid := vigSheet(t, 0)
	e.SetVignette(sid, model.Vignette{Tpl: "t-left", Teachers: 1, Pupils: 6}, nil)
	ph := make([]model.PhotoInfo, 7)
	for i := range ph {
		ph[i] = model.PhotoInfo{ID: fmt.Sprintf("s%d", i), Path: edtest.Abs(fmt.Sprintf("DSC_%04d.jpg", i)), W: 300, H: 400}
	}
	e.FillPlaces(ph, sid) // ученики s0…s5, учитель s6
	for id, who := range map[string]model.Person{"s0": {Last: "Яковлев", First: "Иван"}, "s2": {Last: "ёжиков", First: "Пётр"},
		"s3": {Last: "Ежов", First: "Анна"}, "s4": {Last: "абрамов", First: "Б"}, "s6": {Last: "Ааа"}} {
		e.SetPerson(id, who, "")
	}
	undo := edtest.UndoLen(e)
	e.SortVignette(sid)
	want := []string{"s4", "s2", "s3", "s0", "s1", "s5"}
	if got := rolePhotos(e, 0, model.RolePupil); !slices.Equal(got, want) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("ученики %v, ждали %v", got, want)
	}
	if got := rolePhotos(e, 0, model.RoleTeacher); !slices.Equal(got, []string{"s6"}) {
		t.Fatalf("учитель сдвинулся: %v", got)
	}
	rev := edtest.Rev(e)
	e.SortVignette(sid)
	edtest.Same(t, e, rev, undo+1, "повторная сортировка")
}

func TestVignetteHead(t *testing.T) {
	e, sid := vigSheet(t, 0)
	head := func() *model.TextBlock {
		s := edtest.P(e).Spreads[0]
		for _, tb := range s.Texts {
			if tb.ID == s.Vig.Head {
				return tb
			}
		}
		return nil
	}
	h := head()
	if h == nil || h.Text != model.VigHeadText || h.Auto || h.W <= 0 {
		t.Fatalf("шапка %+v", h)
	}
	e.SetText(h.ID, "Школа № 1", "")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetVignette(sid, vigOf(e, 0), nil)
	edtest.Same(t, e, rev, undo, "«Применить» с правленой шапкой")
	if head().Text != "Школа № 1" {
		t.Fatal("«Применить» сменило текст шапки")
	}
	// Шаблон без шапки: надпись остаётся и не двигается.
	at := *head()
	e.SetVignette(sid, model.Vignette{Tpl: "grid", Teachers: 1, Pupils: 24}, nil)
	if h := head(); h == nil || *h != at {
		t.Fatalf("шапка у шаблона без шапки: %+v, была %+v", h, at)
	}
	// Удалённая шапка при следующем «Применить» появляется снова.
	e.DeleteText(at.ID)
	if edtest.P(e).Spreads[0].Vig.Head != "" {
		t.Fatal("ссылка на удалённую шапку осталась")
	}
	e.SetVignette(sid, model.DefaultVignette(), nil)
	if h := head(); h == nil || h.ID == at.ID || h.Text != model.VigHeadText {
		t.Fatalf("новая шапка %+v", h)
	}
}

func TestVignetteChoicesFast(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, nil)
	sid := edtest.P(e).Spreads[0].ID
	var cs []model.VigChoice
	d := testkit.Fastest(7, func() { cs = e.VignetteChoices(sid, 4, 60, "3:4", nil) })
	// Предел ×50 над обычным временем: страж от квадратичного роста, а не от шума нагрузки.
	// 049: 33 превью ~0,5 мс; 5 мс из спеки не взяты — на 5 мс всплеск нагрузки ронял тест.
	if lim := testkit.Limit(25 * time.Millisecond); d > lim {
		t.Fatalf("33 превью за %v, предел %v", d, lim)
	}
	if len(cs) != 33 {
		t.Fatalf("превью %d", len(cs))
	}
	for _, c := range cs {
		if c.Over != model.VigGroup(c.ID) || len(c.Boxes) != len(c.Roles) || len(c.Boxes) != 64+btoi(c.Over) || c.PupilW <= 0 {
			t.Fatalf("%s: over %v, мест %d, портрет %v", c.ID, c.Over, len(c.Boxes), c.PupilW)
		}
		for _, b := range c.Boxes {
			if b.X < 0 || b.Y < 0 || b.X+b.W > 1+1e-9 || b.Y+b.H > 1+1e-9 {
				t.Fatalf("%s: доли вне листа %+v", c.ID, b)
			}
		}
	}
	sh, ssid := vigSheet(t, 0)
	if cs := sh.VignetteChoices(ssid, 1, 24, "1:1", nil); len(cs) != 29 || cs[0].Small {
		t.Fatalf("превью листа %d", len(cs))
	}
	if cs := sh.VignetteChoices("нет", 1, 24, "", nil); cs != nil {
		t.Fatal("превью неизвестного разворота")
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestVignetteAddClear(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	v, err := e.AddVignette(0)
	if err != nil || len(v.Spreads) != 2 || v.Spreads[0].Kind != model.KindVignette || v.Spreads[0].Vig == nil ||
		v.Spreads[0].Vig.Places != 25 || v.Spreads[0].Vig.Max != model.MaxPlaces || len(v.Spreads[0].Places) != 25 {
		t.Fatalf("новая виньетка: %v, %+v", err, v.Spreads[0].Vig)
	}
	if v.Spreads[0].Places[24].Role != model.RoleTeacher || v.Spreads[0].Places[0].Role != model.RolePupil {
		t.Fatalf("роли в виде %q, %q", v.Spreads[0].Places[0].Role, v.Spreads[0].Places[24].Role)
	}
	sid := v.Spreads[0].ID
	undo := edtest.UndoLen(e)
	v = e.ClearVignette(sid)
	if v.Spreads[0].Kind != "" || v.Spreads[0].Vig != nil || len(v.Spreads[0].Places) != 25 || v.Spreads[0].Places[24].Role != "" ||
		edtest.UndoLen(e) != undo+1 {
		t.Fatalf("обычный разворот: %+v", v.Spreads[0].Vig)
	}
	rev := edtest.Rev(e)
	e.ClearVignette(sid)
	e.SortVignette(sid)
	edtest.Same(t, e, rev, undo+1, "не виньетка")
}

// fakePlan — планировщик с заданным числом мест и шапкой: проверяет защиту тела правки,
// когда план шире лимитов, которые знает панель (VigPlaces считает места по параметрам).
func fakePlan(slots int, head bool) vigsheet.VigPlanner {
	return func(*model.Project, *model.Spread, model.Vignette) vigsheet.VigPlan {
		pl := vigsheet.VigPlan{Slots: make([]vigsheet.VigSlot, slots)}
		for i := range pl.Slots {
			pl.Slots[i] = vigsheet.VigSlot{Box: layout.Rect{X: 10, Y: 10, W: 20, H: 25}, Role: model.RolePupil}
		}
		if head {
			pl.Head = &layout.Rect{X: 10, Y: 2, W: 100, H: 6}
		}
		return pl
	}
}

// TestVignetteBodyGuards — тело правки отказывает без изменений мест: план больше MaxPlaces,
// шапке нет места среди MaxTexts надписей, разворота нет; «Обычный разворот» на несуществующем
// ничего не делает.
func TestVignetteBodyGuards(t *testing.T) {
	p := album.NewVignetteSheet(204, 304)
	s := p.Spreads[0]
	was := slices.Clone(s.Grid)
	if _, err := vigsheet.SetVignette(p, s.ID, model.DefaultVignette(), fakePlan(model.MaxPlaces+1, false), nil, nil); err == nil ||
		!strings.Contains(err.Error(), "не больше 64 мест") {
		t.Fatalf("план шире лимита: %v", err)
	}
	if !slices.Equal(s.Grid, was) {
		t.Fatal("отказ по лимиту поменял места")
	}
	s.Texts = slices.DeleteFunc(s.Texts, func(tb *model.TextBlock) bool { return tb.ID == s.Vig.Head })
	for len(s.Texts) < model.MaxTexts {
		s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Text: "т"})
	}
	s.Vig.Head = ""
	if _, err := vigsheet.SetVignette(p, s.ID, model.DefaultVignette(), fakePlan(3, true), nil, nil); err == nil ||
		!strings.Contains(err.Error(), "шапке нет места") {
		t.Fatalf("шапка при полном листе надписей: %v", err)
	}
	if !slices.Equal(s.Grid, was) || len(s.Texts) != model.MaxTexts {
		t.Fatal("отказ по шапке поменял лист")
	}
	if _, err := vigsheet.SetVignette(p, "нет такого", model.DefaultVignette(), fakePlan(3, false), nil, nil); err == nil {
		t.Fatal("правка несуществующего разворота принята")
	}
	vigsheet.ClearVignette(p, "нет такого")
	if !s.IsVignette() {
		t.Fatal("ClearVignette задел чужой разворот")
	}
}

// TestVignettePasteNames — вставка имён по классу: групповое фото пропускается (031 G5).
func TestVignettePasteNames(t *testing.T) {
	e, sid := vigSheet(t, 0)
	e.SetVignette(sid, model.Vignette{Tpl: "g-top", Pupils: 2}, nil)
	ph := []model.PhotoInfo{edtest.Photo("x1"), edtest.Photo("x2"), edtest.Photo("x3")}
	e.FillPlaces(ph, sid) // ученики x1, x2, групповое x3
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: "Иванов Петя\nПетров Вася\nСидоров Коля"})
	if err != nil || plan.Extra != 1 {
		t.Fatalf("вставка: %v, лишних %d", err, plan.Extra)
	}
	// Имена ложатся табличками мест (071); у группового нет ни таблички, ни своего имени.
	p := edtest.P(e)
	_, pl3 := p.PlaceOf("x3")
	_, pl2 := p.PlaceOf("x2")
	if r := p.Photos["x3"]; r.Person != nil || pl3 == nil || pl3.Person != (model.Person{}) || pl2 == nil || pl2.Person.Last != "Петров" {
		t.Fatalf("имя группового %+v, места %+v %+v", r.Person, pl3, pl2)
	}
}

// TestVignetteInheritsVars — новая виньетка («Новая виньетка», «Сделать виньеткой») берёт
// Заголовок и Год у ближайшей предыдущей виньетки, где они есть (031 dialog §4); подзаголовок
// и руководитель — свои. Виньетка перед ней, повторное «Применить» и свои поля — без наследства.
func TestVignetteInheritsVars(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.NewSpread(e, -1, edtest.Photos("b"))
	if _, err := e.AddVignette(-1); err != nil { // первая: наследовать не у кого
		t.Fatal(err)
	}
	sp := func(i int) *model.Spread { return edtest.P(e).Spreads[i] }
	if sp(2).Vars != nil {
		t.Fatalf("первая виньетка: %+v", sp(2).Vars)
	}
	full := model.Vars{Title: "Гимназия № 1", Subtitle: "11 «А» класс", Year: "2026", Lead: "Иванова И. И."}
	edtest.Change(e, func(p *model.Project) { v := full; p.Spreads[2].Vars = &v })

	undo := edtest.UndoLen(e)
	v, err := e.AddVignette(-1)
	want := model.Vars{Title: full.Title, Year: full.Year}
	if err != nil || sp(3).Vars == nil || *sp(3).Vars != want || v.Spreads[3].Vars != want || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("новая виньетка: %v, %+v, шагов %d", err, sp(3).Vars, edtest.UndoLen(e)-undo)
	}
	if len(v.Spreads[3].Fields) != 8 || len(v.Spreads[0].Fields) != 8 || v.Spreads[0].Vars != (model.Vars{}) {
		t.Fatalf("меню полей: %v / %v, свои у разворота %+v", v.Spreads[3].Fields, v.Spreads[0].Fields, v.Spreads[0].Vars)
	}
	e.Undo() // наследство — в том же шаге, что и виньетка
	if len(edtest.P(e).Spreads) != 3 || *sp(2).Vars != full {
		t.Fatalf("отмена: разворотов %d, поля %+v", len(edtest.P(e).Spreads), sp(2).Vars)
	}

	// «Сделать виньеткой» после виньетки — наследует; перед ней — нет.
	if _, _, err := e.SetVignette(sp(1).ID, model.DefaultVignette(), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.SetVignette(sp(0).ID, model.DefaultVignette(), nil); err != nil {
		t.Fatal(err)
	}
	if sp(0).Vars != nil || sp(1).Vars != nil {
		t.Fatalf("виньетки перед полями: %+v, %+v", sp(0).Vars, sp(1).Vars)
	}
	edtest.NewSpread(e, -1, edtest.Photos("c"))
	if _, _, err := e.SetVignette(sp(3).ID, model.DefaultVignette(), nil); err != nil {
		t.Fatal(err)
	}
	if sp(3).Vars == nil || *sp(3).Vars != want {
		t.Fatalf("«Сделать виньеткой» после виньетки: %+v", sp(3).Vars)
	}
	// Ближайшая предыдущая без Заголовка и Года пропускается.
	edtest.Change(e, func(p *model.Project) { p.Spreads[3].Vars = &model.Vars{Lead: "только свой"} })
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	if sp(4).Vars == nil || *sp(4).Vars != want {
		t.Fatalf("через виньетку без заголовка: %+v", sp(4).Vars)
	}
	// Своё «Применить» на виньетке и её поля не трогает: повтор — холостая правка.
	edtest.Change(e, func(p *model.Project) { p.Spreads[3].Vars = &model.Vars{Year: "2027"} })
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.SetVignette(sp(3).ID, vigOf(e, 3), nil); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "повтор «Применить»")
	if *sp(3).Vars != (model.Vars{Year: "2027"}) {
		t.Fatalf("свои поля после «Применить»: %+v", sp(3).Vars)
	}
	// «Обычный разворот» поля оставляет: шапка-надпись подставляет их и дальше.
	e.ClearVignette(sp(3).ID)
	if sp(3).Vars == nil || sp(3).Vars.Year != "2027" {
		t.Fatalf("после «Обычный разворот»: %+v", sp(3).Vars)
	}
}

// «Отразить» виньетку (жалоба 0.11): шапка отражается вместе с местами — на развороте она
// над левой страницей, и прежде ученики правой страницы уходили под неё. Одна правка, Undo
// возвращает всё; симметричная шапка (страницы, листа) не двигается; чужая область
// холостая.
func TestVignetteMirrorHead(t *testing.T) {
	sp := edtest.Open(t, model.New(), "")
	sv, err := sp.AddVignette(-1)
	if err != nil {
		t.Fatal(err)
	}
	sh, _ := vigSheet(t, 0)
	for _, c := range []struct {
		e         *album.Editor
		id, scope string
		x0, x1, n float64 // шапка до и после; n > 0 — места отражаются в 0…n
	}{
		{sp, sv.Spreads[0].ID, "spread", 12, 312, 600},
		{sp, sv.Spreads[0].ID, "left", 12, 12, 0}, // шапка на всю левую страницу: симметрична в ней
		{sh, edtest.P(sh).Spreads[0].ID, "spread", 8, 8, 204},
	} {
		s := edtest.P(c.e).Spreads[0]
		h := *s.Texts[slices.IndexFunc(s.Texts, func(tb *model.TextBlock) bool { return tb.ID == s.Vig.Head })]
		before := model.Marshal(edtest.P(c.e))
		var places []model.Place // копии: правка меняет места живого проекта
		for _, pl := range s.Grid {
			places = append(places, *pl)
		}
		undo := edtest.UndoLen(c.e)
		c.e.MirrorGrid(c.id, c.scope)
		s = edtest.P(c.e).Spreads[0]
		got := s.Texts[slices.IndexFunc(s.Texts, func(tb *model.TextBlock) bool { return tb.ID == h.ID })]
		if h.X != c.x0 || got.X != c.x1 || got.Y != h.Y || got.W != h.W || got.Text != h.Text || edtest.UndoLen(c.e) != undo+1 {
			t.Fatalf("%s: шапка %v → %v (ждали %v → %v), шагов %d", c.scope, h.X, got.X, c.x0, c.x1, edtest.UndoLen(c.e)-undo)
		}
		hb := model.BoxOf(got).AABB()
		for i, pl := range s.Grid {
			if c.n > 0 && (pl.ID != places[i].ID || pl.X != geom.Round2(c.n-places[i].X-places[i].W)) {
				t.Fatalf("%s: место %d не отражено: %v, было %v", c.scope, i, pl.X, places[i].X)
			}
			if pl.X < hb.X+hb.W && hb.X < pl.X+pl.W && pl.Y < hb.Y+hb.H && hb.Y < pl.Y+pl.H {
				t.Fatalf("%s: место %d под шапкой: %+v, шапка %+v", c.scope, i, *pl, hb)
			}
		}
		if c.e.Undo(); !bytes.Equal(model.Marshal(edtest.P(c.e)), before) {
			t.Fatalf("%s: Undo не вернул разворот", c.scope)
		}
	}
	rev, undo := edtest.Rev(sp), edtest.UndoLen(sp)
	sp.MirrorGrid(sv.Spreads[0].ID, "top")
	edtest.Same(t, sp, rev, undo, "чужая область")

	// Правая страница: сетка её учеников симметрична, а шапка лежит на левой — вне области,
	// не отражается: правка холостая, шапка на месте (иначе «Отразить» правой сдвигало бы чужую шапку).
	s := edtest.P(sp).Spreads[0]
	h := *s.Texts[slices.IndexFunc(s.Texts, func(tb *model.TextBlock) bool { return tb.ID == s.Vig.Head })]
	before := model.Marshal(edtest.P(sp))
	sp.MirrorGrid(sv.Spreads[0].ID, "right")
	edtest.Same(t, sp, rev, undo, "правая страница, шапка на левой")
	s = edtest.P(sp).Spreads[0]
	if got := s.Texts[slices.IndexFunc(s.Texts, func(tb *model.TextBlock) bool { return tb.ID == h.ID })]; *got != h || !bytes.Equal(model.Marshal(edtest.P(sp)), before) {
		t.Fatalf("шапка левой страницы сдвинулась при «Отразить» правой: %+v → %+v", h, *got)
	}
}
