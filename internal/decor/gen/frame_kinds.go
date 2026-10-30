package gen

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Виды встроенных рамочек (079 design §3). Доли толщины считаются по отступам окна своей стороны
// (f.at), поэтому низ полароида шире, а у овала полоса ровная; внутренний край последней полосы
// заходит в окно на frameSeam.

// Постоянные рисунка заморожены вместе с эталоном (правка — новый ключ вида).
const (
	mattLineMM    = 0.4 // паспарту: тонкая линия у окна
	doubleOuter   = 0.4 // двойная линия: наружная полоса, доля толщины
	doubleGapEnd  = 0.7 // …просвет кончается здесь, дальше внутренняя полоса
	goldDarkFrom  = 1.0 / 3
	goldDarkTo    = 2.0 / 3
	stitchInset   = 2.5 // строчка: отступ внутрь, мм
	stitchDash    = 2.5 // штрих
	stitchStep    = 4.0 // шаг штрихов (целое число на контур)
	scallopPeriod = 1.2 // фестоны: длина волны, толщин
	scallopDepth  = 0.4 // …глубина выемки, толщин
	scallopRound  = 0.8 // …минимальное скругление угла, толщин
	scallopSteps  = 16  // точек на волну
	scallopMinN   = 6
	scallopMaxN   = 240
)

// fill кладёт полосу outer…inner цветом ink; inner = nil — сплошная фигура.
func (f *frame) fill(c *canvas, outer fshape, inner *fshape, ink int) {
	band(c, outer, inner)
	c.Fill(f.ink[ink], 1)
}

// inside — контур на отступах o; nil, если его не осталось.
func (f *frame) inside(o [4]float64) *fshape {
	g, ok := f.shape().off(o)
	if !ok {
		return nil
	}
	return &g
}

// drawLine — линия и полароид: одна полоса по контуру места до окна.
func drawLine(c *canvas, f *frame) {
	f.fill(c, f.shape(), f.inside(f.seamed()), 0)
}

// drawDouble — две полосы: наружная и внутренняя, между ними просвет (видно то, что под местом).
func drawDouble(c *canvas, f *frame) {
	f.fill(c, f.shape(), f.inside(f.at(doubleOuter)), 0)
	if out := f.inside(f.at(doubleGapEnd)); out != nil {
		f.fill(c, *out, f.inside(f.seamed()), 0)
	}
}

// drawMat — широкая полоса цвета 1 и тонкая линия цвета 2 у окна.
func drawMat(c *canvas, f *frame) {
	in := f.seamed()
	f.fill(c, f.shape(), f.inside(in), 0)
	var o [4]float64
	for i := range o {
		o[i] = max(f.ins[i]-mattLineMM, 0)
	}
	if out := f.inside(o); out != nil {
		f.fill(c, *out, f.inside(in), 1)
	}
}

// drawGold — полоса из трёх частей, светлая–тёмная–светлая (фаска): тёмная ложится на целую
// светлую, чтобы между частями не просвечивала страница.
func drawGold(c *canvas, f *frame) {
	f.fill(c, f.shape(), f.inside(f.seamed()), 0)
	if out := f.inside(f.at(goldDarkFrom)); out != nil {
		f.fill(c, *out, f.inside(f.at(goldDarkTo)), 1)
	}
}

// drawStitch — штрихи по контуру внутри места, поверх фото: целое число штрихов на периметр.
func drawStitch(c *canvas, f *frame) {
	sh, ok := f.shape().inset(stitchInset)
	if !ok {
		return
	}
	pl := sh.polyline()
	l := pl.length()
	if l < stitchDash {
		return
	}
	n := max(int(math.Round(l/stitchStep)), 1)
	step := l / float64(n)
	for i := range n {
		s := float64(i) * step
		c.Stroke(pl.piece(s, s+stitchDash), false, f.Width)
	}
	c.Fill(f.ink[0], 1)
}

// drawCorners — четыре треугольных уголка по углам рамки места (у овала — габарита).
func drawCorners(c *canvas, f *frame) {
	a := min(f.Width, f.W/2, f.H/2)
	w, h := f.W, f.H
	for _, t := range [4][3]paint.Pt{
		{{X: 0, Y: 0}, {X: a, Y: 0}, {X: 0, Y: a}},
		{{X: w, Y: 0}, {X: w, Y: a}, {X: w - a, Y: 0}},
		{{X: w, Y: h}, {X: w - a, Y: h}, {X: w, Y: h - a}},
		{{X: 0, Y: h}, {X: 0, Y: h - a}, {X: a, Y: h}},
	} {
		c.PolyPos(t[:])
	}
	c.Fill(f.ink[0], 1)
}

// drawScallop — полоса с волнистым наружным краем: выпуклые дуги, между ними острые выемки.
// Угол прямоугольника скругляется не меньше scallopRound·толщины: на остром волна рвётся.
func drawScallop(c *canvas, f *frame) {
	base := f.shape()
	if !base.oval {
		base.r = max(base.r, min(scallopRound*f.Width, base.x1/2, base.y1/2))
	}
	pl := base.polyline()
	n := min(max(int(math.Round(pl.length()/(scallopPeriod*f.Width))), scallopMinN), scallopMaxN)
	depth := scallopDepth * f.Width
	ps := make([]paint.Pt, 0, n*scallopSteps)
	for k := range n {
		for j := range scallopSteps {
			u := float64(j) / scallopSteps
			p := pl.at((float64(k) + u) * pl.length() / float64(n))
			d := depth * (1 - math.Sqrt(max(1-(2*u-1)*(2*u-1), 0)))
			ps = append(ps, paint.Pt{X: p.x - p.nx*d, Y: p.y - p.ny*d})
		}
	}
	c.PolyPos(ps)
	if in := f.inside(f.seamed()); in != nil {
		in.trace(c, true)
	}
	c.Fill(f.ink[0], 1)
}
