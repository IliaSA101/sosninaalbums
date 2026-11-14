package render

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/vector"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Форма места (спека 044, design §5): эллипс, вписанный в рамку, или скругление углов. Маска —
// контур кривыми Безье в vector.Rasterizer (сглаживание то же, что у пути 3): путь 1 умножает
// альфу готовой ячейки (Lanczos сохраняется), путь 3 — маску полосы.

// Дуги — кубические Безье по 45°: отклонение от окружности ~4e-6 радиуса (0,007 px у круга
// 300 мм при 300 dpi); дуги по 90° дали бы 0,48 px.

// hasShape — у фото форма, которую надо маскировать: эллипс или ненулевое скругление.
func hasShape(o geom.Outline) bool { return o.Oval || o.Radius > 0 }

// shapePath пишет в z контур формы o рамки cell, повёрнутой на rot вокруг центра, в пикселях
// холста (k — px на мм), сдвинутый на −off (начало маски). Аффинный образ Безье точен,
// поэтому поворот и масштаб применяются к контрольным точкам.
func shapePath(z *vector.Rasterizer, cell layout.Rect, rot float64, o geom.Outline, k float64, off image.Point) {
	s, c := sincosDeg(rot)
	cx, cy := center(cell)
	at := func(u, v float64) (float32, float32) {
		return float32(k*(cx+u*c-v*s) - float64(off.X)), float32(k*(cy+u*s+v*c) - float64(off.Y))
	}
	started := false
	// arc — дуга эллипса с центром (ox, oy) и полуосями rx, ry от угла a0 до a1 (рад, по
	// часовой при y вниз) кусками по 45°.
	arc := func(ox, oy, rx, ry, a0, a1 float64) {
		n := max(1, int(math.Round((a1-a0)/(math.Pi/4))))
		d := (a1 - a0) / float64(n)
		h := 4.0 / 3 * math.Tan(d/4) // плечо Безье: у дуги 45° — 0,2652165
		for i := range n {
			t0, t1 := a0+float64(i)*d, a0+float64(i+1)*d
			s0, c0 := math.Sincos(t0)
			s1, c1 := math.Sincos(t1)
			x0, y0 := ox+rx*c0, oy+ry*s0
			if !started {
				z.MoveTo(at(x0, y0))
				started = true
			} else if i == 0 {
				z.LineTo(at(x0, y0))
			}
			b1x, b1y := at(x0-h*rx*s0, y0+h*ry*c0)
			b2x, b2y := at(ox+rx*c1+h*rx*s1, oy+ry*s1-h*ry*c1)
			ex, ey := at(ox+rx*c1, oy+ry*s1)
			z.CubeTo(b1x, b1y, b2x, b2y, ex, ey)
		}
	}
	hw, hh := cell.W/2, cell.H/2
	if o.Oval {
		arc(0, 0, hw, hh, 0, 2*math.Pi)
	} else {
		r := min(o.Radius, hw, hh)
		arc(hw-r, hh-r, r, r, 0, math.Pi/2)
		arc(-hw+r, hh-r, r, r, math.Pi/2, math.Pi)
		arc(-hw+r, -hh+r, r, r, math.Pi, 3*math.Pi/2)
		arc(hw-r, -hh+r, r, r, 3*math.Pi/2, 2*math.Pi)
	}
	z.ClosePath()
}

// shapeMask — покрытие формы o рамки cell (повёрнутой на rot) на пикселях r холста.
func shapeMask(r image.Rectangle, cell layout.Rect, rot float64, o geom.Outline, k float64) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, r.Dx(), r.Dy()))
	var z vector.Rasterizer
	drawShape(&z, m, r.Min, cell, rot, o, k)
	return m
}

// drawShape растеризует форму в маску m (её Rect с нуля), начало маски — пиксель off холста.
// Маска перезаписывается целиком: буфер полосы можно не чистить.
func drawShape(z *vector.Rasterizer, m *image.Alpha, off image.Point, cell layout.Rect, rot float64, o geom.Outline, k float64) {
	z.Reset(m.Rect.Dx(), m.Rect.Dy())
	z.DrawOp = draw.Src
	shapePath(z, cell, rot, o, k, off)
	z.Draw(m, m.Rect, image.Opaque, image.Point{})
}

// blitMasked — blit готовой ячейки пути 1 через маску формы m (того же размера, что cell):
// за формой холст не трогается, внутри пиксель копируется, на краю смешивается по
// альфе × маске. Свой цикл, а не draw.DrawMask: у NRGBA с маской там только общий медленный
// путь (+30 % ко всему развороту на круге 300 мм).
func blitMasked(canvas *image.RGBA, cell *image.NRGBA, r image.Rectangle, m *image.Alpha) {
	vis := r.Intersect(canvas.Bounds())
	for y := vis.Min.Y; y < vis.Max.Y; y++ {
		sy := y - r.Min.Y
		src := cell.Pix[sy*cell.Stride:]
		mask := m.Pix[sy*m.Stride:]
		dst := canvas.Pix[(y-canvas.Rect.Min.Y)*canvas.Stride:]
		for x := vis.Min.X; x < vis.Max.X; x++ {
			sx := x - r.Min.X
			if mask[sx] == 0 {
				continue
			}
			s := src[4*sx : 4*sx+4 : 4*sx+4]
			a := (uint32(mask[sx])*uint32(s[3]) + 127) / 255
			dx := 4 * (x - canvas.Rect.Min.X)
			d := dst[dx : dx+4 : dx+4]
			switch a {
			case 0:
			case 255:
				d[0], d[1], d[2], d[3] = s[0], s[1], s[2], 255
			default:
				// Холст — умноженный RGBA, ячейка — неумноженная: d = s·a + d·(1 − a).
				d[0], d[1], d[2], d[3] = mix(s[0], d[0], a), mix(s[1], d[1], a), mix(s[2], d[2], a), mix(255, d[3], a)
			}
		}
	}
}

// mix — s·a + d·(1 − a) в долях 255.
func mix(s, d uint8, a uint32) uint8 {
	return uint8((uint32(s)*a + uint32(d)*(255-a) + 127) / 255) //nolint:gosec // a ≤ 255: итог 0..255
}

// mulMask — маска полосы пути 3: покрытие многоугольника × покрытие формы m.
func mulMask(dst, m []uint8) {
	for i, a := range m {
		if a != 0xff {
			dst[i] = mix(dst[i], 0, uint32(a))
		}
	}
}
