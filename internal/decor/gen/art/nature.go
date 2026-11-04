package art

import (
	"image/color"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Растения и небо: цветок, ромашка, лист, ветка, солнце, облако. Пространство — как в shapes.go.

func drawFlower(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 6 {
		c.Ellipse(a/2+0.2*math.Cos(float64(i)*math.Pi/3), 0.5+0.2*math.Sin(float64(i)*math.Pi/3), 0.17, 0.12, float64(i)*math.Pi/3)
	}
	c.Fill(ink[0], 1) // лепестки одним путём: перекрытия не темнеют при прозрачности
	c.Circle(a/2, 0.5, 0.11)
	c.Fill(ink[1], 1)
}

func drawDaisy(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 14 {
		ang := float64(i) * 2 * math.Pi / 14
		c.Ellipse(a/2+0.3*math.Cos(ang), 0.5+0.3*math.Sin(ang), 0.19, 0.06, ang)
	}
	c.Fill(ink[0], 1)
	c.Circle(a/2, 0.5, 0.17)
	c.Fill(ink[1], 1)
}

func drawLeaf(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	leafAt(c, a/2, 0.97, 0.94, 0.27, -math.Pi/2)
	c.Fill(ink[0], 1)
	c.Stroke(bezier(pt{a / 2, 0.95}, pt{a/2 + 0.02, 0.7}, pt{a/2 - 0.02, 0.4}, pt{a / 2, 0.14}, 12), false, 0.025)
	c.Fill(ink[1], 1)
}

func drawBranch(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	stem := bezier(pt{a * 0.45, 0.98}, pt{a * 0.5, 0.75}, pt{a * 0.42, 0.5}, pt{a * 0.55, 0.3}, 20)
	c.Stroke(stem, false, 0.03)
	c.Fill(ink[0], 1)
	for i := range 5 {
		p := stem[3+i*3]
		side := 1.0
		if i%2 == 1 {
			side = -1
		}
		leafAt(c, p.X, p.Y, 0.3-0.02*float64(i), 0.08, -math.Pi/2+side*0.95)
	}
	leafAt(c, stem[20].X, stem[20].Y, 0.24, 0.07, -math.Pi/2+0.2)
	c.Fill(ink[1], 1)
}

func drawSun(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 12 {
		ang := float64(i) * math.Pi / 6
		sn, cs := math.Sincos(ang)
		nx, ny := -sn, cs
		c.Poly([]pt{
			{a/2 + 0.33*cs + 0.045*nx, 0.5 + 0.33*sn + 0.045*ny},
			{a/2 + 0.49*cs, 0.5 + 0.49*sn},
			{a/2 + 0.33*cs - 0.045*nx, 0.5 + 0.33*sn - 0.045*ny},
		})
	}
	c.Fill(ink[1], 1)
	c.Circle(a/2, 0.5, 0.26)
	c.Fill(ink[0], 1)
}

func drawCloud(c *paint.Canvas, _ Spec, ink []color.NRGBA, a float64) {
	// Облако — три круга на плоском основании; k — размер, (x, y) — середина по габариту.
	puff := func(x, y, k float64) {
		c.Circle(x-0.33*k, y+0.07*k, 0.19*k)
		c.Circle(x-0.02*k, y-0.08*k, 0.28*k)
		c.Circle(x+0.34*k, y+0.05*k, 0.22*k)
		c.PolyPos([]pt{{x - 0.33*k, y - 0.12*k}, {x + 0.34*k, y - 0.17*k}, {x + 0.34*k, y + 0.27*k}, {x - 0.33*k, y + 0.26*k}})
	}
	puff(a/2+0.3, 0.3, 0.7)
	c.Fill(ink[1], 1)
	puff(a/2-0.05, 0.56, 1.25)
	c.Fill(ink[0], 1)
}
