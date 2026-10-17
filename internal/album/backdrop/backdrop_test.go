package backdrop_test

import (
	"bytes"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func TestBackdropSetExclusive(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	v := edtest.SetBg(t, e, s1, "spread", "bg")
	b := v.Spreads[0].Backdrops
	if len(b) != 1 || b[0].Scope != "spread" || b[0].Box.W != 600 || b[0].Box.H != 300 || b[0].Area.W != 600 {
		t.Fatalf("фон на разворот: %+v", b)
	}
	if v.Spreads[0].Photos[0] != "a" || len(v.Spreads[0].Photos) != 2 {
		t.Fatalf("фото фона попало в Photos: %v", v.Spreads[0].Photos)
	}
	if !slices.Equal(v.BackdropUsed, []string{"bg"}) {
		t.Fatalf("BackdropUsed: %v", v.BackdropUsed)
	}
	// Страница снимает фон разворота; вторая страница — рядом.
	v = edtest.SetBg(t, e, s1, "left", "bg2")
	if edtest.BdOf(e, s1, "spread") != nil || len(v.Spreads[0].Backdrops) != 1 {
		t.Fatalf("фон разворота остался: %+v", v.Spreads[0].Backdrops)
	}
	v = edtest.SetBg(t, e, s1, "right", "bg3")
	if got := v.Spreads[0].Backdrops; len(got) != 2 || got[0].Scope != "left" || got[1].Scope != "right" || got[1].Area.X != 300 {
		t.Fatalf("две страницы: %+v", got)
	}
	if _, ok := e.Ref("bg"); ok {
		t.Fatal("снятое фото фона осталось в Photos")
	}
	v = edtest.SetBg(t, e, s1, "spread", "bg")
	if edtest.BdOf(e, s1, "left") != nil || edtest.BdOf(e, s1, "right") != nil || len(v.Spreads[0].Backdrops) != 1 {
		t.Fatal("фоны страниц остались при фоне разворота")
	}
	if _, err := e.SetBackdrop(s1, "top", edtest.Photo("x")); err == nil {
		t.Fatal("неизвестная область принята")
	}
	rev := e.View().Rev
	edtest.SetBg(t, e, "нет", "spread", "bg")
	if e.View().Rev != rev {
		t.Fatal("неизвестный разворот изменил проект")
	}
}

// Замена фото: рамка, поворот и зеркало остаются, кадр сброшен; то же фото — холостая правка.
func TestBackdropSetKeepsBox(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropBox(s1, "left", 20, 30, 200, 150, 15, "")
	e.SetBackdropFrame(s1, "left", 2, 0.5, -0.5)
	e.SetBackdropAngle(s1, "left", 4.5, "")
	e.FlipBackdrop(s1, "left", "h")
	e.FlipBackdrop(s1, "left", "v")
	if b := edtest.BdOf(e, s1, "left"); b.Frame == nil || b.Frame.R == 0 {
		t.Fatalf("до замены наклона нет: %+v", b)
	}
	edtest.SetBg(t, e, s1, "left", "bg2")
	b := edtest.BdOf(e, s1, "left")
	if b.Photo != "bg2" || b.Frame != nil || !b.FlipH || !b.FlipV || b.X != 20 || b.W != 200 || b.H != 150 || b.Rot != 15 {
		t.Fatalf("после замены: %+v", b)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.SetBg(t, e, s1, "left", "bg2")
	edtest.Same(t, e, rev, undo, "то же фото")
}

func TestBackdropBoxClamp(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	cases := []struct {
		name                string
		x, y, w, h, rot     float64
		wantW, wantH, wantR float64
	}{
		{"малая", 10, 10, 1, 2, 0, 10, 10, 0},
		{"большая", 0, 0, 5000, 5000, 0, 900, 900, 0},
		{"угол", 10, 10, 100, 100, 270, 100, 100, -90},
		{"NaN в размере — область", 10, 10, math.NaN(), 50, 30, 300, 300, 0},
		{"нуль — область", 10, 10, 0, 50, 30, 300, 300, 0},
		{"Inf — область", 10, 10, 50, math.Inf(1), 30, 300, 300, 0},
	}
	for _, c := range cases {
		e.SetBackdropBox(s1, "left", c.x, c.y, c.w, c.h, c.rot, "")
		b := edtest.BdOf(e, s1, "left")
		if b.W != c.wantW || b.H != c.wantH || b.Rot != c.wantR {
			t.Errorf("%s: %+v", c.name, b)
		}
	}
	// Центр рамки остаётся в области.
	for _, rot := range []float64{0, 45, 90, -130} {
		e.SetBackdropBox(s1, "left", 5000, -4000, 100, 60, rot, "")
		b := edtest.BdOf(e, s1, "left")
		cx, cy := b.TBox().Center()
		if cx < -0.01 || cx > 300.01 || cy < -0.01 || cy > 300.01 {
			t.Errorf("rot %v: центр (%v, %v) вне области: %+v", rot, cx, cy, b)
		}
	}
	// NaN в положении — край области; сохранённое значение — шаг 0.01.
	e.SetBackdropBox(s1, "left", math.NaN(), math.NaN(), 100, 100.123, 0, "")
	if b := edtest.BdOf(e, s1, "left"); b.X != 0 || b.Y != 0 || b.H != 100.12 {
		t.Errorf("NaN в положении: %+v", b)
	}
	// Правая страница: область сдвинута.
	edtest.SetBg(t, e, s1, "right", "bg")
	e.SetBackdropBox(s1, "right", -1000, 0, 100, 100, 0, "")
	if b := edtest.BdOf(e, s1, "right"); b.X < 300-100/2-0.01 {
		t.Errorf("правая страница: центр левее области: %+v", b)
	}
	// Нет фона / неверная область — холостая правка.
	rev := edtest.Rev(e)
	e.SetBackdropBox("нет", "left", 1, 1, 50, 50, 0, "")
	e.SetBackdropBox(s1, "top", 1, 1, 50, 50, 0, "")
	if edtest.Rev(e) != rev {
		t.Error("правка без фона изменила проект")
	}
}

func TestBackdropBoxMerge(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "spread", "bg")
	undo := edtest.UndoLen(e)
	for i := range 5 {
		e.SetBackdropBox(s1, "spread", float64(i+1), 0, 600, 300, 0, "arrows1")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("серия стрелок: %d шагов отмены", edtest.UndoLen(e)-undo)
	}
	e.SetBackdropBox(s1, "spread", 9, 0, 600, 300, 0, "")
	e.SetBackdropBox(s1, "spread", 10, 0, 600, 300, 0, "")
	if edtest.UndoLen(e) != undo+3 {
		t.Fatalf("без сессии каждый вызов — шаг: %d", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	e.Undo()
	e.Undo()
	if b := edtest.BdOf(e, s1, "spread"); b.X != 0 {
		t.Fatalf("после отмены X = %v", b.X)
	}
}

func TestBackdropFrame(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropFrame(s1, "left", 9, 5, -5)
	if f := edtest.BdOf(e, s1, "left").Frame; f == nil || *f != (model.Frame{S: 4, X: 1, Y: -1}) {
		t.Fatalf("зажим кадра: %+v", f)
	}
	e.SetBackdropFrame(s1, "left", 2, math.NaN(), 0.5)
	if f := edtest.BdOf(e, s1, "left").Frame; *f != (model.Frame{S: 2, X: 0, Y: 0.5}) {
		t.Fatalf("NaN: %+v", f)
	}
	v := e.SetBackdropFrame(s1, "left", 2, 0.5, 0.5)
	if b := v.Spreads[0].Backdrops[0]; b.Frame.S != 2 || b.Img.W <= b.Box.W {
		t.Fatalf("View: %+v", b)
	}
	e.SetBackdropFrame(s1, "left", 1, 0, 0)
	if edtest.BdOf(e, s1, "left").Frame != nil {
		t.Fatal("кадр по умолчанию должен быть nil")
	}
}

// Зеркало меняет X кадра на обратный: видимая часть остаётся той же, только отражённой.
func TestBackdropFlipKeepsVisible(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropFrame(s1, "left", 2, 0.5, -0.25)
	e.FlipBackdrop(s1, "left", "h")
	b := edtest.BdOf(e, s1, "left")
	if !b.FlipH || b.FlipV || *b.Frame != (model.Frame{S: 2, X: -0.5, Y: -0.25}) {
		t.Fatalf("h: %+v %+v", b, b.Frame)
	}
	e.FlipBackdrop(s1, "left", "v")
	b = edtest.BdOf(e, s1, "left")
	if !b.FlipV || b.Frame.Y != 0.25 {
		t.Fatalf("v: %+v %+v", b, b.Frame)
	}
	e.FlipBackdrop(s1, "left", "h")
	e.FlipBackdrop(s1, "left", "v")
	if b = edtest.BdOf(e, s1, "left"); b.FlipH || b.FlipV || *b.Frame != (model.Frame{S: 2, X: 0.5, Y: -0.25}) {
		t.Fatalf("туда и обратно: %+v %+v", b, b.Frame)
	}
	rev := edtest.Rev(e)
	e.FlipBackdrop(s1, "left", "x")
	e.FlipBackdrop(s1, "right", "h")
	if edtest.Rev(e) != rev {
		t.Fatal("неверная ось или нет фона изменили проект")
	}
	// Нулевой ход не даёт «-0» в файле.
	e.SetBackdropFrame(s1, "left", 2, 0, 0)
	e.FlipBackdrop(s1, "left", "h")
	if data := model.Marshal(edtest.P(e)); bytes.Contains(data, []byte("-0")) {
		t.Fatalf("-0 в JSON: %s", data)
	}
}

func TestBackdropFit(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "right", "bg")
	e.SetBackdropBox(s1, "right", 350, 20, 100, 100, 30, "")
	e.SetBackdropFrame(s1, "right", 3, 0.2, 0.2)
	e.FlipBackdrop(s1, "right", "v")
	e.FitBackdrop(s1, "right")
	b := edtest.BdOf(e, s1, "right")
	if b.X != 300 || b.Y != 0 || b.W != 300 || b.H != 300 || b.Rot != 0 || b.Frame != nil || !b.FlipV {
		t.Fatalf("по области: %+v", b)
	}
	rev := edtest.Rev(e)
	e.FitBackdrop(s1, "right")
	if edtest.Rev(e) != rev {
		t.Fatal("повторное «по области» не холостое")
	}
}

func TestBackdropClear(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackground(s1, "left", "#112233")
	v := e.ClearBackdrop(s1, "left")
	if len(v.Spreads[0].Backdrops) != 0 || v.Spreads[0].BgSet[0] != "#112233" || len(v.BackdropUsed) != 0 {
		t.Fatalf("снятие: %+v", v.Spreads[0])
	}
	if _, ok := e.Ref("bg"); ok {
		t.Fatal("фото осталось в Photos")
	}
	if u := e.Undo(); len(u.Spreads[0].Backdrops) != 1 {
		t.Fatal("отмена не вернула фон")
	}
}

func TestBackdropRescale(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "right", "bg")
	e.SetBackdropBox(s1, "right", 330, 30, 150, 90, 20, "")
	s := model.DefaultSettings()
	s.PageW, s.PageH = 150, 600
	e.SetSettings(s)
	b := edtest.BdOf(e, s1, "right")
	if b.X != 165 || b.W != 75 || b.Y != 60 || b.H != 180 || b.Rot != 20 {
		t.Fatalf("после смены формата: %+v", b)
	}
}

// Фото фона не выбрасывается pruneRefs; ушедшее из мест теряет кадр, подпись и зеркало.
func TestBackdropPruneKeepsRef(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	e.SetFrame("a", 2, 0.5, 0.5)
	e.SetCaption("a", "Подпись", "")
	e.SetFlip([]string{"a"}, "h")
	edtest.SetBg(t, e, s1, "spread", "a")
	if v := e.RemovePhotos([]string{"a"}); len(v.Spreads[0].Photos) != 1 {
		t.Fatalf("фото не снято: %v", v.Spreads[0].Photos)
	}
	r, ok := e.Ref("a")
	if !ok {
		t.Fatal("фото фона выброшено из Photos")
	}
	if r.Frame != nil || r.Caption != nil || r.FlipH || r.FlipV {
		t.Fatalf("свойства места пережили уход из мест: %+v", r)
	}
	// Фото в месте и на фоне: свойства места остаются.
	e.SetFrame("b", 2, 0, 0)
	edtest.SetBg(t, e, s1, "spread", "b")
	if r, _ := e.Ref("b"); r.Frame == nil {
		t.Fatal("фото в месте потеряло кадр из-за фона")
	}
	// Фон остался без фото в Photos только после снятия.
	e.ClearBackdrop(s1, "spread")
	if _, ok := e.Ref("a"); ok {
		t.Fatal("фото без места и без фона осталось")
	}
}

// Опустевший разворот с фоном-фото не удаляется normalize.
func TestBackdropKeepsSpread(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	v := e.RemovePhotos([]string{"a", "b"})
	if len(v.Spreads) != 2 || v.Spreads[0].ID != s1 || len(v.Spreads[0].Backdrops) != 1 {
		t.Fatalf("разворот с фоном удалён: %+v", v.Spreads)
	}
}

// Автосборка не берёт фото фона (решение владельца): оно остаётся только на фоне.
func TestAutoBuildSkipsBackdrop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "spread", "bg")
	v, err := e.AutoBuild(edtest.Photos("bg", "x", "y"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range v.Spreads {
		if slices.Contains(s.Photos, "bg") {
			t.Fatalf("фото фона в автосборке: %v", s.Photos)
		}
	}
	if !slices.Equal(v.BackdropUsed, []string{"bg"}) || !edtest.HasRef(edtest.P(e), "x") {
		t.Fatalf("фон %v, x в альбоме %v", v.BackdropUsed, edtest.HasRef(edtest.P(e), "x"))
	}
}

// «Заполнить пустые» тоже обходит фото фона и не считает его несевшим.
func TestFillPlacesSkipsBackdrop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, ""), edtest.Gp(150, 12, 100, 100, "")})
	edtest.SetBg(t, e, "s1", "spread", "bg")
	_, left := e.FillPlaces(edtest.Photos("bg", "c"), "s1")
	if left != 0 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"c", ""}) {
		t.Fatalf("left %d, места %v", left, edtest.PhotosOf(e, 0))
	}
}

// Тот же снимок на том же фоне — холостая правка: кадр и наклон не сбрасываются (045).
func TestBackdropSameKeepsFrameTilt(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg")
	e.SetBackdropAngle(s1, "left", 4.5, "")
	e.SetBackdropAngle(s1, "right", -4.5, "")
	e.SetBackdropFrame(s1, "left", 2, 0.5, -0.5) // после наклона: он держит приближение (057)
	before := edtest.BdOf(e, s1, "left")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.Same(t, e, rev, undo, "то же фото с кадром и наклоном")
	edtest.SetBg(t, e, s1, "right", "bg")
	edtest.Same(t, e, rev, undo, "то же фото на другой области")
	if b := edtest.BdOf(e, s1, "left"); b.Frame == nil || b.Frame.R != before.Frame.R || b.Frame.S != 2 {
		t.Fatalf("кадр изменился: %+v", b)
	}
}
