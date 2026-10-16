package render

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Прозрачность фото фона (спека 017, design §5).

var fadeRed = color.RGBA{200, 30, 30, 255}

func whiteJPEG(t *testing.T) string {
	t.Helper()
	w := color.RGBA{255, 255, 255, 255}
	return quadrants(t, testkit.TempDir(t), [4]color.RGBA{w, w, w, w})
}

// fadeMix — белое фото с прозрачностью fade поверх цвета bg по формуле fadeAlpha.
func fadeMix(bg color.RGBA, fade float64) color.RGBA {
	a := math.Round(255 * (1 - fade))
	ch := func(c uint8) uint8 { return uint8(math.Round((255*a + float64(c)*(255-a)) / 255)) }
	return color.RGBA{ch(bg.R), ch(bg.G), ch(bg.B), 255}
}

func wantExact(t *testing.T, what string, img *image.RGBA, dpi int, x, y float64, want color.RGBA) {
	t.Helper()
	if got := img.RGBAAt(px(x, dpi), px(y, dpi)); !near(got, want, 1) {
		t.Errorf("%s: (%v, %v) мм — %v, ждали %v ±1", what, x, y, got, want)
	}
}

// Белое на красном (200, 30, 30) при 40 % — (233, 165, 165) ±1 во всех трёх путях.
func TestRenderBackdropFade(t *testing.T) {
	white := whiteJPEG(t)
	want := color.RGBA{233, 165, 165, 255}
	if fadeMix(fadeRed, 0.4) != want {
		t.Fatalf("формула: %v", fadeMix(fadeRed, 0.4))
	}
	left := layout.Rect{W: 100, H: 100}
	cases := []struct {
		name string
		bd   Backdrop
		x, y float64
		outX float64 // вне рамки — чистый цвет страницы
		outY float64
	}{
		{"путь 1, без поворота", Backdrop{Area: left, Box: left, Img: left}, 50, 50, 150, 50},
		{"путь 1, рамка под 90°", Backdrop{Area: left, Box: layout.Rect{X: 80, Y: 10, W: 60, H: 40}, Rot: 90,
			Img: layout.Rect{W: 60, H: 40}}, 60, 40, 20, 80},
		{"путь 3, рамка под 30°", Backdrop{Area: left, Box: layout.Rect{X: 20, Y: 30, W: 60, H: 40}, Rot: 30,
			Img: layout.Rect{W: 60, H: 40}}, 36, 62, 90, 10},
	}
	for _, c := range cases {
		c.bd.Path, c.bd.Fade = white, 0.4
		img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, fadeRed}, Backdrops: []Backdrop{c.bd}}, 100)
		wantExact(t, c.name, img, 100, c.x, c.y, want)
		wantExact(t, c.name+": вне рамки", img, 100, c.outX, c.outY, fadeRed)
	}
	// Путь 2 у фона не встречается (рамка без поворота — путь 1, с поворотом — 3), но
	// prepareRot общий: наклон фото в прямой ячейке с Fade смешивается так же.
	cell := layout.Rect{X: 30, Y: 30, W: 40, H: 40}
	ph := framedPhoto(cell, 1, 1.5, 0, 0, white)
	ph.ImgRot, ph.Fade = 5, 0.4
	img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, fadeRed}, Photos: []Photo{ph}}, 100)
	wantExact(t, "путь 2", img, 100, 50, 50, want)

	// Разворот по страницам: левая 30 % над своим цветом, правая непрозрачна; фон
	// «весь разворот» смешивается с цветом каждой половины.
	blue := color.RGBA{20, 40, 160, 255}
	right := layout.Rect{X: 100, W: 100, H: 100}
	s := Spread{Bg: [2]color.RGBA{fadeRed, blue}, Backdrops: []Backdrop{
		{Area: left, Box: left, Img: layout.Rect{W: 100, H: 100}, Path: white, Fade: 0.3},
		{Area: right, Box: right, Img: layout.Rect{W: 100, H: 100}, Path: white},
	}}
	img = renderBd(t, s, 100)
	wantExact(t, "левая 30 %", img, 100, 50, 50, fadeMix(fadeRed, 0.3))
	wantExact(t, "правая 0 %", img, 100, 150, 50, color.RGBA{255, 255, 255, 255})
	all := layout.Rect{W: 200, H: 100}
	s.Backdrops = []Backdrop{{Area: all, Box: all, Img: layout.Rect{Y: -50, W: 200, H: 200}, Path: white, Fade: 0.5}}
	img = renderBd(t, s, 100)
	wantExact(t, "разворот над левой", img, 100, 50, 50, fadeMix(fadeRed, 0.5))
	wantExact(t, "разворот над правой", img, 100, 150, 50, fadeMix(blue, 0.5))
}

// Fade 0 (и всё, что fadeAlpha считает нулём) — те же байты, что прежний вывод.
func TestRenderBackdropFadeZero(t *testing.T) {
	q := quadsBd(t)
	area := layout.Rect{W: 200, H: 100}
	box := layout.Rect{X: 20, Y: 10, W: 120, H: 70}
	img := framedPhoto(layout.Rect{W: 120, H: 70}, 1, 1.2, 0.3, 0, q).Img
	for _, rot := range []float64{0, 90, 30} {
		base := Backdrop{Area: area, Box: box, Rot: rot, Img: img, Path: q, FlipH: true}
		ref := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdCream}, Backdrops: []Backdrop{base}}, 100)
		for _, f := range []float64{-0.5, math.NaN(), 0.001} {
			b := base
			b.Fade = f
			got := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdCream}, Backdrops: []Backdrop{b}}, 100)
			if !bytes.Equal(ref.Pix, got.Pix) {
				t.Errorf("rot %v, fade %v: вывод изменился", rot, f)
			}
		}
	}
}

// Fade 1: фон не рисуется и файл не открывается (несуществующий путь — не ошибка).
func TestRenderBackdropFadeFull(t *testing.T) {
	calls := 0
	old := prepareFn
	t.Cleanup(func() { prepareFn = old })
	prepareFn = func(ph Photo, dpi int, clip image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
		calls++
		return old(ph, dpi, clip)
	}
	missing := filepath.Join(testkit.TempDir(t), "нет.jpg")
	left := layout.Rect{W: 100, H: 100}
	for _, rot := range []float64{0, 90, 30} {
		bd := Backdrop{Area: left, Box: left, Rot: rot, Img: left, Path: missing, Fade: 1}
		if rot == 90 { // поворот вокруг угла: иначе рамка ушла бы за область
			bd.Box.X = 100
		}
		img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, bdGray}, Backdrops: []Backdrop{bd}}, 100)
		wantExact(t, "только цвет страницы", img, 100, 50, 50, fadeRed)
		bd.Fade = 0.99
		if _, err := Render(Spread{Backdrops: []Backdrop{bd}}, Options{PageW: 100, PageH: 100, DPI: 100}); err == nil {
			t.Errorf("rot %v: при 99 %% файл не читался", rot)
		}
	}
	if calls != 2 { // путь 1 (rot 0 и 90) при 99 %
		t.Fatalf("prepareFn вызван %d раз", calls)
	}
}

// fadeAlpha: 0 — без изменений, 1 — альфа 0, подизображение — только свои пиксели.
func TestFadeAlpha(t *testing.T) {
	mk := func() *image.NRGBA {
		im := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		for i := range im.Pix {
			im.Pix[i] = 255
		}
		im.Pix[3] = 101 // 101·153/255 = 60,6: без округления вышло бы 60
		return im
	}
	im := mk()
	fadeAlpha(im, 0)
	fadeAlpha(im, math.NaN())
	fadeAlpha(nil, 0.5)
	if !bytes.Equal(im.Pix, mk().Pix) {
		t.Fatal("fade 0 изменил пиксели")
	}
	fadeAlpha(im, 0.4)
	if im.Pix[3] != 61 || im.Pix[7] != 153 || im.Pix[4] != 255 {
		t.Fatalf("0.4: альфа %d и %d, цвет %d", im.Pix[3], im.Pix[7], im.Pix[4])
	}
	im = mk()
	sub, _ := im.SubImage(image.Rect(1, 1, 3, 3)).(*image.NRGBA)
	if sub.Stride == 4*sub.Rect.Dx() {
		t.Fatal("подизображение без своего Stride")
	}
	fadeAlpha(sub, 1)
	for y := range 4 {
		for x := range 4 {
			in := x >= 1 && x < 3 && y >= 1 && y < 3
			if a := im.NRGBAAt(x, y).A; (a == 0) != in {
				t.Errorf("(%d, %d): альфа %d", x, y, a)
			}
		}
	}
}
