package view_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// fabrikaEd — обложка Фабрики «Фотобумага» на странице формата f (кнопка: зона, обрез, layflat, 300 dpi).
func fabrikaEd(t *testing.T, f model.PrintFormat) *album.Editor {
	t.Helper()
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	s := edtest.P(e).Settings
	s.PageW, s.PageH, s.DPI, s.Layflat, s.Safe, s.Bleed = f.PageW, f.PageH, 300, true, f.Safe, f.Bleed
	e.SetSettings(s)
	if _, err := e.ApplyCoverPreset("fabrika-photo"); err != nil {
		t.Fatal(err)
	}
	return e
}

func rectNear(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X)+math.Abs(a.Y-b.Y)+math.Abs(a.W-b.W)+math.Abs(a.H-b.H) < 1e-6
}

func zoneRects(c *model.CoverView, kind string) []layout.Rect {
	var out []layout.Rect
	for _, z := range c.Zones {
		if z.Kind == kind {
			out = append(out, z.Rect)
		}
	}
	return out
}

func wantRects(t *testing.T, what string, got, want []layout.Rect) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %+v, ждали %+v", what, got, want)
		return
	}
	for i := range want {
		if !rectNear(got[i], want[i]) {
			t.Errorf("%s[%d]: %+v, ждали %+v", what, i, got[i], want[i])
		}
	}
}

// guideRects — ожидаемые зоны по формулам PSD: g — направляющие формата, cg — лист обложки.
func guideRects(g model.PrintGuide, cg model.CoverGeo) (trim layout.Rect, wrap, safe []layout.Rect) {
	w, h := cg.X[7], cg.Y[3]
	trim = layout.Rect{X: g.FoldX, Y: g.FoldY, W: w - 2*g.FoldX, H: h - 2*g.FoldY}
	wrap = []layout.Rect{{W: w, H: g.FoldY}, {Y: h - g.FoldY, W: w, H: g.FoldY}, {Y: g.FoldY, W: g.FoldX, H: h - 2*g.FoldY}, {X: w - g.FoldX, Y: g.FoldY, W: g.FoldX, H: h - 2*g.FoldY}}
	y, sh := g.FoldY+g.FoldSafe, h-2*g.FoldY-2*g.FoldSafe
	safe = []layout.Rect{
		{X: g.FoldX + g.FoldSafe, Y: y, W: cg.X[2] - g.SpineSafe - g.FoldX - g.FoldSafe, H: sh},
		{X: cg.X[5] + g.SpineSafe, Y: y, W: w - g.FoldX - g.FoldSafe - cg.X[5] - g.SpineSafe, H: sh},
	}
	return trim, wrap, safe
}

// TestCoverZonesAxes (123 Б5, 0.26-123-35): обложка Фабрики по шаблону рисует загиб по осям PSD,
// риск шаблона в 3 мм от него и 12 мм от корешка — на каждом из пяти форматов; со своими
// Safe/SpineSafe, не по шаблону и у Фотосферы — прежний расчёт от края сторонки. Данные альбома не
// меняются: ни шага отмены, ни rev, а после Save → Load зоны те же.
func TestCoverZonesAxes(t *testing.T) {
	for _, f := range model.FabrikaFormats() {
		e := fabrikaEd(t, f)
		cg := edtest.P(e).CoverGeo()
		undo, rev := edtest.UndoLen(e), edtest.Rev(e)
		c := e.View().Cover
		trim, wrap, safe := guideRects(f.Guide, cg)
		wantRects(t, f.Name+" загиб", zoneRects(c, "trim"), []layout.Rect{trim})
		wantRects(t, f.Name+" заворот", zoneRects(c, "wrap"), wrap)
		wantRects(t, f.Name+" риск", zoneRects(c, "safe"), safe)
		wantRects(t, f.Name+" CoverView.Safe", c.Safe, safe)
		// Сторонки и корешок — геометрия листа, направляющие их не двигают.
		if b := zoneRects(c, "back"); len(b) != 1 || !rectNear(b[0], layout.Rect{X: cg.X[1], Y: cg.Y[1], W: f.SideW, H: f.SideH}) {
			t.Errorf("%s: задняя %+v, сторонка %v×%v", f.Name, b, f.SideW, f.SideH)
		}
		if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
			t.Errorf("%s: чтение вида создало шаг отмены или rev", f.Name)
		}
		// Save → Load: направляющие — производные, из файла они возникают заново.
		q := edtest.Open(t, edtest.SaveLoad(t, e), "")
		wantRects(t, f.Name+" после Save → Load", zoneRects(q.View().Cover, "safe"), safe)
		wantRects(t, f.Name+" загиб после Save → Load", zoneRects(q.View().Cover, "trim"), []layout.Rect{trim})
	}

	// 30×20, 2 разворота, числами: лист 662 × 240, загиб 20,5 / 18,5, риск 3, корешок 8 и 12 от него.
	var f model.PrintFormat
	for _, x := range model.FabrikaFormats() {
		if x.Name == "30×20" {
			f = x
		}
	}
	c := fabrikaEd(t, f).View().Cover
	if c.Sheet.W != 662 || c.Sheet.H != 240 || c.Spine != 8 {
		t.Fatalf("лист %v × %v, корешок %v", c.Sheet.W, c.Sheet.H, c.Spine)
	}
	wantRects(t, "30×20 загиб", zoneRects(c, "trim"), []layout.Rect{{X: 20.5, Y: 18.5, W: 621, H: 203}})
	wantRects(t, "30×20 риск", zoneRects(c, "safe"), []layout.Rect{{X: 23.5, Y: 21.5, W: 291.5, H: 197}, {X: 347, Y: 21.5, W: 291.5, H: 197}})

	// Прежний расчёт: край сторонки — граница обреза, заворот листа со всех сторон, 0 — Settings.Safe (10).
	old := func(what string, e *album.Editor) {
		t.Helper()
		c := e.View().Cover
		cg := edtest.P(e).CoverGeo()
		wantRects(t, what+" обрез", zoneRects(c, "trim"), []layout.Rect{{X: cg.X[1], Y: cg.Y[1], W: cg.X[6] - cg.X[1], H: cg.Y[2] - cg.Y[1]}})
		wantRects(t, what+" заворот", zoneRects(c, "wrap")[:2], []layout.Rect{{W: cg.X[7], H: cg.Y[1]}, {Y: cg.Y[2], W: cg.X[7], H: cg.Y[1]}})
	}
	fab := func() *album.Editor { return fabrikaEd(t, f) }

	e := fab()
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe = 5 })
	old("свой Safe", e)
	wantRects(t, "свой Safe: линия", zoneRects(e.View().Cover, "safe")[:1], []layout.Rect{{X: 17 + 5, Y: 17 + 5, W: 17 + 310 - 10 - 22, H: 206 - 10}})
	e = fab()
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.SpineSafe = 6 })
	old("свой SpineSafe", e)
	e = fab()
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Wrap = 15 }) // не по шаблону
	old("заворот не шаблона", e)
	e = fab()
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Steps = []model.SpineStep{{Upto: 99, Mm: 5}} })
	old("таблица не шаблона", e)
	if _, err := e.ApplyCoverPreset(model.CoverDefault); err != nil { // Фотосфера на той же странице
		t.Fatal(err)
	}
	if g, ok := edtest.P(e).CoverGuide(); ok {
		t.Fatalf("Фотосфера: направляющие %+v", g)
	}
	old("Фотосфера", e)
	// Отмена правки Safe возвращает направляющие: они не зависят от истории, только от данных.
	e = fab()
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe = 5 })
	e.Undo()
	trim, _, _ := guideRects(f.Guide, edtest.P(e).CoverGeo())
	wantRects(t, "после отмены Safe", zoneRects(e.View().Cover, "trim"), []layout.Rect{trim})
}

// TestBleedStripsAxes (123 Б5): полоса обреза разворота по осям — у 25×25 Фабрики 3 / 3,5, у 30×30
// 3 / 4, у остальных 3 / 3; Settings.Bleed остаётся 3 (ключ кнопки), правка обреза рукой — одно число.
func TestBleedStripsAxes(t *testing.T) {
	for _, f := range model.FabrikaFormats() {
		e := fabrikaEd(t, f)
		st := edtest.P(e).Settings
		g := edtest.P(e).Geo(nil)
		bx, by := f.Guide.BleedX, f.Guide.BleedY
		want := []layout.Rect{{W: g.W, H: by}, {Y: g.H - by, W: g.W, H: by}, {Y: by, W: bx, H: g.H - 2*by}, {X: g.W - bx, Y: by, W: bx, H: g.H - 2*by}}
		v := e.View()
		wantRects(t, f.Name+" обрез", v.Bleed, want)
		if v.Settings.Bleed != 3 || st.Bleed != 3 {
			t.Errorf("%s: Settings.Bleed %v, ключ кнопки должен остаться 3", f.Name, v.Settings.Bleed)
		}
	}
	// Рукой: 5 мм по обеим осям, как до Б5; 0 — полос нет.
	e := fabrikaEd(t, model.FabrikaFormats()[2]) // 25×25
	s := edtest.P(e).Settings
	s.Bleed = 5
	e.SetSettings(s)
	g := edtest.P(e).Geo(nil)
	wantRects(t, "обрез 5", e.View().Bleed, []layout.Rect{{W: g.W, H: 5}, {Y: g.H - 5, W: g.W, H: 5}, {Y: 5, W: 5, H: g.H - 10}, {X: g.W - 5, Y: 5, W: 5, H: g.H - 10}})
	s.Bleed = 0
	e.SetSettings(s)
	if b := e.View().Bleed; b != nil {
		t.Errorf("обрез 0: %+v", b)
	}
}
