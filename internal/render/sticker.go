package render

import (
	"image"
	"math"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Стикеры (048 design §7): свой — Photo в Spread.Stickers (ячейка = рамка, фото целиком, PNG
// ложится с альфой, Fade — как у фона), процедурный — ArtDraw: вектор gen.DrawArt в px печати,
// без пересэмплирования. Оба — в общем стеке по Z с местами и текстами.

// ArtDraw — процедурный стикер: Box — рамка, мм листа (угол до поворота), Rot — градусы по
// часовой вокруг её центра, зеркало — картинки в рамке; Kind, Inks, Seed — параметры gen (поля,
// не gen.Spec: export собирает задание без импорта decor/gen, depguard). Растяжимая фигура
// рисуется в пропорции рамки. Fade 0..1, 1 — не рисуется; Z — слой в общем стеке листа.
type ArtDraw struct {
	Box          layout.Rect
	Rot          float64
	FlipH, FlipV bool
	Kind         string
	Inks         []string
	Seed         uint32
	Fade         float64
	Z            int
}

// artMatrix — рамка (0..1)² → px холста при dpi: зеркало, масштаб до рамки, поворот вокруг
// центра; ok = false — рамка вырождена или не число.
func artMatrix(a ArtDraw, dpi int) (f64.Aff3, bool) {
	b := a.Box
	for _, v := range [...]float64{b.X, b.Y, b.W, b.H, a.Rot} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return f64.Aff3{}, false
		}
	}
	if !(b.W > 0) || !(b.H > 0) {
		return f64.Aff3{}, false
	}
	k := float64(dpi) / 25.4
	sx, sy := b.W*k, b.H*k
	if a.FlipH {
		sx = -sx
	}
	if a.FlipV {
		sy = -sy
	}
	s, c := sincosDeg(a.Rot)
	cx, cy := (b.X+b.W/2)*k, (b.Y+b.H/2)*k
	// (u, v) → центр + R·((u − ½)·sx, (v − ½)·sy).
	return f64.Aff3{c * sx, -s * sy, cx - (c*sx-s*sy)/2, s * sx, c * sy, cy - (s*sx+c*sy)/2}, true
}

// drawArt кладёт процедурный стикер на холст. Рисуется в подхолст по описанной рамке: при
// прозрачности gen держит слой размером с холст назначения, а не с весь лист.
func drawArt(canvas *image.RGBA, a ArtDraw, dpi int) {
	m, ok := artMatrix(a, dpi)
	if !ok || a.Fade >= 1 {
		return
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, uv := range [...][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		x := m[0]*uv[0] + m[1]*uv[1] + m[2]
		y := m[3]*uv[0] + m[4]*uv[1] + m[5]
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	bb := image.Rect(int(math.Floor(x0))-1, int(math.Floor(y0))-1, int(math.Ceil(x1))+1, int(math.Ceil(y1))+1).
		Intersect(canvas.Rect)
	if bb.Empty() {
		return
	}
	sub, _ := canvas.SubImage(bb).(*image.RGBA)
	spec := gen.Spec{Kind: a.Kind, Inks: a.Inks, Seed: a.Seed, Aspect: a.Box.W / a.Box.H}
	gen.DrawArt(sub, m, spec, a.Fade)
}
