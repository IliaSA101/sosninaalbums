package text

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
)

// inkBounds — границы нечисто-белых пикселей.
func inkBounds(img *image.RGBA) (r image.Rectangle) {
	r = image.Rectangle{Min: image.Pt(math.MaxInt32, math.MaxInt32), Max: image.Pt(-1, -1)}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R < 200 {
				r.Min.X, r.Min.Y = min(r.Min.X, x), min(r.Min.Y, y)
				r.Max.X, r.Max.Y = max(r.Max.X, x+1), max(r.Max.Y, y+1)
			}
		}
	}
	return r
}

func TestDrawMatchesLayout(t *testing.T) {
	const dpi = 300
	mmpx := func(mm float64) float64 { return mm / 25.4 * dpi }
	for _, fam := range FamilyIDs() {
		st := Style{Font: fam, Size: 20, Color: "#000000", Align: "center"}
		bw, x0, y0 := 80.0, 20.0, 15.0
		b := Layout("Иван и Мария\nприглашают вас на свою свадьбу в субботу", st, bw)
		img := image.NewRGBA(image.Rect(0, 0, int(mmpx(130)), int(mmpx(70))))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		Draw(img, b, st, x0, y0, 0, dpi)
		ink := inkBounds(img)
		if ink.Empty() {
			t.Fatalf("%s: ничего не нарисовано", fam)
		}
		tol := mmpx(1)
		if float64(ink.Min.X) < mmpx(x0)-tol || float64(ink.Max.X) > mmpx(x0+b.W)+tol ||
			float64(ink.Min.Y) < mmpx(y0)-tol || float64(ink.Max.Y) > mmpx(y0+b.H)+tol {
			t.Errorf("%s: чернила %v вне рамки %.0f..%.0f x %.0f..%.0f px", fam, ink,
				mmpx(x0), mmpx(x0+b.W), mmpx(y0), mmpx(y0+b.H))
		}
	}
}

func TestDrawColorAndNoop(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	Draw(img, Layout("", testStyle, 10), testStyle, 0, 0, 0, 300) // пустой блок
	Draw(img, Layout("A", testStyle, 10), testStyle, 0, 0, 0, 0)  // dpi 0
	if !inkBounds(img).Empty() {
		t.Error("пустой блок или dpi 0 что-то нарисовали")
	}
	st := Style{Font: "montserrat", Size: 30, Color: "#ff0000", Align: "left"}
	Draw(img, Layout("Ш", st, 20), st, 0, 0, 0, 300)
	found := false
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] == 255 && img.Pix[i+1] == 0 && img.Pix[i+2] == 0 && img.Pix[i+3] == 255 {
			found = true
			break
		}
	}
	if !found {
		t.Error("красных пикселей нет")
	}
}
