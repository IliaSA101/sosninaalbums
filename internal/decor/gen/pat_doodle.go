package gen

import "math"

// Дудлы: значки линией одного цвета по клеткам 34 мм, поворот и размер — от сида. Значок —
// ломаные в квадрате (−0,5..0,5)², толщина линии — 6 % размера.

type line struct {
	ps     []pt
	closed bool
}

type icon []line

// doodle рисует значки набора set.
func doodle(set []icon) func(c *canvas, s Spec) {
	return func(c *canvas, s Spec) {
		ink := inks(s)
		step := 34 * s.Scale
		scatter(c, s, step, 12*s.Scale, func(r *rng, x0, y0 float64) {
			if !r.chance(0.9) {
				return
			}
			ic := set[r.intn(len(set))]
			x, y := x0+r.rangeF(0.2, 0.8)*step, y0+r.rangeF(0.2, 0.8)*step
			size := r.rangeF(13, 19) * s.Scale
			sn, cs := math.Sincos(r.rangeF(-0.5, 0.5))
			if !c.visible(x, y, 0.75*size) {
				return
			}
			for _, l := range ic {
				ps := make([]pt, len(l.ps))
				for i, p := range l.ps {
					u, v := p.x*size, p.y*size
					ps[i] = pt{x + u*cs - v*sn, y + u*sn + v*cs}
				}
				c.stroke(ps, l.closed, 0.06*size)
			}
			c.fill(ink[0], 1)
		})
	}
}

// arc — точки дуги окружности (cx, cy, rad) от a0 до a1 (градусы, по часовой при y вниз).
func arc(cx, cy, rad, a0, a1 float64, n int) []pt {
	ps := make([]pt, n+1)
	for i := range ps {
		sn, cs := math.Sincos((a0 + (a1-a0)*float64(i)/float64(n)) * math.Pi / 180)
		ps[i] = pt{cx + rad*cs, cy + rad*sn}
	}
	return ps
}

// ellipsePts — замкнутый эллипс (без повтора первой точки), повёрнутый на rot (градусы).
func ellipsePts(cx, cy, rx, ry, rot float64, n int) []pt {
	sr, cr := math.Sincos(rot * math.Pi / 180)
	ps := make([]pt, n)
	for i := range ps {
		sn, cs := math.Sincos(2 * math.Pi * float64(i) / float64(n))
		u, v := rx*cs, ry*sn
		ps[i] = pt{cx + u*cr - v*sr, cy + u*sr + v*cr}
	}
	return ps
}

func open(ps ...pt) line   { return line{ps: ps} }
func closed(ps ...pt) line { return line{ps: ps, closed: true} }

// schoolIcons — карандаш, звезда, яблоко, книга, линейка.
var schoolIcons = []icon{
	{
		closed(pt{-0.45, -0.09}, pt{0.22, -0.09}, pt{0.45, 0}, pt{0.22, 0.09}, pt{-0.45, 0.09}),
		open(pt{0.22, -0.09}, pt{0.22, 0.09}),
		open(pt{-0.33, -0.09}, pt{-0.33, 0.09}),
	},
	{line{ps: star(0, 0.03, 0.48, 0.2, 0), closed: true}},
	{
		line{ps: arc(0, 0.06, 0.36, -80, 260, 28)},
		open(pt{0, -0.24}, pt{0.05, -0.46}),
		closed(pt{0.04, -0.36}, pt{0.2, -0.46}, pt{0.3, -0.38}, pt{0.14, -0.32}),
	},
	{
		closed(pt{0, -0.25}, pt{-0.45, -0.33}, pt{-0.45, 0.27}, pt{0, 0.35}),
		closed(pt{0, -0.25}, pt{0.45, -0.33}, pt{0.45, 0.27}, pt{0, 0.35}),
	},
	ruler(),
}

func ruler() icon {
	ic := icon{closed(pt{-0.48, -0.12}, pt{0.48, -0.12}, pt{0.48, 0.12}, pt{-0.48, 0.12})}
	for i := 1; i <= 7; i++ {
		x, l := -0.48+float64(i)*0.12, 0.06
		if i%2 == 0 {
			l = 0.11
		}
		ic = append(ic, open(pt{x, -0.12}, pt{x, -0.12 + l}))
	}
	return ic
}

// kidsIcons — солнце, облако, цветок, сердце, шарик.
var kidsIcons = []icon{sun(), cloud(), flower(), heartIcon(), balloon()}

func sun() icon {
	ic := icon{line{ps: ellipsePts(0, 0, 0.2, 0.2, 0, 24), closed: true}}
	for i := range 8 {
		sn, cs := math.Sincos(float64(i) * math.Pi / 4)
		ic = append(ic, open(pt{0.29 * cs, 0.29 * sn}, pt{0.46 * cs, 0.46 * sn}))
	}
	return ic
}

func cloud() icon {
	var ps []pt
	ps = append(ps, arc(-0.24, 0.06, 0.14, 90, 270, 9)...)
	ps = append(ps, arc(0, -0.04, 0.2, 200, 340, 9)...)
	ps = append(ps, arc(0.25, 0.05, 0.15, 270, 450, 9)...)
	return icon{line{ps: ps, closed: true}}
}

func flower() icon {
	ic := icon{line{ps: ellipsePts(0, 0, 0.1, 0.1, 0, 14), closed: true}}
	for i := range 5 {
		a := float64(i)*72 - 90
		sn, cs := math.Sincos(a * math.Pi / 180)
		ic = append(ic, line{ps: ellipsePts(0.27*cs, 0.27*sn, 0.16, 0.09, a, 18), closed: true})
	}
	return ic
}

func heartIcon() icon {
	const n = 36
	ps := make([]pt, n)
	for i := range ps {
		t := 2 * math.Pi * float64(i) / n
		sn := math.Sin(t)
		x := 16 * sn * sn * sn
		y := -(13*math.Cos(t) - 5*math.Cos(2*t) - 2*math.Cos(3*t) - math.Cos(4*t))
		ps[i] = pt{x / 34, y/34 + 0.03}
	}
	return icon{line{ps: ps, closed: true}}
}

func balloon() icon {
	str := make([]pt, 9)
	for i := range str {
		t := float64(i) / 8
		str[i] = pt{0.04 * math.Sin(t*2*math.Pi), 0.24 + 0.24*t}
	}
	return icon{
		line{ps: ellipsePts(0, -0.12, 0.26, 0.32, 0, 30), closed: true},
		closed(pt{0, 0.2}, pt{-0.05, 0.25}, pt{0.05, 0.25}),
		line{ps: str},
	}
}
