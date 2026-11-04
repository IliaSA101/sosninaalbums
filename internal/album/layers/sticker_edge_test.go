package layers_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Края стикеров (048 Ф2): не число в прозрачности, замена чужого или неизвестного, высокая
// пропорция на пределе листа.

// TestStickerFadeNotFinite — NaN и ±Inf в прозрачности: холостая правка у каждого из стикеров,
// значение и серия не тронуты; конечное значение потом применяется.
func TestStickerFadeNotFinite(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	a := addSticker(t, e, s1, 0, own(keyA, 400, 200))
	b := addSticker(t, e, s1, 1, art("plate"))
	e.SetStickerFade(s1, a, 0.4, "fade-1")
	e.SetStickerFade(s1, b, 0.6, "fade-2")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		e.SetStickerFade(s1, a, f, "fade-1")
		e.SetStickerFade(s1, b, f, "")
	}
	edtest.Same(t, e, rev, undo, "не число в прозрачности")
	if stOf(e, a).Fade != 0.4 || stOf(e, b).Fade != 0.6 {
		t.Fatalf("прозрачность тронута: %v %v", stOf(e, a).Fade, stOf(e, b).Fade)
	}
	e.SetStickerFade(s1, b, 0.7, "fade-2") // серия после холостых продолжается тем же шагом
	if stOf(e, b).Fade != 0.7 || edtest.UndoLen(e) != undo {
		t.Fatalf("серия после NaN: fade %v, шагов %d → %d", stOf(e, b).Fade, undo, edtest.UndoLen(e))
	}
}

// TestReplaceStickerMissing — «Заменить…» неизвестного стикера, стикера чужого листа и на
// неизвестном листе — холостая без ошибки: ни шага, ни записи нового фото; у обоих стикеров
// рисунок прежний.
func TestReplaceStickerMissing(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	a := addSticker(t, e, s1, 0, own(keyA, 400, 200))
	b := addSticker(t, e, s2, 0, art("plate"))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, c := range []struct{ sheet, id, name string }{
		{s1, "нет", "неизвестный стикер"},
		{s2, a, "стикер другого листа"},
		{s1, b, "стикер другого листа (обратно)"},
		{"нет", a, "неизвестный лист"},
	} {
		for _, src := range []model.StickerSrc{art("plate"), own(keyB, 100, 400)} {
			_, changed, err := e.ReplaceSticker(c.sheet, c.id, src)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if changed {
				t.Fatalf("%s: холостая замена вернула changed=true", c.name)
			}
			edtest.Same(t, e, rev, undo, c.name)
		}
	}
	if edtest.P(e).Photos[model.LibPhotoID(keyB)].Lib != "" {
		t.Fatal("в Photos осталась запись неприменённой замены")
	}
	if stOf(e, a).Photo != model.LibPhotoID(keyA) || stOf(e, b).Art.Kind != "plate" {
		t.Fatalf("рисунок тронут: %+v %+v", *stOf(e, a), *stOf(e, b))
	}
}

// TestFitStickerTallLimit — высокий рисунок (1:4) шире двух листов: ширина урезается так, чтобы
// высота не превысила предел (2·лист); пропорция держится; несколько высоких разом.
func TestFitStickerTallLimit(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t) // лист 600×300
	a := addSticker(t, e, s1, 0, own(keyA, 100, 400))
	b := addSticker(t, e, s1, 1, own(keyB, 100, 400))
	for _, id := range []string{a, b} {
		e.SetStickerBox(s1, id, 0, 0, 5000, 5000, 0, "")
		st := stOf(e, id)
		if st.W != 300 || st.H != 1200 {
			t.Fatalf("%s: %+v, ждали 300×1200", id, *st)
		}
		if cx, cy := center(st); cx < 0 || cx > 600 || cy < 0 || cy > 300 {
			t.Fatalf("%s: центр (%v, %v) за листом", id, cx, cy)
		}
	}
	edtest.MustGrid(t, e)
}
