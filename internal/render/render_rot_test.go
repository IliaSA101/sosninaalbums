package render

import (
	"image"
	"image/color"
	"math"
	"testing"

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
