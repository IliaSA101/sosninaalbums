package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Геометрия повёрнутого блока текста (design 008 §3). Угол — по часовой на
// экране (ось y вниз), поворот рамки — вокруг её левого верхнего угла (X, Y).
// Зеркало — frontend geom/textbox.ts, паритет держит testdata/parity/textbox.json.

// TextBox — рамка блока: угол до поворота, ширина, высота по тексту, угол.
type TextBox struct{ X, Y, W, H, Rot float64 }

// Rotate — вектор (u, v) рамки, повёрнутый на deg по часовой.
func Rotate(deg, u, v float64) (float64, float64) {
	if deg == 0 {
		return u, v
	}
	s, c := math.Sincos(deg * math.Pi / 180)
	return u*c - v*s, u*s + v*c
}

// NormDeg — угол в (−180, 180] с шагом 0.1; почти ноль (и NaN/Inf) — 0.
func NormDeg(d float64) float64 {
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return 0
	}
	d = math.Mod(d, 360)
	switch {
	case d > 180:
		d -= 360
	case d <= -180:
		d += 360
	}
	d = math.Round(d*10) / 10
	if d <= -180 { // −179.96 округлилось до −180
		d = 180
	}
	if math.Abs(d) < 0.05 {
		return 0
	}
	return d
}

// corners — углы рамки на развороте: P, P+R(W,0), P+R(0,H), P+R(W,H).
func (b TextBox) corners() [4][2]float64 {
	var out [4][2]float64
	for i, uv := range [4][2]float64{{0, 0}, {b.W, 0}, {0, b.H}, {b.W, b.H}} {
		dx, dy := Rotate(b.Rot, uv[0], uv[1])
		out[i] = [2]float64{b.X + dx, b.Y + dy}
	}
	return out
}

// AABB — описанный прямоугольник; без поворота — сама рамка.
func (b TextBox) AABB() layout.Rect {
	if b.Rot == 0 {
		return layout.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
	}
	c := b.corners()
	x0, y0, x1, y1 := c[0][0], c[0][1], c[0][0], c[0][1]
	for _, p := range c[1:] {
		x0, y0 = math.Min(x0, p[0]), math.Min(y0, p[1])
		x1, y1 = math.Max(x1, p[0]), math.Max(y1, p[1])
	}
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// FitIn — угол P, при котором блок целиком внутри разворота sw×ph (model.FitTexts):
// повёрнутый — по описанному прямоугольнику (FitAxis), ровный — зажим.
func (b TextBox) FitIn(sw, ph float64) (float64, float64) {
	if b.Rot != 0 {
		a := b.AABB()
		ox, oy := a.X-b.X, a.Y-b.Y
		return FitAxis(b.X, -ox, sw-a.W-ox), FitAxis(b.Y, -oy, ph-a.H-oy)
	}
	return ClampF(b.X, 0, sw-b.W), ClampF(b.Y, 0, math.Max(0, ph-b.H))
}

// Center — центр рамки на развороте.
func (b TextBox) Center() (float64, float64) {
	dx, dy := Rotate(b.Rot, b.W/2, b.H/2)
	return b.X + dx, b.Y + dy
}

// RotateAbout — угол P рамки, повёрнутой на deg вокруг своего центра.
func (b TextBox) RotateAbout(deg float64) (float64, float64) {
	cx, cy := b.Center()
	dx, dy := Rotate(deg, b.W/2, b.H/2)
	return cx - dx, cy - dy
}

// Anchor — точка верхней стороны, которая стоит при росте ширины «по тексту»:
// левый край, середина или правый край по выравниванию.
func (b TextBox) Anchor(align string) (float64, float64) {
	dx, dy := Rotate(b.Rot, AlignX(align)*b.W, 0)
	return b.X + dx, b.Y + dy
}

// AlignX — доля ширины, которая стоит при росте блока: left 0, right 1, иначе 0.5
// (неизвестное выравнивание text.Style.Clamped делает center).
func AlignX(align string) float64 {
	switch align {
	case "left":
		return 0
	case "right":
		return 1
	}
	return 0.5
}

// FitAxis держит угол повёрнутого блока по одной оси: внутри разворота x ∈ [lo, hi].
// Сдвиг округляется внутрь до 0.01, поэтому повторный вызов ничего не меняет.
// Шире разворота (между lo и hi нет сотых) — к левому (верхнему) краю.
func FitAxis(x, lo, hi float64) float64 {
	const eps = 1e-6
	if math.IsNaN(x) || math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return 0 // границы не посчитать (дикие числа в файле): в начало оси
	}
	in, out := ceil01(lo), floor01(hi)
	switch {
	case in > out:
		if math.Abs(x-in) > eps {
			return in
		}
	case x < lo-eps:
		return in
	case x > hi+eps:
		return out
	}
	return x
}
