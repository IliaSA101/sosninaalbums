package gen

import "math"

// Россыпи: мотив по клеткам (cells), у каждой клетки свой ГСЧ (cellRand) — набор фигур не
// зависит от dpi, размера области и того, какие клетки видны.

// scatter обходит клетки шага step с полем margin; fn получает ГСЧ и угол клетки (мм).
func scatter(c *canvas, s Spec, step, margin float64, fn func(r *rng, x0, y0 float64)) {
	cells(c.w, c.h, step, margin, func(ix, iy int) {
		r := cellRand(s.Seed, s.Kind, ix, iy)
		fn(&r, float64(ix)*step, float64(iy)*step)
	})
}

// dotsScatter — Россыпь точек: 1–2 точки разного размера на клетку 12 мм.
func dotsScatter(c *canvas, s Spec) {
	ink := inks(s)
	step := 12 * s.Scale
	scatter(c, s, step, 3*s.Scale, func(r *rng, x0, y0 float64) {
		for range 1 + r.intn(2) {
			x, y := x0+r.rangeF(0, step), y0+r.rangeF(0, step)
			rad := r.rangeF(0.6, 2.4) * s.Scale
			c.circle(x, y, rad)
			c.fill(ink[r.intn(2)], 1)
		}
	})
}

// confetti — Конфетти: полоски, кружки и треугольники четырёх цветов.
func confetti(c *canvas, s Spec) {
	ink := inks(s)
	step := 16 * s.Scale
	scatter(c, s, step, 3*s.Scale, func(r *rng, x0, y0 float64) {
		for range 2 + r.intn(3) {
			x, y := x0+r.rangeF(0, step), y0+r.rangeF(0, step)
			rot := r.rangeF(0, math.Pi)
			col := ink[r.intn(4)]
			switch r.intn(3) {
			case 0:
				c.rotRect(x, y, 1.4*s.Scale, 4*s.Scale, rot)
			case 1:
				c.circle(x, y, 1.1*s.Scale)
			default:
				c.poly(regular(x, y, 1.9*s.Scale, 3, rot))
			}
			c.fill(col, 1)
		}
	})
}

// stars — Звёзды: 0–2 пятиконечные звезды на клетку 26 мм.
func stars(c *canvas, s Spec) {
	ink := inks(s)
	step := 26 * s.Scale
	scatter(c, s, step, 7*s.Scale, func(r *rng, x0, y0 float64) {
		for range r.intn(3) {
			x, y := x0+r.rangeF(0, step), y0+r.rangeF(0, step)
			rad := r.rangeF(2, 6.5) * s.Scale
			c.poly(star(x, y, rad, 0.45*rad, r.rangeF(0, 2*math.Pi/5)))
			c.fill(ink[r.intn(2)], 1)
		}
	})
}

// hearts — Сердца: одно сердце на клетку 30 мм, слегка наклонено.
func hearts(c *canvas, s Spec) {
	ink := inks(s)
	step := 30 * s.Scale
	scatter(c, s, step, 14*s.Scale, func(r *rng, x0, y0 float64) {
		if !r.chance(0.85) {
			return
		}
		x, y := x0+r.rangeF(0.15, 0.85)*step, y0+r.rangeF(0.15, 0.85)*step
		size := r.rangeF(7, 13) * s.Scale
		heart(c, x, y, size, r.rangeF(-0.45, 0.45))
		c.fill(ink[r.intn(2)], 1)
	})
}

// heart — сердце размера size (ширина ≈ высота) с центром (x, y), наклон rot (рад).
func heart(c *canvas, x, y, size, rot float64) {
	sn, cs := math.Sincos(rot)
	at := func(u, v float64) (float64, float64) {
		u, v = u*size, v*size
		return x + u*cs - v*sn, y + u*sn + v*cs
	}
	c.moveTo(at(0, 0.4))
	x1, y1 := at(-0.55, 0.05)
	x2, y2 := at(-0.35, -0.5)
	x3, y3 := at(0, -0.18)
	c.cubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = at(0.35, -0.5)
	x2, y2 = at(0.55, 0.05)
	x3, y3 = at(0, 0.4)
	c.cubeTo(x1, y1, x2, y2, x3, y3)
	c.closePath()
}

// regular — правильный n-угольник радиуса rad с поворотом rot.
func regular(x, y, rad float64, n int, rot float64) []pt {
	ps := make([]pt, n)
	for i := range ps {
		sn, cs := math.Sincos(rot + 2*math.Pi*float64(i)/float64(n))
		ps[i] = pt{x + rad*cs, y + rad*sn}
	}
	return ps
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
