package render

import (
	"crypto/sha256"
	"image/color"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Результат рисования с поворотом не зависит от числа потоков: полосы фиксированной
// высоты, потоки только раздают их (маска vector строится в координатах полосы).
func TestRotatedSameForAnyProcs(t *testing.T) {
	path := quadrants(t, testkit.TempDir(t), [4]color.RGBA{
		{220, 40, 40, 255}, {40, 200, 60, 255}, {40, 60, 220, 255}, {230, 210, 40, 255},
	})
	bg := color.RGBA{250, 250, 245, 255}
	cell := layout.Rect{X: 20, Y: 15, W: 160, H: 120}
	// Путь 2: наклон фото в прямой ячейке с прозрачностью.
	p2 := framedPhoto(cell, 1.5, 1.6, 0.3, -0.2, path)
	p2.ImgRot, p2.Fade = 7, 0.25
	// Путь 3: повёрнутая ячейка (маска многоугольника).
	p3 := framedPhoto(cell, 1.5, 1.6, 0.3, -0.2, path)
	p3.Rot = 23
	// Путь 3: фото целиком в рамке под наклоном, поворот ячейки и прозрачность.
	p4 := Photo{Cell: cell, Img: containImg(cell, 1.5, 11), Path: path, Rot: -17, ImgRot: 11, Fade: 0.4}

	cases := []struct {
		name string
		ph   Photo
	}{{"путь 2", p2}, {"маска", p3}, {"маска, прозрачность", p4}}
	opt := Options{PageW: 100, PageH: 150, DPI: 150}
	render := func(ph Photo) [32]byte {
		img, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Photos: []Photo{ph}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(img.Pix)
	}

	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })
	for _, c := range cases {
		runtime.GOMAXPROCS(1)
		want := render(c.ph)
		for _, n := range []int{2, 4, 8} {
			runtime.GOMAXPROCS(n)
			if got := render(c.ph); got != want {
				t.Errorf("%s: GOMAXPROCS=%d даёт другие пиксели, чем 1", c.name, n)
			}
		}
	}
}
