package layers_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// pjson — проект целиком: сравнить итог двух путей правки.
func pjson(t *testing.T, e *album.Editor) string {
	t.Helper()
	b, err := json.Marshal(edtest.P(e))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// groupEd — два места и текст под −15° на развороте 562×284; ids группы.
func groupEd(t *testing.T) (*album.Editor, []string) {
	t.Helper()
	b := edtest.Gp(270, 110, 80, 50, "b")
	b.Rot = 30
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(150, 90, 100, 60, "a"), b})
	tx := edtest.AddText(t, e, "s1", 0)
	e.SetTextBox(tx, 170, 170, 120)
	e.SetTextAngle(tx, -15)
	g := edtest.PlaceIDsOf(e)
	return e, []string{g[0], g[1], tx}
}

func TestRotateGroupOneStep(t *testing.T) {
	e, ids := groupEd(t)
	was, undo := pjson(t, e), edtest.UndoLen(e)
	s := edtest.P(e).Spreads[0]
	c0 := [2]float64{s.Grid[0].X + 50, s.Grid[0].Y + 30}
	c1 := [2]float64{s.Grid[1].X + 40, s.Grid[1].Y + 25}
	p0 := [2]float64{s.Texts[0].X, s.Texts[0].Y}
	e.RotateGroup("s1", ids, 25, 230, 140)
	edtest.MustGrid(t, e)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
	}
	s = edtest.P(e).Spreads[0]
	if s.Grid[0].Rot != 25 || s.Grid[1].Rot != 55 || s.Texts[0].Rot != 10 || s.Grid[0].W != 100 || s.Grid[1].H != 50 {
		t.Fatalf("углы и размеры: %+v %+v %+v", *s.Grid[0], *s.Grid[1], *s.Texts[0])
	}
	// Жёстко: смещения центров и угла текста от первого места повернулись на 25°.
	n0 := [2]float64{s.Grid[0].X + 50, s.Grid[0].Y + 30}
	for _, c := range []struct{ was, got [2]float64 }{
		{c1, [2]float64{s.Grid[1].X + 40, s.Grid[1].Y + 25}},
		{p0, [2]float64{s.Texts[0].X, s.Texts[0].Y}},
	} {
		ux, uy := geom.Rotate(25, c.was[0]-c0[0], c.was[1]-c0[1])
		if math.Abs(c.got[0]-n0[0]-ux) > 0.02 || math.Abs(c.got[1]-n0[1]-uy) > 0.02 {
			t.Errorf("смещение (%.3f, %.3f), ждали (%.3f, %.3f)", c.got[0]-n0[0], c.got[1]-n0[1], ux, uy)
		}
	}
	e.Undo()
	if pjson(t, e) != was {
		t.Fatal("отмена не вернула группу")
	}
}

func TestRotateGroupNoop(t *testing.T) {
	e, ids := groupEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.RotateGroup("s1", ids, 0, 230, 140)
	e.RotateGroup("s1", ids, 360, 230, 140)
	e.RotateGroup("s1", ids, 0.04, 230, 140)
	e.RotateGroup("s1", ids, math.NaN(), 230, 140)
	e.RotateGroup("s1", ids, 30, math.Inf(1), 140)
	e.RotateGroup("s1", []string{"нет такого"}, 30, 230, 140)
	e.RotateGroup("нет такого", ids, 30, 230, 140)
	edtest.Same(t, e, rev, undo, "холостые")
}

// TestRotateGroupNoLimit — угол не упирается: широкая группа доворачивается до 90°.
func TestRotateGroupNoLimit(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(21, 17, 250, 250, "a"), edtest.Gp(291, 17, 250, 250, "b")})
	e.RotateGroup("s1", edtest.PlaceIDsOf(e), 90, 281, 142)
	edtest.MustGrid(t, e)
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if pl.Rot != 90 || pl.W != 250 || pl.H != 250 {
			t.Fatalf("место %+v", *pl)
		}
	}
}

func TestRotateGroupFreezesAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	undo := edtest.UndoLen(e)
	e.RotateGroup(id, []string{"a", "b"}, 20, 140, 140)
	edtest.MustGrid(t, e)
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 3 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("мест %d, шагов %d", len(s.Grid), edtest.UndoLen(e)-undo)
	}
	if s.Grid[0].Rot != 20 || s.Grid[1].Rot != 20 || s.Grid[2].Rot != 0 {
		t.Fatalf("углы %v %v %v", s.Grid[0].Rot, s.Grid[1].Rot, s.Grid[2].Rot)
	}
	// Закрепление и поворот — один шаг: отмена возвращает «Авто».
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("отмена оставила сетку")
	}
	// Холостой поворот «Авто» не закрепляет.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.RotateGroup(id, []string{"a", "b"}, 360, 140, 140)
	edtest.Same(t, e, rev, undo, "холостой")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостой поворот закрепил «Авто»")
	}
}

func TestRotateGroupTextsOnlyNoFreeze(t *testing.T) {
	e, id := edtest.AutoEd(t)
	t1, t2 := edtest.AddText(t, e, id, 0), edtest.AddText(t, e, id, 1)
	undo := edtest.UndoLen(e)
	e.RotateGroup(id, []string{t1, t2}, 15, 281, 142)
	s := edtest.P(e).Spreads[0]
	if s.Grid != nil || edtest.UndoLen(e) != undo+1 || s.Texts[0].Rot != 15 || s.Texts[1].Rot != 15 {
		t.Fatalf("сетка %v, шагов %d, углы %v %v", s.Grid, edtest.UndoLen(e)-undo, s.Texts[0].Rot, s.Texts[1].Rot)
	}
}

func TestRotateGroupCaption(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(100, 100, 120, 80, "a"), edtest.Gp(240, 100, 120, 80, "b")})
	ref := edtest.P(e).Photos["a"]
	ref.Caption = &model.Caption{Text: "Первый танец", Style: model.DefaultCaptionStyle()}
	edtest.P(e).Photos["a"] = ref
	e.RotateGroup("s1", edtest.PlaceIDsOf(e), 30, 230, 140)
	if c := e.View().Spreads[0].Captions[0]; c.Rot != 30 {
		t.Fatalf("подпись не повернулась: %+v", c)
	}
}

func TestScaleGroupAxesFreezesAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	undo := edtest.UndoLen(e)
	e.ScaleGroupAxes(id, []string{"a", "b"}, 0.8, 1, 0, 0, 0)
	edtest.MustGrid(t, e)
	if len(edtest.P(e).Spreads[0].Grid) != 3 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("мест %d, шагов %d", len(edtest.P(e).Spreads[0].Grid), edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("отмена оставила сетку")
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, k := range [][2]float64{{1, 1}, {0, 2}, {2, -1}, {math.NaN(), 1}, {1, math.Inf(1)}} {
		e.ScaleGroupAxes(id, []string{"a", "b"}, k[0], k[1], 0, 0, 0)
	}
	e.ScaleGroupAxes(id, []string{"нет такого"}, 2, 1, 0, 0, 0)
	edtest.Same(t, e, rev, undo, "холостые")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая правка закрепила «Авто»")
	}
}

// TestScaleGroupAxesUniformSame — kx = ky = k: тот же итог, что TransformGroup(k).
func TestScaleGroupAxesUniformSame(t *testing.T) {
	e, ids := groupEd(t)
	was := pjson(t, e)
	e.ScaleGroupAxes("s1", ids, 0.7, 0.7, 20, 150, 90)
	axes := pjson(t, e)
	e.Undo()
	e.TransformGroup("s1", ids, 0.7, 150, 90, 0, 0, "")
	if got := pjson(t, e); got != axes || got == was {
		t.Fatal("kx = ky разошлось с TransformGroup")
	}
}

// TestScaleGroupAxesAligned — место под углом рамки растягивается ровно вдоль её оси.
func TestScaleGroupAxesAligned(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(150, 90, 100, 60, "a"), edtest.Gp(300, 120, 80, 50, "b")})
	g := edtest.PlaceIDsOf(e)
	e.SetPlaceAngle("s1", g[0], 20)
	e.SetPlaceAngle("s1", g[1], 20)
	undo := edtest.UndoLen(e)
	e.ScaleGroupAxes("s1", g, 1.3, 1, 20, 150, 120)
	edtest.MustGrid(t, e)
	s := edtest.P(e).Spreads[0]
	if edtest.UndoLen(e) != undo+1 || s.Grid[0].W != 130 || s.Grid[0].H != 60 || s.Grid[1].W != 104 || s.Grid[1].H != 50 || s.Grid[1].Rot != 20 {
		t.Fatalf("шагов %d, места %+v %+v", edtest.UndoLen(e)-undo, *s.Grid[0], *s.Grid[1])
	}
}

// TestScaleGroupAxesAutoText — «по тексту»: после fitTexts точка выравнивания там,
// куда её увёз масштаб, кегль — по меньшему множителю.
func TestScaleGroupAxesAutoText(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 100, 100, "a")})
	tx := edtest.AddText(t, e, "s1", 0)
	tb := edtest.P(e).Spreads[0].Texts[0]
	if !tb.Auto {
		t.Fatal("новый текст не «по тексту»")
	}
	size := tb.Style.Size
	ax0, ay0 := model.BoxOf(tb).Anchor(tb.Style.Align)
	e.ScaleGroupAxes("s1", []string{tx}, 0.5, 1, 0, 20, 20)
	tb = edtest.P(e).Spreads[0].Texts[0]
	if want := math.Round(size*0.5*10) / 10; tb.Style.Size != want || !tb.Auto {
		t.Fatalf("кегль %v, ждали %v; %+v", tb.Style.Size, want, *tb)
	}
	if tb.W != edtest.P(e).SheetAutoW(nil, tb) {
		t.Fatalf("ширина %v не по тексту", tb.W)
	}
	ax, ay := model.BoxOf(tb).Anchor(tb.Style.Align)
	if math.Abs(ax-(20+(ax0-20)*0.5)) > 0.02 || math.Abs(ay-ay0) > 0.02 {
		t.Fatalf("точка выравнивания (%.3f, %.3f), ждали (%.3f, %.3f)", ax, ay, 20+(ax0-20)*0.5, ay0)
	}
	// Растяжка по одной оси кегль не растит.
	e.ScaleGroupAxes("s1", []string{tx}, 1.5, 1, 0, 20, 20)
	if got := edtest.P(e).Spreads[0].Texts[0].Style.Size; got != math.Round(size*0.5*10)/10 {
		t.Fatalf("кегль вырос: %v", got)
	}
}

// TestScaleGroupAxesMinTextW — закреплённая ширина 12 мм × 0.5 = 6 < minTextW:
// ширина растёт до предела вокруг точки выравнивания, центр не уезжает (было 2 мм).
func TestScaleGroupAxesMinTextW(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 100, 100, "a")})
	tx := edtest.AddText(t, e, "s1", 0)
	edtest.StyleAlign(t, e, tx, "center")
	e.SetTextBox(tx, 100, 150, 12)
	tb := edtest.P(e).Spreads[0].Texts[0]
	if tb.Auto || tb.W != 12 {
		t.Fatalf("исходный блок: %+v", *tb)
	}
	ax0, ay0 := model.BoxOf(tb).Anchor("center")
	e.ScaleGroupAxes("s1", []string{tx}, 0.5, 1, 0, 20, 20)
	tb = edtest.P(e).Spreads[0].Texts[0]
	ax, ay := model.BoxOf(tb).Anchor("center")
	if tb.W != model.MinTextW || math.Abs(ax-(20+(ax0-20)*0.5)) > 0.01 || math.Abs(ay-ay0) > 0.01 {
		t.Fatalf("ширина %v, центр (%.2f, %.2f), ждали (%.2f, %.2f)", tb.W, ax, ay, 20+(ax0-20)*0.5, ay0)
	}
}

// TestTransformGroupMinTextW — пропорциональный масштаб: закреплённая ширина 12 мм × 0.5 < MinTextW,
// ширина растёт вокруг точки выравнивания, центр не уезжает (было 2 мм); место рядом едет с ним.
func TestTransformGroupMinTextW(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 100, 100, "a")})
	tx := edtest.AddText(t, e, "s1", 0)
	edtest.StyleAlign(t, e, tx, "center")
	e.SetTextBox(tx, 100, 150, 12)
	g := edtest.PlaceIDsOf(e)
	tb := edtest.P(e).Spreads[0].Texts[0]
	if tb.Auto || tb.W != 12 {
		t.Fatalf("исходный блок: %+v", *tb)
	}
	ax0, ay0 := model.BoxOf(tb).Anchor("center")
	undo := edtest.UndoLen(e)
	e.TransformGroup("s1", []string{g[0], tx}, 0.5, 20, 20, 0, 0, "")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
	}
	tb = edtest.P(e).Spreads[0].Texts[0]
	ax, ay := model.BoxOf(tb).Anchor("center")
	wx, wy := 20+(ax0-20)*0.5, 20+(ay0-20)*0.5
	if tb.W != model.MinTextW || math.Abs(ax-wx) > 0.01 || math.Abs(ay-wy) > 0.01 {
		t.Fatalf("ширина %v, центр (%.3f, %.3f), ждали (%.3f, %.3f)", tb.W, ax, ay, wx, wy)
	}
	e.Undo()
	if got := edtest.P(e).Spreads[0].Texts[0]; got.W != 12 || got.X != 100 {
		t.Fatalf("отмена не вернула текст: %+v", *got)
	}
}

// TestTransformGroupAutoAnchor — «по тексту», справа: правый край идёт за масштабом без
// накопления (ширина нового кегля ≠ W·k на сотые, раньше 0,035 и 0,043 мм), ширина — по тексту.
func TestTransformGroupAutoAnchor(t *testing.T) {
	for _, k := range []float64{0.73, 1.37} {
		e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 100, 100, "a")})
		tx := edtest.AddText(t, e, "s1", 0)
		edtest.StyleAlign(t, e, tx, "right")
		g := edtest.PlaceIDsOf(e)
		tb := edtest.P(e).Spreads[0].Texts[0]
		if !tb.Auto {
			t.Fatal("новый текст не «по тексту»")
		}
		ax0, ay0 := model.BoxOf(tb).Anchor("right")
		e.TransformGroup("s1", []string{g[0], tx}, k, 20, 20, 0, 0, "")
		kk := edtest.P(e).Spreads[0].Grid[0].W / 100 // действующий k: зажим мог его урезать
		tb = edtest.P(e).Spreads[0].Texts[0]
		ax, ay := model.BoxOf(tb).Anchor("right")
		wx, wy := 20+(ax0-20)*kk, 20+(ay0-20)*kk
		if math.Abs(ax-wx) > 0.015 || math.Abs(ay-wy) > 0.015 {
			t.Errorf("k %v: правый край (%.3f, %.3f), ждали (%.3f, %.3f)", k, ax, ay, wx, wy)
		}
		if !tb.Auto || tb.W != edtest.P(e).SheetAutoW(nil, tb) {
			t.Errorf("k %v: ширина %v не по тексту, %+v", k, tb.W, *tb)
		}
	}
}

func TestTransformGroupScaleWithText(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(120, 10, 100, 100, "b")})
	tx := edtest.AddText(t, e, "s1", 1)
	e.SetTextBox(tx, 300, 60, 120) // закреплённая ширина
	g := edtest.PlaceIDsOf(e)
	was := *edtest.P(e).Spreads[0].Texts[0]
	undo := edtest.UndoLen(e)
	// Масштаб ×0.5 от угла (10, 10): неподвижный угол стоит, остальное едет к нему.
	e.TransformGroup("s1", []string{g[0], g[1], tx}, 0.5, 10, 10, 0, 0, "")
	edtest.MustGrid(t, e)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
	}
	s := edtest.P(e).Spreads[0]
	want := []layout.Rect{{X: 10, Y: 10, W: 50, H: 50}, {X: 65, Y: 10, W: 50, H: 50}}
	for i, pl := range s.Grid {
		if (layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}) != want[i] || pl.Photo == "" {
			t.Fatalf("место %d: %+v", i, *pl)
		}
	}
	got := s.Texts[0]
	if math.Abs(got.X-(10+(was.X-10)*0.5)) > 0.011 || math.Abs(got.Y-(10+(was.Y-10)*0.5)) > 0.011 || got.W != 60 || got.Auto {
		t.Fatalf("текст: %+v, был %+v", *got, was)
	}
	if size := math.Round(was.Style.Size*0.5*10) / 10; got.Style.Size != size {
		t.Fatalf("кегль %v, ждали %v", got.Style.Size, size)
	}
	// Отмена возвращает всё одним шагом.
	e.Undo()
	if *edtest.P(e).Spreads[0].Texts[0] != was || edtest.P(e).Spreads[0].Grid[1].X != 120 {
		t.Fatal("отмена не вернула группу")
	}
}

func TestTransformGroupClamp(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(120, 10, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	sw, ph := 562.0, 284.0 // страница 281×284, разворот ×2
	// Огромный k: группа занимает не больше разворота, d прижимает её внутрь.
	e.TransformGroup("s1", g, 50, 10, 10, 9999, -9999, "")
	edtest.MustGrid(t, e)
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		x0, y0 = math.Min(x0, pl.X), math.Min(y0, pl.Y)
		x1, y1 = math.Max(x1, pl.X+pl.W), math.Max(y1, pl.Y+pl.H)
	}
	if x1-x0 > sw+0.02 || y1-y0 > ph+0.02 || x0 < -0.01 || y0 < -0.01 || x1 > sw+0.01 || y1 > ph+0.01 {
		t.Fatalf("группа вышла из разворота: %v %v %v %v", x0, y0, x1, y1)
	}
	if y0 != 0 { // dy = −9999 прижал вверх
		t.Fatalf("y0 = %v", y0)
	}
	// Высокая группа (gh/PH > gw/SW): масштаб ограничивает высота разворота.
	e3 := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 40, 120, "a")})
	e3.TransformGroup("s1", edtest.PlaceIDsOf(e3), 50, 10, 10, 0, 0, "")
	if pl := edtest.P(e3).Spreads[0].Grid[0]; pl.H > ph+0.01 || pl.H < ph-0.02 || pl.W > 40*ph/120+0.02 {
		t.Fatalf("высокая группа: %+v", *pl)
	}
	// Крошечный k: места не меньше geom.MinPlace.
	e.TransformGroup("s1", g, 0.001, 0, 0, 0, 0, "")
	edtest.MustGrid(t, e)
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if pl.W < geom.MinPlace-0.01 || pl.H < geom.MinPlace-0.01 {
			t.Fatalf("место меньше предела: %+v", *pl)
		}
	}
	// Пределы кегля: текст в группе не даёт k опуститься ниже MinSize/size.
	e2 := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	tx := edtest.AddText(t, e2, "s1", 1)
	size := edtest.P(e2).Spreads[0].Texts[0].Style.Size
	e2.TransformGroup("s1", []string{tx}, 0.001, 0, 0, 0, 0, "")
	if got := edtest.P(e2).Spreads[0].Texts[0].Style.Size; got != text.MinSize {
		t.Fatalf("кегль %v при исходном %v, ждали %v", got, size, text.MinSize)
	}
}

func TestTransformGroupFreezesAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	undo := edtest.UndoLen(e)
	e.TransformGroup(id, []string{"a", "b"}, 0.8, 0, 0, 5, 7, "")
	edtest.MustGrid(t, e)
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 3 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("мест %d, шагов %d", len(s.Grid), edtest.UndoLen(e)-undo)
	}
	// Закрепление и правка — один шаг: отмена возвращает «Авто».
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("отмена оставила сетку")
	}
	// Холостые правки не закрепляют и не оставляют шага.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.TransformGroup(id, []string{"a", "b"}, 1, 0, 0, 0, 0, "")
	e.TransformGroup(id, []string{"a"}, math.NaN(), 0, 0, 0, 0, "")
	e.TransformGroup(id, []string{"a"}, -1, 0, 0, 0, 0, "")
	e.TransformGroup(id, []string{"нет такого"}, 0.5, 0, 0, 3, 3, "")
	edtest.Same(t, e, rev, undo, "холостые")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая правка закрепила «Авто»")
	}
}

func TestTransformGroupTextsOnlyNoFreeze(t *testing.T) {
	e, id := edtest.AutoEd(t)
	t1 := edtest.AddText(t, e, id, 0)
	was := edtest.P(e).Spreads[0].Texts[0].Style.Size
	undo := edtest.UndoLen(e)
	e.TransformGroup(id, []string{t1}, 1.5, 100, 100, 0, 0, "")
	s := edtest.P(e).Spreads[0]
	if s.Grid != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("сетка %v, шагов %d", s.Grid, edtest.UndoLen(e)-undo)
	}
	if got := s.Texts[0].Style.Size; got != math.Round(was*1.5*10)/10 {
		t.Fatalf("кегль %v, был %v", got, was)
	}
	// Блок «по тексту» остаётся таким, ширину пересчитал fitTexts.
	if !s.Texts[0].Auto || s.Texts[0].W != edtest.P(e).SheetAutoW(nil, s.Texts[0]) {
		t.Fatalf("ширина: %+v", *s.Texts[0])
	}
}

func TestTransformGroupMerge(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(120, 10, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	undo := edtest.UndoLen(e)
	e.TransformGroup("s1", g, 1, 0, 0, 5, 0, "s")
	e.TransformGroup("s1", g, 1, 0, 0, 5, 0, "s")
	if edtest.UndoLen(e) != undo+1 || edtest.P(e).Spreads[0].Grid[0].X != 20 {
		t.Fatalf("серия: шагов %d, x %v", edtest.UndoLen(e)-undo, edtest.P(e).Spreads[0].Grid[0].X)
	}
	// Вернулись к началу — шага нет.
	e.TransformGroup("s1", g, 1, 0, 0, -10, 0, "s")
	if edtest.UndoLen(e) != undo || edtest.P(e).Spreads[0].Grid[0].X != 10 {
		t.Fatalf("возврат: шагов %d, x %v", edtest.UndoLen(e)-undo, edtest.P(e).Spreads[0].Grid[0].X)
	}
	// Другая сессия или пустая — отдельный шаг.
	e.TransformGroup("s1", g, 1, 0, 0, 5, 0, "s")
	e.TransformGroup("s1", g, 1, 0, 0, 5, 0, "")
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов %d", edtest.UndoLen(e)-undo)
	}
	// У «Авто» с фото merge не действует: закрепление не влипает в серию.
	e2, id := edtest.AutoEd(t)
	u := edtest.UndoLen(e2)
	e2.TransformGroup(id, []string{"a"}, 1, 0, 0, 5, 0, "s")
	e2.TransformGroup(id, []string{"a"}, 1, 0, 0, 5, 0, "s")
	if edtest.UndoLen(e2) != u+2 {
		t.Fatalf("«Авто»: шагов %d", edtest.UndoLen(e2)-u)
	}
}

// TestTransformGroupKeepsPlaceFields — поля места, кроме геометрии, переживают правку.
func TestTransformGroupKeepsPlaceFields(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{{X: 10, Y: 10, W: 100, H: 100, Photo: "a", Fit: model.FitContain}})
	e.TransformGroup("s1", []string{"a"}, 0.5, 0, 0, 0, 0, "")
	pl := edtest.P(e).Spreads[0].Grid[0]
	if pl.Photo != "a" || pl.Fit != model.FitContain || pl.W != 50 {
		t.Fatalf("%+v", *pl)
	}
}

// fieldAnchorEd — разворот 562×284: места 100×60 под 30° и 80×120, текст под −15° (кегль 32).
// Возвращает редактор и ids группы (места и текст).
func fieldAnchorEd(t *testing.T) (*album.Editor, []string) {
	t.Helper()
	a, b := edtest.Gp(60, 40, 100, 60, "a"), edtest.Gp(200, 30, 80, 120, "b")
	a.Rot = 30
	e := edtest.GridEd(t, []*model.Place{a, b})
	tx := edtest.AddText(t, e, "s1", 1)
	e.SetTextBox(tx, 330, 100, 120)
	tb := edtest.P(e).Spreads[0].Texts[0]
	tb.Rot, tb.Style.Size = -15, 32
	g := edtest.PlaceIDsOf(e)
	return e, []string{g[0], g[1], tx}
}

// groupBB — описанная рамка группы (aabb мест и текстов, как рамка в UI) и она же по hold.
func groupBB(e *album.Editor) (bb, hd layout.Rect) {
	s := edtest.P(e).Spreads[0]
	var bs, hs []layout.Rect
	for _, pl := range s.Grid {
		bs, hs = append(bs, model.PlaceAABB(pl)), append(hs, model.Hold(pl))
	}
	for _, tb := range s.Texts {
		r := model.BoxOf(tb).AABB()
		bs, hs = append(bs, r), append(hs, r)
	}
	union := func(rs []layout.Rect) layout.Rect {
		x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, r := range rs {
			x0, y0 = math.Min(x0, r.X), math.Min(y0, r.Y)
			x1, y1 = math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
		}
		return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	return union(bs), union(hs)
}

// TestTransformGroupFieldAnchor — контракт полей карточки группы (спека 014, design §2):
// Ш = TransformGroup(k = v/bb.w, a = левый верхний угол рамки), X = перенос на v − bb.x.
// Угол рамки стоит, пока группа помещается; у края группа сдвигается внутрь; шаг отмены один.
func TestTransformGroupFieldAnchor(t *testing.T) {
	const tol = 0.02
	near := func(what string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > tol {
			t.Errorf("%s: %.3f, ждали %.3f", what, got, want)
		}
	}
	t.Run("помещается", func(t *testing.T) {
		e, ids := fieldAnchorEd(t)
		bb, _ := groupBB(e)
		v := math.Round(bb.W*1.1*100) / 100
		undo := edtest.UndoLen(e)
		e.TransformGroup("s1", ids, v/bb.W, bb.X, bb.Y, 0, 0, "")
		got, _ := groupBB(e)
		near("x", got.X, bb.X)
		near("y", got.Y, bb.Y)
		near("w", got.W, v)
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
		}
		edtest.MustGrid(t, e)
	})
	t.Run("у правого края — сдвиг внутрь", func(t *testing.T) {
		e, ids := fieldAnchorEd(t)
		bb, _ := groupBB(e)
		v := math.Round(bb.W*1.3*100) / 100
		undo := edtest.UndoLen(e)
		e.TransformGroup("s1", ids, v/bb.W, bb.X, bb.Y, 0, 0, "")
		got, hd := groupBB(e)
		near("w", got.W, v)
		if got.X >= bb.X-1 {
			t.Errorf("группа не сдвинулась внутрь: x %.2f, был %.2f", got.X, bb.X)
		}
		if hd.X < -tol || hd.X+hd.W > 562+tol {
			t.Errorf("hold вне разворота: %+v", hd)
		}
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
		}
		edtest.MustGrid(t, e)
	})
	t.Run("X переносит рамку", func(t *testing.T) {
		e, ids := fieldAnchorEd(t)
		bb, _ := groupBB(e)
		v := bb.X + 25
		undo := edtest.UndoLen(e)
		e.TransformGroup("s1", ids, 1, 0, 0, v-bb.X, 0, "")
		got, _ := groupBB(e)
		near("x", got.X, v)
		near("y", got.Y, bb.Y)
		near("w", got.W, bb.W)
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
		}
	})
}
