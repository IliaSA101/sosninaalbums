package design_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Выпрямить по линии» (спека 045, design §2.2): линия в мм листа → наклон фото.

// straighten — выпрямить фото id листа sheet по линии длиной 60 мм под углом deg к краю
// листа (по часовой, y вниз).
func straighten(e *album.Editor, sheet, id string, deg float64) model.View {
	s, c := math.Sincos(deg * math.Pi / 180)
	return e.StraightenPhoto(sheet, id, 60*c, 60*s)
}

// Три фото разом: обычное, в повёрнутом месте и после ↻ 90°. Четверти и ячейки стоят,
// наклон упирается в ±44,9, у места линия ровняется по его краю.
func TestStraightenPhotoKeepsTurn(t *testing.T) {
	rot := edtest.Gp(150, 20, 80, 60, "b")
	rot.Rot = 10
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 20, 90, 60, "a"), rot, edtest.Gp(10, 150, 60, 90, "c")})
	e.TurnPhotos([]string{"c"}, 1)
	edtest.Tilt(e, "c", 40)
	cells := map[string]any{}
	for _, id := range []string{"a", "b", "c"} {
		cells[id] = edtest.RectOf(e.View(), 0, id)
	}
	straighten(e, "s1", "a", 4.5)
	straighten(e, "s1", "b", 14.5)
	v := straighten(e, "s1", "c", -10) // 40 + 10 = 50 → упор
	for id, want := range map[string][2]float64{"a": {0, -4.5}, "b": {0, -4.5}, "c": {1, 44.9}} {
		f := frameOf(t, v, id)
		if float64(f.Turn) != want[0] || f.Tilt != want[1] {
			t.Errorf("%s: четверть %d, наклон %v; ждали %v", id, f.Turn, f.Tilt, want)
		}
		if edtest.RectOf(v, 0, id) != cells[id] {
			t.Errorf("%s: ячейка сдвинулась", id)
		}
		if !coversCell(f.Img, edtest.RectOf(v, 0, id), f.R) {
			t.Errorf("%s: рамка открылась: %+v", id, f)
		}
	}
	// Вертикаль под 93° к краю листа — наклон −3; у фото с наклоном он складывается.
	v = straighten(e, "s1", "a", 93)
	if f := frameOf(t, v, "a"); f.Tilt != -7.5 || f.Turn != 0 {
		t.Fatalf("вертикаль: %+v", f)
	}
}

// Одна правка — один шаг отмены; Undo возвращает прежний наклон, Redo — новый.
func TestStraightenPhotoOneStep(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 20, 90, 60, "a"), edtest.Gp(150, 20, 80, 60, "b")})
	edtest.Tilt(e, "a", 2)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v := straighten(e, "s1", "a", -4.5)
	if f := frameOf(t, v, "a"); f.Tilt != 6.5 || edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("наклон %v, rev %d → %d, undo %d → %d", f.Tilt, rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	if f := frameOf(t, v, "b"); f.R != 0 {
		t.Fatalf("соседнее фото: %+v", f)
	}
	if f := frameOf(t, e.Undo(), "a"); f.Tilt != 2 {
		t.Fatalf("Undo: %v", f.Tilt)
	}
	if f := frameOf(t, e.Redo(), "a"); f.Tilt != 6.5 {
		t.Fatalf("Redo: %v", f.Tilt)
	}
}

// Холостая правка без шага и rev: линии нет, не число, уже ровно (меньше 0,05°), фото на
// другом листе, чужое фото, фото фона, неизвестный лист. Обложка — свой лист.
func TestStraightenPhotoNoop(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(10, 20, 90, 60, "a")}, []string{"x"})
	edtest.SetBg(t, e, "s1", "spread", "bg")
	straighten(e, "s1", "a", 3)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, c := range []struct {
		name, sheet, id string
		dx, dy          float64
	}{
		{"нулевая", "s1", "a", 0, 0},
		{"короче 0,01 мм", "s1", "a", 0.005, 0},
		{"NaN", "s1", "a", math.NaN(), 1},
		{"Inf", "s1", "a", 10, math.Inf(1)},
		{"ровная", "s1", "a", 60, 0},
		{"почти ровная", "s1", "a", 100, 0.05},
		{"чужой лист", "s2", "a", 60, 5},
		{"нет фото", "s1", "zz", 60, 5},
		{"фото фона", "s1", "bg", 60, 5},
		{"нет листа", "s9", "a", 60, 5},
		{"обложка не та", model.CoverID, "a", 60, 5},
	} {
		e.StraightenPhoto(c.sheet, c.id, c.dx, c.dy)
		edtest.Same(t, e, rev, undo, c.name)
	}
	// Фото обложки выпрямляется на своём листе.
	straighten(e, model.CoverID, "k", 4.5)
	if r := edtest.P(e).Photos["k"].FrameOrDefault().R; r != -4.5 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("обложка: R %v, undo %d", r, edtest.UndoLen(e))
	}
}

// «Авто»: выпрямление пропорций не меняет — разворот не закрепляется и не пересобирается.
func TestStraightenPhotoAutoNoFreeze(t *testing.T) {
	e, sid := edtest.AutoEd(t)
	before := e.View().Spreads[0]
	v := straighten(e, sid, "b", -30)
	s := v.Spreads[0]
	if s.Grid || s.Layout != before.Layout || len(s.Rects) != len(before.Rects) {
		t.Fatalf("разворот закрепился или пересобрался: grid %v, %q → %q", s.Grid, before.Layout, s.Layout)
	}
	for i := range s.Rects {
		if s.Rects[i] != before.Rects[i] {
			t.Fatalf("ячейка %d: %+v → %+v", i, before.Rects[i], s.Rects[i])
		}
	}
	if f := frameOf(t, v, "b"); f.Tilt != 30 || f.Turn != 0 {
		t.Fatalf("наклон в «Авто»: %+v", f)
	}
}

// Обложка — лист, как и разворот: фото на лицевой выпрямляется по той же линии, фото разворота
// не тронуто.
func TestStraightenPhotoCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(12, 12, 127, 127, "a")})
	v := straighten(e, model.CoverID, "cv", 3)
	if r := edtest.P(e).Photos["cv"].FrameOrDefault().R; r != -3 {
		t.Fatalf("R=%v", r)
	}
	if f := frameOf(t, v, "a"); f.R != 0 {
		t.Fatalf("фото разворота тронуто: %+v", f)
	}
}

// Упор ±44,9 (решение 20) в обе стороны: линия под 40° дважды не переключает четверть, обратный
// ход считается от упора.
func TestStraightenPhotoLimit(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 20, 90, 60, "a"), edtest.Gp(150, 20, 80, 60, "b")})
	straighten(e, "s1", "b", 40)
	v := straighten(e, "s1", "b", 40)
	if f := frameOf(t, v, "b"); f.Turn != 0 || f.Tilt != -model.MaxTilt {
		t.Fatalf("минус: %+v", f)
	}
	v = straighten(e, "s1", "b", -40)
	if f := frameOf(t, v, "b"); f.Turn != 0 || f.Tilt != -4.9 {
		t.Fatalf("обратно: %+v", f)
	}
	straighten(e, "s1", "b", -40)
	v = straighten(e, "s1", "b", -40)
	if f := frameOf(t, v, "b"); f.Turn != 0 || f.Tilt != model.MaxTilt {
		t.Fatalf("плюс: %+v", f)
	}
	if f := frameOf(t, v, "a"); f.R != 0 {
		t.Fatalf("соседнее фото: %+v", f)
	}
}
