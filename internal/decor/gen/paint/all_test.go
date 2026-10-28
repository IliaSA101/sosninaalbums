package paint_test

import (
	"image"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"golang.org/x/image/math/f64"
)

// TestCanvasAllGens — холст покрыт всеми рисовальщиками: каждый узор и стикер на 60×40 мм при
// 4 px/мм рисует не пусто (внешний тест: gen импортирует paint, не наоборот).
func TestCanvasAllGens(t *testing.T) {
	const k = 4
	w, h := 60*k, 40*k
	nonEmpty := func(img *image.RGBA) bool {
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				return true
			}
		}
		return false
	}
	pats := gen.Patterns()
	if len(pats) == 0 {
		t.Fatal("нет узоров")
	}
	for _, m := range pats {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		gen.DrawPattern(img, img.Rect, gen.Spec{Kind: m.Kind, Seed: 3}, k, 0)
		if !nonEmpty(img) {
			t.Errorf("узор %s: пусто", m.Kind)
		}
	}
	arts := gen.Arts()
	if len(arts) == 0 {
		t.Fatal("нет стикеров")
	}
	for _, m := range arts {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		gen.DrawArt(img, f64.Aff3{float64(h), 0, 0, 0, float64(h), 0}, gen.Spec{Kind: m.Kind, Seed: 3, Aspect: m.Aspect}, 0)
		if !nonEmpty(img) {
			t.Errorf("стикер %s: пусто", m.Kind)
		}
	}
}
