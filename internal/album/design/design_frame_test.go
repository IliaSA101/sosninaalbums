package design_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Поворот фото в рамке (спека 010, design-rotation §2.2) и зеркало (SetFlip).

// coversCell — фото img, повёрнутое на r вокруг своего центра, закрывает ячейку cell:
// все её углы, переведённые в оси фото, лежат внутри img.
func coversCell(img, cell layout.Rect, r float64) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	const eps = 1e-7
	for _, c := range [4][2]float64{{0, 0}, {cell.W, 0}, {0, cell.H}, {cell.W, cell.H}} {
		u, v := geom.Rotate(-r, cell.X+c[0]-cx, cell.Y+c[1]-cy)
		if math.Abs(u) > img.W/2+eps || math.Abs(v) > img.H/2+eps {
			return false
		}
	}
	return true
}

// Ничья round(R/90): ±45, ±135 не должны дважды читаться как разные пропорции.
func TestFrameTiltTie(t *testing.T) {
	for _, c := range []struct {
		in, r float64
		turn  int
		tilt  float64
	}{
		{45, 45.1, 1, -44.9}, {-45, -45.1, 3, 44.9},
		{135, 135.1, 2, -44.9}, {-135, -135.1, 2, 44.9},
		{44.9, 44.9, 0, 44.9}, {-44.9, -44.9, 0, -44.9},
		{90, 90, 1, 0}, {-90, -90, 3, 0}, {180, 180, 2, 0}, {-180, 180, 2, 0}, {270.04, -90, 3, 0},
		{50, 50, 1, -40},
	} {
		r := model.ClampR(c.in)
		turn, tilt := model.SplitR(r)
		if r != c.r || turn != c.turn || tilt != c.tilt || model.ClampR(r) != r || math.Signbit(tilt) && tilt == 0 {
			t.Errorf("R=%v: clampR %v, turn %d, tilt %v; ждали %v, %d, %v", c.in, r, turn, tilt, c.r, c.turn, c.tilt)
		}
	}

	e, _, _ := edtest.NewTwo(t)
	cellA := func() layout.Rect { return edtest.RectOf(e.View(), 0, "a") }
	cell0 := cellA()
	// Наклон ±50 зажимается; пропорции и раскладка стоят.
	for _, tilt := range []float64{50, -50, 44.9, -44.9, 0.04} {
		v := e.SetFrameAngle("a", tilt)
		f := frameOf(t, v, "a")
		want := math.Max(-44.9, math.Min(44.9, math.Round(tilt*10)/10))
		if math.Abs(want) < 0.05 {
			want = 0
		}
		if f.Tilt != want || f.Turn != 0 || f.R != want || cellA() != cell0 {
			t.Fatalf("наклон %v: %+v, ячейка %v → %v", tilt, f, cell0, cellA())
		}
	}
	// ↻ из крайнего наклона: наклон остаётся, четверть +1, пропорции меняются один раз.
	e.SetFrameAngle("a", 44.9)
	f := frameOf(t, e.TurnPhotos([]string{"a"}, 1), "a")
	if f.Turn != 1 || f.Tilt != 44.9 || f.R != 134.9 {
		t.Fatalf("↻ из 44,9: %+v", f)
	}
	e.SetFrameAngle("a", -44.9)
	f = frameOf(t, e.View(), "a")
	if f.Turn != 1 || f.Tilt != -44.9 || f.R != 45.1 {
		t.Fatalf("наклон −44,9 при ↻: %+v", f)
	}
	before := edtest.P(e).Aspects([]string{"a"})[0]
	e.SetFrameAngle("a", 44.9)
	if edtest.P(e).Aspects([]string{"a"})[0] != before {
		t.Fatal("наклон поменял пропорции")
	}
	// NaN/Inf и тот же наклон — холостая.
	n := edtest.UndoLen(e)
	e.SetFrameAngle("a", math.NaN())
	e.SetFrameAngle("a", math.Inf(1))
	e.SetFrameAngle("a", 44.9)
	e.SetFrameAngle("нет", 10)
	if edtest.UndoLen(e) != n {
		t.Fatal("холостая правка записана")
	}
}

// Сдвиг, зум, зеркало и сброс не теряют поворот.
func TestSetFrameKeepsR(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetFrameAngle("a", 10)
	if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != (model.Frame{S: 1, R: 10}) {
		t.Fatalf("наклон: %+v", got)
	}
	e.SetFrame("a", 2, 0.5, -0.5)
	if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != (model.Frame{S: 2, X: 0.5, Y: -0.5, R: 10}) {
		t.Fatalf("SetFrame: %+v", got)
	}
	e.SetFlip([]string{"a"}, "h")
	if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != (model.Frame{S: 2, X: -0.5, Y: -0.5, R: -10}) {
		t.Fatalf("SetFlip: %+v", got)
	}
	e.ResetFrame("a")
	if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != (model.Frame{S: 1, R: -10}) {
		t.Fatalf("ResetFrame: %+v", got)
	}
	e.SetFrameAngle("a", 0)
	if edtest.P(e).Photos["a"].Frame != nil {
		t.Fatalf("кадр по умолчанию — nil: %+v", edtest.P(e).Photos["a"].Frame)
	}
}

// Зеркало меняет знак наклона по любой оси, повтор возвращает; четверти не теряются.
func TestFlipNegatesFrameR(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetFrame("a", 2, 0.25, 0.5)
	e.SetFrameAngle("a", 12.5)
	e.TurnPhotos([]string{"a"}, 1) // R = 102.5
	for _, c := range []struct {
		axis string
		want model.Frame
	}{
		{"h", model.Frame{S: 2, X: -0.25, Y: 0.5, R: -102.5}},
		{"v", model.Frame{S: 2, X: -0.25, Y: -0.5, R: 102.5}},
	} {
		e.SetFlip([]string{"a"}, c.axis)
		if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != c.want {
			t.Fatalf("%s: %+v, ждали %+v", c.axis, got, c.want)
		}
	}
	e.SetFlip([]string{"a"}, "h")
	e.SetFlip([]string{"a"}, "v")
	if got := edtest.P(e).Photos["a"].FrameOrDefault(); got != (model.Frame{S: 2, X: 0.25, Y: 0.5, R: 102.5}) {
		t.Fatalf("круг: %+v", got)
	}
	// 180°: знак не меняется, -0 не появляется.
	e.SetFrameAngle("b", 0)
	e.TurnPhotos([]string{"b"}, 2)
	e.SetFlip([]string{"b"}, "h")
	if got := edtest.P(e).Photos["b"].FrameOrDefault(); got != (model.Frame{S: 1, R: 180}) {
		t.Fatalf("180°: %+v", got)
	}
	// Два фото: включено не у всех — включается у недостающего, у остальных стоит.
	e, _, _ = edtest.NewTwo(t)
	e.SetFrameAngle("a", 20)
	e.SetFrameAngle("b", 30)
	e.SetFlip([]string{"a"}, "v")
	e.SetFlip([]string{"a", "b"}, "v")
	if a, b := edtest.P(e).Photos["a"].FrameOrDefault().R, edtest.P(e).Photos["b"].FrameOrDefault().R; a != -20 || b != -30 {
		t.Fatalf("↕ двоим: a %v, b %v", a, b)
	}
	e.SetFlip([]string{"a", "b"}, "v")
	if a, b := edtest.P(e).Photos["a"].FrameOrDefault().R, edtest.P(e).Photos["b"].FrameOrDefault().R; a != 20 || b != 30 {
		t.Fatalf("снятие: a %v, b %v", a, b)
	}
	// 200 → −160, зеркало → 160.
	e.TurnPhotos([]string{"a"}, 2)
	e.SetFlip([]string{"a"}, "h")
	if r := edtest.P(e).Photos["a"].FrameOrDefault().R; r != 160 {
		t.Fatalf("200 → −160 → зеркало: %v", r)
	}
}

// ↻ меняет действующие пропорции, геометрия фото — по файлу; фото на фоне не искажается.
func TestTurnPhotosSwapsAspect(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	rev := e.View().Rev
	v := e.TurnPhotos([]string{"a", "a", "нет"}, 1)
	if v.Rev != rev+1 {
		t.Fatalf("TurnPhotos — не одна правка: %d → %d", rev, v.Rev)
	}
	if got := edtest.P(e).Aspects([]string{"a", "b"}); math.Abs(got[0]-2.0/3) > 1e-9 || got[1] != 1.5 {
		t.Fatalf("aspects %v", got)
	}
	if edtest.P(e).RawAspect("a") != 1.5 {
		t.Fatal("rawAspect не должен меняться")
	}
	f := frameOf(t, v, "a")
	if f.Turn != 1 || f.R != 90 || math.Abs(f.Img.W/f.Img.H-1.5) > 1e-9 {
		t.Fatalf("кадр: %+v", f)
	}
	if !coversCell(f.Img, edtest.RectOf(v, 0, "a"), f.R) {
		t.Fatal("повёрнутое фото не закрывает ячейку")
	}
	if frameOf(t, v, "b").Turn != 0 {
		t.Fatal("b повернулось")
	}
	// Холостые: другое число четвертей, нет фото; 4 раза ↻ — круг.
	n := edtest.UndoLen(e)
	e.TurnPhotos([]string{"a"}, 0)
	e.TurnPhotos([]string{"a"}, 3)
	e.TurnPhotos([]string{"нет"}, 1)
	if edtest.UndoLen(e) != n {
		t.Fatal("холостая правка записана")
	}
	for range 3 {
		e.TurnPhotos([]string{"a"}, 1)
	}
	if edtest.P(e).Photos["a"].Frame != nil {
		t.Fatalf("после 4×↻ кадр %+v", edtest.P(e).Photos["a"].Frame)
	}
	e.TurnPhotos([]string{"a"}, -1)
	if edtest.P(e).Photos["a"].FrameOrDefault().R != -90 {
		t.Fatalf("↺: %+v", edtest.P(e).Photos["a"].FrameOrDefault())
	}
	// «Под фото» в сетке — по действующим пропорциям повёрнутого фото.
	if _, err := e.ArrangePlaces(edtest.P(e).Spreads[0].ID, "fitPhoto", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	pl := edtest.P(e).Spreads[0].PlaceRef("a")
	if pl == nil || math.Abs(pl.W/pl.H-2.0/3) > 0.01 {
		t.Fatalf("«Под фото» повёрнутого: %+v", pl)
	}
}

// «Авто»: ↻ пересобирает разворот под новые пропорции, ячейка — под повёрнутое фото.
func TestTurnPhotosAutoRelayout(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	c0 := edtest.RectOf(e.View(), 0, "a")
	v := e.TurnPhotos([]string{"a"}, 1)
	c1 := edtest.RectOf(v, 0, "a")
	if c0 == c1 || c1.W/c1.H > 1 || c0.W/c0.H < 1 {
		t.Fatalf("ячейка %+v → %+v", c0, c1)
	}
	if v.Spreads[0].Grid {
		t.Fatal("разворот закрепился после ↻")
	}
	f := frameOf(t, v, "a")
	// Рамка без обрезки: описанный прямоугольник рамки в осях фото совпадает с фото.
	if math.Abs(f.Img.W-c1.H) > 1e-6 || math.Abs(f.Img.H-c1.W) > 1e-6 {
		t.Fatalf("фото %+v в ячейке %+v: ждали без обрезки", f.Img, c1)
	}
	// Ключ раскладки действует для новых пропорций.
	if _, ok := layout.Build(v.Spreads[0].Layout, edtest.P(e).Aspects([]string{"a"}), edtest.P(e).Params()); !ok {
		t.Fatal("раскладка не строится")
	}
	// Наклон раскладку не трогает.
	e.SetFrameAngle("a", 20)
	if edtest.RectOf(e.View(), 0, "a") != c1 {
		t.Fatal("наклон пересобрал разворот")
	}
	if e.View().Spreads[0].Grid {
		t.Fatal("разворот закрепился после наклона")
	}
}

// В «Сетке» ↻ не двигает место: геометрия фото — по файлу.
func TestTurnPhotosGridKeepsPlace(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 120, 80, "a")})
	pl := *edtest.P(e).Spreads[0].Grid[0]
	v := e.TurnPhotos([]string{"a"}, 1)
	if got := *edtest.P(e).Spreads[0].Grid[0]; got != pl {
		t.Fatalf("место %+v → %+v", pl, got)
	}
	f := frameOf(t, v, "a")
	if f.Turn != 1 || math.Abs(f.Img.W/f.Img.H-1.5) > 1e-9 || f.Img.W < 80 {
		t.Fatalf("кадр %+v", f)
	}
	// contain: повёрнутое фото целиком в месте.
	edtest.P(e).Spreads[0].Grid[0].Fit = model.FitContain
	f = frameOf(t, e.View(), "a")
	if f.R != 90 || f.S != 1 || f.Img.W > 80+1e-9 || f.Img.H > 120+1e-9 || math.Abs(f.Img.W-80) > 1e-9 {
		t.Fatalf("contain: %+v", f)
	}
}

// Фон берёт пропорции файла, а не фото в месте: ↻ на месте не растягивает тот же снимок на фоне.
func TestBackdropRawAspectTurned(t *testing.T) {
	e, _, s2 := edtest.BdEd(t)
	e.TurnPhotos([]string{"a"}, 1)
	v := edtest.SetBg(t, e, s2, "spread", "a")
	var bd *model.BackdropView
	for i, b := range v.Spreads[1].Backdrops {
		if b.Photo == "a" {
			bd = &v.Spreads[1].Backdrops[i]
		}
	}
	if bd == nil {
		t.Fatal("фона нет")
	}
	if math.Abs(bd.Img.W/bd.Img.H-1.5) > 1e-9 {
		t.Fatalf("Img %+v: ждали пропорции файла 3:2", bd.Img)
	}
}

// Рамка фона R не носит: clampBackdrop его сбрасывает.
func TestBackdropIgnoresFrameR(t *testing.T) {
	e, _, _ := edtest.BdEd(t)
	b := &model.Backdrop{Photo: "a", X: 0, Y: 0, W: 100, H: 100, Frame: &model.Frame{S: 2, X: 0.5, R: 30}}
	got := edtest.P(e).ClampBackdrop(b, "spread")
	if got == nil || got.Frame == nil || got.Frame.R != 0 || got.Frame.S != 2 || got.Frame.X != 0.5 {
		t.Fatalf("кадр фона: %+v", got)
	}
	b.Frame = &model.Frame{S: 1, R: 30}
	if got := edtest.P(e).ClampBackdrop(b, "spread"); got.Frame != nil {
		t.Fatalf("кадр по умолчанию → nil: %+v", got.Frame)
	}
	// ↻ фото на месте фон не трогает: у фона R остаётся 0.
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	v := e.TurnPhotos([]string{"bg"}, 1)
	if bd := v.Spreads[0].Backdrops; len(bd) != 1 || bd[0].Frame.R != 0 {
		t.Fatalf("фон после TurnPhotos: %+v", bd)
	}
	if r := edtest.BdOf(e, s1, "left").Frame; r != nil && r.R != 0 {
		t.Fatalf("Frame.R фона: %+v", r)
	}
}

// flipCell — ячейка фото id в виде.
func flipCell(t *testing.T, v model.View, id string) layout.Rect {
	t.Helper()
	for _, s := range v.Spreads {
		for i, pid := range s.Photos {
			if pid == id {
				return s.Rects[i]
			}
		}
	}
	t.Fatalf("фото %s нет в виде", id)
	return layout.Rect{}
}

// Зеркало: Img отражается внутри ячейки — видимая часть та же, только зеркальная;
// повтор возвращает всё как было. Ячейка и раскладка не меняются.
func TestFlipKeepsVisible(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetFrame("a", 2, 0.5, -0.25)
	v0 := e.View()
	cell, img := flipCell(t, v0, "a"), frameOf(t, v0, "a").Img
	layout0 := v0.Spreads[0].Layout

	v := e.SetFlip([]string{"a"}, "h")
	f := frameOf(t, v, "a")
	if !f.FlipH || f.FlipV || f.X != -0.5 || f.Y != -0.25 || f.S != 2 {
		t.Fatalf("после ↔: %+v", f)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	if !near(f.Img.X, 2*cell.X+cell.W-img.X-img.W) || !near(f.Img.Y, img.Y) || !near(f.Img.W, img.W) || !near(f.Img.H, img.H) {
		t.Fatalf("Img %+v не зеркало %+v в ячейке %+v", f.Img, img, cell)
	}
	if flipCell(t, v, "a") != cell || v.Spreads[0].Layout != layout0 {
		t.Fatal("ячейка или раскладка изменились")
	}

	v = e.SetFlip([]string{"a"}, "v")
	f = frameOf(t, v, "a")
	if !f.FlipH || !f.FlipV || f.X != -0.5 || f.Y != 0.25 {
		t.Fatalf("после ↕: %+v", f)
	}
	e.SetFlip([]string{"a"}, "h")
	f = frameOf(t, e.SetFlip([]string{"a"}, "v"), "a")
	if f.FlipH || f.FlipV || f.X != 0.5 || f.Y != -0.25 || f.Img != img {
		t.Fatalf("повтор не вернул кадр: %+v, ждали Img %+v", f, img)
	}
	// Нулевой ход не даёт «-0» в файле.
	e.SetFrame("c", 2, 0, 0)
	e.SetFlip([]string{"c"}, "h")
	if x := edtest.P(e).Photos["c"].Frame.X; math.Signbit(x) {
		t.Fatalf("X = -0")
	}
	_ = s1
}

// Группа — всем или никому: не у всех включено → всем включить, иначе всем выключить.
// Чужие и повторные ID пропускаются; нет ни одного — холостая правка.
func TestFlipAllOrNone(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetFlip([]string{"a"}, "h")
	flags := func() [3]bool {
		return [3]bool{edtest.P(e).Photos["a"].FlipH, edtest.P(e).Photos["b"].FlipH, edtest.P(e).Photos["c"].FlipH}
	}
	e.SetFlip([]string{"a", "b", "c", "a", "нет", ""}, "h")
	if got := flags(); got != [3]bool{true, true, true} {
		t.Fatalf("не у всех → всем: %v", got)
	}
	e.SetFlip([]string{"a", "b", "c"}, "h")
	if got := flags(); got != [3]bool{} {
		t.Fatalf("у всех → никому: %v", got)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetFlip([]string{"нет", ""}, "h")
	e.SetFlip([]string{"a"}, "x")
	e.SetFlip(nil, "v")
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостая правка: rev %d → %d, undo %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	e.SetFlip([]string{"a", "b"}, "v")
	if !edtest.P(e).Photos["a"].FlipV || !edtest.P(e).Photos["b"].FlipV || edtest.P(e).Photos["a"].FlipH {
		t.Fatal("ось v задела h или не включилась")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("группа — один шаг undo, шагов +%d", edtest.UndoLen(e)-undo)
	}
}

// «Авто» не закрепляется зеркалом; в «Ручной» места те же.
func TestFlipAutoNoFreeze(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetFlip([]string{"a", "b"}, "h")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("«Авто» закреплена зеркалом")
	}
	if v := e.View(); v.Spreads[0].Grid || v.Spreads[0].ID != s1 {
		t.Fatalf("разворот стал сеткой: %+v", v.Spreads[0].Grid)
	}

	g := edtest.GridEd(t, edtest.Calendar())
	before := marshalGrid(g)
	v := g.SetFlip([]string{"a"}, "v")
	if marshalGrid(g) != before {
		t.Fatal("места изменились")
	}
	if f := frameOf(t, v, "a"); !f.FlipV || f.FlipH {
		t.Fatalf("сетка: %+v", f)
	}
	edtest.MustGrid(t, g)
}

func marshalGrid(e *album.Editor) string {
	out := ""
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		out += string(model.Marshal(&model.Project{Spreads: []*model.Spread{{Grid: []*model.Place{pl}}}}))
	}
	return out
}
