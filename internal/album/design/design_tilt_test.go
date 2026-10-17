package design_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Наклон держит приближение (спека 057): правило размера, сессия жеста, «Целиком», места.

// tiltEd — сетка: a — место 150×100, b — круг 100×100, c — «Целиком» 150×100 (фото 3:2);
// второй разворот s2 «Авто» с фото d.
func tiltEd(t *testing.T) *album.Editor {
	t.Helper()
	circle := edtest.Gp(170, 20, 100, 100, "b")
	circle.Shape = model.ShapeOval
	contain := edtest.Gp(10, 150, 150, 100, "c")
	contain.Fit = model.FitContain
	return edtest.GridEd(t, []*model.Place{edtest.Gp(10, 20, 150, 100, "a"), circle, contain}, []string{"d"})
}

// frameAt — кадр фото id в проекте.
func frameAt(e *album.Editor, id string) model.Frame { return edtest.P(e).Photos[id].FrameOrDefault() }

// shownW — ширина фото id на экране, мм (вид листа).
func shownW(t *testing.T, e *album.Editor, id string) float64 {
	t.Helper()
	return frameOf(t, e.View(), id).Img.W
}

func TestSetFrameAngleKeepsZoom(t *testing.T) {
	e := tiltEd(t)
	e.SetFrame("a", 1.5, 0.5, -0.2)
	e.SetFrame("b", 1.5, 0, 0)
	e.SetFrame("c", 2, 0.3, 0.3)
	start, w0, wb := frameAt(e, "a"), shownW(t, e, "a"), shownW(t, e, "b")

	// Сессия: 0 → 3 → 5 — один шаг; 5 — от начала сессии; → 0 — кадр побитно, шага нет.
	n := edtest.UndoLen(e)
	e.SetFrameAngle("s1", "a", 3, "tl-1")
	v := e.SetFrameAngle("s1", "a", 5, "tl-1")
	direct := geom.TiltCrop(layout.Rect{W: 150, H: 100}, 1.5, geom.Crop{S: 1.5, X: 0.5, Y: -0.2}, 5, geom.Outline{})
	if f := frameOf(t, v, "a"); edtest.UndoLen(e) != n+1 || math.Abs(f.S-direct.S) > 5e-4 || math.Abs(f.X-direct.X) > 5e-4 || f.Tilt != 5 {
		t.Fatalf("сессия 0 → 3 → 5: %+v, ждали %+v, шагов %d", f, direct, edtest.UndoLen(e)-n)
	}
	if w := frameOf(t, v, "a").Img.W; math.Abs(w-w0) > 1e-3*w0 {
		t.Fatalf("ширина %v → %v", w0, w)
	}
	e.SetFrameAngle("s1", "a", 0, "tl-1")
	if f := frameAt(e, "a"); f != start || edtest.UndoLen(e) > n+1 {
		t.Fatalf("0 → 5 → 0: %+v, ждали %+v; шагов %d", f, start, edtest.UndoLen(e)-n)
	}

	// Вне сессии: 150 % → 3° → 0° — 1,5 ± 0,001 (каждый шаг от текущего, с округлением).
	edtest.Tilt(e, "a", 3)
	if f := frameAt(e, "a"); !(f.S < 1.5) {
		t.Fatalf("3° без сессии: %+v", f)
	}
	edtest.Tilt(e, "a", 0)
	if f := frameAt(e, "a"); math.Abs(f.S-1.5) > 1e-3 || math.Abs(f.X-0.5) > 2e-3 || math.Abs(f.Y+0.2) > 2e-3 || f.R != 0 {
		t.Fatalf("3° → 0°: %+v", f)
	}

	// Круг: габарит не меняется — S стоит, ширина та же.
	edtest.Tilt(e, "b", 10)
	if f := frameAt(e, "b"); f.S != 1.5 || f.R != 10 || math.Abs(shownW(t, e, "b")-wb) > 1e-9 {
		t.Fatalf("круг: %+v", f)
	}
	// «Целиком»: меняется только R.
	edtest.Tilt(e, "c", -7)
	if f := frameAt(e, "c"); f != (model.Frame{S: 2, X: 0.3, Y: 0.3, R: -7}) {
		t.Fatalf("целиком: %+v", f)
	}
	// «Авто»: ячейка раскладки, размер держится, раскладка не закрепляется.
	e.SetFrame("d", 1.6, 0, 0)
	wd := shownW(t, e, "d")
	edtest.Tilt(e, "d", 4)
	if f, s := frameAt(e, "d"), edtest.P(e).Sheet("s2"); !(f.S < 1.6) || f.R != 4 || math.Abs(shownW(t, e, "d")-wd) > 1e-3*wd || len(s.Grid) != 0 {
		t.Fatalf("«Авто»: %+v, ширина %v → %v, мест %d", f, wd, shownW(t, e, "d"), len(s.Grid))
	}

	// Save→Load — тот же кадр у всех (ID после Load — по путям).
	q := edtest.SaveLoad(t, e)
	byPath := map[string]model.Frame{}
	for _, r := range q.Photos {
		byPath[r.Path] = r.FrameOrDefault()
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if got, ok := byPath[edtest.P(e).Photos[id].Path]; !ok || got != frameAt(e, id) {
			t.Errorf("%s после Load: %+v, ждали %+v", id, got, frameAt(e, id))
		}
	}
}

// Без приближения S = 1 остаётся 1 при любом наклоне и в новом жесте обратно (решение 9).
func TestSetFrameAngleNoZoom(t *testing.T) {
	e := tiltEd(t)
	for _, tilt := range []float64{3, 25, -44.9, 0} {
		edtest.Tilt(e, "a", tilt)
		if f := frameAt(e, "a"); f.S != 1 {
			t.Fatalf("наклон %v: %+v", tilt, f)
		}
	}
	if edtest.P(e).Photos["a"].Frame != nil {
		t.Fatalf("100 %% и 0° — кадр по умолчанию: %+v", edtest.P(e).Photos["a"].Frame)
	}
	// Предел 4: 400 % при 20° обратно к 0° — S в упоре, фото чуть меньше (решение 13).
	edtest.Tilt(e, "a", 20)
	e.SetFrame("a", 4, 0, 0)
	edtest.Tilt(e, "a", 0)
	if f := frameAt(e, "a"); f.S != 4 || f.R != 0 {
		t.Fatalf("предел: %+v", f)
	}
}

// Сессии: ключ — фото и сессия; другая правка рвёт цепочку; холостые не пишутся.
func TestSetFrameAngleSession(t *testing.T) {
	e := tiltEd(t)
	e.SetFrame("a", 1.5, 0, 0)
	e.SetFrame("b", 2, 0, 0)
	n := edtest.UndoLen(e)
	// Два фото с одной сессией — два шага: ключ включает фото.
	e.SetFrameAngle("s1", "a", 3, "tl-1")
	e.SetFrameAngle("s1", "b", 3, "tl-1")
	e.SetFrameAngle("s1", "b", 4, "tl-1") // b: шаг тот же
	if edtest.UndoLen(e) != n+2 {
		t.Fatalf("шагов %d, ждали 2", edtest.UndoLen(e)-n)
	}
	// Другая правка рвёт цепочку: новая сессия считает от текущего кадра.
	e.SetFrameAngle("s1", "a", 3, "tl-2")
	e.SetFrame("a", 2, 0, 0)
	e.SetFrameAngle("s1", "a", 0, "tl-2")
	if f := frameAt(e, "a"); !(f.S > 2) {
		t.Fatalf("после SetFrame база — текущий кадр: %+v", f)
	}
	// Undo после сессии — к кадру до её начала.
	e.SetFrame("a", 1.5, 0, 0)
	e.SetFrameAngle("s1", "a", 2, "tl-3")
	e.SetFrameAngle("s1", "a", 6, "tl-3")
	e.Undo()
	if f := frameAt(e, "a"); f != (model.Frame{S: 1.5}) {
		t.Fatalf("Undo сессии: %+v", f)
	}
	// Холостые: не число, тот же наклон, не тот лист, нет листа, нет фото.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for name, f := range map[string]func(){
		"NaN":      func() { e.SetFrameAngle("s1", "a", math.NaN(), "tl-4") },
		"Inf":      func() { e.SetFrameAngle("s1", "a", math.Inf(1), "") },
		"тот же":   func() { e.SetFrameAngle("s1", "a", 0.04, "tl-4") },
		"чужой":    func() { e.SetFrameAngle("s2", "a", 5, "") },
		"нет":      func() { e.SetFrameAngle("нет", "a", 5, "") },
		"нет фото": func() { e.SetFrameAngle("s1", "нет", 5, "") },
	} {
		f()
		edtest.Same(t, e, rev, undo, name)
	}
	// Сессия неверного вида — без слияния: два шага.
	e.SetFrameAngle("s1", "a", 2, "плохая сессия")
	e.SetFrameAngle("s1", "a", 3, "плохая сессия")
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("неверная сессия слилась: %d", edtest.UndoLen(e)-undo)
	}
}

// Обложка — как сетка: место на лицевой, размер держится.
func TestSetFrameAngleCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 20, 150, 100, "a")})
	e.SetFrame("cv", 1.8, -0.4, 0)
	cell := layout.Rect{W: 120, H: 80}
	w0 := model.PlaceImage(cell, 1.5, frameAt(e, "cv")).W
	e.SetFrameAngle(model.CoverID, "cv", -6, "tl-1")
	f := frameAt(e, "cv")
	if w := model.PlaceImage(cell, 1.5, f).W; f.R != -6 || !(f.S < 1.8) || math.Abs(w-w0) > 1e-3*w0 {
		t.Fatalf("обложка: %+v, ширина %v → %v", f, w0, w)
	}
}

// «Выпрямить по линии»: приближение держится, один шаг, без сессии.
func TestStraightenKeepsZoom(t *testing.T) {
	e := tiltEd(t)
	e.SetFrame("a", 1.5, 0.5, 0)
	e.SetFrame("b", 1.5, 0, 0)
	w0, n := shownW(t, e, "a"), edtest.UndoLen(e)
	straighten(e, "s1", "a", 3)
	f := frameAt(e, "a")
	if f.R != -3 || math.Abs(f.S-1.393) > 1e-3 || math.Abs(shownW(t, e, "a")-w0) > 1e-3*w0 || edtest.UndoLen(e) != n+1 {
		t.Fatalf("выпрямить: %+v, ширина %v → %v", f, w0, shownW(t, e, "a"))
	}
	straighten(e, "s1", "b", -8) // круг: S стоит
	if f := frameAt(e, "b"); f != (model.Frame{S: 1.5, R: 8}) {
		t.Fatalf("круг: %+v", f)
	}
	if f := frameOf(t, e.Undo(), "b"); f.S != 1.5 || f.R != 0 {
		t.Fatalf("Undo: %+v", f)
	}
}

// BaseFrame — кадр фото в снимке начала сессии наклона: копия; снимка и фото в нём нет — nil
// (база — текущий кадр); у фото без кадра — кадр по умолчанию {S: 1}, а не nil.
func TestBaseFrame(t *testing.T) {
	e := tiltEd(t)
	e.SetFrame("a", 1.5, 0.5, -0.2)
	p0 := edtest.P(e).Clone()

	if f := design.BaseFrame(nil, "a"); f != nil {
		t.Fatalf("нет снимка: %+v", f)
	}
	if f := design.BaseFrame(p0, "нет"); f != nil {
		t.Fatalf("нет фото в снимке: %+v", f)
	}
	got := design.BaseFrame(p0, "a")
	if got == nil || *got != (model.Frame{S: 1.5, X: 0.5, Y: -0.2}) {
		t.Fatalf("кадр a: %+v", got)
	}
	if f := design.BaseFrame(p0, "b"); f == nil || *f != (model.Frame{S: 1}) {
		t.Fatalf("фото без кадра: %+v, ждали {S: 1}", f)
	}
	got.S = 4 // копия: снимок не меняется
	if f := p0.Photos["a"].FrameOrDefault(); f.S != 1.5 {
		t.Fatalf("BaseFrame отдал не копию: %+v", f)
	}
}
