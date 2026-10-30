package paint

import "math"

// Фигуры, которые нужны и узорам, и стикерам: общие места — здесь, остальные у своего рисовальщика.

// Arc — точки дуги окружности (cx, cy, rad) от a0 до a1 (градусы, по часовой при y вниз).
func Arc(cx, cy, rad, a0, a1 float64, n int) []Pt {
	ps := make([]Pt, n+1)
	for i := range ps {
		sn, cs := math.Sincos((a0 + (a1-a0)*float64(i)/float64(n)) * math.Pi / 180)
		ps[i] = Pt{cx + rad*cs, cy + rad*sn}
	}
	return ps
}

// Regular — правильный n-угольник радиуса rad с поворотом rot.
func Regular(x, y, rad float64, n int, rot float64) []Pt {
	ps := make([]Pt, n)
	for i := range ps {
		sn, cs := math.Sincos(rot + 2*math.Pi*float64(i)/float64(n))
		ps[i] = Pt{x + rad*cs, y + rad*sn}
	}
	return ps
}

// Heart кладёт сердце размера size (ширина ≈ высота) с центром (x, y), наклон rot (рад).
func Heart(c *Canvas, x, y, size, rot float64) {
	sn, cs := math.Sincos(rot)
	at := func(u, v float64) (float64, float64) {
		u, v = u*size, v*size
		return x + u*cs - v*sn, y + u*sn + v*cs
	}
	c.MoveTo(at(0, 0.4))
	x1, y1 := at(-0.55, 0.05)
	x2, y2 := at(-0.35, -0.5)
	x3, y3 := at(0, -0.18)
	c.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = at(0.35, -0.5)
	x2, y2 = at(0.55, 0.05)
	x3, y3 = at(0, 0.4)
	c.CubeTo(x1, y1, x2, y2, x3, y3)
	c.ClosePath()
}
