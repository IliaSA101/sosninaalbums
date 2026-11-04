package paint

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/math/f64"
)

// halfImg — картинка 64×32: левая половина красная, правая прозрачная.
func halfImg() *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 32 {
			im.SetNRGBA(x, y, color.NRGBA{200, 0, 0, 255})
		}
	}
	return im
}

func drawRaster(r *Raster, dstW, dstH int, m f64.Aff3, w, h float64) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	if c := New(dst, dst.Rect, m, w, h, 0); c != nil {
		c.Raster(r, w, h)
	}
	return dst
}

// TestRaster — картинка ложится в рамку; зеркало и поворот переносят её; рамка частично за
// холстом и пустая картинка не роняют; цвета остаются корректными (r,g,b не больше альфы).
func TestRaster(t *testing.T) {
	r := NewRaster(halfImg())
	if w, h := r.Size(); w != 64 || h != 32 {
		t.Fatalf("размер %d×%d", w, h)
	}
	if NewRaster(nil) != nil || NewRaster(image.NewNRGBA(image.Rect(0, 0, 0, 0))) != nil {
		t.Error("пустая картинка дала растр")
	}
	var none *Raster
	if w, h := none.Size(); w != 0 || h != 0 {
		t.Error("nil-растр с размером")
	}
	plain := drawRaster(r, 100, 50, f64.Aff3{100, 0, 0, 0, 50, 0}, 1, 1)
	flip := drawRaster(r, 100, 50, f64.Aff3{-100, 0, 100, 0, 50, 0}, 1, 1)
	vflip := drawRaster(r, 100, 50, f64.Aff3{100, 0, 0, 0, -50, 50}, 1, 1)
	rot := drawRaster(r, 50, 100, f64.Aff3{0, -50, 50, 100, 0, 0}, 1, 1) // 90° по часовой
	if plain.RGBAAt(10, 25).A != 255 || plain.RGBAAt(90, 25).A != 0 {
		t.Error("без преобразования: не левая половина")
	}
	if flip.RGBAAt(10, 25).A != 0 || flip.RGBAAt(90, 25).A != 255 {
		t.Error("зеркало: не правая половина")
	}
	if vflip.RGBAAt(10, 25).A != 255 || vflip.RGBAAt(90, 25).A != 0 {
		t.Error("зеркало по вертикали сломало левую половину")
	}
	if rot.RGBAAt(25, 10).A != 255 || rot.RGBAAt(25, 90).A != 0 {
		t.Errorf("поворот: левая половина должна уйти наверх: %v %v", rot.RGBAAt(25, 10), rot.RGBAAt(25, 90))
	}
	// Рамка выходит за холст со всех сторон и целиком мимо него.
	for _, m := range []f64.Aff3{{200, 0, -50, 0, 100, -25}, {100, 0, 500, 0, 50, 0}, {-100, 0, 0, 0, 50, 0}} {
		img := drawRaster(r, 100, 50, m, 1, 1)
		for i := 0; i < len(img.Pix); i += 4 {
			p := img.Pix[i : i+4]
			if p[0] > p[3] || p[1] > p[3] || p[2] > p[3] {
				t.Fatalf("цвет больше альфы: %v", p)
			}
		}
	}
	// Сильное уменьшение идёт через копии: красный не пропадает, не темнеет.
	small := drawRaster(r, 8, 4, f64.Aff3{8, 0, 0, 0, 4, 0}, 1, 1)
	if p := small.RGBAAt(1, 2); p.A < 250 || p.R < 190 {
		t.Errorf("миниатюра: %v", p)
	}
	// Холст nil (fade 1) и нулевая рамка — без паники.
	drawRaster(r, 10, 10, f64.Aff3{10, 0, 0, 0, 10, 0}, 0, 1)
	var c Canvas
	c.Raster(nil, 1, 1)
}
