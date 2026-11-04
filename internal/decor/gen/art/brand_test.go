package art_test

import (
	"image"
	"image/png"
	"os"
	"testing"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

var brandKinds = []string{"brand-albums-black", "brand-albums-white", "brand-films-black", "brand-films-white"}

// TestBrandStickers — фирменные стикеры (спека 116): пропорция совпадает с файлом, рисунок не
// пуст и не выходит за рамку, цвета генератора не принимаются; на 300 dpi (экспорт) рисуется
// чётко: чёрный «sosnina» и красный «albums/films» на месте, цвета корректны (не больше альфы).
func TestBrandStickers(t *testing.T) {
	for _, kind := range brandKinds {
		m, ok := gen.Lookup(kind)
		if !ok || !m.Image || m.Aspect < 2 || m.Aspect > 4 || len(m.Inks) != 0 || m.Stretch || m.Random {
			t.Fatalf("%s: %+v", kind, m)
		}
		f, err := os.Open("brand/" + kind + ".png")
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		_ = f.Close()
		if err != nil || m.Aspect != float64(cfg.Width)/float64(cfg.Height) {
			t.Errorf("%s: пропорция %v, файл %+v %v", kind, m.Aspect, cfg, err)
		}
		if got := gen.Clamp(gen.Spec{Kind: kind, Inks: []string{"#112233"}, Seed: 5, Aspect: 9}); got.Inks != nil || got.Seed != 0 || got.Aspect != 0 {
			t.Errorf("%s: параметры не зажаты: %+v", kind, got)
		}
		w, h := 1181, int(1181/m.Aspect)
		img := image.NewRGBA(image.Rect(0, 0, w+20, h+20))
		gen.DrawArt(img, f64.Aff3{float64(w), 0, 10, 0, float64(h), 10}, gen.Spec{Kind: kind}, 0)
		red, dark, light, out := brandPixels(t, img, kind, w, h)
		if out != 0 || red < 1000 || dark+light < 20000 || (dark > 0) == (light > 0) {
			t.Errorf("%s: за рамкой %d, красных %d, тёмных %d, светлых %d", kind, out, red, dark, light)
		}
	}
}

// TestBrandFlipRot — зеркало и поворот фирменного стикера: в рамке остаётся то же число
// закрашенных px (±5 %), а не пустота.
func TestBrandFlipRot(t *testing.T) {
	count := func(m f64.Aff3, w, h int) int {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		gen.DrawArt(img, m, gen.Spec{Kind: "brand-films-black"}, 0)
		n := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 128 {
				n++
			}
		}
		return n
	}
	a := count(f64.Aff3{300, 0, 0, 0, 127, 0}, 300, 127)
	b := count(f64.Aff3{-300, 0, 300, 0, 127, 0}, 300, 127)
	c := count(f64.Aff3{0, -127, 127, 300, 0, 0}, 127, 300)
	for _, n := range []int{b, c} {
		if a < 5000 || n < a*95/100 || n > a*105/100 {
			t.Errorf("закрашено %d и %d", a, n)
		}
	}
}

// brandPixels считает закрашенные px: красные, тёмные, светлые и за рамкой (поле 10 px); цвет
// больше альфы — ошибка.
func brandPixels(t *testing.T, img *image.RGBA, kind string, w, h int) (red, dark, light, out int) {
	t.Helper()
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			p := img.RGBAAt(x, y)
			if p.R > p.A || p.G > p.A || p.B > p.A {
				t.Fatalf("%s: цвет больше альфы %v", kind, p)
			}
			if p.A == 0 {
				continue
			}
			if x < 10 || y < 10 || x >= w+10 || y >= h+10 {
				out++
			}
			switch {
			case p.A == 255 && p.R > 200 && p.G < 60:
				red++
			case p.A == 255 && p.R < 20 && p.G < 20:
				dark++
			case p.A == 255 && p.R > 240 && p.G > 240:
				light++
			}
		}
	}
	return
}
