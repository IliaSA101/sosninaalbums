package backdrop_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Наклон фото на фоне (спека 045): фото крутится в стоящей рамке (Frame.R), закрывая её.

// bdView — вид фона области scope первого разворота.
func bdView(t *testing.T, v model.View, scope string) model.BackdropView {
	t.Helper()
	for _, b := range v.Spreads[0].Backdrops {
		if b.Scope == scope {
			return b
		}
	}
	t.Fatalf("фона %s нет", scope)
	return model.BackdropView{}
}

// covers — фото img (в осях рамки), повёрнутое на r вокруг своего центра, закрывает рамку w×h.
func covers(img layout.Rect, w, h, r float64) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	for _, c := range [4][2]float64{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		u, v := geom.Rotate(-r, c[0]-cx, c[1]-cy)
		if math.Abs(u) > img.W/2+1e-7 || math.Abs(v) > img.H/2+1e-7 {
			return false
		}
	}
	return true
}

// heldZoom — наклон держит приближение (057): фото того же размера (до шага кадра 0,001) и
// та же точка фото в центре рамки (доля размера фото в его осях).
func heldZoom(b0, b1 model.BackdropView) bool {
	pt := func(b model.BackdropView) (float64, float64) {
		u, v := geom.Rotate(-b.Frame.R, b.Img.X+b.Img.W/2-b.Box.W/2, b.Img.Y+b.Img.H/2-b.Box.H/2)
		return u / b.Img.W, v / b.Img.H
	}
	x0, y0 := pt(b0)
	x1, y1 := pt(b1)
	return math.Abs(b1.Img.W-b0.Img.W) <= 1e-3*b0.Img.W && math.Abs(x1-x0) < 1e-3 && math.Abs(y1-y0) < 1e-3
}

// Зажим: четверть отбрасывается, наклон ±44,9 и шаг 0,1 остаются; не число — 0; идемпотентно.
func TestClampBackdropTilt(t *testing.T) {
	e, _, _ := edtest.BdEd(t)
	p := edtest.P(e)
	for _, c := range []struct{ in, want float64 }{
		{4.5, 4.5}, {-44.9, -44.9}, {44.9, 44.9}, {95, 5}, {50, -40}, {-50, 40}, {180, 0}, {-90.04, 0},
		{4.56, 4.6}, {math.NaN(), 0}, {math.Inf(1), 0}, {0, 0},
	} {
		b := &model.Backdrop{Photo: "a", W: 100, H: 100, Frame: &model.Frame{S: 1.5, R: c.in}}
		got := p.ClampBackdrop(nil, b, "spread")
		if r := got.FrameOrDefault().R; math.Abs(r-c.want) > 1e-9 || math.Signbit(r) && r == 0 {
			t.Errorf("R %v → %v, ждали %v", c.in, r, c.want)
		}
		if again := p.ClampBackdrop(nil, got, "spread"); *again.Frame != *got.Frame {
			t.Errorf("R %v: не идемпотентно: %+v → %+v", c.in, got.Frame, again.Frame)
		}
	}
	// Огромное R из файла — конечный наклон в пределе.
	got := p.ClampBackdrop(nil, &model.Backdrop{Photo: "a", W: 100, H: 100, Frame: &model.Frame{S: 1, R: 1e300}}, "spread")
	if r := got.FrameOrDefault().R; !(r >= -model.MaxTilt && r <= model.MaxTilt) {
		t.Errorf("R 1e300 → %v вне ±44,9", r)
	}
}

// SetBackdropAngle: зажим, один шаг, холостая — тот же наклон, не число, нет фона, чужая область.
// Масштаб и сдвиг кадра стоят; два фона страниц наклоняются независимо.
func TestSetBackdropAngle(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	b0 := bdView(t, e.SetBackdropFrame(s1, "left", 1.5, 0.3, -0.2), "left")
	undo := edtest.UndoLen(e)
	v := e.SetBackdropAngle(s1, "left", 4.54, "")
	// 057: масштаб падает, а фото на экране того же размера и с той же точкой в центре.
	if b := bdView(t, v, "left"); b.Frame.R != 4.5 || !(b.Frame.S < 1.5) || !heldZoom(b0, b) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("наклон: %+v, undo %d → %d", b.Frame, undo, edtest.UndoLen(e))
	}
	if f := bdView(t, v, "right").Frame; f.R != 0 {
		t.Fatalf("правая страница: %+v", f)
	}
	if f := bdView(t, e.SetBackdropAngle(s1, "right", -60, ""), "right").Frame; f.R != -44.9 {
		t.Fatalf("упор: %+v", f)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for name, f := range map[string]func(){
		"тот же":       func() { e.SetBackdropAngle(s1, "left", 4.5, "") },
		"почти тот же": func() { e.SetBackdropAngle(s1, "left", 4.53, "") },
		"NaN":          func() { e.SetBackdropAngle(s1, "left", math.NaN(), "") },
		"Inf":          func() { e.SetBackdropAngle(s1, "left", math.Inf(-1), "") },
		"нет фона":     func() { e.SetBackdropAngle(s1, "spread", 3, "") },
		"область":      func() { e.SetBackdropAngle(s1, "top", 3, "") },
		"лист":         func() { e.SetBackdropAngle("нет", "left", 3, "") },
	} {
		f()
		edtest.Same(t, e, rev, undo, name)
	}
	if f := bdView(t, e.Undo(), "right").Frame; f.R != 0 {
		t.Fatalf("Undo: %+v", f)
	}
}

// Наклон закрывает рамку при ±4,5 и ±44,9 у областей «разворот», «левая», «правая», в том
// числе с увеличением, сдвигом и повёрнутой рамкой; Img в осях фото, кадр — в виде.
func TestBackdropTiltCovers(t *testing.T) {
	for _, scope := range []string{"spread", "left", "right"} {
		e, s1, _ := edtest.BdEd(t)
		edtest.SetBg(t, e, s1, scope, "bg")
		for _, fr := range [][3]float64{{1, 0, 0}, {1.7, 1, -1}, {2.5, -0.4, 0.8}} {
			e.SetBackdropFrame(s1, scope, fr[0], fr[1], fr[2])
			for _, tilt := range []float64{4.5, -4.5, 44.9, -44.9} {
				b0 := bdView(t, e.View(), scope)
				v := e.SetBackdropAngle(s1, scope, tilt, "")
				b := bdView(t, v, scope)
				if b.Frame.R != tilt || !covers(b.Img, b.Box.W, b.Box.H, b.Frame.R) {
					t.Errorf("%s, кадр %v, наклон %v: открыто %+v в рамке %+v", scope, fr, tilt, b.Img, b.Box)
				}
				// Размер (057): без приближения S = 1; с ним — тот же размер, либо S упёрлось в 1.
				if fr[0] == 1 && b.Frame.S != 1 || fr[0] > 1 && b.Frame.S > 1 && !heldZoom(b0, b) || b.Frame.S == 1 && b.Img.W < b0.Img.W-1e-9 {
					t.Errorf("%s, кадр %v, наклон %v: S %v → %v, ширина %v → %v", scope, fr, tilt, b0.Frame.S, b.Frame.S, b0.Img.W, b.Img.W)
				}
			}
		}
		// Повёрнутая рамка: наклон тот же, Img в осях рамки — закрыта так же.
		e.SetBackdropBox(s1, scope, 20, 10, 150, 120, 30, "")
		b := bdView(t, e.SetBackdropAngle(s1, scope, -20, ""), scope)
		if b.Box.Rot != 30 || b.Frame.R != -20 || !covers(b.Img, b.Box.W, b.Box.H, -20) {
			t.Errorf("%s, рамка под 30°: %+v", scope, b)
		}
	}
}

// dpi фона 6000×4000 на весь разворот 600×300 мм: наклон 4,5° увеличивает фото в 1,0361 раза
// (design §2.1), и dpi падает так же; «Мало разрешения» видит уже увеличенный кадр.
func TestBackdropDPITilt(t *testing.T) {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 300, 300
	e := edtest.Open(t, p, "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a")).Spreads[0].ID
	v, err := e.SetBackdrop(sid, "spread", model.PhotoInfo{ID: "bg", Path: edtest.Abs("bg.jpg"), W: 6000, H: 4000})
	if err != nil {
		t.Fatal(err)
	}
	b0 := bdView(t, v, "spread")
	b1 := bdView(t, e.SetBackdropAngle(sid, "spread", 4.5, ""), "spread")
	if k := b1.Img.W / b0.Img.W; math.Abs(k-1.0361) > 1e-4 {
		t.Fatalf("увеличение %v, ждали 1,0361", k)
	}
	if b0.DPI != 254 || b1.DPI != 245 {
		t.Fatalf("dpi %d → %d, ждали 254 → 245", b0.DPI, b1.DPI)
	}
}

// Кадр (масштаб, сдвиг) и рамка (перенос, поворот) наклон не сбрасывают.
func TestSetBackdropFrameKeepsTilt(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "spread", "bg")
	e.SetBackdropAngle(s1, "spread", -7.3, "")
	e.SetBackdropFrame(s1, "spread", 2, -0.5, 0.25)
	if f := edtest.BdOf(e, s1, "spread").Frame; *f != (model.Frame{S: 2, X: -0.5, Y: 0.25, R: -7.3}) {
		t.Fatalf("кадр: %+v", f)
	}
	e.SetBackdropBox(s1, "spread", 30, 20, 400, 200, 12, "")
	e.SetBackdropFrame(s1, "spread", 1, 0, 0) // «Сбросить кадр»: наклон остаётся
	if f := edtest.BdOf(e, s1, "spread").Frame; f == nil || *f != (model.Frame{S: 1, R: -7.3}) {
		t.Fatalf("после рамки и сброса кадра: %+v", f)
	}
}

// Зеркало меняет знак наклона (горизонт остаётся ровным), повтор возвращает; без наклона «-0» нет.
func TestFlipBackdropNegatesTilt(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	e.SetBackdropAngle(s1, "left", 4.5, "")
	e.FlipBackdrop(s1, "left", "h")
	if b := edtest.BdOf(e, s1, "left"); !b.FlipH || b.Frame.R != -4.5 {
		t.Fatalf("↔: %+v %+v", b, b.Frame)
	}
	e.FlipBackdrop(s1, "left", "v")
	if b := edtest.BdOf(e, s1, "left"); !b.FlipV || b.Frame.R != 4.5 {
		t.Fatalf("↕: %+v %+v", b, b.Frame)
	}
	e.FlipBackdrop(s1, "right", "h")
	if data := model.Marshal(edtest.P(e)); bytes.Contains(data, []byte("-0")) || edtest.BdOf(e, s1, "right").Frame != nil {
		t.Fatalf("без наклона: -0 или лишний кадр: %s", data)
	}
}

// «По области»: рамка = область, поворот рамки и кадр сброшены, наклон и зеркало остаются.
func TestFitBackdropKeepsTilt(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "right", "bg")
	e.SetBackdropBox(s1, "right", 350, 20, 100, 100, 30, "")
	e.SetBackdropFrame(s1, "right", 3, 0.2, 0.2)
	e.SetBackdropAngle(s1, "right", 12, "")
	e.FlipBackdrop(s1, "right", "v")
	e.FitBackdrop(s1, "right")
	b := edtest.BdOf(e, s1, "right")
	if b.X != 300 || b.Y != 0 || b.W != 300 || b.H != 300 || b.Rot != 0 || b.Frame == nil || *b.Frame != (model.Frame{S: 1, R: -12}) || !b.FlipV {
		t.Fatalf("по области: %+v %+v", b, b.Frame)
	}
	rev := edtest.Rev(e)
	e.FitBackdrop(s1, "right")
	if edtest.Rev(e) != rev {
		t.Fatal("повторное «по области» не холостое")
	}
}

// Выпрямить фон по линии: по краю рамки (у повёрнутой — по её краю), холостая — ровная линия,
// нет фона, не число; один шаг отмены.
func TestStraightenBackdrop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	e.SetBackdropBox(s1, "right", 320, 20, 200, 150, 10, "")
	at := func(deg float64) (float64, float64) { s, c := math.Sincos(deg * math.Pi / 180); return 80 * c, 80 * s }
	undo := edtest.UndoLen(e)
	dx, dy := at(4.5)
	e.StraightenBackdrop(s1, "left", dx, dy)
	dx, dy = at(14.5)
	v := e.StraightenBackdrop(s1, "right", dx, dy)
	if l, r := bdView(t, v, "left").Frame.R, bdView(t, v, "right").Frame.R; l != -4.5 || r != -4.5 || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("левая %v, правая %v, шагов %d", l, r, edtest.UndoLen(e)-undo)
	}
	dx, dy = at(-93) // вертикаль: +3 к прежнему
	if r := bdView(t, e.StraightenBackdrop(s1, "left", dx, dy), "left").Frame.R; r != -1.5 {
		t.Fatalf("вертикаль: %v", r)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.StraightenBackdrop(s1, "left", 50, 0)
	e.StraightenBackdrop(s1, "left", 0, 0)
	e.StraightenBackdrop(s1, "left", math.NaN(), 3)
	e.StraightenBackdrop(s1, "spread", 50, 3)
	e.StraightenBackdrop(s1, "top", 50, 3)
	edtest.Same(t, e, rev, undo, "холостые")
	if r := bdView(t, e.Undo(), "left").Frame.R; r != -4.5 {
		t.Fatalf("Undo: %v", r)
	}
}

// Наклон фона держит приближение (057, решение 14): сессия — один шаг и расчёт от её начала,
// без сессии — от текущего; S = 1 остаётся 1; области не сливаются; выпрямление; Save→Load.
func TestBackdropAngleKeepsZoom(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	b0 := bdView(t, e.SetBackdropFrame(s1, "left", 1.5, 0.4, -0.3), "left")
	start, n := *edtest.BdOf(e, s1, "left").Frame, edtest.UndoLen(e)
	e.SetBackdropAngle(s1, "left", 3, "tl-1")
	b := bdView(t, e.SetBackdropAngle(s1, "left", 5, "tl-1"), "left")
	if b.Frame.R != 5 || !heldZoom(b0, b) || edtest.UndoLen(e) != n+1 {
		t.Fatalf("сессия 0 → 3 → 5: %+v, шагов %d", b.Frame, edtest.UndoLen(e)-n)
	}
	e.SetBackdropAngle(s1, "left", 0, "tl-1")
	if f := *edtest.BdOf(e, s1, "left").Frame; f != start || edtest.UndoLen(e) > n+1 {
		t.Fatalf("0 → 5 → 0: %+v, ждали %+v", f, start)
	}
	// Без сессии: 3° → 0° — 1,5 ± 0,001.
	if f := bdView(t, e.SetBackdropAngle(s1, "left", 3, ""), "left").Frame; !(f.S < 1.5) {
		t.Fatalf("3°: %+v", f)
	}
	if f := bdView(t, e.SetBackdropAngle(s1, "left", 0, ""), "left").Frame; math.Abs(f.S-1.5) > 1e-3 {
		t.Fatalf("3° → 0°: %+v", f)
	}
	// Правая без приближения: S = 1 при любом наклоне; та же сессия у другой области — свой шаг.
	n = edtest.UndoLen(e)
	e.SetBackdropAngle(s1, "left", 2, "tl-2")
	for _, tilt := range []float64{8, -30, 0.5} {
		if f := bdView(t, e.SetBackdropAngle(s1, "right", tilt, "tl-2"), "right").Frame; f.S != 1 || f.R != tilt {
			t.Fatalf("правая %v: %+v", tilt, f)
		}
	}
	if edtest.UndoLen(e) != n+2 {
		t.Fatalf("области слились: шагов %d", edtest.UndoLen(e)-n)
	}
	// Выпрямить по линии — держит приближение, один шаг.
	b0 = bdView(t, e.SetBackdropFrame(s1, "left", 2, 0, 0.5), "left")
	n = edtest.UndoLen(e)
	s, c := math.Sincos(4 * math.Pi / 180)
	if b := bdView(t, e.StraightenBackdrop(s1, "left", 80*c, 80*s), "left"); b.Frame.R != -2 || !heldZoom(b0, b) || edtest.UndoLen(e) != n+1 {
		t.Fatalf("выпрямить: %+v → %+v", b0.Frame, b.Frame)
	}
	// Save→Load — те же кадры обеих страниц.
	q := edtest.SaveLoad(t, e)
	for _, scope := range []string{"left", "right"} {
		if got, want := (*q.Sheet(s1).Slot(scope)).Frame, edtest.BdOf(e, s1, scope).Frame; got == nil || *got != *want {
			t.Errorf("%s после Load: %+v, ждали %+v", scope, got, want)
		}
	}
}
