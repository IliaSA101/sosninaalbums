package gen

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Встроенные рамочки мест (079 design §3): рисунок вида по мм места. Правила 048 №90 те же, что у
// узоров: геометрия в мм, случайности нет, выпущенный вид не меняется (эталон TestFrameGolden,
// в том числе с Width = 0), правка — новый ключ.

// FrameSpec — рамочка одного места: вид, цвета, толщина и форма места, мм.
type FrameSpec struct {
	Kind   string
	Inks   []string // #rrggbb; короче цветов вида — дополняется цветами вида
	Width  float64  // мм; 0 — по умолчанию вида
	W, H   float64  // размер места, мм
	Oval   bool
	Radius float64 // скругление прямоугольника, мм; у овала не используется
}

// maxFrameMM — предел стороны места: разворот и обложка — до 2 м; мусор не выделяет память.
const maxFrameMM = 5000.0

// frameSeam — на сколько внутренний край полосы заходит в окно фото, мм: край фото не
// просвечивает цветом страницы при сглаживании (TestFrameNoSeam).
const frameSeam = 0.15

// ClampFrame — спецификация после зажима: цвета и толщина — ClampFrameInks/ClampFrameWidth, у
// известного вида пустое достроено значениями вида (Width = 0 → толщина вида), размер места 0…5 м
// (NaN → 0, и тогда не рисуется), скругление 0…половина короткой стороны, у овала 0. Идемпотентна.
func ClampFrame(s FrameSpec) FrameSpec {
	s.Inks = ClampFrameInks(s.Kind, s.Inks)
	s.Width = ClampFrameWidth(s.Width)
	if m, ok := FrameLookup(s.Kind); ok {
		for i := len(s.Inks); i < len(m.Inks); i++ {
			s.Inks = append(s.Inks, m.Inks[i])
		}
		if s.Width == 0 {
			s.Width = m.Width
		}
	}
	s.W, s.H = frameSide(s.W), frameSide(s.H)
	if s.Oval || !(s.Radius > 0) || s.W == 0 || s.H == 0 {
		s.Radius = 0
	} else {
		s.Radius = min(s.Radius, s.W/2, s.H/2)
	}
	return s
}

func frameSide(v float64) float64 {
	if !(v > 0) { // и NaN
		return 0
	}
	return min(v, maxFrameMM)
}

// frame — рамочка для рисовальщика: зажатая спека, вид, цвета и отступы окна.
type frame struct {
	FrameSpec
	meta FrameMeta
	ink  []color.NRGBA
	ins  [4]float64 // отступы окна l, t, r, b, мм (0 — накладка)
}

func newFrame(s FrameSpec) (*frame, bool) {
	s = ClampFrame(s)
	m, ok := FrameLookup(s.Kind)
	if !ok || s.W == 0 || s.H == 0 {
		return nil, false
	}
	f := &frame{FrameSpec: s, meta: m, ink: make([]color.NRGBA, len(s.Inks))}
	for i, v := range s.Inks {
		f.ink[i] = paint.RGB(v)
	}
	for i, k := range m.Ins {
		f.ins[i] = k * s.Width
	}
	if s.Oval { // у овала все четыре равны наибольшему (model.BorderIns)
		e := max(f.ins[0], f.ins[1], f.ins[2], f.ins[3])
		f.ins = [4]float64{e, e, e, e}
	}
	f.ins = clampIns(f.ins, s.W, s.H)
	return f, true
}

// clampIns — то же, что geom.ClampInset (gen зависит только от stdlib и x/image, depguard):
// пара отступов оси не больше 0,7 стороны, иначе сжимается пропорционально. Совпадение держит
// TestFrameInsParity по testdata/parity/inset.json.
func clampIns(ins [4]float64, w, h float64) [4]float64 {
	var out [4]float64
	for i, v := range ins {
		if v > 0 && !math.IsInf(v, 1) {
			out[i] = v
		}
	}
	pair := func(a, b *float64, side float64) {
		lim := 0.7 * max(side, 0)
		if s := *a + *b; s > lim {
			k := lim / s
			*a, *b = *a*k, *b*k
		}
	}
	pair(&out[0], &out[2], w)
	pair(&out[1], &out[3], h)
	return out
}

// at — отступы на доле t толщины окна (0 — край места, 1 — край окна).
func (f *frame) at(t float64) [4]float64 {
	return [4]float64{f.ins[0] * t, f.ins[1] * t, f.ins[2] * t, f.ins[3] * t}
}

// seamed — отступы края окна плюс шов: полоса заходит под край фото.
func (f *frame) seamed() [4]float64 {
	o := f.ins
	for i := range o {
		o[i] += frameSeam
	}
	return o
}

// frameDraw — рисовальщики видов; Mask-виды (soft) в DrawFrame не участвуют.
var frameDraw = map[string]func(c *canvas, f *frame){
	"line": drawLine, "double": drawDouble, "mat": drawMat, "polaroid": drawLine,
	"stitch": drawStitch, "corners": drawCorners, "gold": drawGold, "scallop": drawScallop,
}

func finiteMatrix(m f64.Aff3) bool {
	for _, v := range m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return m[0]*m[4]-m[1]*m[3] != 0
}

// DrawFrame кладёт рамочку s на dst (draw.Over): m переводит мм места в px холста — сдвиг, масштаб,
// поворот; рисуется вектором, без пересэмплирования. Неизвестный вид, Mask-вид (soft: его рисует
// FrameMask), вырожденная матрица и пустое место — ничего.
func DrawFrame(dst *image.RGBA, m f64.Aff3, s FrameSpec) {
	if dst == nil || !finiteMatrix(m) {
		return
	}
	f, ok := newFrame(s)
	if !ok || f.meta.Mask {
		return
	}
	draw, ok := frameDraw[f.Kind]
	if !ok {
		return
	}
	if c := paint.New(dst, dst.Rect, m, f.W, f.H, 0); c != nil {
		draw(c, f)
	}
}

// FrameMask пишет в dst альфу окна Mask-вида (soft) места s: 0 вне формы и у края, линейно до 255
// на глубине толщины внутрь; m — как у DrawFrame. Весь dst перезаписывается. Другие виды и
// негодная матрица — dst не трогается.
func FrameMask(dst *image.Alpha, m f64.Aff3, s FrameSpec) {
	if dst == nil || !finiteMatrix(m) {
		return
	}
	f, ok := newFrame(s)
	if !ok || !f.meta.Mask {
		return
	}
	det := m[0]*m[4] - m[1]*m[3]
	sh := f.shape()
	b := dst.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := dst.Pix[dst.PixOffset(b.Min.X, y):]
		for x := b.Min.X; x < b.Max.X; x++ {
			dx, dy := float64(x)+0.5-m[2], float64(y)+0.5-m[5]
			u, v := (m[4]*dx-m[1]*dy)/det, (m[0]*dy-m[3]*dx)/det // точка в мм места
			row[x-b.Min.X] = uint8(math.Round(255 * min(max(sh.depth(u, v)/f.Width, 0), 1)))
		}
	}
}

// depth — расстояние от точки (u, v) до края формы внутрь, мм (< 0 — снаружи). У эллипса —
// приближение k0·(k0 − 1)/k1: точно у края, где оно и нужно (глубина в пределах толщины).
func (f fshape) depth(u, v float64) float64 {
	cx, cy := (f.x0+f.x1)/2, (f.y0+f.y1)/2
	hw, hh := (f.x1-f.x0)/2, (f.y1-f.y0)/2
	px, py := u-cx, v-cy
	if f.oval {
		k0 := math.Hypot(px/hw, py/hh)
		k1 := math.Hypot(px/(hw*hw), py/(hh*hh))
		if k1 == 0 {
			return min(hw, hh)
		}
		return -k0 * (k0 - 1) / k1
	}
	qx, qy := math.Abs(px)-(hw-f.r), math.Abs(py)-(hh-f.r)
	out := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - f.r
	return -out
}
