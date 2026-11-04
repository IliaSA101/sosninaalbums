package view_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// sidesEd — обложка группы group на странице формата name: настройки страницы формата, пресет preset.
func sidesEd(t *testing.T, group, name, preset string) *album.Editor {
	t.Helper()
	i := slices.IndexFunc(model.GroupFormats(group), func(f model.PrintFormat) bool { return f.Name == name })
	if i < 0 {
		t.Fatalf("нет формата %s %s", group, name)
	}
	f := model.GroupFormats(group)[i]
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	s := edtest.P(e).Settings
	s.PageW, s.PageH, s.DPI, s.Layflat, s.Safe, s.Bleed = f.PageW, f.PageH, 300, true, f.Safe, f.Bleed
	e.SetSettings(s)
	if _, err := e.ApplyCoverPreset(preset); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestCoverSidesSameAsSafe (127-1): CoverView.Safe бит-в-бит такой же, как до переноса зоны в model
// (эталон снят с кода до переноса: view.coverZones), и равен Sides[].Box. Случаи: Фотосфера 30×30,
// Фабрика 30×20 (загиб Б5), не по шаблону, свои Safe/SpineSafe, зона W ≤ 0 и H ≤ 0 (сторонки нет).
func TestCoverSidesSameAsSafe(t *testing.T) {
	fsfera := func() *album.Editor { return sidesEd(t, model.CoverGroupFsfera, "30×30", "fsfera-thin") }
	fabrika := func() *album.Editor { return sidesEd(t, model.CoverGroupFabrika, "30×20", "fabrika-photo") }
	rects := func(a, b layout.Rect) []layout.Rect { return []layout.Rect{a, b} }
	r := func(x, y, w, h float64) layout.Rect { return layout.Rect{X: x, Y: y, W: w, H: h} }
	for _, c := range []struct {
		name string
		ed   func() *album.Editor
		spec func(*model.CoverSpec)
		want []layout.Rect // nil — сторонок нет
	}{
		{"Фотосфера 30×30", fsfera, nil, rects(r(28, 28, 266, 270), r(324, 28, 266, 270))},
		{"Фабрика 30×20 (загиб Б5)", fabrika, nil, rects(r(23.5, 21.5, 291.5, 197), r(347, 21.5, 291.5, 197))},
		{"не по шаблону", fabrika, func(s *model.CoverSpec) { s.Wrap = 15 }, rects(r(25, 25, 290, 186), r(343, 25, 290, 186))},
		{"свои Safe/SpineSafe", fabrika, func(s *model.CoverSpec) { s.Safe, s.SpineSafe = 5, 6 }, rects(r(22, 22, 299, 196), r(341, 22, 299, 196))},
		{"W ≤ 0", fsfera, func(s *model.CoverSpec) { s.W, s.Safe, s.SpineSafe = 50, 30, 30 }, nil},
		{"H ≤ 0", fsfera, func(s *model.CoverSpec) { s.H, s.Safe = 50, 30 }, nil},
	} {
		e := c.ed()
		if c.spec != nil {
			edtest.Change(e, func(p *model.Project) { c.spec(&p.Cover.Spec) })
		}
		checkSides(t, c.name, e, c.want)
	}
}

// checkSides — вид обложки e: Safe бит-в-бит как want (пусто — сторонок нет), Sides совпадают с ним.
func checkSides(t *testing.T, name string, e *album.Editor, want []layout.Rect) {
	t.Helper()
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	cv := e.View().Cover
	if len(want) == 0 {
		if b, _ := json.Marshal(cv.Sides); len(cv.Safe) != 0 || string(b) != "[]" {
			t.Errorf("%s: Safe %+v, Sides %s (ждали пустой список без паники)", name, cv.Safe, b)
		}
		return
	}
	if !slices.Equal(cv.Safe, want) { // ==: бит-в-бит, без допуска
		t.Fatalf("%s: Safe %+v, эталон %+v", name, cv.Safe, want)
	}
	if len(cv.Sides) != 2 || cv.Sides[0].Scope != model.ScopeLeft || cv.Sides[0].Label != "Задняя" ||
		cv.Sides[1].Scope != model.ScopeRight || cv.Sides[1].Label != "Лицевая" {
		t.Fatalf("%s: сторонки %+v", name, cv.Sides)
	}
	// Места считает Editor (127 G3): sidesEd ставит одно место на лицевую, на задней мест нет.
	for i, sd := range cv.Sides {
		if sd.Box != want[i] || sd.Places != i {
			t.Errorf("%s: сторонка %+v, Safe %+v, мест ждали %d", name, sd, want[i], i)
		}
	}
	// Зоны «safe» направляющих — те же числа; чтение вида правок не создаёт; после Save → Load то же.
	if got := zoneRects(cv, "safe"); !slices.Equal(got, want) {
		t.Errorf("%s: зоны safe %+v, Safe %+v", name, got, want)
	}
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Errorf("%s: чтение вида создало шаг отмены или rev", name)
	}
	if got := edtest.Open(t, edtest.SaveLoad(t, e), "").View().Cover.Safe; !slices.Equal(got, want) {
		t.Errorf("%s: после Save → Load Safe %+v, было %+v", name, got, want)
	}
}

// TestCoverSidesFoldLine — страж против расхождения с видом 123 Б5: у каждого формата Фабрики
// Sides[].Box лежат по линии риска шаблона (загиб по осям PSD + FoldSafe, корешок + SpineSafe);
// после «Не по шаблону» и при своих линиях — от края сторонки; без шаблона и без обложки зон нет.
func TestCoverSidesFoldLine(t *testing.T) {
	for _, f := range model.FabrikaFormats() {
		e := fabrikaEd(t, f)
		_, _, safe := guideRects(f.Guide, edtest.P(e).CoverGeo())
		sides := e.View().Cover.Sides
		if len(sides) != 2 {
			t.Fatalf("%s: сторонок %d", f.Name, len(sides))
		}
		for i := range safe {
			if !rectNear(sides[i].Box, safe[i]) {
				t.Errorf("%s: %s %+v, линия PSD %+v", f.Name, sides[i].Label, sides[i].Box, safe[i])
			}
		}
		// Своя Safe снимает направляющие: рамка от края сторонки, а не от загиба.
		edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe = 5 })
		cg := edtest.P(e).CoverGeo()
		if b := e.View().Cover.Sides[0].Box; !rectNear(b, layout.Rect{X: cg.X[1] + 5, Y: cg.Y[1] + 5, W: cg.X[2] - edtest.P(e).Settings.Safe - cg.X[1] - 5, H: cg.Y[2] - cg.Y[1] - 10}) {
			t.Errorf("%s: свой Safe, рамка %+v", f.Name, b)
		}
	}
	if s := (&model.Project{}).CoverSides(); s == nil || len(s) != 0 {
		t.Errorf("проект без обложки: %+v, нужен пустой список", s)
	}
	if z := (&model.Project{}).CoverZones(); z != nil {
		t.Errorf("проект без обложки: зоны %+v", z)
	}
}
