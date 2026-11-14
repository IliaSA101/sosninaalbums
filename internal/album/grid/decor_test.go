package grid_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Копировать оформление» (спека 049 Ф2, design §5): режим копирования без мест.

// plate — процедурный стикер «плашка» (растяжимый: рамка остаётся как задана).
func plate(x, y, w, h, rot float64) *model.Sticker {
	return &model.Sticker{ID: model.NewID(), Art: &model.Art{Kind: "plate"}, X: x, Y: y, W: w, H: h, Rot: rot}
}

// putStickers добавляет стикеры на лист sp одной правкой.
func putStickers(e *album.Editor, sp string, sts ...*model.Sticker) {
	edtest.Change(e, func(p *model.Project) {
		_, s := p.Find(sp)
		s.Stickers = append(s.Stickers, sts...)
	})
}

// sheetOf — лист по ID из живого проекта (тест читает между правками).
func sheetOf(e *album.Editor, id string) *model.Spread {
	_, s := edtest.P(e).Find(id)
	return s
}

// onPage — стикеры листа, чей центр на странице page (0 — левая, 1 — правая; страница 281 мм).
func onPage(s *model.Spread, page int) []*model.Sticker {
	var out []*model.Sticker
	for _, st := range s.Stickers {
		if (st.X+st.W/2 >= 281) == (page == 1) {
			out = append(out, st)
		}
	}
	return out
}

// jsonOf — JSON значения (сверка «не тронуто»).
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// decorEd — s1: места a (левая) и b (правая); на левой — цвет, узор, фото фона "bg", текст и
// стикеры k1 (над местами) и k2 (под местами). s2 — «Авто» с фото d, s3 — пустой.
func decorEd(t *testing.T) (e *album.Editor, k1, k2 *model.Sticker) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 127, 127, "b")}, []string{"d"}, []string{})
	e.SetBackground("s1", model.ScopeLeft, "#aabbcc")
	mustPattern(t, e, "s1", model.ScopeLeft, "dots")
	edtest.SetBg(t, e, "s1", model.ScopeLeft, "bg")
	edtest.AddText(t, e, "s1", 0)
	k1, k2 = plate(20, 30, 40, 20, 30), plate(150, 200, 60, 30, 0)
	putStickers(e, "s1", k1, k2)
	edtest.Change(e, func(p *model.Project) { // k2 — под всеми местами (фоновый орнамент)
		s := p.Spreads[0]
		st := slices.DeleteFunc(s.Stack(), func(id string) bool { return id == k2.ID })
		s.SetStack(append([]string{k2.ID}, st...))
	})
	edtest.MustGrid(t, e)
	return e, k1, k2
}

// TestCopyDecor — цвет, узор, фото фона и стикеры левой страницы — на правую того же листа,
// на «Авто» (обе страницы) и на пустой лист одной правкой; стикеры цели заменены, места, фото
// и тексты (без флажка) не тронуты; копии стикеров — новые ID, сдвиг на страницу, порядок слоёв
// источника (под местами — под местами); отмена — одним шагом; круг Save → Load.
func TestCopyDecor(t *testing.T) {
	e, k1, k2 := decorEd(t)
	old := plate(300, 40, 30, 30, 0)
	putStickers(e, "s2", old)
	e.AddText("s2", 1, "")
	s1 := sheetOf(e, "s1")
	grid0, photos0 := jsonOf(t, s1.Grid), slices.Clone(s1.Photos)
	texts2 := jsonOf(t, sheetOf(e, "s2").Texts)
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)

	to := []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}, {Spread: "s2", Scope: model.ScopeSpread}, {Spread: "s3", Scope: model.ScopeLeft}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, false, false); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	s1 = sheetOf(e, "s1")
	if jsonOf(t, s1.Grid) != grid0 || !slices.Equal(s1.Photos, photos0) || len(s1.Texts) != 1 {
		t.Fatalf("места, фото или тексты источника тронуты: %s", model.Marshal(edtest.P(e)))
	}
	if s1.BgR != "#aabbcc" || s1.PatR == nil || s1.PatR.Kind != "dots" || s1.BgPhotoR == nil || s1.BgPhotoR.Photo != s1.BgPhotoL.Photo {
		t.Fatalf("оформление правой: %q %+v %+v", s1.BgR, s1.PatR, s1.BgPhotoR)
	}
	right := onPage(s1, 1)
	if len(right) != 2 || right[0].ID == k1.ID || right[1].ID == k2.ID {
		t.Fatalf("стикеры правой: %+v", right)
	}
	// Порядок источника: k2 под местами, k1 над; копии — так же относительно места b.
	st, b := s1.Stack(), s1.Grid[1].ID
	c2, c1 := right[slices.IndexFunc(right, func(s *model.Sticker) bool { return s.X == k2.X+281 })], right[slices.IndexFunc(right, func(s *model.Sticker) bool { return s.X == k1.X+281 })]
	if slices.Index(st, c2.ID) > slices.Index(st, b) || slices.Index(st, c1.ID) < slices.Index(st, b) || c1.Rot != 30 || c1.Y != k1.Y {
		t.Fatalf("стек %v, копии %+v %+v", st, *c1, *c2)
	}
	s2 := sheetOf(e, "s2")
	if s2.Grid != nil || !slices.Equal(s2.Photos, []string{"d"}) || s2.BgL != "#aabbcc" || s2.BgR != "#aabbcc" || jsonOf(t, s2.Texts) != texts2 {
		t.Fatalf("«Авто» s2: %s", model.Marshal(edtest.P(e)))
	}
	if len(s2.Stickers) != 4 || slices.ContainsFunc(s2.Stickers, func(s *model.Sticker) bool { return s.ID == old.ID }) {
		t.Fatalf("стикеры s2: %+v", s2.Stickers)
	}
	if s3 := sheetOf(e, "s3"); s3.BgL != "#aabbcc" || s3.BgR != "" || len(onPage(s3, 0)) != 2 || len(onPage(s3, 1)) != 0 {
		t.Fatalf("s3: %s", model.Marshal(edtest.P(e)))
	}
	after := model.Marshal(edtest.P(e))
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("одна отмена не вернула всё")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("повтор (Redo) не вернул копию")
	}
	// Круг Save → Load: ID фото меняет Rekey — сверяются цвета, узоры и рамки стикеров.
	q := edtest.SaveLoad(t, e)
	for i, s := range q.Spreads {
		w := edtest.P(e).Spreads[i]
		if s.BgL != w.BgL || s.BgR != w.BgR || kindsOf(s) != kindsOf(w) || (s.BgPhotoR == nil) != (w.BgPhotoR == nil) || len(s.Stickers) != len(w.Stickers) {
			t.Fatalf("лист %d после записи и чтения", i)
		}
		for j, st := range s.Stickers {
			if g := w.Stickers[j]; st.ID != g.ID || st.X != g.X || st.Y != g.Y || st.Rot != g.Rot || st.Art.Kind != g.Art.Kind {
				t.Fatalf("лист %d, стикер %d: %+v, ждали %+v", i, j, *st, *g)
			}
		}
	}
}

// TestCopyDecorTexts — с флажком надписи области цели заменяются (другая страница — нет);
// без флажка — не трогаются. Шапка виньетки — часть виньетки: у цели стоит, с источника не едет.
func TestCopyDecorTexts(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, []string{"d"}, []string{})
	sid := edtest.AddText(t, e, "s1", 0)
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Texts[0].Text = "Школа № 5" }) // не как новый текст цели
	src := edtest.Block(t, e, sid)
	keep := edtest.AddText(t, e, "s2", 0)
	gone := edtest.AddText(t, e, "s2", 1)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}
	plan := e.PlanCopyDecor("s1", model.ScopeLeft, to, true, false)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, false); err != nil {
		t.Fatal(err)
	}
	ts := sheetOf(e, "s2").Texts
	if plan.Texts != 1 || len(ts) != 2 || ts[0].ID != keep || ts[1].ID == gone || ts[1].ID == src.ID || ts[1].X != src.X+281 || ts[1].Text != src.Text {
		t.Fatalf("план %+v, тексты %s", plan, jsonOf(t, ts))
	}
	// Без флажка: надписи цели стоят, источника не едут.
	if _, err := e.CopyDecor("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s3", Scope: model.ScopeSpread}, {Spread: "s2", Scope: model.ScopeLeft}}, false, false); err != nil {
		t.Fatal(err)
	}
	if len(sheetOf(e, "s3").Texts) != 0 || len(sheetOf(e, "s2").Texts) != 2 {
		t.Fatalf("без флажка: %s", model.Marshal(edtest.P(e)))
	}

	// Виньетка — цель: шапка остаётся та же (ID, текст, место), виньетка цела.
	v, err := e.AddVignette(-1)
	if err != nil {
		t.Fatal(err)
	}
	vid := v.Spreads[len(v.Spreads)-1].ID
	vs := sheetOf(e, vid)
	if !vs.IsVignette() || vs.VigHead(vs.Vig.Head) == nil {
		t.Fatal("у новой виньетки нет шапки")
	}
	head, grid0 := *vs.VigHead(vs.Vig.Head), jsonOf(t, vs.Grid)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, []model.GridTarget{{Spread: vid, Scope: model.ScopeSpread}}, true, false); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	vs = sheetOf(e, vid)
	if h := vs.VigHead(vs.Vig.Head); h == nil || *h != head || jsonOf(t, vs.Grid) != grid0 || len(vs.Texts) != 3 {
		t.Fatalf("виньетка после копии: %s", jsonOf(t, vs))
	}
	// Виньетка — источник: шапка не едет, остальные надписи — да.
	if _, err := e.CopyDecor(vid, model.ScopeSpread, []model.GridTarget{{Spread: "s3", Scope: model.ScopeSpread}}, true, false); err != nil {
		t.Fatal(err)
	}
	s3 := sheetOf(e, "s3")
	if len(s3.Texts) != 2 || slices.ContainsFunc(s3.Texts, func(t *model.TextBlock) bool { return t.Text == head.Text }) {
		t.Fatalf("с виньетки: %s", jsonOf(t, s3.Texts))
	}
}

// TestCopyDecorMirror — «Зеркально»: стикеры на странице другой стороны отражаются (x, FlipH,
// угол −Rot), на той же стороне и с источника-листа — как есть; надписи только сдвигаются;
// повтор зеркальной копии — холостой.
func TestCopyDecorMirror(t *testing.T) {
	e, k1, _ := decorEd(t)
	src := sheetOf(e, "s1").Texts[0]
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeLeft}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	find := func(sts []*model.Sticker, y float64) *model.Sticker {
		i := slices.IndexFunc(sts, func(s *model.Sticker) bool { return s.Y == y })
		if i < 0 {
			t.Fatalf("нет стикера с y=%v: %+v", y, sts)
		}
		return sts[i]
	}
	m := find(onPage(sheetOf(e, "s2"), 1), k1.Y) // зеркало левой страницы в правую: x → 562 − x − w
	if m.X != 562-k1.X-k1.W || !m.FlipH || m.Rot != -30 || m.W != k1.W || m.H != k1.H {
		t.Fatalf("отражённый %+v (источник %+v)", *m, *k1)
	}
	if tx := sheetOf(e, "s2").Texts; len(tx) != 1 || tx[0].X != src.X+281 || tx[0].Rot != src.Rot {
		t.Fatalf("надпись сдвигом: %s", jsonOf(t, tx))
	}
	if s := find(onPage(sheetOf(e, "s3"), 0), k1.Y); s.X != k1.X || s.FlipH || s.Rot != 30 {
		t.Fatalf("та же сторона — как есть: %+v", *s)
	}
	// Дробные мм и угол у края: отражённая рамка зажата, как при постановке (инвариант
	// стикеров), повтор и после Save → Load — холостой.
	putStickers(e, "s1", plate(3.37, 150.29, 21.19, 20.13, -12.5))
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, true); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "повтор зеркальной копии")
	f := edtest.Open(t, edtest.SaveLoad(t, e), "")
	rev, undo = edtest.Rev(f), edtest.UndoLen(f)
	if _, err := f.CopyDecor("s1", model.ScopeLeft, to, true, true); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, f, rev, undo, "повтор зеркальной копии после Save → Load")
	// Источник-лист: флажок ничего не отражает.
	if _, err := e.CopyDecor("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s3", Scope: model.ScopeSpread}}, false, true); err != nil {
		t.Fatal(err)
	}
	if s := find(sheetOf(e, "s3").Stickers, k1.Y); s.X != k1.X || s.FlipH {
		t.Fatalf("лист → лист: %+v", *s)
	}
}

// TestCopyDecorAutoSource — источник и цели «Авто» не закрепляются (места от оформления не
// зависят); источник без мест и без фото — не ошибка «нет мест».
func TestCopyDecorAutoSource(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	edtest.NewSpread(e, -1, edtest.Photos("c"))
	v := edtest.NewSpread(e, -1, nil)
	ids := edtest.SpreadIDs(v)
	e.SetBackground(ids[0], model.ScopeSpread, "#102030")
	putStickers(e, ids[0], plate(30, 30, 40, 40, 0))
	if _, err := e.CopyDecor(ids[0], model.ScopeSpread, []model.GridTarget{{Spread: ids[1], Scope: model.ScopeSpread}}, true, false); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	if p.Spreads[0].Grid != nil || p.Spreads[1].Grid != nil || p.Spreads[1].BgL != "#102030" || len(p.Spreads[1].Stickers) != 1 {
		t.Fatalf("«Авто»: %s", model.Marshal(p))
	}
	// Пустой лист без мест — источник.
	e.SetBackground(ids[2], model.ScopeSpread, "#405060")
	if _, err := e.CopyDecor(ids[2], model.ScopeSpread, []model.GridTarget{{Spread: ids[0], Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	if p := edtest.P(e); p.Spreads[0].BgL != "#405060" || p.Spreads[0].Stickers != nil || p.Spreads[0].Grid != nil || !slices.Equal(p.Spreads[0].Photos, []string{"a", "b"}) {
		t.Fatalf("с пустого листа: %s", model.Marshal(p))
	}
	edtest.MustGrid(t, e)
}

// TestCopyDecorNoop — холостые: повтор той же копии (и после Save → Load), цель = источник,
// пустой и nil список целей, неизвестные лист и цель, обложка — ни цель, ни источник. Повтор
// на две цели, из которых одна уже такая же, — шаг только с другой.
func TestCopyDecorNoop(t *testing.T) {
	e, _, _ := decorEd(t)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, false); err != nil {
		t.Fatal(err)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, c := range []struct {
		src, from string
		to        []model.GridTarget
		what      string
	}{
		{"s1", model.ScopeLeft, to, "повтор"},
		{"s1", model.ScopeLeft, []model.GridTarget{{Spread: "s1", Scope: model.ScopeLeft}}, "цель = источник"},
		{"s1", model.ScopeLeft, nil, "nil"},
		{"s1", model.ScopeLeft, []model.GridTarget{}, "пусто"},
		{"s9", model.ScopeLeft, to, "нет источника"},
		{"s1", model.ScopeLeft, []model.GridTarget{{Spread: "s9", Scope: model.ScopeLeft}}, "нет цели"},
		{"s1", model.ScopeLeft, []model.GridTarget{{Spread: model.CoverID, Scope: model.ScopeLeft}}, "обложка — цель"},
		{model.CoverID, model.ScopeLeft, to, "обложка — источник"},
	} {
		if _, err := e.CopyDecor(c.src, c.from, c.to, true, false); err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		edtest.Same(t, e, rev, undo, c.what)
	}
	// Одна из двух целей уже такая же: шаг один, план — только по другой.
	putStickers(e, "s3", plate(40, 40, 20, 20, 0))
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	two := append(slices.Clone(to), model.GridTarget{Spread: "s3", Scope: model.ScopeLeft})
	if plan := e.PlanCopyDecor("s1", model.ScopeLeft, two, true, false); plan != (model.GridPlan{Stickers: 1}) {
		t.Fatalf("план: %+v", plan)
	}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, two, true, false); err != nil || edtest.UndoLen(e) != undo+1 || edtest.Rev(e) == rev {
		t.Fatalf("две цели: %v, шагов %d → %d", err, undo, edtest.UndoLen(e))
	}
	// После записи и чтения тот же повтор — холостой (зажимы Load не сдвигают копии).
	f := edtest.Open(t, edtest.SaveLoad(t, e), "")
	rev, undo = edtest.Rev(f), edtest.UndoLen(f)
	if _, err := f.CopyDecor("s1", model.ScopeLeft, two, true, false); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, f, rev, undo, "повтор после Save → Load")
}

// TestCopyDecorLimit — сверх MaxStickers на листе, MaxTotalStickers в проекте и MaxTexts —
// ошибка всей правки без шага; ровно предел — проходит.
func TestCopyDecorLimit(t *testing.T) {
	extra := make([][]string, 33)
	for i := range extra {
		extra[i] = []string{}
	}
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, extra...)
	for range 5 {
		putStickers(e, "s1", plate(20, 20, 30, 30, 0))
	}
	fill := func(sp string, n int, x float64) {
		sts := make([]*model.Sticker, n)
		for i := range sts {
			sts[i] = plate(x, 20, 30, 30, 0)
		}
		putStickers(e, sp, sts...)
	}
	fill("s2", 60, 300) // правая страница: 60 + 5 > 64
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	left := []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, left, false, false); !errors.Is(err, model.ErrStickers) {
		t.Fatalf("сверх MaxStickers: %v", err)
	}
	edtest.Same(t, e, rev, undo, "MaxStickers")
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Stickers = p.Spreads[1].Stickers[:59] })
	if _, err := e.CopyDecor("s1", model.ScopeLeft, left, false, false); err != nil || len(sheetOf(e, "s2").Stickers) != model.MaxStickers {
		t.Fatalf("ровно 64: %v, %d", err, len(sheetOf(e, "s2").Stickers))
	}

	// Проект: 30 листов по 64 + 64 на s2 + 5 у источника = 1989; обе страницы s33 (+10) — 1999,
	// ещё обе страницы s34 (+10) — 2009 > 2000.
	for i := 3; i <= 32; i++ {
		fill(fmt.Sprintf("s%d", i), model.MaxStickers, 300)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	both := []model.GridTarget{{Spread: "s33", Scope: model.ScopeSpread}, {Spread: "s34", Scope: model.ScopeSpread}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, both, false, false); err == nil {
		t.Fatal("сверх MaxTotalStickers без ошибки")
	}
	edtest.Same(t, e, rev, undo, "MaxTotalStickers")
	if _, err := e.CopyDecor("s1", model.ScopeLeft, both[:1], false, false); err != nil {
		t.Fatalf("до предела проекта: %v", err)
	}

	// Надписи: у цели на левой 64, на правую придёт ещё одна — сверх MaxTexts.
	edtest.AddText(t, e, "s1", 0)
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[33]
		for len(s.Texts) < model.MaxTexts {
			s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), X: 10, Y: 10, W: 50, Text: "x", Style: model.PlainStyle()})
		}
	})
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s34", Scope: model.ScopeRight}}, true, false); err == nil {
		t.Fatal("сверх MaxTexts без ошибки")
	}
	edtest.Same(t, e, rev, undo, "MaxTexts")
}

// TestPlanCopyDecor — план не меняет проект (ни rev); считает заменяемые стикеры и (с флажком)
// надписи; мест и «Авто» не касается; совпадает с итогом; ошибка — пустой план. Лист 031 —
// только «весь лист».
func TestPlanCopyDecor(t *testing.T) {
	e, _, _ := decorEd(t)
	putStickers(e, "s2", plate(20, 40, 30, 30, 0), plate(300, 40, 30, 30, 0), plate(320, 90, 30, 30, 0))
	e.AddText("s2", 1, "")
	data, rev := model.Marshal(edtest.P(e)), edtest.Rev(e)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeSpread}}
	plan := e.PlanCopyDecor("s1", model.ScopeLeft, to, true, false)
	if !bytes.Equal(data, model.Marshal(edtest.P(e))) || edtest.Rev(e) != rev {
		t.Fatal("план поменял проект")
	}
	if plan != (model.GridPlan{Stickers: 2, Texts: 1}) {
		t.Fatalf("план %+v", plan)
	}
	if p := e.PlanCopyDecor("s1", model.ScopeLeft, to, false, true); p != (model.GridPlan{Stickers: 2}) {
		t.Fatalf("без надписей: %+v", p)
	}
	if p := e.PlanCopyDecor("s1", "nope", to, true, false); p != (model.GridPlan{}) {
		t.Fatalf("ошибка — пустой план: %+v", p)
	}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, true, false); err != nil || len(onPage(sheetOf(e, "s2"), 1)) != 2 || len(onPage(sheetOf(e, "s2"), 0)) != 1 {
		t.Fatalf("копия по плану: %v", err)
	}
	edtest.MustGrid(t, e)
	// Лист 031: страница — ни источник, ни цель; весь лист — можно.
	f := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, []string{})
	edtest.Change(f, func(p *model.Project) { p.Settings.Sheet = true })
	f.SetBackground("s1", model.ScopeSpread, "#aabbcc")
	if _, err := f.CopyDecor("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); !errors.Is(err, model.ErrScope) {
		t.Fatalf("лист 031, страница-источник: %v", err)
	}
	if _, err := f.CopyDecor("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, false, false); err == nil {
		t.Fatal("лист 031, страница-цель без ошибки")
	}
	if _, err := f.CopyDecor("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil || sheetOf(f, "s2").BgL != "#aabbcc" {
		t.Fatalf("лист 031, весь лист: %v", err)
	}
	edtest.MustGrid(t, f)
}

// TestCopyDecorTargets — больше 3·MaxSpreads целей — ошибка до правки (без шага, план пуст);
// ровно предел проходит; одна цель много раз и «страница + весь лист» считаются один раз:
// шаг один, итог как у одиночной.
func TestCopyDecorTargets(t *testing.T) {
	e, _, _ := decorEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	many := make([]model.GridTarget, 3*model.MaxSpreads+1)
	for i := range many {
		many[i] = model.GridTarget{Spread: "s3", Scope: model.ScopeLeft}
	}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, many, false, false); err == nil {
		t.Fatal("сверх 3·MaxSpreads целей без ошибки")
	}
	edtest.Same(t, e, rev, undo, "слишком много целей")
	if p := e.PlanCopyDecor("s1", model.ScopeLeft, many, false, false); p != (model.GridPlan{}) {
		t.Fatalf("план при отказе: %+v", p)
	}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, many[:3*model.MaxSpreads], false, false); err != nil {
		t.Fatalf("ровно предел: %v", err)
	}
	if n := len(onPage(sheetOf(e, "s3"), 0)); n != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("стикеров на s3 %d (ждали 2), шагов %d → %d", n, undo, edtest.UndoLen(e))
	}

	// Дубли: те же цели по два раза — тот же итог, что у списка без повторов.
	one := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeSpread}}
	dup := append(append([]model.GridTarget{}, one...), one...)
	f, _, _ := decorEd(t)
	g, _, _ := decorEd(t)
	planDup, planOne := f.PlanCopyDecor("s1", model.ScopeLeft, dup, false, false), g.PlanCopyDecor("s1", model.ScopeLeft, one, false, false)
	if planDup != planOne {
		t.Fatalf("план: дубли %+v, без дублей %+v", planDup, planOne)
	}
	rev, undo = edtest.Rev(f), edtest.UndoLen(f)
	if _, err := f.CopyDecor("s1", model.ScopeLeft, dup, false, false); err != nil || edtest.UndoLen(f) != undo+1 || edtest.Rev(f) == rev {
		t.Fatalf("дубли: %v, шагов %d → %d", err, undo, edtest.UndoLen(f))
	}
	if _, err := g.CopyDecor("s1", model.ScopeLeft, one, false, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"s2", "s3"} {
		if a, b := len(sheetOf(f, id).Stickers), len(sheetOf(g, id).Stickers); a != b || a == 0 {
			t.Fatalf("%s: стикеров с дублями %d, без %d", id, a, b)
		}
	}
	edtest.MustGrid(t, f)
}

// TestCopyDecorBrokenSticker — стикер источника, который не зажимается (фото не в проекте,
// ни фото ни рисунка), в копию не едет: остальные доезжают, ошибки нет; повтор — холостой
// (сверка видит ту же отсеянную копию).
func TestCopyDecorBrokenSticker(t *testing.T) {
	e, _, _ := decorEd(t)
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		s.Stickers = append(s.Stickers,
			&model.Sticker{ID: model.NewID(), Photo: "ghost.jpg", X: 30, Y: 30, W: 20, H: 20},
			&model.Sticker{ID: model.NewID(), X: 40, Y: 40, W: 20, H: 20})
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	to := []model.GridTarget{{Spread: "s3", Scope: model.ScopeLeft}, {Spread: "s2", Scope: model.ScopeSpread}}
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, false, false); err != nil || edtest.UndoLen(e) != undo+1 || edtest.Rev(e) == rev {
		t.Fatalf("копия: %v, шагов %d → %d", err, undo, edtest.UndoLen(e))
	}
	for _, id := range []string{"s3", "s2"} {
		if n := len(onPage(sheetOf(e, id), 0)); n != 2 {
			t.Fatalf("%s: стикеров на левой %d, ждали 2 (битые не едут)", id, n)
		}
		for _, st := range sheetOf(e, id).Stickers {
			if st.Photo == "ghost.jpg" || (st.Photo == "" && st.Art == nil) {
				t.Fatalf("%s: поехал битый стикер %+v", id, st)
			}
		}
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, to, false, false); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "повтор с битым источником")
}
