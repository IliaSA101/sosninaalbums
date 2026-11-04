package render

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Наклон фото фона (спека 045): ImgRot крутит фото внутри рамки, рамка стоит и закрыта.

// bdTilt — фон области area с рамкой box под углом rot и наклоном фото tilt; фото квадратное
// (quadrants), кадр по умолчанию. Img — в осях рамки, как BackdropView.Img.
func bdTilt(area, box layout.Rect, rot, tilt float64, path string) Backdrop {
	img := tiltImg(layout.Rect{W: box.W, H: box.H}, 1, 1, 0, 0, tilt)
	return Backdrop{Area: area, Box: box, Rot: rot, ImgRot: tilt, Img: img, Path: path}
}

// checkBdCover проверяет каждый пиксель холста: внутри рамки и области (с полем 1,5 px от
// краёв) — цвет фото, вне рамки или вне области — цвет страницы. Рамка повёрнута на Rot вокруг
// своего угла (X, Y), как у фона.
func checkBdCover(t *testing.T, what string, img *image.RGBA, dpi int, b Backdrop, photo, page color.RGBA) {
	t.Helper()
	k := float64(dpi) / 25.4
	m := 1.5 / k
	s, c := sincosDeg(b.Rot)
	a := b.Area
	bad := 0
	for py := range img.Bounds().Dy() {
		for px := range img.Bounds().Dx() {
			x, y := (float64(px)+0.5)/k, (float64(py)+0.5)/k
			dx, dy := x-b.Box.X, y-b.Box.Y
			u, v := dx*c+dy*s, -dx*s+dy*c
			inFrame := u > m && u < b.Box.W-m && v > m && v < b.Box.H-m
			outFrame := u < -m || u > b.Box.W+m || v < -m || v > b.Box.H+m
			inArea := x > a.X+m && x < a.X+a.W-m && y > a.Y+m && y < a.Y+a.H-m
			outArea := x < a.X-m || x > a.X+a.W+m || y < a.Y-m || y > a.Y+a.H+m
			got := img.RGBAAt(px, py)
			switch {
			case inFrame && inArea && !near(got, photo, 14):
				bad++
				if bad <= 3 {
					t.Errorf("%s: (%.1f, %.1f) мм в рамке — %v, ждали фото %v", what, x, y, got, photo)
				}
			case (outFrame || outArea) && !near(got, page, 14):
				bad++
				if bad <= 3 {
					t.Errorf("%s: (%.1f, %.1f) мм вне рамки или области — %v, ждали страницу %v", what, x, y, got, page)
				}
			}
		}
	}
	if bad > 3 {
		t.Errorf("%s: всего неверных пикселей %d", what, bad)
	}
}

// Наклон 4,5° и предельный 44,9° у фона на весь разворот и на страницу: в области нет цвета
// страницы, за областью — фото. Рамка больше области — фото обрезается областью.
func TestRenderBackdropTilt(t *testing.T) {
	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	const dpi = 60
	areas := map[string]layout.Rect{"разворот": {W: 200, H: 100}, "левая": {W: 100, H: 100}, "правая": {X: 100, W: 100, H: 100}}
	for name, area := range areas {
		for _, tilt := range []float64{4.5, -4.5, 44.9, -44.9} {
			b := bdTilt(area, area, 0, tilt, solid)
			img := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{b}}, dpi)
			checkBdCover(t, name, img, dpi, b, bdCream, bdGray)
		}
	}
	// Рамка шире области: справа от левой страницы фото нет.
	left := areas["левая"]
	b := bdTilt(left, layout.Rect{X: -50, Y: -20, W: 290, H: 140}, 0, 44.9, solid)
	img := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{b}}, dpi)
	checkBdCover(t, "рамка шире области", img, dpi, b, bdCream, bdGray)
	// Без наклона — прежний путь (ImgRot = 0 не уводит в путь 2).
	if ph, clip, ok := backdropPhoto(bdTilt(left, left, 0, 0, solid), dpi); !ok || !clip.Empty() || ph.ImgRot != 0 {
		t.Fatalf("без наклона: %+v %v %v", ph, clip, ok)
	}
}

// Наклон вместе с зеркалом и поворотом рамки (¼ оборота и произвольный): рамка закрыта, четверти
// фото там, где их показывает экран — поворот на Rot + ImgRot после зеркала в осях фото.
func TestRenderBackdropTiltFlipRot(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	const dpi = 60
	area := layout.Rect{W: 200, H: 100}
	box := layout.Rect{X: 100, Y: 20, W: 60, H: 60}
	cases := []struct {
		rot, tilt    float64
		flipH, flipV bool
	}{
		{0, 4.5, false, false}, {0, -44.9, true, false},
		{90, 4.5, false, false}, {90, -4.5, false, true},
		{10, -4.5, true, false}, {10, 44.9, true, true}, {-30, 4.5, false, true},
	}
	cols := [4]color.RGBA{c.red, c.grn, c.blu, c.yel} // ЛВ, ПВ, ЛН, ПН файла
	for _, cs := range cases {
		b := bdTilt(area, box, cs.rot, cs.tilt, solid)
		b.FlipH, b.FlipV = cs.flipH, cs.flipV
		img := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{b}}, dpi)
		checkBdCover(t, "сплошное", img, dpi, b, bdCream, bdGray)

		b.Path = q
		img = renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{b}}, dpi)
		// Центр рамки: её угол (X, Y) плюс повёрнутая половина.
		s, co := sincosDeg(cs.rot)
		cx, cy := box.X+box.W/2*co-box.H/2*s, box.Y+box.W/2*s+box.H/2*co
		ts, tc := sincosDeg(cs.rot + cs.tilt)
		for _, d := range [4][2]float64{{-12, -12}, {12, -12}, {-12, 12}, {12, 12}} {
			u, v := d[0], d[1] // точка в осях фото на экране (после зеркала)
			fu, fv := u, v     // та же точка в файле
			if cs.flipH {
				fu = -fu
			}
			if cs.flipV {
				fv = -fv
			}
			i := 0
			if fu > 0 {
				i++
			}
			if fv > 0 {
				i += 2
			}
			x, y := cx+u*tc-v*ts, cy+u*ts+v*tc
			wantAt(t, "четверть", img, dpi, x, y, cols[i])
		}
	}
}

// bdTiltBig — вход бенчмарка и теста бюджета: разворот 600×300 мм при 300 dpi, фон 6000×4000
// на весь разворот с наклоном 4,5° (путь 2).
func bdTiltBig(tb testing.TB) (Spread, Options) {
	tb.Helper()
	path := testkit.JPEG(tb, testkit.TempDir(tb), "big.jpg", 6000, 4000)
	area := layout.Rect{W: 600, H: 300}
	bd := Backdrop{Area: area, Box: area, ImgRot: 4.5, Img: tiltImg(area, 1.5, 1, 0, 0, 4.5), Path: path}
	return Spread{Backdrops: []Backdrop{bd}}, Options{PageW: 300, PageH: 300, DPI: 300}
}

// Критерий спеки 045: разворот с наклонённым фоном 6000×4000 при 300 dpi — не дольше 3 с.
// Тяжёлый (секунды на кадр), под -short не идёт.
func TestRenderBackdropTiltBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("тяжёлый: разворот 600×300 мм при 300 dpi")
	}
	s, opt := bdTiltBig(t)
	var err error
	d := testkit.Fastest(3, func() { _, err = Render(s, opt) })
	if err != nil {
		t.Fatal(err)
	}
	if lim := testkit.Limit(3 * time.Second); d > lim {
		t.Fatalf("рендер с наклоном фона: %v, бюджет %v", d, lim)
	}
}

// BenchmarkRenderBackdropTilt — тот же вход (бюджет — 3 с на разворот, design 045 §5).
func BenchmarkRenderBackdropTilt(b *testing.B) {
	s, opt := bdTiltBig(b)
	b.ResetTimer()
	for range b.N {
		if _, err := Render(s, opt); err != nil {
			b.Fatal(err)
		}
	}
}
