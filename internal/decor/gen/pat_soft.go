package gen

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Мягкие узоры (акварель, мазки): рисуются в слое разрешения softK и растягиваются — в печати
// края мягкие, а время обложки — как у экрана (research §3: акварель в полном 300 dpi — 3 с).

// brush — Мазки: изогнутые ленты с сужением к концам, общий наклон от сида.
func brush(c *canvas, s Spec) {
	ink := inks(s)
	g := paint.CellRand(s.Seed, s.Kind+"/угол", 0, 0)
	tilt := g.RangeF(-0.6, 0.6)
	step := 70 * s.Scale
	c.Soft(func(sc *canvas) {
		scatter(sc, s, step, 50*s.Scale, func(r *rng, x0, y0 float64) {
			for range 1 + r.Intn(2) {
				st := brushStroke{
					x: x0 + r.RangeF(0, step), y: y0 + r.RangeF(0, step),
					l: r.RangeF(45, 90) * s.Scale, w: r.RangeF(10, 18) * s.Scale,
					rot: tilt + r.RangeF(-0.35, 0.35), bend: r.RangeF(-0.15, 0.15),
					ph1: r.RangeF(0, 2*math.Pi), ph2: r.RangeF(0, 2*math.Pi),
				}
				col := ink[r.Intn(4)]
				sc.Poly(st.outline())
				sc.Fill(col, 0.85)
			}
		})
	})
}

// brushStroke — мазок кисти: центр, длина, ширина, наклон, изгиб (доля длины), фазы неровности края.
type brushStroke struct{ x, y, l, w, rot, bend, ph1, ph2 float64 }

func (st brushStroke) outline() []pt {
	const n = 28
	sn, cs := math.Sincos(st.rot)
	left := make([]pt, 0, n+1)
	right := make([]pt, 0, n+1)
	for i := range n + 1 {
		t := float64(i) / n
		u := (t - 0.5) * st.l
		v := st.bend * st.l * 4 * t * (1 - t)
		hw := st.w / 2 * math.Pow(math.Sin(math.Pi*t), 0.6)
		e1 := hw * (1 + 0.08*math.Sin(9*t+st.ph1))
		e2 := hw * (1 + 0.08*math.Sin(11*t+st.ph2))
		left = append(left, pt{st.x + u*cs - (v-e1)*sn, st.y + u*sn + (v-e1)*cs})
		right = append(right, pt{st.x + u*cs - (v+e2)*sn, st.y + u*sn + (v+e2)*cs})
	}
	for i := len(right) - 1; i >= 0; i-- {
		left = append(left, right[i])
	}
	return left
}

// watercolor — Акварель: пятна из шести полупрозрачных слоёв с неровным краем.
func watercolor(c *canvas, s Spec) {
	ink := inks(s)
	step := 80 * s.Scale
	c.Soft(func(sc *canvas) {
		scatter(sc, s, step, 48*s.Scale, func(r *rng, x0, y0 float64) {
			for range 1 + r.Intn(2) {
				x, y := x0+r.RangeF(0, step), y0+r.RangeF(0, step)
				rad := r.RangeF(22, 40) * s.Scale
				col := ink[r.Intn(3)]
				show := sc.Visible(x, y, 1.5*rad)
				for l := range 6 {
					var amp, ph [3]float64
					for j := range 3 {
						amp[j] = r.RangeF(0.04, 0.12) / float64(j+1)
						ph[j] = r.RangeF(0, 2*math.Pi)
					}
					if show {
						sc.Poly(blob(x, y, rad*(1-0.12*float64(l)), amp, ph))
						sc.Fill(col, 0.16)
					}
				}
			}
		})
	})
}

// blob — замкнутое пятно радиуса rad: радиус колеблется гармониками 2, 3, 5 с амплитудами amp.
// sin(j·a + φ) раскрыт через таблицу blobTrig: тригонометрия — 2 вызова на гармонику, а не 40.
func blob(x, y, rad float64, amp, ph [3]float64) []pt {
	var sp, cp [3]float64
	for j := range 3 {
		sp[j], cp[j] = math.Sincos(ph[j])
	}
	ps := make([]pt, blobN)
	for i := range ps {
		t := &blobTrig[i]
		k := 1.0
		for j := range 3 {
			k += amp[j] * (t[j+1][0]*cp[j] + t[j+1][1]*sp[j])
		}
		ps[i] = pt{x + rad*k*t[0][1], y + rad*k*t[0][0]}
	}
	return ps
}

const blobN = 40

// blobTrig[i] — sin и cos угла a = 2π·i/40 и гармоник 2a, 3a, 5a.
var blobTrig = func() (out [blobN][4][2]float64) {
	for i := range out {
		a := 2 * math.Pi * float64(i) / blobN
		for j, h := range [4]float64{1, 2, 3, 5} {
			out[i][j][0], out[i][j][1] = math.Sincos(h * a)
		}
	}
	return out
}()
