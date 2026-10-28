package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Окно рамочки места (079 design §2): фото садится не в место, а в окно внутри него — место
// минус отступы рамочки. Зеркало для живого изменения размера — во фронте, паритет держат
// векторы testdata/parity/inset.json.

// MaxInsetPair — доля стороны места, которую могут занять два отступа одной оси: окно не
// меньше 30 % стороны.
const MaxInsetPair = 0.7

// ClampInset — отступы l, t, r, b (мм) места w×h: NaN, Inf и минус — 0; пара оси больше
// MaxInsetPair стороны сжимается пропорционально. Идемпотентна.
func ClampInset(ins [4]float64, w, h float64) [4]float64 {
	var out [4]float64
	for i, v := range ins {
		if v > 0 && !math.IsInf(v, 1) {
			out[i] = v
		}
	}
	pair := func(a, b *float64, side float64) {
		lim := MaxInsetPair * max(side, 0)
		if s := *a + *b; s > lim {
			k := lim / s
			*a, *b = *a*k, *b*k
		}
	}
	pair(&out[0], &out[2], w)
	pair(&out[1], &out[3], h)
	return out
}

// InsetBox — окно в месте cell формы o при отступах ins (до зажима ClampInset): место минус
// отступы; форма окна — овал, вписанный в окно, или скругление max(R − min(ins), 0). Без
// отступов — само место и его форма.
func InsetBox(cell layout.Rect, o Outline, ins [4]float64) (layout.Rect, Outline) {
	ins = ClampInset(ins, cell.W, cell.H)
	if ins == ([4]float64{}) {
		return cell, o
	}
	win := layout.Rect{X: cell.X + ins[0], Y: cell.Y + ins[1], W: cell.W - ins[0] - ins[2], H: cell.H - ins[1] - ins[3]}
	wo := Outline{Oval: o.Oval}
	if !o.Oval && o.Radius > 0 {
		wo.Radius = max(o.Radius-min(ins[0], ins[1], ins[2], ins[3]), 0)
	}
	return win, wo
}
