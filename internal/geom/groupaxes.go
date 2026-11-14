package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Масштаб группы по осям рамки (спека 018, design §3.3): p → a + M(p − a),
// M = R(φ)·diag(kx, ky)·R(−φ). Перекоса нет: место сохраняет угол, его стороны —
// длины образов своих осей (как grid.Rescale). Зеркало — frontend
// geom/groupaxes.ts, паритет testdata/parity/groupaxes.json.

// GxAxesIn — вход GroupAxes: разворот, члены, множители по осям рамки, её угол Phi
// (градусы) и неподвижная точка a.
type GxAxesIn struct {
	SW     float64   `json:"sw"`
	PH     float64   `json:"ph"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
	KX     float64   `json:"kx"`
	KY     float64   `json:"ky"`
	Phi    float64   `json:"phi"`
	AX     float64   `json:"ax"`
	AY     float64   `json:"ay"`
}

// GxAxesOut — итог: действующие множители после зажима, сдвиг внутрь, места (шаг
// 0.01, зажаты как clampPlace) и тексты (H — оценка по кеглю, точную даст fitTexts).
type GxAxesOut struct {
	KX     float64   `json:"kx"`
	KY     float64   `json:"ky"`
	DX     float64   `json:"dx"`
	DY     float64   `json:"dy"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
}

// GroupAxes тянет группу по осям рамки. Кегль — Size·min(kx, ky) (решение 3
// владельца): при растяжке по одной оси он стоит и у повёрнутого текста. Меньший
// множитель зажат сверху по MaxSize/Size, как k у GroupXf, — иначе кегль упрётся
// в 200 и отстанет от группы. Снизу оба не меньше, чем места остаются ≥ MinPlace
// (образ оси ≥ min(kx, ky)) и кегль ≥ MinSize; нижний сильнее верхнего. Не
// помещается в разворот — бисекция по отрезку (1, 1) → (kx, ky). У текста едет
// точка выравнивания.
func GroupAxes(in GxAxesIn) GxAxesOut {
	out := GxAxesOut{KX: 1, KY: 1, Places: []RotRect{}, Texts: []GxText{}} // не nil: в JSON [] для UI
	if len(in.Places)+len(in.Texts) == 0 {
		return out
	}
	kx, ky := in.KX, in.KY
	hi, lo := math.Inf(1), 0.0
	for _, t := range in.Texts {
		hi = math.Min(hi, text.MaxSize/t.Size)
		lo = math.Max(lo, text.MinSize/t.Size)
	}
	// Больший множитель кегль не трогает (тянет только ширину), при равных — оба.
	if m := math.Min(kx, ky); m > hi {
		if kx == m {
			kx = hi
		}
		if ky == m {
			ky = hi
		}
	}
	for _, r := range in.Places {
		lo = math.Max(lo, MinPlace/math.Min(r.W, r.H))
	}
	kx, ky = math.Max(kx, lo), math.Max(ky, lo)
	fits := func(kx, ky float64) bool {
		x0, y0, x1, y1, _ := groupBound(axesAt(in, kx, ky))
		return x1-x0 <= in.SW+1e-9 && y1-y0 <= in.PH+1e-9
	}
	if !fits(kx, ky) {
		t0, t1 := 0.0, 1.0
		for range 30 {
			m := (t0 + t1) / 2
			if fits(1+m*(kx-1), 1+m*(ky-1)) {
				t0 = m
			} else {
				t1 = m
			}
		}
		kx, ky = 1+t0*(kx-1), 1+t0*(ky-1)
	}
	out.KX, out.KY = kx, ky
	pls, txs := axesAt(in, kx, ky)
	x0, y0, x1, y1, _ := groupBound(pls, txs)
	out.DX, out.DY = shiftIn(x0, x1, in.SW), shiftIn(y0, y1, in.PH)
	for _, r := range pls {
		cx, cy := r.X+r.W/2+out.DX, r.Y+r.H/2+out.DY
		r.W, r.H = Round2(r.W), Round2(r.H)
		r.X, r.Y = Round2(cx-r.W/2), Round2(cy-r.H/2)
		ClampPlaceIn(&r, in.SW, in.PH)
		out.Places = append(out.Places, r)
	}
	for _, t := range txs {
		t.X, t.Y, t.W = Round2(t.X+out.DX), Round2(t.Y+out.DY), Round2(t.W)
		out.Texts = append(out.Texts, t)
	}
	return out
}

// axesAt — члены после масштаба (kx, ky) по осям рамки, без сдвига и округления.
func axesAt(in GxAxesIn, kx, ky float64) ([]RotRect, []GxText) {
	tr := func(x, y float64) (float64, float64) {
		u, v := Rotate(-in.Phi, x-in.AX, y-in.AY)
		dx, dy := Rotate(in.Phi, u*kx, v*ky)
		return in.AX + dx, in.AY + dy
	}
	// sides — множители своих осей объекта под углом rot: длины образов (1, 0) и (0, 1).
	sides := func(rot float64) (float64, float64) {
		s, c := math.Sincos((rot - in.Phi) * math.Pi / 180)
		return math.Hypot(kx*c, ky*s), math.Hypot(kx*s, ky*c)
	}
	pls := make([]RotRect, len(in.Places))
	for i, r := range in.Places {
		cx, cy := tr(r.X+r.W/2, r.Y+r.H/2)
		sx, sy := sides(r.Rot)
		w, h := r.W*sx, r.H*sy
		pls[i] = RotRect{X: cx - w/2, Y: cy - h/2, W: w, H: h, Rot: r.Rot}
	}
	txs := make([]GxText, len(in.Texts))
	for i, t := range in.Texts {
		sx, _ := sides(t.Rot) // кегль — по min(kx, ky), не по осям текста
		ux, uy := Rotate(t.Rot, t.Al*t.W, 0)
		ax, ay := tr(t.X+ux, t.Y+uy)
		size := math.Min(math.Max(math.Round(t.Size*math.Min(kx, ky)*10)/10, text.MinSize), text.MaxSize)
		w := t.W * sx
		ux, uy = Rotate(t.Rot, t.Al*w, 0)
		txs[i] = GxText{X: ax - ux, Y: ay - uy, W: w, H: t.H * size / t.Size, Rot: t.Rot, Size: size, Al: t.Al}
	}
	return pls, txs
}
