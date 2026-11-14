package art

import (
	"image/color"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Предметы: шары, шапка, карандаш, книга, яблоко, колокольчик, кольца. Пространство — как в
// shapes.go.

// balloonAt — шар с центром (x, y), высота 2·ry, нитка вниз до (x, ye) цветом str.
func balloonAt(c *paint.Canvas, x, y, rx, ry, ye float64, body, str color.NRGBA) {
	c.Stroke(bezier(pt{x, y + ry}, pt{x - 0.05, y + ry + (ye-y-ry)*0.4}, pt{x + 0.05, y + ry + (ye-y-ry)*0.7}, pt{x, ye}, 14), false, 0.012)
	c.Fill(str, 1)
	c.Ellipse(x, y, rx, ry, 0)
	c.Poly([]pt{{x, y + ry - 0.01}, {x - 0.04, y + ry + 0.05}, {x + 0.04, y + ry + 0.05}})
	c.Fill(body, 1)
	c.Ellipse(x-rx*0.4, y-ry*0.4, rx*0.14, ry*0.26, 0.5) // блик
	c.Fill(color.NRGBA{255, 255, 255, 255}, 0.4)
}

func drawBalloon(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	balloonAt(c, a/2, 0.4, 0.3, 0.37, 0.98, ink[0], ink[1])
}

func drawBalloons(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	str := color.NRGBA{140, 140, 140, 255}
	balloonAt(c, a*0.3, 0.46, 0.19, 0.23, 0.95, ink[0], str)
	balloonAt(c, a*0.7, 0.46, 0.19, 0.23, 0.95, ink[1], str)
	balloonAt(c, a*0.5, 0.3, 0.19, 0.23, 0.95, ink[2], str)
}

func drawCap(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	cx := a / 2
	// Тулья под доской.
	c.MoveTo(cx-0.4, 0.5)
	c.CubeTo(cx-0.4, 0.8, cx+0.4, 0.8, cx+0.4, 0.5)
	c.LineTo(cx, 0.62)
	c.ClosePath()
	c.Fill(ink[0], 1)
	c.Poly([]pt{{cx, 0.12}, {cx + 0.65, 0.38}, {cx, 0.64}, {cx - 0.65, 0.38}})
	c.Fill(ink[0], 1)
	c.Circle(cx, 0.38, 0.03)
	c.Stroke([]pt{{cx, 0.38}, {cx + 0.5, 0.45}, {cx + 0.52, 0.8}}, false, 0.022)
	c.Circle(cx+0.52, 0.8, 0.034)
	c.Poly([]pt{{cx + 0.5, 0.8}, {cx + 0.54, 0.8}, {cx + 0.56, 0.93}, {cx + 0.48, 0.93}})
	c.Fill(ink[1], 1)
}

func drawPencil(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	l := a - 0.9 // длина корпуса без заточенной части
	c.PolyPos(rrect(0.05, 0.25, 0.5, 0.75, 0.12))
	c.Fill(ink[2], 1) // ластик
	c.Poly([]pt{{0.5, 0.25}, {l, 0.25}, {l, 0.75}, {0.5, 0.75}})
	c.Fill(ink[0], 1)
	c.Poly([]pt{{l, 0.25}, {a - 0.04, 0.5}, {l, 0.75}})
	c.Fill(ink[1], 1)
	c.Poly([]pt{{a - 0.3, 0.41}, {a - 0.04, 0.5}, {a - 0.3, 0.59}})
	c.Fill(ink[2], 1)
	c.Poly([]pt{{0.5, 0.45}, {l, 0.45}, {l, 0.5}, {0.5, 0.5}}) // грань
	c.Fill(color.NRGBA{255, 255, 255, 255}, 0.3)
}

func drawBook(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	cx := a / 2
	c.PolyPos(rrect(0.03, 0.2, a-0.03, 0.9, 0.04))
	c.Fill(ink[1], 1)
	for _, side := range [2]float64{-1, 1} {
		x := func(u float64) float64 { return cx + side*u }
		c.MoveTo(x(0.02), 0.26)
		c.CubeTo(x(0.25), 0.2, x(0.5), 0.2, x(a/2-0.1), 0.28)
		c.LineTo(x(a/2-0.1), 0.82)
		c.CubeTo(x(0.5), 0.74, x(0.25), 0.74, x(0.02), 0.82)
		c.ClosePath()
	}
	c.Fill(ink[0], 1)
	for _, side := range [2]float64{-1, 1} {
		for i := range 4 {
			y := 0.38 + 0.1*float64(i)
			c.Poly(orient([]pt{{cx + side*0.14, y}, {cx + side*(a/2-0.26), y}, {cx + side*(a/2-0.26), y + 0.018}, {cx + side*0.14, y + 0.018}}, true))
		}
	}
	c.Fill(ink[1], 0.7)
}

func drawApple(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	cx := a / 2
	smooth(c, []pt{
		{cx, 0.3}, {cx + 0.18, 0.24}, {cx + 0.38, 0.35}, {cx + 0.43, 0.58}, {cx + 0.31, 0.85},
		{cx + 0.15, 0.95}, {cx, 0.9}, {cx - 0.15, 0.95}, {cx - 0.31, 0.85}, {cx - 0.43, 0.58}, {cx - 0.38, 0.35}, {cx - 0.18, 0.24},
	})
	c.Fill(ink[0], 1)
	c.Stroke([]pt{{cx, 0.32}, {cx + 0.02, 0.18}, {cx + 0.05, 0.08}}, false, 0.035)
	leafAt(c, cx+0.04, 0.14, 0.3, 0.08, -0.35)
	c.Fill(ink[1], 1)
}

func drawBell(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	cx := a / 2
	smooth(c, []pt{
		{cx, 0.13}, {cx + 0.15, 0.2}, {cx + 0.2, 0.45}, {cx + 0.28, 0.68}, {cx + 0.39, 0.8}, {cx + 0.39, 0.85},
		{cx - 0.39, 0.85}, {cx - 0.39, 0.8}, {cx - 0.28, 0.68}, {cx - 0.2, 0.45}, {cx - 0.15, 0.2},
	})
	c.Fill(ink[0], 1)
	c.Circle(cx, 0.1, 0.05)
	c.Circle(cx, 0.91, 0.07)
	c.Fill(ink[1], 1)
}

func drawRings(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i, x := range [2]float64{a/2 - 0.22, a/2 + 0.22} {
		band(c, x, 0.55, 0.26, 0.37, 0, 180)
		band(c, x, 0.55, 0.26, 0.37, 180, 360)
		c.Fill(ink[i], 1)
	}
}
