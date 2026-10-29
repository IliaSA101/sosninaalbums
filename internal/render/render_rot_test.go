package render

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// rotatedPx — описанный прямоугольник рамки (dx, dy, w, h) от точки (x, y), повёрнутой на deg
// по часовой вокруг (x, y); мм → пиксели холста. Та же геометрия, что у geom.RotRect.AABB и у экрана.
func rotatedPx(x, y, dx, dy, w, h, deg float64, dpi int) image.Rectangle {
	s, c := math.Sincos(deg * math.Pi / 180)
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range [4][2]float64{{dx, dy}, {dx + w, dy}, {dx, dy + h}, {dx + w, dy + h}} {
		X, Y := x+p[0]*c-p[1]*s, y+p[0]*s+p[1]*c
		x0, y0, x1, y1 = math.Min(x0, X), math.Min(y0, Y), math.Max(x1, X), math.Max(y1, Y)
	}
	k := float64(dpi) / 25.4
	return image.Rect(int(math.Floor(x0*k)), int(math.Floor(y0*k)), int(math.Ceil(x1*k)), int(math.Ceil(y1*k)))
}

// Повёрнутый блок ложится на холст так же, как на экране: чернила внутри описанного прямоугольника
// повёрнутой рамки и внутри повёрнутых чернил неповёрнутого блока; на прямых углах габарит совпадает.
func TestRenderRotatedText(t *testing.T) {
	opt := Options{PageW: 150, PageH: 150, DPI: 100}
	st := text.Style{Font: "montserrat", Size: 20, Color: "#000000", Align: "left"}
	white := color.RGBA{255, 255, 255, 255}
	const x, y, w = 150.0, 70.0, 60.0
	blk := textBlock("Иван и Мария", x, y, w, st)
	k := float64(opt.DPI) / 25.4

	render := func(deg float64) image.Rectangle {
		b := blk
		b.Rot = deg
		img, err := Render(Spread{Texts: []TextDraw{b}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		return inkBounds(img, white)
	}
	base := render(0)
	if base.Empty() {
		t.Fatal("неповёрнутый блок не нарисован")
	}
	// Чернила неповёрнутого блока в мм от угла блока.
	rx, ry := float64(base.Min.X)/k-x, float64(base.Min.Y)/k-y
	rw, rh := float64(base.Dx())/k, float64(base.Dy())/k
	near := func(a, b int) bool { return max(a-b, b-a) <= 2 }

	for _, deg := range []float64{30, -45, 135, 90, -90, 180} {
		ink := render(deg)
		frame := rotatedPx(x, y, 0, 0, w, blk.Box.H, deg, opt.DPI).Inset(-2)
		if ink.Empty() || !ink.In(frame) {
			t.Errorf("rot %v: чернила %v вне описанного прямоугольника рамки %v", deg, ink, frame)
		}
		want := rotatedPx(x, y, rx, ry, rw, rh, deg, opt.DPI)
		if !ink.In(want.Inset(-2)) {
			t.Errorf("rot %v: чернила %v вне повёрнутых чернил %v", deg, ink, want)
		}
		if int(math.Abs(deg))%90 == 0 && (!near(ink.Min.X, want.Min.X) || !near(ink.Min.Y, want.Min.Y) ||
			!near(ink.Max.X, want.Max.X) || !near(ink.Max.Y, want.Max.Y)) {
			t.Errorf("rot %v: чернила %v, ждали %v", deg, ink, want)
		}
	}
}

var flipCols = struct{ red, grn, blu, yel color.RGBA }{
	color.RGBA{220, 20, 20, 255}, color.RGBA{20, 200, 20, 255}, color.RGBA{20, 20, 220, 255}, color.RGBA{220, 220, 20, 255},
}

// quadColors — цвета четырёх четвертей ячейки cell (px при dpi 100) в порядке ЛВ, ПВ, ЛН, ПН.
func quadColors(t *testing.T, ph Photo, cell layout.Rect) [4]color.RGBA {
	t.Helper()
	img, err := Render(Spread{Photos: []Photo{ph}}, Options{PageW: 100, PageH: 100, DPI: 100})
	if err != nil {
		t.Fatal(err)
	}
	x0, y0, x1, y1 := px(cell.X, 100), px(cell.Y, 100), px(cell.X+cell.W, 100), px(cell.Y+cell.H, 100)
	mx, my, d := (x0+x1)/2, (y0+y1)/2, 4
	return [4]color.RGBA{img.RGBAAt(mx-d-3, my-d-3), img.RGBAAt(mx+d+3, my-d-3), img.RGBAAt(mx-d-3, my+d+3), img.RGBAAt(mx+d+3, my+d+3)}
}

func checkQuads(t *testing.T, what string, got, want [4]color.RGBA) {
	t.Helper()
	for i := range got {
		if !near(got[i], want[i], 14) {
			t.Fatalf("%s: четверть %d — %v, ждали %v", what, i, got[i], want[i])
		}
	}
}

// ↔: картинка в ячейке зеркальна по горизонтали, вертикаль не тронута.
func TestRenderFlipH(t *testing.T) {
	c := flipCols
	ph := quadrants(t, testkit.TempDir(t), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	cell := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	p := coverPhoto(cell, 1, ph)
	checkQuads(t, "без зеркала", quadColors(t, p, cell), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	p.FlipH = true
	checkQuads(t, "↔", quadColors(t, p, cell), [4]color.RGBA{c.grn, c.red, c.yel, c.blu})

	// Увеличенный кадр: проект при зеркале меняет X → −X, видимая часть остаётся той же
	// (левый верхний угол снимка), только отражённой: ячейка целиком красная.
	z := framedPhoto(cell, 1, 2, 1, -1, ph)
	z.FlipH = true
	checkQuads(t, "↔ с кадром", quadColors(t, z, cell), [4]color.RGBA{c.red, c.red, c.red, c.red})
}

// ↕: зеркало по вертикали.
func TestRenderFlipV(t *testing.T) {
	c := flipCols
	ph := quadrants(t, testkit.TempDir(t), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	cell := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	p := coverPhoto(cell, 1, ph)
	p.FlipV = true
	checkQuads(t, "↕", quadColors(t, p, cell), [4]color.RGBA{c.blu, c.yel, c.red, c.grn})
	p.FlipH = true
	checkQuads(t, "↔↕", quadColors(t, p, cell), [4]color.RGBA{c.yel, c.blu, c.grn, c.red})

	z := framedPhoto(cell, 1, 2, -1, 1, ph)
	z.FlipV = true
	checkQuads(t, "↕ с кадром", quadColors(t, z, cell), [4]color.RGBA{c.red, c.red, c.red, c.red})
}
