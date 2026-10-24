package grid_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Узнавание формы сторонки, память последней формы и края (127 G2).

// noLast — память Editor пуста (новый Editor, переоткрытый файл).
var noLast [2][]model.Box

// TestStepCoverGridRemembers (127-6): ▦ «3» → фото → › → замена фото портретом (места не тронуты):
// ряд стал другим, но форма узнана по памяти, › идёт по новому ряду с первой раскладки. Без памяти
// та же форма — «Ручная». Свой шаблон («Мои») узнаётся только памятью.
func TestStepCoverGridRemembers(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	c.apply(scope, c.choices(scope, 3).Builtin[0].Boxes) // ▦ «3» на пустой сторонке
	c.seat(scope, nil, "l1", "p2", "l3")
	if !c.step(scope, 1) {
		t.Fatal("› не сработал")
	}
	oldRow0 := c.choices(scope, 3).ByPhoto[0].Boxes
	c.wantShape(scope, oldRow0, "› с первой")

	c.seat(scope, nil, "v4", "v5", "v6") // замена фото портретом, места стоят
	if st := c.state(scope); !st.Known || st.Places != 3 {
		t.Fatalf("после замены фото: %+v, ждали Known по памяти", st)
	}
	if st := grid.CoverSideStates(c.p(), noLast)[1]; st.Known {
		t.Fatalf("без памяти форма не должна узнаваться: %+v", st)
	}
	newRow := c.choices(scope, 3).ByPhoto
	if sameFrac(newRow[0].Boxes, oldRow0) {
		t.Fatal("подготовка: ряд не изменился")
	}
	if !c.step(scope, 1) {
		t.Fatal("› по памяти ничего не сделал")
	}
	c.wantShape(scope, newRow[0].Boxes, "› по новому ряду — первая")

	// Свой шаблон узнаётся только памятью; › с первой раскладки ряда.
	c.apply(scope, mine("m", 3).Boxes)
	if st := c.state(scope); !st.Known {
		t.Fatalf("свой шаблон по памяти: %+v", st)
	}
	if st := grid.CoverSideStates(c.p(), noLast)[1]; st.Known {
		t.Fatalf("свой шаблон без памяти: %+v", st)
	}
	if shape, err := edtestStep(c, scope, 1, noLast); err != nil || shape != nil {
		t.Fatalf("› без памяти: %v, %v (ждали холостую)", shape, err)
	}
	c.step(scope, 1)
	c.wantShape(scope, c.choices(scope, 3).ByPhoto[0].Boxes, "› после своего шаблона")
}

// sameFrac — формы совпадают в пределах 0,01 доли зоны.
func sameFrac(a, b []model.Box) bool {
	for i := range a {
		if math.Abs(a[i].X-b[i].X) > 0.01 || math.Abs(a[i].Y-b[i].Y) > 0.01 || math.Abs(a[i].W-b[i].W) > 0.01 || math.Abs(a[i].H-b[i].H) > 0.01 {
			return false
		}
	}
	return true
}

// edtestStep — StepCoverGrid с произвольной памятью, без записи её в cv.
func edtestStep(c *cv, scope string, delta int, last [2][]model.Box) (shape []model.Box, err error) {
	_, err = edtest.Try(c.e, "covergrid:"+scope, func(p *model.Project) (err error) {
		shape, err = grid.StepCoverGrid(p, scope, delta, last)
		return err
	})
	return shape, err
}

// TestStepCoverGridNoPlaces (127-7): сторонка без мест — Known = false (даже при памяти), Step
// холостой, места сами не появляются.
func TestStepCoverGridNoPlaces(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeLeft
	c.last[0] = c.choices(scope, 1).Builtin[0].Boxes // память есть, мест нет
	if st := c.state(scope); st.Places != 0 || st.Known || st.Alone {
		t.Fatalf("пустая сторонка: %+v", st)
	}
	rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
	for _, d := range []int{1, -1} {
		if c.step(scope, d) {
			t.Fatalf("Step %+d на пустой сторонке изменил проект", d)
		}
	}
	edtest.Same(t, c.e, rev, undo, "шаги на пустой сторонке")
	if n := len(c.places(scope)); n != 0 {
		t.Fatalf("появилось %d мест", n)
	}
	// Пустая «1»: вкладка по умолчанию, ряда «По фото» нет, встроенные есть.
	ch := c.choices(scope, 0)
	if ch.N != 1 || ch.Places != 0 || len(ch.ByPhoto) != 0 || len(ch.Builtin) == 0 {
		t.Fatalf("галерея пустой сторонки: N=%d places=%d byPhoto=%d builtin=%d", ch.N, ch.Places, len(ch.ByPhoto), len(ch.Builtin))
	}
}

// TestStepCoverGridAfterSpine (127-8): корешок сдвинул зону и места по X — форма узнана, › работает.
func TestStepCoverGridAfterSpine(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "h3")
	c.apply(scope, c.choices(scope, 3).ByPhoto[1].Boxes)
	boxBefore, xBefore := c.box(scope), c.places(scope)[0].X
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Spec.Hinge += 5 })
	if boxAfter := c.box(scope); boxAfter.X-boxBefore.X < 4 || boxAfter.W != boxBefore.W {
		t.Fatalf("подготовка: зона %+v → %+v, ждали сдвиг по X", boxBefore, boxAfter)
	}
	if c.places(scope)[0].X-xBefore < 4 {
		t.Fatalf("подготовка: места не поехали с корешком")
	}
	if st := c.state(scope); !st.Known || st.Alone || st.Places != 3 {
		t.Fatalf("после сдвига корешка: %+v", st)
	}
	if !c.step(scope, 1) {
		t.Fatal("› после сдвига корешка не сработал")
	}
	c.wantShape(scope, c.choices(scope, 3).ByPhoto[2].Boxes, "› после корешка")
	c.inside(scope, "после корешка")
}

// TestStepCoverGridAfterResize (127-8): сменили размер сторонки — форма не узнана, Step холостой,
// новая раскладка встаёт в новую зону.
func TestStepCoverGridAfterResize(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "h3")
	c.apply(scope, c.choices(scope, 3).ByPhoto[1].Boxes)
	edtest.Change(c.e, func(p *model.Project) {
		p.Cover.Spec.W, p.Cover.Spec.H = p.CoverGeo().X[2]-p.CoverGeo().X[1]-20, p.CoverGeo().Y[2]-p.CoverGeo().Y[1]-30
	})
	if st := c.state(scope); st.Known || st.Places != 3 {
		t.Fatalf("после смены размера: %+v, ждали «Ручная»", st)
	}
	rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
	if c.step(scope, 1) || c.step(scope, -1) {
		t.Fatal("Step после смены размера изменил места")
	}
	edtest.Same(t, c.e, rev, undo, "шаги после смены размера")
	c.apply(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	c.inside(scope, "новая зона")
	if st := c.state(scope); !st.Known {
		t.Fatalf("после новой плитки: %+v", st)
	}
}

// TestCoverGrid17 (127-9): 17 мест — «По фото» пуст, форма не узнаётся даже при памяти, Step холостой;
// 16 мест — ряд есть, форма из него узнаётся и листается.
func TestCoverGrid17(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	mine17 := mine("m17", 17)
	c.apply(scope, mine17.Boxes)
	ch := c.choices(scope, 7, mine17)
	if ch.Places != 17 || ch.N != 7 || len(ch.ByPhoto) != 0 || len(ch.Mine) != 1 {
		t.Fatalf("17 мест: places=%d N=%d byPhoto=%d mine=%d", ch.Places, ch.N, len(ch.ByPhoto), len(ch.Mine))
	}
	if st := c.state(scope); st.Places != 17 || st.Known || st.Alone {
		t.Fatalf("17 мест: %+v", st)
	}
	if c.step(scope, 1) {
		t.Fatal("Step на 17 местах изменил проект")
	}

	mine16 := mine("m16", 16)
	c.apply(scope, mine16.Boxes)
	ch = c.choices(scope, 7)
	if ch.Places != 16 || len(ch.ByPhoto) == 0 {
		t.Fatalf("16 мест: places=%d byPhoto=%d", ch.Places, len(ch.ByPhoto))
	}
	c.apply(scope, ch.ByPhoto[0].Boxes)
	if st := c.state(scope); st.Places != 16 || !st.Known {
		t.Fatalf("16 мест из ряда: %+v", st)
	}
	if len(ch.ByPhoto) > 1 {
		if !c.step(scope, 1) {
			t.Fatal("Step на 16 местах не сработал")
		}
		c.wantShape(scope, ch.ByPhoto[1].Boxes, "16 мест, ›")
	}
	c.inside(scope, "16 мест")
}

// TestStepCoverGridAlone (127-5, 127-8): одно место — в ряду одна раскладка, она стоит: Alone, Step
// холостой; сдвинутое или повёрнутое место — «Ручная» (Known и Alone сняты), Step тоже холостой.
func TestStepCoverGridAlone(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1")
	ch := c.choices(scope, 1)
	if len(ch.ByPhoto) != 1 {
		t.Fatalf("ряд из одного места: %d раскладок", len(ch.ByPhoto))
	}
	c.apply(scope, ch.ByPhoto[0].Boxes)
	if st := c.state(scope); !st.Known || !st.Alone || st.Places != 1 {
		t.Fatalf("одно место из ряда: %+v", st)
	}
	rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
	if c.step(scope, 1) || c.step(scope, -1) {
		t.Fatal("Step при alone изменил проект")
	}
	edtest.Same(t, c.e, rev, undo, "alone")
	if st := grid.CoverSideStates(c.p(), noLast)[1]; !st.Known || !st.Alone {
		t.Errorf("alone без памяти: %+v", st)
	}

	// Сдвиг в пределах допуска 0,1 мм форму сохраняет, больше — «Ручная»: Known и Alone сняты.
	for _, c2 := range []struct {
		dx    float64
		known bool
	}{{0.04, true}, {0.5, false}, {3, false}} {
		c.apply(scope, ch.ByPhoto[0].Boxes)
		edtest.Change(c.e, func(p *model.Project) { c.places(scope)[0].X -= c2.dx })
		if st := c.state(scope); st.Known != c2.known || st.Alone != c2.known || st.Places != 1 {
			t.Errorf("сдвиг на %v мм: %+v", c2.dx, st)
		}
	}
}

// TestStepCoverGridRotated: повёрнутое место — форма не узнана, даже если x, y, w, h на месте.
func TestStepCoverGridRotated(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "h3")
	c.apply(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	if !c.state(scope).Known {
		t.Fatal("подготовка: форма не узнана")
	}
	edtest.Change(c.e, func(p *model.Project) { c.places(scope)[1].Rot = 7 })
	if st := c.state(scope); st.Known || st.Alone || st.Places != 3 {
		t.Fatalf("повёрнутое место: %+v", st)
	}
	if c.step(scope, 1) {
		t.Fatal("Step с повёрнутым местом изменил проект")
	}
}

// TestCoverGridChoicesMine (127-12): в «Мои» только шаблоны страницы без виньеток (шаблон разворота
// растянул бы места вдвое), число мест — как у вкладки (7 — от 7), «−N» — фото сторонки минус места.
func TestCoverGridChoicesMine(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "s3", "l4", "p5")
	vig := mine("vig3", 3)
	vig.Kind = model.TplVignette
	spread := mine("spread3", 3)
	spread.Scope = model.TplSpread
	tpls := []model.Template{mine("page3", 3), spread, vig, mine("page8", 8), mine("page2", 2), mine("page3b", 3)}

	ch := c.choices(scope, 3, tpls...)
	if ids := tplIDs(ch.Mine); strings.Join(ids, ",") != "page3,page3b" {
		t.Fatalf("Мои на 3: %v", ids)
	}
	for _, m := range ch.Mine {
		if m.Drop != 2 || m.Scope != model.TplPage {
			t.Errorf("Мои %s: Drop %d, scope %s", m.ID, m.Drop, m.Scope)
		}
	}
	if len(ch.ByPhoto) != 0 || ch.Photos != 5 || ch.Places != 5 {
		t.Errorf("вкладка 3 при 5 местах: byPhoto=%d photos=%d places=%d", len(ch.ByPhoto), ch.Photos, ch.Places)
	}
	for _, g := range ch.Builtin {
		if g.Drop != 2 || len(g.Boxes) != 3 {
			t.Fatalf("встроенная %s: Drop %d, мест %d", g.ID, g.Drop, len(g.Boxes))
		}
	}
	if ids := tplIDs(c.choices(scope, 7, tpls...).Mine); strings.Join(ids, ",") != "page8" {
		t.Errorf("Мои на 7+: %v", ids)
	}
	if m := c.choices(scope, 7, tpls...).Mine; m[0].Drop != 0 {
		t.Errorf("Drop шаблона на 8 мест при 5 фото: %d", m[0].Drop)
	}

	// Вкладка по числу мест (n = 0): «По фото» с Drop 0 и без форм, уже бывших в ряду.
	ch = c.choices(scope, 0)
	if ch.N != 5 || len(ch.ByPhoto) == 0 {
		t.Fatalf("n = 0: N=%d byPhoto=%d", ch.N, len(ch.ByPhoto))
	}
	seen := map[string]bool{}
	for _, g := range ch.ByPhoto {
		seen[grid.ShapeKey(rectsMM(ch.Box, g.Boxes))] = true
	}
	for _, g := range ch.Builtin {
		if seen[grid.ShapeKey(rectsMM(ch.Box, g.Boxes))] {
			t.Errorf("встроенная %s повторяет форму «По фото»", g.ID)
		}
	}
	// Одинаковые 3:2: часть встроенных повторяет «По фото» и отсекается, остальные наборы остаются.
	h := fsfera(t)
	h.fill(scope, "h1", "h2", "h3")
	hc := h.choices(scope, 3)
	if len(hc.ByPhoto) == 0 || len(hc.Builtin) == 0 {
		t.Fatalf("три 3:2: byPhoto=%d builtin=%d", len(hc.ByPhoto), len(hc.Builtin))
	}
	hseen := map[string]bool{}
	for _, g := range hc.ByPhoto {
		hseen[grid.ShapeKey(rectsMM(hc.Box, g.Boxes))] = true
	}
	for _, g := range hc.Builtin {
		if hseen[grid.ShapeKey(rectsMM(hc.Box, g.Boxes))] {
			t.Errorf("три 3:2: встроенная %s повторяет форму «По фото»", g.ID)
		}
	}
	// Зажим вкладки.
	for n, want := range map[int]int{-3: 5, 100: 7, 1: 1} {
		if got := c.choices(scope, n).N; got != want {
			t.Errorf("n=%d: вкладка %d, ждали %d", n, got, want)
		}
	}
}

func tplIDs(cs []model.GridChoice) (out []string) {
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

// TestCoverViewKnownFast (127-13): состояние сторонок для вида при 7 + 7 местах, прогретое, — не
// дольше 1 мс (стоимость узнавания формы на каждой сборке вида обложки).
func TestCoverViewKnownFast(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "h1", "v2", "s3", "l4", "p5", "h6", "v7")
	c.fill(model.ScopeRight, "h8", "v9", "sa", "lb", "pc", "hd", "ve")
	c.apply(model.ScopeLeft, c.choices(model.ScopeLeft, 7).ByPhoto[2].Boxes)
	// Без памяти: левая узнаётся рядом «По фото», правая — формой встроенной сетки, третьим
	// кандидатом, самый дорогой путь.
	last := noLast
	st := grid.CoverSideStates(c.p(), last)
	if !st[0].Known || !st[1].Known || st[0].Places != 7 || st[1].Places != 7 {
		t.Fatalf("подготовка: %+v", st)
	}
	p := c.p()
	layout.ResetCache()
	cold := testkit.Fastest(1, func() { grid.CoverSideStates(p, last) })
	d := testkit.Fastest(7, func() { grid.CoverSideStates(p, last) })
	if limit := testkit.Limit(1e6); d > limit { // 1 мс
		t.Errorf("состояние сторонок 7 + 7 мест: %v, предел %v", d, limit)
	}
	t.Logf("7 + 7 мест: прогретое %v, холодное %v", d, cold)
}

// TestCoverGridBadInput: недоверенные доли и крайние значения — ошибка или зажим, не паника и не мусор.
func TestCoverGridBadInput(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2")
	good := c.choices(scope, 2).Builtin[0].Boxes
	nan, inf := math.NaN(), math.Inf(1)
	for name, frac := range map[string][]model.Box{
		"пусто":    nil,
		"NaN":      {{X: nan, Y: 0, W: 0.5, H: 0.5}},
		"Inf":      {{X: 0, Y: 0, W: inf, H: 0.5}},
		"NaN во 2": {good[0], {X: 0, Y: nan, W: 0.5, H: 0.5}},
	} {
		rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
		if _, err := edtest.Try(c.e, "", func(p *model.Project) error { _, err := grid.ApplyCoverGrid(p, scope, frac); return err }); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
		edtest.Same(t, c.e, rev, undo, name)
	}
	// Огромные, нулевые и отрицательные доли зажимаются в зону; слишком много мест — MaxPlaces.
	huge := []model.Box{{X: -1e308, Y: 1e308, W: 1e308, H: -5}, {X: 0.5, Y: 0.5, W: 0, H: 0}, {X: 2, Y: -2, W: 3, H: 3}}
	c.apply(scope, huge)
	c.inside(scope, "огромные доли")
	many := make([]model.Box, 3*model.MaxPlaces)
	for i := range many {
		many[i] = model.Box{X: 0.01 * float64(i%50), Y: 0.01 * float64(i%50), W: 0.2, H: 0.2}
	}
	c.apply(scope, many)
	if got := len(c.places(scope)); got != model.MaxPlaces {
		t.Errorf("мест %d, ждали зажим до %d", got, model.MaxPlaces)
	}
	c.inside(scope, "много мест")
	edtest.MustGrid(t, c.e)

	// Неизвестная сторонка и корешок; Step с предельными delta и нулём.
	for _, sc := range []string{"", "spread", model.ScopeSpine, "bogus"} {
		if _, err := edtest.Try(c.e, "", func(p *model.Project) error { _, err := grid.ApplyCoverGrid(p, sc, good); return err }); err == nil || !strings.Contains(err.Error(), "област") {
			t.Errorf("scope %q: ошибка %v, ждали ErrScope", sc, err)
		}
		if _, err := edtestStep(c, sc, 1, noLast); err == nil {
			t.Errorf("Step scope %q: ждали ErrScope", sc)
		}
	}
	c.fill(scope, "h1", "v2")
	c.apply(scope, c.choices(scope, 2).ByPhoto[0].Boxes)
	for _, d := range []int{0, math.MaxInt, math.MinInt, math.MaxInt32 + 7} {
		c.step(scope, d)
		c.inside(scope, "предельный delta")
	}
}

// TestCoverGridNoSideNoCover: нет обложки и нет безопасной зоны (W ≤ 0) — без паники, холостые
// правки, пустые ряды, понятная ошибка.
func TestCoverGridNoSideNoCover(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar())
	p := edtest.P(e)
	good := []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}
	if left, err := grid.ApplyCoverGrid(p, model.ScopeRight, good); left != 0 || err != nil {
		t.Errorf("без обложки: %d, %v", left, err)
	}
	if shape, err := grid.StepCoverGrid(p, model.ScopeRight, 1, noLast); shape != nil || err != nil {
		t.Errorf("Step без обложки: %v, %v", shape, err)
	}
	if st := grid.CoverSideStates(p, noLast); st != [2]model.CoverSideState{} {
		t.Errorf("состояния без обложки: %+v", st)
	}
	for _, sc := range []string{model.ScopeRight, model.ScopeSpine, ""} { // и неизвестная сторонка
		ch := grid.CoverGridChoicesOf(grid.CoverInput(p, sc), 3, nil)
		if ch.Scope != sc || ch.Box.W != 0 || len(ch.Builtin)+len(ch.ByPhoto)+len(ch.Mine) != 0 {
			t.Errorf("галерея %q без обложки: %+v", sc, ch)
		}
	}
	ch := grid.CoverGridChoicesOf(grid.CoverInput(p, model.ScopeRight), 3, nil)
	if ch.ByPhoto == nil || ch.Builtin == nil || ch.Mine == nil || len(ch.Builtin) != 0 || ch.Box.W != 0 || ch.N != 3 {
		t.Errorf("галерея без обложки: %+v", ch)
	}

	c := fsfera(t)
	c.fill(model.ScopeRight, "h1")
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Spec.W, p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 50, 30, 30 })
	if len(c.p().CoverSides()) != 0 {
		t.Fatal("подготовка: сторонки остались")
	}
	rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
	_, err := edtest.Try(c.e, "", func(p *model.Project) error { _, err := grid.ApplyCoverGrid(p, model.ScopeRight, good); return err })
	if err == nil || !strings.Contains(err.Error(), "Безопасная зона сторонки пуста") {
		t.Errorf("зоны нет: ошибка %v", err)
	}
	if c.step(model.ScopeRight, 1) {
		t.Error("Step без зоны изменил проект")
	}
	edtest.Same(t, c.e, rev, undo, "нет зоны")
	if st := c.state(model.ScopeRight); st != (model.CoverSideState{}) {
		t.Errorf("состояние без зоны: %+v", st)
	}
	ch = c.choices(model.ScopeRight, 2)
	if ch.Box.W != 0 || len(ch.Builtin) != 0 || len(ch.ByPhoto) != 0 || ch.Mine == nil {
		t.Errorf("галерея без зоны: %+v", ch)
	}
}

// TestCoverGridSaveLoad: итог плиток — обычные места: после Save → Load они те же, а форма
// узнаётся без памяти (ряд «По фото» и встроенные), формат файла не меняется.
func TestCoverGridSaveLoad(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "h1", "v2", "s3")
	c.fill(model.ScopeRight, "l4", "p5", "h6")
	c.apply(model.ScopeLeft, c.choices(model.ScopeLeft, 3).Builtin[1].Boxes)
	c.apply(model.ScopeRight, c.choices(model.ScopeRight, 3).ByPhoto[1].Boxes)
	q := edtest.SaveLoad(t, c.e)
	// ID фото при Load перевыдаются (хеши), места — те же по порядку и геометрии.
	for _, sc := range []string{model.ScopeLeft, model.ScopeRight} {
		got := rectsOfPlaces(q, sc)
		want := rectsOfPlaces(c.p(), sc)
		if len(got) != 3 || !slices.Equal(got, want) {
			t.Fatalf("%s после Save → Load: %v, было %v", sc, got, want)
		}
	}
	if err := edtest.Reopen(q); err != nil {
		t.Fatal(err)
	}
	st := grid.CoverSideStates(q, noLast)
	if !st[0].Known || !st[1].Known || st[0].Places != 3 || st[1].Places != 3 {
		t.Errorf("после Load без памяти: %+v", st)
	}
	if q.Version != model.FormatVersion {
		t.Errorf("версия формата %d", q.Version)
	}
}

// rectsOfPlaces — x, y, w, h мест сторонки по порядку.
func rectsOfPlaces(p *model.Project, scope string) (out [][4]float64) {
	for _, pl := range p.Cover.Sheet.Grid {
		if grid.InArea(p, p.Cover.Sheet, pl.X, pl.W, scope) {
			out = append(out, [4]float64{pl.X, pl.Y, pl.W, pl.H})
		}
	}
	return out
}
