package gen

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/math/f64"
	"golang.org/x/image/vector"
)

// stripPx — предел маски растеризатора (px): фигура во всю обложку (волна, полоса) растрируется
// полосами по строкам, а не одной маской 33 Мп × 8 байт.
var stripPx = 1 << 20 // var: тест сверяет полосы с одной маской

// softK — px на единицу координат мягкого слоя (акварель, мазки; design §4 п. 4): при 300 dpi
// (11,8 px/мм) — около ¼, на экране и в плитке — как есть.
const softK = 3.0

type pt struct{ x, y float64 }

// canvas — куда рисует генератор: холст, обрезка и перевод его координат (узор — мм от угла
// области, стикер — доли рамки) в px. Фигура копится путём и кладётся fill — по своей рамке,
// draw.Over однотонным цветом (design §4 п. 4).
type canvas struct {
	dst   *image.RGBA
	clip  image.Rectangle // область ∩ холст
	m     f64.Aff3        // координаты генератора → px холста
	w, h  float64         // размер области в координатах генератора
	alpha float64         // 1 − fade
	z     vector.Rasterizer
	ops   []pathOp
	box   [4]float32 // рамка пути в px: minX, minY, maxX, maxY
}

type pathOp struct {
	n   uint8 // opMove…opClose
	pts [3][2]float32
}

const (
	opMove = iota
	opLine
	opQuad
	opCube
	opClose
)

func (c *canvas) at(x, y float64) [2]float32 {
	m := &c.m
	return [2]float32{float32(m[0]*x + m[1]*y + m[2]), float32(m[3]*x + m[4]*y + m[5])}
}

// add — шаг пути; точки уже в px (at), рамка пути растёт.
func (c *canvas) add(op pathOp, n int) {
	for i := range n {
		q := op.pts[i]
		if len(c.ops) == 0 && i == 0 {
			c.box = [4]float32{q[0], q[1], q[0], q[1]}
		}
		c.box[0], c.box[1] = min(c.box[0], q[0]), min(c.box[1], q[1])
		c.box[2], c.box[3] = max(c.box[2], q[0]), max(c.box[3], q[1])
	}
	c.ops = append(c.ops, op)
}

func (c *canvas) moveTo(x, y float64) { c.add(pathOp{n: opMove, pts: [3][2]float32{c.at(x, y)}}, 1) }
func (c *canvas) lineTo(x, y float64) { c.add(pathOp{n: opLine, pts: [3][2]float32{c.at(x, y)}}, 1) }
func (c *canvas) closePath()          { c.ops = append(c.ops, pathOp{n: opClose}) }

func (c *canvas) cubeTo(x1, y1, x2, y2, x, y float64) {
	c.add(pathOp{n: opCube, pts: [3][2]float32{c.at(x1, y1), c.at(x2, y2), c.at(x, y)}}, 3)
}

// fill кладёт накопленный путь цветом ink с непрозрачностью a·(1 − fade) и очищает путь.
// Перекрытия подпутей одного направления сливаются (покрытие зажато в 1): штрих из кусков
// не темнеет на стыках.
func (c *canvas) fill(ink color.NRGBA, a float64) {
	ops := c.ops
	c.ops = c.ops[:0]
	if len(ops) == 0 {
		return
	}
	r := image.Rect(int(math.Floor(float64(c.box[0]))), int(math.Floor(float64(c.box[1]))),
		int(math.Ceil(float64(c.box[2]))), int(math.Ceil(float64(c.box[3])))).Intersect(c.clip)
	alpha := math.Round(float64(ink.A) * min(max(a*c.alpha, 0), 1))
	if r.Empty() || alpha <= 0 {
		return
	}
	ink.A = uint8(alpha)
	src := image.Uniform{C: ink}
	rows := max(1, stripPx/r.Dx())
	for y0 := r.Min.Y; y0 < r.Max.Y; y0 += rows {
		y1 := min(y0+rows, r.Max.Y)
		c.z.Reset(r.Dx(), y1-y0)
		ox, oy := float32(r.Min.X), float32(y0)
		for _, op := range ops {
			p := op.pts
			switch op.n {
			case opMove:
				c.z.MoveTo(p[0][0]-ox, p[0][1]-oy)
			case opLine:
				c.z.LineTo(p[0][0]-ox, p[0][1]-oy)
			case opQuad:
				c.z.QuadTo(p[0][0]-ox, p[0][1]-oy, p[1][0]-ox, p[1][1]-oy)
			case opCube:
				c.z.CubeTo(p[0][0]-ox, p[0][1]-oy, p[1][0]-ox, p[1][1]-oy, p[2][0]-ox, p[2][1]-oy)
			case opClose:
				c.z.ClosePath()
			}
		}
		c.z.Draw(c.dst, image.Rect(r.Min.X, y0, r.Max.X, y1), &src, image.Point{})
	}
}

// visible — круг (x, y, r) в координатах генератора задевает обрезку; при повороте (стикер) —
// всегда: там фигур единицы. Отсев до построения пути: ГСЧ уже прочитан, рисунок не меняется.
func (c *canvas) visible(x, y, r float64) bool {
	m := &c.m
	if m[1] != 0 || m[3] != 0 {
		return true
	}
	kx, ky := math.Abs(m[0]), math.Abs(m[4])
	px, py := m[0]*x+m[2], m[4]*y+m[5]
	return px+r*kx >= float64(c.clip.Min.X) && px-r*kx <= float64(c.clip.Max.X) &&
		py+r*ky >= float64(c.clip.Min.Y) && py-r*ky <= float64(c.clip.Max.Y)
}

// circle — окружность четырьмя Безье, угол растёт (то же направление, что у polyPos).
func (c *canvas) circle(cx, cy, r float64) { c.ellipse(cx, cy, r, r, 0) }

// ellipse — эллипс с полуосями rx, ry, повёрнутый на rot (рад).
func (c *canvas) ellipse(cx, cy, rx, ry, rot float64) {
	const h = 0.5522847498307936 // плечо Безье дуги 90°
	s, k := math.Sincos(rot)
	at := func(u, v float64) (float64, float64) { return cx + u*k - v*s, cy + u*s + v*k }
	c.moveTo(at(rx, 0))
	pts := [4][3][2]float64{
		{{rx, h * ry}, {h * rx, ry}, {0, ry}},
		{{-h * rx, ry}, {-rx, h * ry}, {-rx, 0}},
		{{-rx, -h * ry}, {-h * rx, -ry}, {0, -ry}},
		{{h * rx, -ry}, {rx, -h * ry}, {rx, 0}},
	}
	for _, q := range pts {
		x1, y1 := at(q[0][0], q[0][1])
		x2, y2 := at(q[1][0], q[1][1])
		x3, y3 := at(q[2][0], q[2][1])
		c.cubeTo(x1, y1, x2, y2, x3, y3)
	}
	c.closePath()
}

// poly — замкнутый многоугольник как есть.
func (c *canvas) poly(ps []pt) {
	if len(ps) < 3 {
		return
	}
	c.moveTo(ps[0].x, ps[0].y)
	for _, p := range ps[1:] {
		c.lineTo(p.x, p.y)
	}
	c.closePath()
}

// polyPos — многоугольник в направлении окружностей: куски штриха и фигуры в одном пути
// сливаются, а не вычитаются.
func (c *canvas) polyPos(ps []pt) {
	var a float64
	for i, p := range ps {
		q := ps[(i+1)%len(ps)]
		a += p.x*q.y - q.x*p.y
	}
	if a >= 0 {
		c.poly(ps)
		return
	}
	rev := make([]pt, len(ps))
	for i, p := range ps {
		rev[len(ps)-1-i] = p
	}
	c.poly(rev)
}

// rotRect — прямоугольник w×h с центром (cx, cy), повёрнутый на rot (рад).
func (c *canvas) rotRect(cx, cy, w, h, rot float64) {
	s, k := math.Sincos(rot)
	hw, hh := w/2, h/2
	ps := make([]pt, 4)
	for i, d := range [4]pt{{-hw, -hh}, {hw, -hh}, {hw, hh}, {-hw, hh}} {
		ps[i] = pt{cx + d.x*k - d.y*s, cy + d.x*s + d.y*k}
	}
	c.polyPos(ps)
}

// stroke — линия ширины wd по точкам: прямоугольник на отрезок, круг на излом круче 20° и на
// концах (скругление; у линии тоньше пикселя — без них); x/image/vector обводку не умеет
// (research §2).
func (c *canvas) stroke(ps []pt, closed bool, wd float64) {
	n := len(ps)
	if n < 2 {
		return
	}
	segs := n - 1
	if closed {
		segs = n
	}
	for i := range segs {
		a, b := ps[i], ps[(i+1)%n]
		dx, dy := b.x-a.x, b.y-a.y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		// Обход a−n → b−n → b+n → a+n — то же направление, что у окружностей (площадь > 0).
		nx, ny := -dy/l*wd/2, dx/l*wd/2
		c.moveTo(a.x-nx, a.y-ny)
		c.lineTo(b.x-nx, b.y-ny)
		c.lineTo(b.x+nx, b.y+ny)
		c.lineTo(a.x+nx, a.y+ny)
		c.closePath()
	}
	if wd*math.Hypot(c.m[0], c.m[3]) < 1 {
		return // линия тоньше пикселя (плитка каталога): скругления не видны, а путь вдвое длиннее
	}
	cos20 := math.Cos(20 * math.Pi / 180)
	for i, p := range ps {
		end := !closed && (i == 0 || i == n-1)
		if end || bendCos(ps[(i+n-1)%n], p, ps[(i+1)%n]) < cos20 {
			c.circle(p.x, p.y, wd/2)
		}
	}
}

// bendCos — косинус поворота пути в точке b (1 — прямо).
func bendCos(a, b, d pt) float64 {
	ux, uy, vx, vy := b.x-a.x, b.y-a.y, d.x-b.x, d.y-b.y
	l := math.Hypot(ux, uy) * math.Hypot(vx, vy)
	if l == 0 {
		return 1
	}
	return (ux*vx + uy*vy) / l
}

// soft рисует fn во временный холст разрешения softK и растягивает его на область
// билинейно (upscale); при мелком масштабе (экран, плитка) — прямо в холст.
func (c *canvas) soft(fn func(sc *canvas)) { c.softAt(softK, fn) }

// softAt — soft с разрешением слоя kq px на единицу координат.
func (c *canvas) softAt(kq float64, fn func(sc *canvas)) {
	k := math.Hypot(c.m[0], c.m[3])
	if k <= kq || c.clip.Empty() {
		fn(c)
		return
	}
	cw, ch := c.clip.Dx(), c.clip.Dy()
	qw := max(1, int(math.Ceil(float64(cw)*kq/k)))
	qh := max(1, int(math.Ceil(float64(ch)*kq/k)))
	sx, sy := float64(qw)/float64(cw), float64(qh)/float64(ch)
	q := image.NewRGBA(image.Rect(0, 0, qw, qh))
	ox, oy := float64(c.clip.Min.X), float64(c.clip.Min.Y)
	sc := &canvas{dst: q, clip: q.Rect, w: c.w, h: c.h, alpha: 1, m: f64.Aff3{
		c.m[0] * sx, c.m[1] * sx, (c.m[2] - ox) * sx,
		c.m[3] * sy, c.m[4] * sy, (c.m[5] - oy) * sy,
	}}
	fn(sc)
	c.upscale(q)
}

// upscale кладёт слой q (умноженный RGBA) на обрезку билинейно, draw.Over, с альфой 1 − fade.
// Свой цикл, а не x/image/draw: тот на обложке 300 dpi — ~1 с, здесь строка слоя смешивается
// по вертикали один раз, а пустые строки пропускаются.
func (c *canvas) upscale(q *image.RGBA) {
	cw, ch := c.clip.Dx(), c.clip.Dy()
	qw, qh := q.Rect.Dx(), q.Rect.Dy()
	tap := func(d, n, qn int) (int, int, uint32) {
		u := max((float64(d)+0.5)*float64(qn)/float64(n)-0.5, 0)
		i := int(u)
		if i >= qn-1 {
			return qn - 1, qn - 1, 0
		}
		return i, i + 1, uint32(math.Round((u - float64(i)) * 256))
	}
	type col struct {
		i0, i1 int // смещения соседних пикселей слоя в строке
		f      uint32
	}
	xs := make([]col, cw)
	for x := range xs {
		i0, i1, f := tap(x, cw, qw)
		xs[x] = col{4 * i0, 4 * i1, f}
	}
	a := uint32(math.Round(c.alpha * 256))
	row := make([]uint32, 4*qw)
	for y := range ch {
		j0, j1, fy := tap(y, ch, qh)
		r0, r1 := q.Pix[j0*q.Stride:j0*q.Stride+4*qw], q.Pix[j1*q.Stride:j1*q.Stride+4*qw]
		empty := true
		for i := range row {
			row[i] = uint32(r0[i])*(256-fy) + uint32(r1[i])*fy
			empty = empty && row[i] == 0
		}
		if empty {
			continue
		}
		off := c.dst.PixOffset(c.clip.Min.X, c.clip.Min.Y+y)
		px := c.dst.Pix[off : off+4*cw]
		for x, t := range xs {
			var s [4]uint32
			for k := range 4 {
				v := (row[t.i0+k]*(256-t.f) + row[t.i1+k]*t.f + 1<<15) >> 16
				s[k] = (v*a + 128) >> 8
			}
			if s[3] == 0 && s[0]|s[1]|s[2] == 0 {
				continue
			}
			p := px[4*x : 4*x+4 : 4*x+4]
			na := 255 - s[3]
			for k := range 4 {
				p[k] = uint8(min(s[k]+(uint32(p[k])*na+127)/255, 255))
			}
		}
	}
}

// overAlpha кладёт слой (умноженный RGBA, координаты холста) на dst draw.Over с альфой a:
// d = s·a + d·(1 − s.a·a). Свой цикл: draw.DrawMask с Uniform-маской идёт медленным общим путём,
// а пустые пиксели слоя (большая часть узора) здесь пропускаются.
func overAlpha(dst, layer *image.RGBA, a float64) {
	r := layer.Rect.Intersect(dst.Rect)
	k := uint32(math.Round(min(max(a, 0), 1) * 255))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		so, do := layer.PixOffset(r.Min.X, y), dst.PixOffset(r.Min.X, y)
		src, px := layer.Pix[so:so+4*r.Dx()], dst.Pix[do:do+4*r.Dx()]
		for i := 0; i < len(src); i += 4 {
			sp := src[i : i+4 : i+4]
			if sp[0]|sp[1]|sp[2]|sp[3] == 0 {
				continue
			}
			var s [4]uint32
			for ch := range 4 {
				s[ch] = (uint32(sp[ch])*k + 127) / 255
			}
			p := px[i : i+4 : i+4]
			na := 255 - s[3]
			for ch := range 4 {
				p[ch] = uint8(min(s[ch]+(uint32(p[ch])*na+127)/255, 255))
			}
		}
	}
}

// rgb — цвет #rrggbb (после Resolve — всегда правильный).
func rgb(s string) color.NRGBA {
	h := func(i int) uint8 {
		v := s[i]
		switch {
		case v >= 'a':
			return v - 'a' + 10
		default:
			return v - '0'
		}
	}
	if len(s) != 7 {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{h(1)<<4 | h(2), h(3)<<4 | h(4), h(5)<<4 | h(6), 255}
}
