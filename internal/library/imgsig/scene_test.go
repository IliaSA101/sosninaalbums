package imgsig

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand/v2"
	"testing"
)

// blob — цветное пятно сцены.
type blob struct {
	x, y, r  float64
	rr, g, b float64
}

// scene — синтетический «кадр»: диагональный градиент и пятна. Рисуется в любое окно (сдвиг,
// приближение), как кадры серии, но без файлов и фото в репозитории.
type scene struct {
	base  [3]float64
	slope [3]float64
	blobs []blob
}

func newScene(seed uint64) scene {
	rng := rand.New(rand.NewPCG(seed, seed*7919+13))
	s := scene{}
	for c := range 3 {
		s.base[c] = 40 + rng.Float64()*120
		s.slope[c] = (rng.Float64() - 0.5) * 120
	}
	for range 14 {
		s.blobs = append(s.blobs, blob{
			x: rng.Float64(), y: rng.Float64(), r: 0.04 + rng.Float64()*0.2,
			rr: (rng.Float64() - 0.5) * 300, g: (rng.Float64() - 0.5) * 300, b: (rng.Float64() - 0.5) * 300,
		})
	}
	return s
}

// view — окно в сцене: левый верх и размер в долях кадра.
type view struct{ x, y, w, h float64 }

var (
	full  = view{0, 0, 1, 1}
	shift = view{0.03, 0.01, 0.97, 0.97} // сдвиг рамки на 3 %
	zoom5 = view{0.05, 0.05, 0.90, 0.90} // ближе на 5 % с каждой стороны
)

func clamp8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, v)))
}

// render рисует окно сцены в w×h; k — яркость (1 — как есть), gray — без цвета.
func (s scene) render(v view, w, h int, k float64, gray bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for py := range h {
		fy := v.y + (float64(py)+0.5)/float64(h)*v.h
		for px := range w {
			fx := v.x + (float64(px)+0.5)/float64(w)*v.w
			var c [3]float64
			for i := range c {
				c[i] = s.base[i] + s.slope[i]*(fx+fy-1)
			}
			for _, b := range s.blobs {
				d2 := ((fx-b.x)*(fx-b.x) + (fy-b.y)*(fy-b.y)) / (b.r * b.r)
				wgt := math.Exp(-d2)
				c[0] += b.rr * wgt
				c[1] += b.g * wgt
				c[2] += b.b * wgt
			}
			if gray {
				m := 0.299*c[0] + 0.587*c[1] + 0.114*c[2]
				c = [3]float64{m, m, m}
			}
			img.SetRGBA(px, py, color.RGBA{clamp8(c[0] * k), clamp8(c[1] * k), clamp8(c[2] * k), 255})
		}
	}
	return img
}

// thumb — как миниатюра кэша: JPEG q82 и обратно (получается *image.YCbCr).
func thumb(t testing.TB, img image.Image, q int) image.Image {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// thumbOf — миниатюра 400×300 окна сцены.
func thumbOf(t testing.TB, s scene, v view, k float64) image.Image {
	t.Helper()
	return thumbSize(t, s, v, k, 400, 300)
}

// thumbSize — миниатюра w×h (JPEG q82) окна сцены.
func thumbSize(t testing.TB, s scene, v view, k float64, w, h int) image.Image {
	t.Helper()
	return thumb(t, s.render(v, w, h, k, false), 82)
}
