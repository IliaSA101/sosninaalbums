package art

import (
	"image/color"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// --- общие построения ---

// spikes — звезда с n лучами: вершины радиуса ro, впадины ri; первый луч вверх при rot 0.
func spikes(x, y, ro, ri float64, n int, rot float64) []pt {
	ps := make([]pt, 2*n)
	for i := range ps {
		r := ro
		if i%2 == 1 {
			r = ri
		}
		sn, cs := math.Sincos(rot - math.Pi/2 + math.Pi*float64(i)/float64(n))
		ps[i] = pt{x + r*cs, y + r*sn}
	}
	return ps
}

// smooth — замкнутая гладкая кривая через точки (Катмулл—Ром в кубических Безье).
func smooth(c *paint.Canvas, ps []pt) {
	n := len(ps)
	if n < 3 {
		return
	}
	c.MoveTo(ps[0].X, ps[0].Y)
	for i := range n {
		p0, p1, p2, p3 := ps[(i+n-1)%n], ps[i], ps[(i+1)%n], ps[(i+2)%n]
		c.CubeTo(p1.X+(p2.X-p0.X)/6, p1.Y+(p2.Y-p0.Y)/6, p2.X-(p3.X-p1.X)/6, p2.Y-(p3.Y-p1.Y)/6, p2.X, p2.Y)
	}
	c.ClosePath()
}

// orient — многоугольник с нужным направлением обхода: у отверстия оно обратное внешнему.
func orient(ps []pt, positive bool) []pt {
	var a float64
	for i, p := range ps {
		q := ps[(i+1)%len(ps)]
		a += p.X*q.Y - q.X*p.Y
	}
	if (a >= 0) == positive {
		return ps
	}
	rev := make([]pt, len(ps))
	for i, p := range ps {
		rev[len(ps)-1-i] = p
	}
	return rev
}

// rrect — скруглённый прямоугольник (радиус зажат полу-меньшей стороной), по часовой.
func rrect(x0, y0, x1, y1, r float64) []pt {
	r = max(0, min(r, (x1-x0)/2, (y1-y0)/2))
	var ps []pt
	for _, k := range [4][3]float64{{x0 + r, y0 + r, 180}, {x1 - r, y0 + r, 270}, {x1 - r, y1 - r, 0}, {x0 + r, y1 - r, 90}} {
		ps = append(ps, paint.Arc(k[0], k[1], r, k[2], k[2]+90, 8)...)
	}
	return ps
}

// band — часть кольца между радиусами r0 < r1 от угла a0 до a1 (градусы).
func band(c *paint.Canvas, cx, cy, r0, r1, a0, a1 float64) {
	n := max(8, int(math.Abs(a1-a0)/3))
	outer := paint.Arc(cx, cy, r1, a0, a1, n)
	inner := paint.Arc(cx, cy, r0, a1, a0, n)
	c.PolyPos(append(outer, inner...))
}

// bezier — n+1 точка кубической кривой.
func bezier(p0, p1, p2, p3 pt, n int) []pt {
	ps := make([]pt, n+1)
	for i := range ps {
		t := float64(i) / float64(n)
		u := 1 - t
		ps[i] = pt{
			u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
			u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y,
		}
	}
	return ps
}

// leafAt — заострённый лист длины l и ширины w: основание (x, y), направление rot (рад).
func leafAt(c *paint.Canvas, x, y, l, w, rot float64) {
	sn, cs := math.Sincos(rot)
	at := func(u, v float64) (float64, float64) { return x + u*cs - v*sn, y + u*sn + v*cs }
	c.MoveTo(x, y)
	x1, y1 := at(0.25*l, -w)
	x2, y2 := at(0.7*l, -0.8*w)
	x3, y3 := at(l, 0)
	c.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = at(0.7*l, 0.8*w)
	x2, y2 = at(0.25*l, w)
	c.CubeTo(x1, y1, x2, y2, x, y)
	c.ClosePath()
}

// heartAt кладёт сердце шириной ≈ 0,68·size так, что середина его высоты — в (x, y).
func heartAt(c *paint.Canvas, x, y, size, rot float64) {
	sn, cs := math.Sincos(rot)
	dy := 0.069 * size
	paint.Heart(c, x+dy*sn, y-dy*cs, size, rot)
}

// --- фигуры ---

func drawStar(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.Poly(spikes(a/2, 0.54, 0.49, 0.2, 5, 0))
	c.Fill(ink[0], 1)
}

func drawBurst(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.Poly(spikes(a/2, 0.5, 0.49, 0.37, 14, 0))
	c.Fill(ink[0], 1)
	c.Circle(a/2, 0.5, 0.27)
	c.Fill(ink[1], 1)
}

func drawSparkle(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	four := func(x, y, r float64) {
		c.MoveTo(x, y-r)
		k := 0.24 * r
		for i := range 4 {
			// Дуга к следующему лучу прогибается к центру: контрольные точки у центра.
			sn, cs := math.Sincos(math.Pi/4 + math.Pi/2*float64(i))
			nx, ny := x+r*math.Sin(math.Pi/2*float64(i+1)), y-r*math.Cos(math.Pi/2*float64(i+1))
			c.CubeTo(x+k*sn, y-k*cs, x+k*sn, y-k*cs, nx, ny)
		}
		c.ClosePath()
	}
	four(a*0.45, 0.55, 0.42)
	c.Fill(ink[0], 1)
	four(a*0.8, 0.2, 0.17)
	c.Fill(ink[1], 1)
}

func drawHeart(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	heartAt(c, a/2, 0.5, 1.42, 0)
	c.Fill(ink[0], 1)
	c.MoveTo(0.3, 0.36) // блик
	c.CubeTo(0.28, 0.3, 0.34, 0.27, 0.4, 0.3)
	c.CubeTo(0.37, 0.32, 0.34, 0.36, 0.3, 0.36)
	c.ClosePath()
	c.Fill(ink[1], 1)
}

func drawHeartsPair(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	heartAt(c, a*0.4, 0.58, 1.15, -0.2)
	c.Fill(ink[0], 1)
	heartAt(c, a*0.74, 0.34, 0.8, 0.28)
	c.Fill(ink[1], 1)
}

func drawCircle(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.Circle(a/2, 0.5, 0.47)
	c.Fill(ink[0], 1)
}

func drawBlob(c *paint.Canvas, s Spec, ink []color.NRGBA, a float64) {
	r := paint.CellRand(s.Seed, s.Kind, 0, 0)
	const n = 9
	ps := make([]pt, n)
	for i := range ps {
		ang := 2*math.Pi*float64(i)/n + r.RangeF(-0.2, 0.2)
		k := r.RangeF(0.74, 0.97)
		sn, cs := math.Sincos(ang)
		ps[i] = pt{a/2 + 0.5*a*0.96*k*cs, 0.5 + 0.5*0.96*k*sn}
	}
	smooth(c, ps)
	c.Fill(ink[0], 1)
}

// drawPlate — плашка под текст: скругление от короткой стороны, не растягивается (решение №136).
func drawPlate(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	m := min(a, 1)
	pad := 0.02 * m
	c.PolyPos(rrect(pad, pad, a-pad, 1-pad, 0.22*m))
	c.Fill(ink[0], 1)
	in, w := 0.07*m, 0.018*m
	c.Poly(orient(rrect(in, in, a-in, 1-in, 0.17*m), true))
	c.Poly(orient(rrect(in+w, in+w, a-in-w, 1-in-w, 0.17*m-w), false))
	c.Fill(ink[1], 1)
}

func drawBanner(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for _, mir := range [2]float64{0, 1} {
		x := func(u float64) float64 { return math.Abs(mir*a - u) }
		c.Poly([]pt{{x(0.03), 0.3}, {x(0.5), 0.3}, {x(0.5), 0.95}, {x(0.03), 0.95}, {x(0.19), 0.625}})
		c.Fill(ink[1], 1)
	}
	c.Poly([]pt{{0.3, 0.1}, {a - 0.3, 0.1}, {a - 0.3, 0.8}, {0.3, 0.8}})
	c.Fill(ink[0], 1)
	// Сгибы: тёмные треугольники под лентой у краёв.
	c.Poly([]pt{{0.3, 0.8}, {0.5, 0.8}, {0.5, 0.95}})
	c.Poly([]pt{{a - 0.3, 0.8}, {a - 0.5, 0.8}, {a - 0.5, 0.95}})
	c.Fill(ink[1], 1)
}

// drawWaveBand — 3 волны от заднего слоя к переднему; длина и размах — от короткой стороны.
func drawWaveBand(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	m := min(a, 1)
	length, amp := 1.1*m, 0.1*m
	step := length / 32
	for i := range 3 {
		base := 0.34 + 0.2*float64(i)
		ph := float64(i) * length * 0.31
		c.MoveTo(0, 1)
		for x := 0.0; ; x += step {
			x = min(x, a)
			c.LineTo(x, base+amp*math.Sin(2*math.Pi*(x+ph)/length))
			if x >= a {
				break
			}
		}
		c.LineTo(a, 1)
		c.ClosePath()
		c.Fill(ink[i], 1)
	}
}

func drawRainbow(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 4 {
		r1 := 0.93 - 0.12*float64(i)
		band(c, a/2, 0.96, r1-0.12, r1, 180, 360)
		c.Fill(ink[i], 1)
	}
}

func drawArches(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 3 {
		w, h := 0.8-0.24*float64(i), 0.96-0.24*float64(i)
		x0, x1, bot := a/2-w/2, a/2+w/2, 0.98
		ps := []pt{{x0, bot}}
		ps = append(ps, paint.Arc(a/2, bot-h+w/2, w/2, 180, 360, 24)...)
		ps = append(ps, pt{x1, bot})
		c.PolyPos(ps)
		c.Fill(ink[i], 1)
	}
}

func drawCorner(c *paint.Canvas, _ Spec, ink []color.NRGBA, _ float64) {
	c.Poly([]pt{{0.03, 0.03}, {0.97, 0.03}, {0.03, 0.97}})
	c.Poly(orient([]pt{{0.2, 0.2}, {0.55, 0.2}, {0.2, 0.55}}, false))
	c.Fill(ink[0], 1)
}

func drawStroke(c *paint.Canvas, s Spec, ink []color.NRGBA, a float64) {
	r := paint.CellRand(s.Seed, s.Kind, 0, 0)
	const n = 7
	var top, bot []pt
	for i := range n {
		x := 0.12 + (a-0.24)*float64(i)/(n-1)
		taper := 0.1 + 0.24*math.Sin(math.Pi*(float64(i)+0.5)/n)
		mid := 0.5 + r.RangeF(-0.08, 0.08)
		top = append(top, pt{x, mid - taper - r.RangeF(0, 0.06)})
		bot = append(bot, pt{x, mid + taper + r.RangeF(0, 0.06)})
	}
	ps := append([]pt{{0.02, 0.5}}, top...)
	ps = append(ps, pt{a - 0.02, 0.5})
	for i := n - 1; i >= 0; i-- {
		ps = append(ps, bot[i])
	}
	smooth(c, ps)
	c.Fill(ink[0], 1)
}

func drawArrow(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	p := bezier(pt{0.12, 0.78}, pt{0.5, 0.95}, pt{a - 0.9, 0.1}, pt{a - 0.3, 0.3}, 28)
	c.Stroke(p, false, 0.07)
	e := p[len(p)-1]
	d := pt{e.X - p[len(p)-4].X, e.Y - p[len(p)-4].Y}
	l := math.Hypot(d.X, d.Y)
	d = pt{d.X / l, d.Y / l}
	nx, ny := -d.Y, d.X
	c.Poly([]pt{{e.X + d.X*0.2, e.Y + d.Y*0.2}, {e.X + nx*0.14, e.Y + ny*0.14}, {e.X - nx*0.14, e.Y - ny*0.14}})
	c.Fill(ink[0], 1)
}

func drawConfettiPile(c *paint.Canvas, s Spec, ink []color.NRGBA, a float64) {
	r := paint.CellRand(s.Seed, s.Kind, 0, 0)
	for range 46 {
		ang, rad := r.RangeF(0, 2*math.Pi), 0.46*math.Sqrt(r.Float())
		sn, cs := math.Sincos(ang)
		x, y := a/2+rad*cs, 0.5+rad*sn
		rot, col := r.RangeF(0, math.Pi), ink[r.Intn(4)]
		switch r.Intn(3) {
		case 0:
			c.RotRect(x, y, 0.03, 0.09, rot)
		case 1:
			c.Circle(x, y, 0.03)
		default:
			c.Poly(paint.Regular(x, y, 0.05, 3, rot))
		}
		c.Fill(col, 1)
	}
}
