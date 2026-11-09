package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Crop — кадр фото в рамке: значения model.Frame (S, X, Y, R), теги JSON те же
// (векторы testdata/parity/frames.json).
type Crop struct {
	S float64 `json:"s"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	R float64 `json:"r,omitempty"`
}

// ContainImage — фото целиком внутри места по центру (Fit = contain). С r≠0 фото
// повёрнуто на r вокруг центра и целиком в ячейке (Img — в осях фото).
func ContainImage(cell layout.Rect, aspect, r float64) layout.Rect {
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	var w, h float64
	if r == 0 {
		w, h = cell.W, cell.W/aspect
		if h > cell.H {
			h, w = cell.H, cell.H*aspect
		}
	} else {
		sn, cs := sinCosDeg(r)
		s, c := math.Abs(sn), math.Abs(cs)
		w = math.Min(cell.W/(c+s/aspect), cell.H/(s+c/aspect))
		h = w / aspect
	}
	return layout.Rect{X: cell.X + (cell.W-w)/2, Y: cell.Y + (cell.H-h)/2, W: w, H: h}
}

// PlaceImage — фото целиком (мм) для ячейки cell: «закрыть ячейку» × S, сдвиг
// X/Y — доля свободного хода. С R≠0 Img — прямоугольник фото в его осях, на экране
// он повёрнут на R вокруг своего центра и закрывает ячейку (design-rotation §2.2).
// Зеркало — frontend geom/frame.ts, паритет держит testdata/parity/frames.json.
func PlaceImage(cell layout.Rect, aspect float64, f Crop) layout.Rect {
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	if f.R != 0 {
		return placeImageRot(cell, aspect, f)
	}
	// Без поворота — прежняя формула дословно: иной порядок сложений меняет
	// последний бит, и px() старых альбомов сдвинулся бы на пиксель.
	w, h := cell.W, cell.W/aspect
	if h < cell.H {
		h, w = cell.H, cell.H*aspect
	}
	w, h = w*f.S, h*f.S
	fx, fy := w-cell.W, h-cell.H // ≥ 0
	return layout.Rect{X: cell.X - fx*(1+f.X)/2, Y: cell.Y - fy*(1+f.Y)/2, W: w, H: h}
}

// sinCosDeg — синус и косинус угла в градусах; кратные 90° точные (без хвоста 1e-16).
func sinCosDeg(deg float64) (s, c float64) {
	if math.Mod(deg, 90) == 0 {
		switch ((int(deg/90) % 4) + 4) % 4 {
		case 0:
			return 0, 1
		case 1:
			return 1, 0
		case 2:
			return 0, -1
		default:
			return -1, 0
		}
	}
	return math.Sincos(deg * math.Pi / 180)
}

// placeImageRot — PlaceImage при R≠0. Фото закрывает повёрнутую рамку ⇔ описанный
// прямоугольник рамки в осях фото (W', H') лежит внутри фото; ход сдвига задан в
// осях фото, поэтому остаётся прямоугольником.
func placeImageRot(cell layout.Rect, aspect float64, f Crop) layout.Rect {
	return placeImageRotIn(cell, aspect, f, Outline{})
}

// placeImageRotIn — placeImageRot для формы o (044 design §2.1): форма выпукла, поэтому
// она внутри фото ⇔ её габарит в осях фото внутри; меняется только габарит.
func placeImageRotIn(cell layout.Rect, aspect float64, f Crop, o Outline) layout.Rect {
	sn, cs := sinCosDeg(f.R)
	s, c := math.Abs(sn), math.Abs(cs)
	bw, bh := o.bound(cell.W, cell.H, c, s)
	w, h := bw, bw/aspect
	if h < bh {
		h, w = bh, bh*aspect
	}
	w, h = w*f.S, h*f.S
	fx, fy := w-bw, h-bh // ≥ 0
	ux, uy := -fx*f.X/2, -fy*f.Y/2
	ox, oy := ux*cs-uy*sn, ux*sn+uy*cs
	cx, cy := cell.X+cell.W/2, cell.Y+cell.H/2
	return layout.Rect{X: cx + ox - w/2, Y: cy + oy - h/2, W: w, H: h}
}

// Outline — форма места для расчётов (спека 044): эллипс, вписанный в рамку, или
// прямоугольник со скруглением Radius мм. Нулевая — прямоугольник. Теги JSON — векторы
// testdata/parity/frames.json (поле "outline").
type Outline struct {
	Oval   bool    `json:"oval,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

// radius — радиус скругления в пределах рамки w×h: не больше половины меньшей стороны.
func (o Outline) radius(w, h float64) float64 {
	if o.Oval || !(o.Radius > 0) {
		return 0
	}
	return min(o.Radius, min(w, h)/2)
}

// bound — габарит формы рамки w×h в осях фото, повёрнутого на θ (c = |cos θ|, s = |sin θ|),
// 044 design §2.1. При θ = 0 у всех форм — w×h.
func (o Outline) bound(w, h, c, s float64) (bw, bh float64) {
	if o.Oval {
		a, b := w/2, h/2
		return 2 * math.Sqrt(a*a*c*c+b*b*s*s), 2 * math.Sqrt(a*a*s*s+b*b*c*c)
	}
	if r := o.radius(w, h); r > 0 {
		iw, ih := w-2*r, h-2*r
		return iw*c + ih*s + 2*r, iw*s + ih*c + 2*r
	}
	return w*c + h*s, w*s + h*c
}

// contains — точка (x, y) от центра рамки w×h внутри формы (с допуском eps мм).
func (o Outline) contains(w, h, x, y, eps float64) bool {
	x, y = math.Abs(x), math.Abs(y)
	if o.Oval {
		a, b := w/2+eps, h/2+eps
		return x*x/(a*a)+y*y/(b*b) <= 1
	}
	if x > w/2+eps || y > h/2+eps {
		return false
	}
	r := o.radius(w, h)
	dx, dy := x-(w/2-r), y-(h/2-r)
	return dx <= 0 || dy <= 0 || dx*dx+dy*dy <= (r+eps)*(r+eps)
}

// PlaceImageIn — PlaceImage для места формы o: фото закрывает форму, а не рамку. Без
// наклона габарит любой формы — рамка, поэтому R = 0 — PlaceImage побитно.
func PlaceImageIn(cell layout.Rect, aspect float64, f Crop, o Outline) layout.Rect {
	if f.R == 0 || o == (Outline{}) {
		return PlaceImage(cell, aspect, f)
	}
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	return placeImageRotIn(cell, aspect, f, o)
}

// ContainImageIn — ContainImage для формы o: фото целиком внутри формы, по центру
// (044 design §2.2). Эллипс — наибольший прямоугольник пропорций aspect под углом r;
// скругление — прямоугольный ответ, уменьшенный, пока углы фото не войдут в форму.
func ContainImageIn(cell layout.Rect, aspect, r float64, o Outline) layout.Rect {
	img := ContainImage(cell, aspect, r)
	if o == (Outline{}) {
		return img
	}
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	sn, cs := sinCosDeg(r)
	var w float64
	if o.Oval {
		a2, b2 := cell.W*cell.W/4, cell.H*cell.H/4
		q := 0.0
		for _, sg := range [2]float64{1, -1} {
			x := cs/2 - sg*sn/(2*aspect)
			y := sn/2 + sg*cs/(2*aspect)
			q = max(q, x*x/a2+y*y/b2)
		}
		w = 1 / math.Sqrt(q)
	} else {
		// Форма выпукла и содержит центр: «углы внутри» монотонно по масштабу t.
		in := func(t float64) bool {
			for _, uv := range [2][2]float64{{1, 1}, {1, -1}} {
				x, y := rotXY(sn, cs, uv[0]*img.W*t/2, uv[1]*img.H*t/2)
				if !o.contains(cell.W, cell.H, x, y, 1e-9) {
					return false
				}
			}
			return true
		}
		if in(1) {
			return img
		}
		lo, hi := 0.0, 1.0
		for range 60 {
			if m := (lo + hi) / 2; in(m) {
				lo = m
			} else {
				hi = m
			}
		}
		w = img.W * lo
	}
	h := w / aspect
	return layout.Rect{X: cell.X + (cell.W-w)/2, Y: cell.Y + (cell.H-h)/2, W: w, H: h}
}

// rotXY — точка (x, y), повёрнутая по часовой (y вниз) по готовым sin и cos.
func rotXY(sn, cs, x, y float64) (float64, float64) {
	return x*cs - y*sn, x*sn + y*cs
}

// InnerBox — прямоугольник внутри формы для подписи «на фото» (044 design §2.3): эллипс —
// W/√2 × H/√2 по центру (углы на эллипсе), скругление — рамка, сжатая на ρ·(1 − 1/√2).
func InnerBox(cell layout.Rect, o Outline) layout.Rect {
	var dx, dy float64
	switch {
	case o.Oval:
		dx, dy = cell.W*(1-math.Sqrt2/2)/2, cell.H*(1-math.Sqrt2/2)/2
	case o.radius(cell.W, cell.H) > 0:
		dx = o.radius(cell.W, cell.H) * (1 - math.Sqrt2/2)
		dy = dx
	default:
		return cell
	}
	return layout.Rect{X: cell.X + dx, Y: cell.Y + dy, W: cell.W - 2*dx, H: cell.H - 2*dy}
}

// FrameDPI — эффективное разрешение фото шириной px пикселей, растянутого на
// img.W мм; 0 — размер фото неизвестен.
func FrameDPI(px int, img layout.Rect) int {
	if px <= 0 || !(img.W > 0) {
		return 0
	}
	return int(math.Round(float64(px) / (img.W / 25.4)))
}

// StraightenDelta — на сколько градусов (по часовой) повернуть фото, чтобы линия (dx, dy)
// мм листа (y вниз) встала параллельно краю рамки, повёрнутой на frameRot (спека 045,
// design §2.2). Направление линии не важно; круче 45° — выпрямляется вертикаль, ровно 45° —
// горизонталь. |delta| ≤ 45. ok = false — линии нет (короче 0,01 мм) или не число.
func StraightenDelta(dx, dy, frameRot float64) (delta float64, ok bool) {
	for _, v := range [...]float64{dx, dy, frameRot} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
	}
	if !(math.Hypot(dx, dy) >= 0.01) {
		return 0, false
	}
	t := math.Mod(math.Atan2(dy, dx)*180/math.Pi-frameRot, 180)
	// Хвост 1e-14 от atan2 не должен решать, горизонталь это или вертикаль при ровно 45°.
	t = math.Round(t*1e9) / 1e9
	switch {
	case t > 90:
		t -= 180
	case t <= -90:
		t += 180
	}
	switch {
	case t > 45:
		t -= 90
	case t < -45:
		t += 90
	}
	if t == 0 {
		return 0, true // без «-0»
	}
	return -t, true
}
