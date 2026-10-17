package cover_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Фото корешка обложки (061) через Editor: бросок, взаимоисключение, раскладки, пересчёт
// корешка, надпись без плашки под фото.

// spineAlbum — coverAlbum(n) в раскладке «Задняя + корешок + лицевая» с фото pid(1) на корешке.
func spineAlbum(t *testing.T, n int) *album.Editor {
	t.Helper()
	e := coverAlbum(t, n)
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, pid(1))
	return e
}

// spineArea — область фото корешка обложки сейчас.
func spineArea(e *album.Editor) (x, w, h float64) {
	a := edtest.P(e).BdArea(sheet(e), model.ScopeSpine)
	return a.X, a.W, a.H
}

// TestSetBackdropSpine — фото на корешок: только у обложки и только в both (иначе ErrScope с
// понятным текстом и без шага отмены); рамка — по области корешка, один шаг; то же фото —
// холостая; Undo снимает.
func TestSetBackdropSpine(t *testing.T) {
	e := coverAlbum(t, 6)
	var rev, undo int
	for _, kind := range []string{"front", "wrap"} {
		if _, err := e.SetCoverLayout(kind); err != nil {
			t.Fatal(err)
		}
		rev, undo = edtest.Rev(e), edtest.UndoLen(e)
		if _, err := e.SetBackdrop(model.CoverID, model.ScopeSpine, info(pid(1))); !errors.Is(err, model.ErrSpineLayout) || !errors.Is(err, model.ErrScope) {
			t.Fatalf("%s: корешок не в both: %v", kind, err)
		}
		edtest.Same(t, e, rev, undo, kind+": отказ корешка")
	}
	if _, err := e.SetBackdrop(edtest.P(e).Spreads[0].ID, model.ScopeSpine, info(pid(1))); !errors.Is(err, model.ErrScope) || errors.Is(err, model.ErrSpineLayout) {
		t.Fatalf("корешок у разворота: %v", err)
	}
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	undo = edtest.UndoLen(e)
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, pid(1))
	b := sheet(e).BgPhotoS
	x, w, h := spineArea(e)
	if b == nil || b.Photo != pid(1) || b.X != x || b.W != w || b.H != h || b.Y != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("фото корешка %+v, область %v %v %v, шагов %d", b, x, w, h, edtest.UndoLen(e)-undo)
	}
	if sheet(e).BgPhotoR == nil {
		t.Fatal("фото корешка сняло лицевую")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, pid(1))
	edtest.Same(t, e, rev, undo, "то же фото на корешок")
	e.Undo()
	if sheet(e).BgPhotoS != nil {
		t.Fatal("Undo не снял фото корешка")
	}
}

// TestSpineExclusive — одно фото через лист исключает фото корешка и обратно (061 I2); фото
// сторонки корешок не трогает; Undo возвращает всё.
func TestSpineExclusive(t *testing.T) {
	e := spineAlbum(t, 6)
	edtest.SetBg(t, e, model.CoverID, model.ScopeLeft, pid(2))
	if s := sheet(e); s.BgPhotoS == nil || s.BgPhotoL == nil || s.BgPhotoR == nil {
		t.Fatalf("задняя сняла корешок: %+v", s)
	}
	before := string(model.Marshal(edtest.P(e)))
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpread, pid(4))
	if s := sheet(e); s.BgPhoto == nil || s.BgPhotoL != nil || s.BgPhotoR != nil || s.BgPhotoS != nil {
		t.Fatalf("одно фото через лист: %+v", s)
	}
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, pid(1))
	if s := sheet(e); s.BgPhoto != nil || s.BgPhotoS == nil {
		t.Fatalf("корешок после фото через лист: %+v", s)
	}
	e.Undo()
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("Undo не вернул три фото")
	}
	// Узор на сторонку корешок не трогает, узор на весь лист снимает все фото фона (с корешком).
	pat := model.Pattern{Kind: "dots", Scale: 1}
	if _, err := e.ApplyPattern(model.CoverID, model.ScopeLeft, pat, ""); err != nil || sheet(e).BgPhotoS == nil || sheet(e).BgPhotoL != nil {
		t.Fatalf("узор на задней: %v %+v", err, sheet(e))
	}
	if _, err := e.ApplyPattern(model.CoverID, model.ScopeSpread, pat, ""); err != nil || len(sheet(e).Backdrops()) != 0 {
		t.Fatalf("узор на весь лист: %v %+v", err, sheet(e).Backdrops())
	}
}

// TestSetLayoutSpine — все 9 переходов раскладки: фото корешка остаётся только при both → both
// (холостая правка), из both уходит (Undo возвращает байт в байт), в both приходят с пустым корешком.
func TestSetLayoutSpine(t *testing.T) {
	kinds := []string{"front", "wrap", "both"}
	for _, from := range kinds {
		for _, to := range kinds {
			layoutSpine(t, from, to)
		}
	}
}

// layoutSpine — один переход from → to для TestSetLayoutSpine.
func layoutSpine(t *testing.T, from, to string) {
	t.Helper()
	e := coverAlbum(t, 6)
	if _, err := e.SetCoverLayout(from); err != nil {
		t.Fatal(err)
	}
	if from == "both" {
		edtest.SetBg(t, e, model.CoverID, model.ScopeLeft, pid(2))
		edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, pid(1))
	}
	before, rev, undo := string(model.Marshal(edtest.P(e))), edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SetCoverLayout(to); err != nil {
		t.Fatal(err)
	}
	if from == to {
		edtest.Same(t, e, rev, undo, from+" → "+to)
		if from == "both" && sheet(e).BgPhotoS == nil {
			t.Fatal("both → both сняло фото корешка")
		}
		return
	}
	if sheet(e).BgPhotoS != nil {
		t.Fatalf("%s → %s: фото корешка осталось", from, to)
	}
	if d := e.View().Cover.Drop; to == "both" && (len(d) == 0 || d[len(d)-1].Kind != "spine") {
		t.Fatalf("%s → %s: нет пустой зоны корешка: %+v", from, to, d)
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatalf("%s → %s: Undo не вернул обложку", from, to)
	}
}

// TestReflowSpine — разворотов 5 → 20 (корешок 6 → 14 мм), затем пресет с расставом 9:
// рамка фото корешка едет за областью [X2, X5], кадр и наклон остаются; Undo — одним шагом.
func TestReflowSpine(t *testing.T) {
	e := spineAlbum(t, 5)
	e.SetBackdropFrame(model.CoverID, model.ScopeSpine, 1.5, 0.3, -0.2)
	e.SetBackdropAngle(model.CoverID, model.ScopeSpine, 4, "")
	fr := *sheet(e).BgPhotoS.Frame
	check := func(what string) {
		t.Helper()
		b := sheet(e).BgPhotoS
		x, w, h := spineArea(e)
		if b == nil || !near(b.X, x) || !near(b.W, w) || !near(b.H, h) || b.Y != 0 || *b.Frame != fr {
			t.Fatalf("%s: фото корешка %+v, область x %v w %v h %v, кадр был %+v", what, b, x, w, h, fr)
		}
	}
	if _, w, _ := spineArea(e); w != 6 {
		t.Fatalf("корешок при 5 разворотах %v", w)
	}
	for i := range 15 {
		edtest.NewSpread(e, -1, []model.PhotoInfo{info(pid(50 + i))})
	}
	if _, w, _ := spineArea(e); w != 14 {
		t.Fatalf("корешок при 20 разворотах %v", w)
	}
	check("20 разворотов")
	undo := edtest.UndoLen(e)
	if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil {
		t.Fatal(err)
	}
	cg := edtest.P(e).CoverGeo()
	if cg.X[3]-cg.X[2] != 9 {
		t.Fatalf("расстав %v", cg.X[3]-cg.X[2])
	}
	check("расстав 9")
	e.Undo()
	check("Undo пресета")
	if edtest.UndoLen(e) != undo {
		t.Fatalf("пресет — не один шаг: %d", edtest.UndoLen(e)-undo)
	}
}

// TestSpineTextNoPlateOverPhoto — «Надпись» на корешке с фото: плашка не ставится (ушла бы
// под фото); без фото — по-прежнему цвет лицевой; своя плашка остаётся.
func TestSpineTextNoPlateOverPhoto(t *testing.T) {
	e := spineAlbum(t, 6)
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover; c.SpineBg != "" || edtest.P(e).SpineTextID() == "" {
		t.Fatalf("плашка под фото корешка %q, надпись %q", c.SpineBg, edtest.P(e).SpineTextID())
	}
	e = spineAlbum(t, 6)
	e.SetCoverSpineBg("#102030")
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover; c.SpineBg != "#102030" {
		t.Fatalf("своя плашка %q", c.SpineBg)
	}
	e = coverAlbum(t, 6)
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CoverSpineText(); err != nil || edtest.P(e).Cover.SpineBg == "" {
		t.Fatalf("без фото корешка плашка не поставлена: %v", err)
	}
}

// TestSpineBackdropEdits — фото корешка правится как фон страницы: рамка, кадр, наклон,
// зеркало, прозрачность, «По области», «Убрать»; каждая правка — шаг, повтор «По области» —
// холостой.
func TestSpineBackdropEdits(t *testing.T) {
	e := spineAlbum(t, 6)
	x, w, h := spineArea(e)
	steps := []struct {
		what string
		do   func()
		ok   func(b *model.Backdrop) bool
	}{
		{"рамка 90°", func() { e.SetBackdropBox(model.CoverID, model.ScopeSpine, x, 0, h, 30, 90, "") },
			func(b *model.Backdrop) bool { return b.Rot == 90 && b.W == h && b.H == 30 }},
		{"кадр", func() { e.SetBackdropFrame(model.CoverID, model.ScopeSpine, 2, 0.5, 0) },
			func(b *model.Backdrop) bool { return b.Frame != nil && b.Frame.S == 2 }},
		{"наклон", func() { e.SetBackdropAngle(model.CoverID, model.ScopeSpine, -3, "") },
			func(b *model.Backdrop) bool { return b.Frame.R == -3 }},
		{"зеркало", func() { e.FlipBackdrop(model.CoverID, model.ScopeSpine, "h") },
			func(b *model.Backdrop) bool { return b.FlipH }},
		{"прозрачность", func() { e.SetBackdropFade(model.CoverID, model.ScopeSpine, 0.4, "") },
			func(b *model.Backdrop) bool { return b.Fade == 0.4 }},
		{"по области", func() { e.FitBackdrop(model.CoverID, model.ScopeSpine) },
			func(b *model.Backdrop) bool { return b.X == x && b.W == w && b.H == h && b.Rot == 0 }},
	}
	for _, st := range steps {
		undo := edtest.UndoLen(e)
		st.do()
		if b := sheet(e).BgPhotoS; b == nil || !st.ok(b) || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%s: %+v, шагов %d", st.what, b, edtest.UndoLen(e)-undo)
		}
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.FitBackdrop(model.CoverID, model.ScopeSpine)
	edtest.Same(t, e, rev, undo, "повтор «По области»")
	e.ClearBackdrop(model.CoverID, model.ScopeSpine)
	if sheet(e).BgPhotoS != nil || sheet(e).BgPhotoR == nil {
		t.Fatalf("«Убрать фон» корешка: %+v", sheet(e))
	}
}
