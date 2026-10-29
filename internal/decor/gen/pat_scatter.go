package gen

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Россыпи: мотив по клеткам (paint.Cells), у каждой клетки свой ГСЧ (paint.CellRand) — набор фигур не
// зависит от dpi, размера области и того, какие клетки видны.

// scatter обходит клетки шага step с полем margin; fn получает ГСЧ и угол клетки (мм).
func scatter(c *canvas, s Spec, step, margin float64, fn func(r *rng, x0, y0 float64)) {
	paint.Cells(c.W, c.H, step, margin, func(ix, iy int) {
		r := paint.CellRand(s.Seed, s.Kind, ix, iy)
		fn(&r, float64(ix)*step, float64(iy)*step)
	})
}

// dotsScatter — Россыпь точек: 1–2 точки разного размера на клетку 12 мм.
func dotsScatter(c *canvas, s Spec) {
	ink := inks(s)
	step := 12 * s.Scale
	scatter(c, s, step, 3*s.Scale, func(r *rng, x0, y0 float64) {
		for range 1 + r.Intn(2) {
			x, y := x0+r.RangeF(0, step), y0+r.RangeF(0, step)
			rad := r.RangeF(0.6, 2.4) * s.Scale
			c.Circle(x, y, rad)
			c.Fill(ink[r.Intn(2)], 1)
		}
	})
}

// confetti — Конфетти: полоски, кружки и треугольники четырёх цветов.
func confetti(c *canvas, s Spec) {
	ink := inks(s)
	step := 16 * s.Scale
	scatter(c, s, step, 3*s.Scale, func(r *rng, x0, y0 float64) {
		for range 2 + r.Intn(3) {
			x, y := x0+r.RangeF(0, step), y0+r.RangeF(0, step)
			rot := r.RangeF(0, math.Pi)
			col := ink[r.Intn(4)]
			switch r.Intn(3) {
			case 0:
				c.RotRect(x, y, 1.4*s.Scale, 4*s.Scale, rot)
			case 1:
				c.Circle(x, y, 1.1*s.Scale)
			default:
				c.Poly(paint.Regular(x, y, 1.9*s.Scale, 3, rot))
			}
			c.Fill(col, 1)
		}
	})
}

// stars — Звёзды: 0–2 пятиконечные звезды на клетку 26 мм.
func stars(c *canvas, s Spec) {
	ink := inks(s)
	step := 26 * s.Scale
	scatter(c, s, step, 7*s.Scale, func(r *rng, x0, y0 float64) {
		for range r.Intn(3) {
			x, y := x0+r.RangeF(0, step), y0+r.RangeF(0, step)
			rad := r.RangeF(2, 6.5) * s.Scale
			c.Poly(star(x, y, rad, 0.45*rad, r.RangeF(0, 2*math.Pi/5)))
			c.Fill(ink[r.Intn(2)], 1)
		}
	})
}

// hearts — Сердца: одно сердце на клетку 30 мм, слегка наклонено.
func hearts(c *canvas, s Spec) {
	ink := inks(s)
	step := 30 * s.Scale
	scatter(c, s, step, 14*s.Scale, func(r *rng, x0, y0 float64) {
		if !r.Chance(0.85) {
			return
		}
		x, y := x0+r.RangeF(0.15, 0.85)*step, y0+r.RangeF(0.15, 0.85)*step
		size := r.RangeF(7, 13) * s.Scale
		paint.Heart(c, x, y, size, r.RangeF(-0.45, 0.45))
		c.Fill(ink[r.Intn(2)], 1)
	})
}

// star — пятиконечная звезда: лучи radius, впадины inner, первый луч вверх при rot 0.
func star(x, y, radius, inner, rot float64) []pt {
	ps := make([]pt, 10)
	for i := range ps {
		rad := radius
		if i%2 == 1 {
			rad = inner
		}
		sn, cs := math.Sincos(rot - math.Pi/2 + math.Pi*float64(i)/5)
		ps[i] = pt{x + rad*cs, y + rad*sn}
	}
	return ps
}
