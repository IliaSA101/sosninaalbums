package design_test

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
)

// Прозрачность фото места (079, PhotoRef.Fade): едет с фото, как тон.

// fadeOf — прозрачность фото id в проекте.
func fadeOf(t *testing.T, e *album.Editor, id string) float64 {
	t.Helper()
	r, ok := e.Ref(id)
	if !ok {
		t.Fatalf("фото %s нет", id)
	}
	return r.Fade
}

// SetPhotoFade: зажим 0…1 шагом 0,01, серия одной сессией — один шаг, у «Авто» без закрепления;
// NaN/Inf, то же значение, чужие ID и пустой список — холостые; вид кадра несёт Fade; едет с
// фото на другой разворот; круг Save → Load; отмена — прежние, 0 не пишется.
func TestSetPhotoFade(t *testing.T) {
	e, _, s2 := edtest.NewTwo(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		e.SetPhotoFade([]string{"a"}, f, "")
	}
	e.SetPhotoFade([]string{"нет", ""}, 0.5, "")
	e.SetPhotoFade([]string{"a", "b"}, -3, "")
	e.SetPhotoFade(nil, 0.5, "")
	edtest.Same(t, e, rev, undo, "холостые")
	e.SetPhotoFade([]string{"a", "b", "a", "нет"}, 0.257, "f1")
	v := e.SetPhotoFade([]string{"a", "b"}, 1.7, "f1")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("серия — один шаг, шагов +%d", edtest.UndoLen(e)-undo)
	}
	if fadeOf(t, e, "a") != 1 || fadeOf(t, e, "b") != 1 || fadeOf(t, e, "c") != 0 {
		t.Fatalf("прозрачность: %v %v %v", fadeOf(t, e, "a"), fadeOf(t, e, "b"), fadeOf(t, e, "c"))
	}
	if f := frameOf(t, v, "a"); f.Fade != 1 {
		t.Fatalf("вид: %+v", f)
	}
	for _, sv := range v.Spreads {
		if sv.Grid {
			t.Fatal("прозрачность закрепила «Авто»")
		}
	}
	e.SetPhotoFade([]string{"a"}, 0.257, "f2")
	if fadeOf(t, e, "a") != 0.26 {
		t.Fatalf("шаг 0,01: %v", fadeOf(t, e, "a"))
	}
	e.MovePhoto("a", s2, -1)
	if fadeOf(t, e, "a") != 0.26 {
		t.Fatalf("после переноса: %v", fadeOf(t, e, "a"))
	}
	// После Load ID фото — от пути (Rekey): сверка по набору значений.
	var fades []float64
	for _, r := range edtest.SaveLoad(t, e).Photos {
		fades = append(fades, r.Fade)
	}
	if slices.Sort(fades); !slices.Equal(fades, []float64{0, 0.26, 1}) {
		t.Fatalf("Save → Load: %v", fades)
	}
	e.Undo()
	e.Undo()
	if fadeOf(t, e, "a") != 1 {
		t.Fatalf("отмена: %v", fadeOf(t, e, "a"))
	}
	e.Undo()
	if fadeOf(t, e, "a") != 0 || fadeOf(t, e, "b") != 0 {
		t.Fatal("отмена серии")
	}
	// Тон и прозрачность независимы; 0 снимает прозрачность, тон остаётся.
	e.SetTone("a", 10, 5)
	e.SetPhotoFade([]string{"a"}, 0.4, "")
	e.SetPhotoFade([]string{"a"}, 0, "")
	if r, _ := e.Ref("a"); r.Fade != 0 || r.Adj == nil || r.Adj.Bright != 10 {
		t.Fatalf("после нуля: %+v", r)
	}
}

// Чужая сессия ползунка — новый шаг (не сливается с прежней серией); отмена — по одной серии.
func TestSetPhotoFadeSeries(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo := edtest.UndoLen(e)
	for _, f := range []float64{0.2, 0.5, 0.8} {
		e.SetPhotoFade([]string{"a", "b"}, f, "s1")
	}
	e.SetPhotoFade([]string{"a", "b"}, 0.9, "s2")
	if edtest.UndoLen(e) != undo+2 || fadeOf(t, e, "b") != 0.9 {
		t.Fatalf("две серии: шагов +%d, b %v", edtest.UndoLen(e)-undo, fadeOf(t, e, "b"))
	}
	e.Undo()
	if fadeOf(t, e, "a") != 0.8 || fadeOf(t, e, "b") != 0.8 {
		t.Fatalf("отмена второй серии: %v %v", fadeOf(t, e, "a"), fadeOf(t, e, "b"))
	}
	e.Undo()
	if fadeOf(t, e, "a") != 0 || fadeOf(t, e, "b") != 0 {
		t.Fatal("отмена первой серии не вернула нули")
	}
}
