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
	k := c.m[0]
	cols := make([]color.NRGBA, c.clip.Dx())
	for i := range cols {
		x := (float64(c.clip.Min.X+i) + 0.5 - c.m[2]) / k / c.w
		cols[i] = lerp(ink[0], ink[1], x)
	}
	c.shade(func(_ float64, row []color.NRGBA) { copy(row, cols) })
}

// gradD — Диагональный градиент: из левого верхнего угла в правый нижний.
func gradD(c *canvas, s Spec) {
	ink := inks(s)
	rp := newRamp(ink[0], ink[1])
	k := c.m[0]
	xs := make([]float64, c.clip.Dx())
	for i := range xs {
		xs[i] = (float64(c.clip.Min.X+i) + 0.5 - c.m[2]) / k / c.w
	}
	c.shade(func(y float64, row []color.NRGBA) {
		ty := y / c.h
		for i, x := range xs {
			row[i] = rp.at((x + ty) / 2)
		}
	})
}

// glow — Свечение: первый цвет в центре, второй к углам; Scale — радиус пятна.
func glow(c *canvas, s Spec) {
	ink := inks(s)
	rp := newRamp(ink[0], ink[1])
	k := c.m[0]
	cx, cy := c.w/2, c.h/2
	r := math.Hypot(cx, cy) * s.Scale
	xs := make([]float64, c.clip.Dx())
	for i := range xs {
		xs[i] = (float64(c.clip.Min.X+i)+0.5-c.m[2])/k - cx
	}
	c.shade(func(y float64, row []color.NRGBA) {
		dy := y - cy
		for i, dx := range xs {
			t := min(math.Sqrt(dx*dx+dy*dy)/r, 1)
			row[i] = rp.at(t * t * (3 - 2*t))
		}
	})
}

// shade заполняет обрезку построчно: fn получает y центра строки в мм и пишет цвета строки.
func (c *canvas) shade(fn func(y float64, row []color.NRGBA)) {
	row := make([]color.NRGBA, c.clip.Dx())
	a := uint32(math.Round(255 * c.alpha))
	for y := c.clip.Min.Y; y < c.clip.Max.Y; y++ {
		fn((float64(y)+0.5-c.m[5])/c.m[4], row)
		off := c.dst.PixOffset(c.clip.Min.X, y)
		px := c.dst.Pix[off : off+4*len(row)]
		for i, v := range row {
			p := px[4*i : 4*i+4 : 4*i+4]
			if a == 255 {
				p[0], p[1], p[2], p[3] = v.R, v.G, v.B, 255
				continue
			}
			// Холст — умноженный RGBA: d = s·a + d·(1 − a).
			p[0] = blend(uint32(v.R), uint32(p[0]), a)
			p[1] = blend(uint32(v.G), uint32(p[1]), a)
			p[2] = blend(uint32(v.B), uint32(p[2]), a)
			p[3] = blend(255, uint32(p[3]), a)
		}
	}
}

func blend(s, d, a uint32) uint8 { return uint8((s*a + d*(255-a) + 127) / 255) } //nolint:gosec // a ≤ 255: итог 0..255

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
