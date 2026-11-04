package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// MaxScale — предел увеличения кадра (model.MaxScale берёт его отсюда): дальше него
// наклон приближение не держит (057, решение 13).
const MaxScale = 4.0

// TiltCrop — кадр base, повёрнутый до r (четверти + наклон), по правилу 057 «наклон держит
// приближение»: фото на экране того же размера, точка фото в центре места остаётся в центре
// (поворот вокруг центра места, решение 10). S₀ ≤ 1 — приближения нет: S′ = 1, фото ровно
// закрывает место (решение 9). Тот же r — base как есть: цепочка 0 → 3 → 0 возвращает кадр
// побитно. Без округления — зажим ClampFrame у вызывающего; «Целиком» сюда не идёт (у него
// меняется только R). Зеркало — frontend geom/frame.ts tiltFrame, паритет — tilt.json.
func TiltCrop(cell layout.Rect, aspect float64, base Crop, r float64, o Outline) Crop {
	if r == base.R {
		return base
	}
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	bw0, bh0, cov0 := tiltCover(cell, aspect, base.R, o)
	bw1, bh1, cov1 := tiltCover(cell, aspect, r, o)
	out := Crop{S: 1, R: r}
	if base.S > 1 && cov1 > 0 {
		out.S = ClampF(base.S*cov0/cov1, 1, MaxScale)
	}
	w0, w1 := cov0*base.S, cov1*out.S
	h0, h1 := w0/aspect, w1/aspect
	out.X = tiltShift(base.X, w0-bw0, w0, w1-bw1, w1)
	out.Y = tiltShift(base.Y, h0-bh0, h0, h1-bh1, h1)
	return out
}

// tiltCover — габарит формы в осях фото, повёрнутого на r, и ширина фото, закрывающего
// её при S = 1 (как placeImageRotIn).
func tiltCover(cell layout.Rect, aspect, r float64, o Outline) (bw, bh, cov float64) {
	sn, cs := sinCosDeg(r)
	bw, bh = o.bound(cell.W, cell.H, math.Abs(cs), math.Abs(sn))
	return bw, bh, max(bw, bh*aspect)
}

// tiltShift — сдвиг по одной оси фото: центр фото от центра места u = −f·x/2 (f — ход)
// держится долей размера фото w; новый ход f′ ≈ 0 — сдвигать некуда.
func tiltShift(x, f0, w0, f1, w1 float64) float64 {
	if !(f1 > 1e-9) || !(w0 > 0) {
		return 0
	}
	u := -f0 * x / 2 / w0 * w1
	return ClampF(-2*u/f1, -1, 1)
}
