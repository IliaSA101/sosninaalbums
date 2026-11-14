package backdrop_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
)

// TestTurnBackdropAllScopes — поворот на 90° у фона разворота и обеих страниц: стороны рамки
// меняются местами вокруг центра, Save → Load сохраняет рамку, каждая правка — один шаг.
func TestTurnBackdropAllScopes(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	e.SetBackdropBox(s1, "left", 20, 30, 200, 100, 0, "")
	e.SetBackdropBox(s1, "right", 340, 40, 150, 90, 0, "")
	type was struct{ cx, cy, w, h float64 }
	before := map[string]was{}
	for _, scope := range []string{"left", "right"} {
		b := edtest.BdOf(e, s1, scope)
		cx, cy := b.TBox().Center()
		before[scope] = was{cx, cy, b.W, b.H}
	}
	// Несколько фонов сразу: каждый поворачивается своей правкой, соседний не трогается.
	undo := edtest.UndoLen(e)
	e.TurnBackdrop(s1, "left", 90, "")
	r := edtest.BdOf(e, s1, "right")
	if r.Rot != 0 || r.W != 150 || r.H != 90 {
		t.Fatalf("правая тронута поворотом левой: %+v", r)
	}
	e.TurnBackdrop(s1, "right", 90, "")
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов %d, ждали 2", edtest.UndoLen(e)-undo)
	}
	for _, scope := range []string{"left", "right"} {
		w := before[scope]
		b := edtest.BdOf(e, s1, scope)
		cx, cy := b.TBox().Center()
		if b.Rot != 90 || b.W != w.h || b.H != w.w || math.Abs(cx-w.cx) > 0.02 || math.Abs(cy-w.cy) > 0.02 {
			t.Errorf("%s: %+v, центр (%v, %v), было %+v", scope, b, cx, cy, w)
		}
	}
	// Круг Save → Load.
	q := edtest.SaveLoad(t, e)
	for _, scope := range []string{"left", "right"} {
		want := edtest.BdOf(e, s1, scope)
		got := *q.Sheet(s1).Slot(scope)
		if got == nil || got.X != want.X || got.Y != want.Y || got.W != want.W || got.H != want.H || got.Rot != want.Rot {
			t.Errorf("%s после Load: %+v, ждали %+v", scope, got, want)
		}
	}
	// Фон на весь разворот.
	edtest.SetBg(t, e, s1, "spread", "bg")
	e.SetBackdropBox(s1, "spread", 50, 20, 400, 200, 0, "")
	e.TurnBackdrop(s1, "spread", 90, "")
	if b := edtest.BdOf(e, s1, "spread"); b.Rot != 90 || b.W != 200 || b.H != 400 {
		t.Errorf("фон разворота: %+v", b)
	}
}

// Поворот без смены ориентации — на месте вокруг центра, стороны прежние; угол нормируется.
func TestTurnBackdropKeepsSidesAndNormalizes(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropBox(s1, "left", 20, 30, 200, 100, 0, "")
	b0 := edtest.BdOf(e, s1, "left")
	cx0, cy0 := b0.TBox().Center()
	e.TurnBackdrop(s1, "left", 30, "")
	b := edtest.BdOf(e, s1, "left")
	cx, cy := b.TBox().Center()
	if b.W != 200 || b.H != 100 || b.Rot != 30 || math.Abs(cx-cx0) > 0.02 || math.Abs(cy-cy0) > 0.02 {
		t.Fatalf("30°: %+v, центр (%v, %v) → (%v, %v)", b, cx0, cy0, cx, cy)
	}
	e.TurnBackdrop(s1, "left", 270, "")
	if b = edtest.BdOf(e, s1, "left"); b.Rot != -90 || b.W != 100 || b.H != 200 {
		t.Fatalf("270° → −90°: %+v", b)
	}
}

// Отмена: поворот — один шаг (в том числе серия одной сессии поля), Undo возвращает рамку.
// Холостые правки — тот же угол, нет фона, неверная область, NaN — без шага и без rev.
func TestTurnBackdropUndoOneStep(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropBox(s1, "left", 20, 30, 200, 100, 0, "")
	b0 := *edtest.BdOf(e, s1, "left")
	undo := edtest.UndoLen(e)
	e.TurnBackdrop(s1, "left", 90, "")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if b := *edtest.BdOf(e, s1, "left"); b != b0 {
		t.Fatalf("после Undo: %+v, ждали %+v", b, b0)
	}
	// Серия сессии: 30° → 90° → 120° — один шаг, Undo — к исходной рамке.
	undo = edtest.UndoLen(e)
	for _, deg := range []float64{30, 90, 120} {
		e.TurnBackdrop(s1, "left", deg, "ses1")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("серия: шагов %d, ждали 1", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if b := *edtest.BdOf(e, s1, "left"); b != b0 {
		t.Fatalf("после Undo серии: %+v, ждали %+v", b, b0)
	}
	// Холостые.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.TurnBackdrop(s1, "left", 0, "")   // тот же угол
	e.TurnBackdrop(s1, "right", 90, "") // нет фона
	e.TurnBackdrop(s1, "top", 90, "")   // область неизвестна
	e.TurnBackdrop("нет", "left", 90, "")
	e.TurnBackdrop(s1, "left", math.NaN(), "")
	e.TurnBackdrop(s1, "left", math.Inf(1), "")
	edtest.Same(t, e, rev, undo, "холостые повороты")
}

// Повтор поворота на тот же угол — холостая правка для всех областей: после зажима центра
// Y мог выйти «-0» (шум float), что в JSON отличалось от «0» и давало лишний шаг (070 Г).
func TestTurnBackdropRepeatIsNoop(t *testing.T) {
	for _, scope := range []string{"left", "right", "spread"} {
		t.Run(scope, func(t *testing.T) {
			e, s1, _ := edtest.BdEd(t)
			edtest.SetBg(t, e, s1, scope, "bg")
			for _, deg := range []float64{90, -90, 180, 90} { // рамка «по области» и повороты туда-обратно
				e.TurnBackdrop(s1, scope, deg, "")
				rev, undo := edtest.Rev(e), edtest.UndoLen(e)
				e.TurnBackdrop(s1, scope, deg, "")
				edtest.Same(t, e, rev, undo, scope+": повтор угла")
				if b := edtest.BdOf(e, s1, scope); math.Signbit(b.X) && b.X == 0 || math.Signbit(b.Y) && b.Y == 0 {
					t.Fatalf("%s %v°: минус ноль в рамке %+v", scope, deg, b)
				}
			}
			q := edtest.SaveLoad(t, e) // круг Save → Load держит рамку
			if got, want := *q.Sheet(s1).Slot(scope), edtest.BdOf(e, s1, scope); got == nil || got.X != want.X || got.Y != want.Y || got.W != want.W || got.H != want.H || got.Rot != want.Rot {
				t.Fatalf("после Load: %+v, ждали %+v", got, want)
			}
		})
	}
}
