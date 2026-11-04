package gen

import (
	"image/color"
	"math"
)

// Сплошные узоры: градиенты и свечение — цвет каждого пикселя по его центру в мм, без
// растеризатора и без шума («печать без зерна»).

// gradH — Градиент: слева первый цвет, справа второй.
func gradH(c *canvas, s Spec) {
	ink := inks(s)
	k := c.M[0]
	cols := make([]color.NRGBA, c.Clip.Dx())
	for i := range cols {
		x := (float64(c.Clip.Min.X+i) + 0.5 - c.M[2]) / k / c.W
		cols[i] = lerp(ink[0], ink[1], x)
	}
	c.Shade(func(_ float64, row []color.NRGBA) { copy(row, cols) })
}

// gradD — Диагональный градиент: из левого верхнего угла в правый нижний.
func gradD(c *canvas, s Spec) {
	ink := inks(s)
	rp := newRamp(ink[0], ink[1])
	k := c.M[0]
	xs := make([]float64, c.Clip.Dx())
	for i := range xs {
		xs[i] = (float64(c.Clip.Min.X+i) + 0.5 - c.M[2]) / k / c.W
	}
	c.Shade(func(y float64, row []color.NRGBA) {
		ty := y / c.H
		for i, x := range xs {
			row[i] = rp.at((x + ty) / 2)
		}
	})
}

// glow — Свечение: первый цвет в центре, второй к углам; Scale — радиус пятна.
func glow(c *canvas, s Spec) {
	ink := inks(s)
	rp := newRamp(ink[0], ink[1])
	k := c.M[0]
	cx, cy := c.W/2, c.H/2
	r := math.Hypot(cx, cy) * s.Scale
	xs := make([]float64, c.Clip.Dx())
	for i := range xs {
		xs[i] = (float64(c.Clip.Min.X+i)+0.5-c.M[2])/k - cx
	}
	c.Shade(func(y float64, row []color.NRGBA) {
		dy := y - cy
		for i, dx := range xs {
			t := min(math.Sqrt(dx*dx+dy*dy)/r, 1)
			row[i] = rp.at(t * t * (3 - 2*t))
		}
	})
}

// lerp — цвет между a и b в доле t (зажата в 0..1), в каналах sRGB.
func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	t = min(max(t, 0), 1)
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// ramp — lerp(a, b, t) таблицей на 1024 шага: попиксельный градиент обложки без трёх Round на
// пиксель (шаг 1/1023 меньше полутона канала).
type ramp [1024]color.NRGBA

func newRamp(a, b color.NRGBA) *ramp {
	var r ramp
	for i := range r {
		r[i] = lerp(a, b, float64(i)/1023)
	}
	return &r
}

func (r *ramp) at(t float64) color.NRGBA {
	return r[int(min(max(t, 0), 1)*1023+0.5)]
}
