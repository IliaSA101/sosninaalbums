package geom

import "math"

// Поворот группы (спека 018, design §3.2): каждый член поворачивается вокруг общего
// центра C, группа жёсткая. Угол не упирается (решение владельца 2026-10-09): что
// вылезло, обрезается краем, как у одиночного повёрнутого места. Зеркало — frontend
// geom/grouprot.ts, паритет testdata/parity/grouprot.json.

// GxRotIn — вход GroupRot: размер разворота, члены группы, угол (по часовой) и центр.
type GxRotIn struct {
	SW     float64   `json:"sw"`
	PH     float64   `json:"ph"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
	Deg    float64   `json:"deg"`
	CX     float64   `json:"cx"`
	CY     float64   `json:"cy"`
}

// GxRotOut — итог: угол (NormDeg входа), общий сдвиг внутрь, места и тексты
// (шаг 0.01, каждый уже зажат, как его зажмут clampPlace и fitTexts).
type GxRotOut struct {
	Deg    float64   `json:"deg"`
	DX     float64   `json:"dx"`
	DY     float64   `json:"dy"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
}

// GroupRot поворачивает группу на Deg вокруг (CX, CY): у места центр идёт по дуге,
// у текста — угол рамки P (текст повёрнут вокруг P, поэтому блок едет жёстко).
// Потом группа целиком сдвигается внутрь разворота по hold мест и aabb текстов;
// не помещается — встаёт по центру вылета, и крайние поджимаются по одному.
func GroupRot(in GxRotIn) GxRotOut {
	out := GxRotOut{Places: []RotRect{}, Texts: []GxText{}} // не nil: в JSON [] для UI
	th := NormDeg(in.Deg)
	if th == 0 {
		out.Places = append(out.Places, in.Places...)
		out.Texts = append(out.Texts, in.Texts...)
		return out
	}
	out.Deg = th
	turn := func(x, y float64) (float64, float64) {
		dx, dy := Rotate(th, x-in.CX, y-in.CY)
		return in.CX + dx, in.CY + dy
	}
	pls := make([]RotRect, len(in.Places))
	for i, r := range in.Places {
		cx, cy := turn(r.X+r.W/2, r.Y+r.H/2)
		pls[i] = RotRect{X: cx - r.W/2, Y: cy - r.H/2, W: r.W, H: r.H, Rot: NormDeg(r.Rot + th)}
	}
	txs := make([]GxText, len(in.Texts))
	for i, t := range in.Texts {
		txs[i] = t
		txs[i].X, txs[i].Y = turn(t.X, t.Y)
		txs[i].Rot = NormDeg(t.Rot + th)
	}
	if x0, y0, x1, y1, ok := groupBound(pls, txs); ok {
		out.DX, out.DY = shiftIn(x0, x1, in.SW), shiftIn(y0, y1, in.PH)
	}
	for _, r := range pls {
		r.X, r.Y = r.X+out.DX, r.Y+out.DY
		ClampPlaceIn(&r, in.SW, in.PH)
		out.Places = append(out.Places, r)
	}
	for _, t := range txs {
		b := TextBox{Round2(t.X + out.DX), Round2(t.Y + out.DY), t.W, t.H, t.Rot}
		t.X, t.Y = b.FitIn(in.SW, in.PH)
		out.Texts = append(out.Texts, t)
	}
	return out
}

// groupBound — описанный прямоугольник группы для зажима: hold мест и aabb текстов.
func groupBound(pls []RotRect, txs []GxText) (x0, y0, x1, y1 float64, ok bool) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(x, y, w, h float64) {
		x0, y0 = math.Min(x0, x), math.Min(y0, y)
		x1, y1 = math.Max(x1, x+w), math.Max(y1, y+h)
	}
	for _, r := range pls {
		h := r.Hold()
		add(h.X, h.Y, h.W, h.H)
	}
	for _, t := range txs {
		a := TextBox{t.X, t.Y, t.W, t.H, t.Rot}.AABB()
		add(a.X, a.Y, a.W, a.H)
	}
	return x0, y0, x1, y1, len(pls)+len(txs) > 0
}

// shiftIn — сдвиг по оси, при котором [v0, v1] внутри [0, side]: помещается — наименьший,
// нет — вылет поровну с двух сторон (сдвиг непрерывен, рамка не прыгает при жесте).
func shiftIn(v0, v1, side float64) float64 {
	lo, hi := -v0, side-v1
	if lo > hi {
		return (lo + hi) / 2
	}
	return ClampF(0, lo, hi)
}
