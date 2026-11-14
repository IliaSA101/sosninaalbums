package gen

import "math"

// Правильные узоры: горох, полосы, клетка, линейка, волны. Размеры — мм при Scale 1; фигуры
// заходят за край области на 1 мм, чтобы край области не получал полупрозрачную кромку.

// dots — Горох: шахматная сетка кругов.
func dots(c *canvas, s Spec) {
	ink := inks(s)
	p, r := 14*s.Scale, 2.6*s.Scale
	for j := 0; ; j++ {
		y := p/4 + float64(j)*p/2
		if y-r > c.H {
			break
		}
		for x := p/4 + float64(j%2)*p/2; x-r <= c.W; x += p {
			c.Circle(x, y, r)
			c.Fill(ink[0], 1)
		}
	}
}

// stripes — Полосы: вертикальные полосы одного цвета.
func stripes(c *canvas, s Spec) {
	ink := inks(s)
	p, w := 10*s.Scale, 3.5*s.Scale
	for x := (p - w) / 2; x <= c.W; x += p {
		c.Poly([]pt{{x, -1}, {x + w, -1}, {x + w, c.H + 1}, {x, c.H + 1}})
		c.Fill(ink[0], 1)
	}
}

// bands — Широкие полосы: широкая первого цвета и узкая второго, по горизонтали.
func bands(c *canvas, s Spec) {
	ink := inks(s)
	p := 40 * s.Scale
	for y := 0.0; y <= c.H; y += p {
		for i, b := range [2][2]float64{{4, 18}, {24, 27}} {
			y0, y1 := y+b[0]*s.Scale, y+b[1]*s.Scale
			c.Poly([]pt{{-1, y0}, {c.W + 1, y0}, {c.W + 1, y1}, {-1, y1}})
			c.Fill(ink[i], 1)
		}
	}
}

// grid5 — Клетка 5 мм: все линии одним путём — на пересечениях цвет не темнеет и при
// прозрачности.
func grid5(c *canvas, s Spec) {
	ink := inks(s)
	p, w := 5*s.Scale, 0.22*math.Sqrt(s.Scale)
	for x := p; x-w < c.W; x += p {
		c.PolyPos([]pt{{x - w/2, -1}, {x + w/2, -1}, {x + w/2, c.H + 1}, {x - w/2, c.H + 1}})
	}
	for y := p; y-w < c.H; y += p {
		c.PolyPos([]pt{{-1, y - w/2}, {c.W + 1, y - w/2}, {c.W + 1, y + w/2}, {-1, y + w/2}})
	}
	c.Fill(ink[0], 1)
}

// lines — Линейка с полем: строки первым цветом, поле слева вторым (поверх, как чернила).
func lines(c *canvas, s Spec) {
	ink := inks(s)
	p, w := 8*s.Scale, 0.25*math.Sqrt(s.Scale)
	for y := p; y-w < c.H; y += p {
		c.Poly([]pt{{-1, y - w/2}, {c.W + 1, y - w/2}, {c.W + 1, y + w/2}, {-1, y + w/2}})
	}
	c.Fill(ink[0], 1)
	x, wm := 22*s.Scale, 0.35*math.Sqrt(s.Scale)
	c.Poly([]pt{{x - wm/2, -1}, {x + wm/2, -1}, {x + wm/2, c.H + 1}, {x - wm/2, c.H + 1}})
	c.Fill(ink[1], 1)
}

// waves — Волны снизу: три слоя синусоид от заднего (первый цвет, выше всех) к переднему.
func waves(c *canvas, s Spec) {
	ink := inks(s)
	a, l := 5*s.Scale, 90*s.Scale
	step := l / 48
	for i := range 3 {
		base := c.H - float64(38-12*i)*s.Scale
		ph := float64(i) * l * 0.31
		c.MoveTo(-1, c.H+1)
		for x := -1.0; ; x += step {
			x = min(x, c.W+1)
			c.LineTo(x, base+a*math.Sin(2*math.Pi*(x+ph)/l))
			if x >= c.W+1 {
				break
			}
		}
		c.LineTo(c.W+1, c.H+1)
		c.ClosePath()
		c.Fill(ink[i], 1)
	}
}
