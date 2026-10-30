package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// GxText — блок текста для расчёта: угол рамки до поворота, ширина, высота по
// тексту (на входе), угол и кегль (пт). Al — доля выравнивания (AlignX): нужна
// только GroupAxes, у GroupXf и GroupRot — 0 (векторы groupxf.json прежние).
type GxText struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Rot  float64 `json:"rot"`
	Size float64 `json:"size"`
	Al   float64 `json:"al,omitempty"`
}

// GxIn — вход GroupXf: размер разворота, члены группы и преобразование.
type GxIn struct {
	SW     float64   `json:"sw"`
	PH     float64   `json:"ph"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
	K      float64   `json:"k"`
	AX     float64   `json:"ax"`
	AY     float64   `json:"ay"`
	DX     float64   `json:"dx"`
	DY     float64   `json:"dy"`
}

// GxOut — итог: действующие k и d после зажима, новые места и тексты (шаг 0.01,
// кегль 0.1). Ширина текста — W·k и для «по тексту»: её дальше пересчитает fitTexts.
type GxOut struct {
	K      float64   `json:"k"`
	DX     float64   `json:"dx"`
	DY     float64   `json:"dy"`
	Places []RotRect `json:"places"`
	Texts  []GxText  `json:"texts"`
}

// GroupXf считает перенос и масштаб группы. k зажимается: не больше, чем группа
// помещается в разворот и чем позволяет верхний предел кегля; не меньше, чем места
// остаются ≥ MinPlace и кегль ≥ нижнего предела (нижний сильнее верхнего). d
// сдвигается так, чтобы описанный прямоугольник группы после масштаба лежал внутри
// разворота; повёрнутое место входит в него своим Hold (тогда model.ClampPlace
// после группы его не сдвигает).
func GroupXf(in GxIn) GxOut {
	out := GxOut{K: in.K, DX: in.DX, DY: in.DY, Places: []RotRect{}, Texts: []GxText{}} // не nil: в JSON [] для UI
	var bb []layout.Rect
	for _, r := range in.Places {
		bb = append(bb, r.Hold())
	}
	for _, t := range in.Texts {
		bb = append(bb, TextBox{t.X, t.Y, t.W, t.H, t.Rot}.AABB())
	}
	if len(bb) == 0 {
		out.K, out.DX, out.DY = 1, 0, 0
		return out
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range bb {
		x0, y0 = math.Min(x0, r.X), math.Min(y0, r.Y)
		x1, y1 = math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
	}
	gw, gh := x1-x0, y1-y0
	k := in.K
	if gw > 0 {
		k = math.Min(k, in.SW/gw)
	}
	if gh > 0 {
		k = math.Min(k, in.PH/gh)
	}
	for _, t := range in.Texts {
		k = math.Min(k, text.MaxSize/t.Size)
	}
	for _, r := range in.Places {
		k = math.Max(k, MinPlace/math.Min(r.W, r.H))
	}
	for _, t := range in.Texts {
		k = math.Max(k, text.MinSize/t.Size)
	}
	nx, ny := in.AX+(x0-in.AX)*k, in.AY+(y0-in.AY)*k
	out.K = k
	out.DX = ClampF(in.DX, -nx, in.SW-gw*k-nx)
	out.DY = ClampF(in.DY, -ny, in.PH-gh*k-ny)
	mx := func(v float64) float64 { return Round2(in.AX + (v-in.AX)*k + out.DX) }
	my := func(v float64) float64 { return Round2(in.AY + (v-in.AY)*k + out.DY) }
	for _, r := range in.Places {
		out.Places = append(out.Places, RotRect{X: mx(r.X), Y: my(r.Y), W: Round2(r.W * k), H: Round2(r.H * k), Rot: r.Rot})
	}
	for _, t := range in.Texts {
		size := math.Min(math.Max(math.Round(t.Size*k*10)/10, text.MinSize), text.MaxSize)
		out.Texts = append(out.Texts, GxText{X: mx(t.X), Y: my(t.Y), W: Round2(t.W * k), H: t.H * k, Rot: t.Rot, Size: size})
	}
	return out
}
