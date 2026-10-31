package gen

import (
	"math"
	"sort"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Контуры рамочек (079 design §3): прямоугольник, скругление, эллипс в мм места. Обход — как у
// окружностей paint (площадь > 0); обратный обход даёт дырку (vector — ненулевое правило).

const kappa = 0.5522847498307936 // плечо Безье дуги 90°

// fshape — контур: рамка x0,y0–x1,y1, скругление r (мм) или эллипс, вписанный в рамку.
type fshape struct {
	x0, y0, x1, y1, r float64
	oval              bool
}

// shape — само место.
func (s FrameSpec) shape() fshape {
	return fshape{0, 0, s.W, s.H, s.Radius, s.Oval}
}

// off — контур, сдвинутый внутрь на o = l, t, r, b (мм); скругление max(r − min(o), 0), как у
// окна фото (geom.InsetBox). false — контура не осталось.
func (f fshape) off(o [4]float64) (fshape, bool) {
	g := fshape{f.x0 + o[0], f.y0 + o[1], f.x1 - o[2], f.y1 - o[3], 0, f.oval}
	if g.x1 <= g.x0 || g.y1 <= g.y0 {
		return g, false
	}
	if !f.oval {
		g.r = min(max(f.r-min(o[0], o[1], o[2], o[3]), 0), (g.x1-g.x0)/2, (g.y1-g.y0)/2)
	}
	return g, true
}

// inset — контур на расстоянии d внутрь по всем сторонам.
func (f fshape) inset(d float64) (fshape, bool) { return f.off([4]float64{d, d, d, d}) }

// trace кладёт контур в путь холста; rev — обратным обходом (дырка).
func (f fshape) trace(c *canvas, rev bool) {
	if f.oval {
		ry := (f.y1 - f.y0) / 2
		if rev {
			ry = -ry // зеркало по y разворачивает обход
		}
		c.Ellipse((f.x0+f.x1)/2, (f.y0+f.y1)/2, (f.x1-f.x0)/2, ry, 0)
		return
	}
	r := f.r
	// corner — дуга угла с вершиной (vx, vy): от точки на стороне d0 до точки на стороне d3.
	corner := func(vx, vy, d0x, d0y, d3x, d3y float64) {
		if r <= 0 {
			return
		}
		k := (1 - kappa) * r
		c.CubeTo(vx+k*d0x, vy+k*d0y, vx+k*d3x, vy+k*d3y, vx+r*d3x, vy+r*d3y)
	}
	c.MoveTo(f.x0+r, f.y0)
	if !rev {
		c.LineTo(f.x1-r, f.y0)
		corner(f.x1, f.y0, -1, 0, 0, 1)
		c.LineTo(f.x1, f.y1-r)
		corner(f.x1, f.y1, 0, -1, -1, 0)
		c.LineTo(f.x0+r, f.y1)
		corner(f.x0, f.y1, 1, 0, 0, -1)
		c.LineTo(f.x0, f.y0+r)
		corner(f.x0, f.y0, 0, 1, 1, 0)
	} else {
		corner(f.x0, f.y0, 1, 0, 0, 1)
		c.LineTo(f.x0, f.y1-r)
		corner(f.x0, f.y1, 0, -1, 1, 0)
		c.LineTo(f.x1-r, f.y1)
		corner(f.x1, f.y1, -1, 0, 0, -1)
		c.LineTo(f.x1, f.y0+r)
		corner(f.x1, f.y0, 0, 1, -1, 0)
	}
	c.ClosePath()
}

// band кладёт полосу между контурами outer и inner (дырка); inner = nil — сплошная фигура.
func band(c *canvas, outer fshape, inner *fshape) {
	outer.trace(c, false)
	if inner != nil {
		inner.trace(c, true)
	}
}

// cpt — точка контура с внешней нормалью.
type cpt struct{ x, y, nx, ny float64 }

// polyline — замкнутый контур, размеченный длиной дуги; обход тот же, что у trace.
type polyline struct {
	pts []cpt
	cum []float64 // cum[i] — длина до pts[i]; последняя запись — периметр (возврат в pts[0])
}

func (pl *polyline) length() float64 { return pl.cum[len(pl.cum)-1] }

// sampleStep — шаг точек по кривой, мм: ошибка хорды при радиусе ≥ 1 мм — тысячные доли мм.
const sampleStep = 0.2

// polyline — контур точками; прямые стороны — концами, дуги и эллипс — с шагом sampleStep.
func (f fshape) polyline() polyline {
	var pts []cpt
	add := func(x, y, nx, ny float64) { pts = append(pts, cpt{x, y, nx, ny}) }
	if f.oval {
		cx, cy, rx, ry := (f.x0+f.x1)/2, (f.y0+f.y1)/2, (f.x1-f.x0)/2, (f.y1-f.y0)/2
		per := math.Pi * (3*(rx+ry) - math.Sqrt((3*rx+ry)*(rx+3*ry))) // Рамануджан
		n := min(max(int(math.Ceil(per/sampleStep)), 64), 4096)
		for i := range n {
			sn, cs := math.Sincos(2 * math.Pi * float64(i) / float64(n))
			nx, ny := cs/rx, sn/ry
			l := math.Hypot(nx, ny)
			add(cx+rx*cs, cy+ry*sn, nx/l, ny/l)
		}
	} else {
		r := f.r
		arc := func(cx, cy float64, a0 int) {
			if r <= 0 {
				return
			}
			n := max(int(math.Ceil(math.Pi/2*r/sampleStep)), 4)
			for i := 0; i <= n; i++ {
				sn, cs := math.Sincos((float64(a0) + 90*float64(i)/float64(n)) * math.Pi / 180)
				add(cx+r*cs, cy+r*sn, cs, sn)
			}
		}
		// У острого угла (r = 0) вместо дуги — вершина с диагональной нормалью.
		vertex := func(x, y, nx, ny float64) {
			if r <= 0 {
				add(x, y, nx*math.Sqrt2/2, ny*math.Sqrt2/2)
			}
		}
		add(f.x0+r, f.y0, 0, -1)
		add(f.x1-r, f.y0, 0, -1)
		vertex(f.x1, f.y0, 1, -1)
		arc(f.x1-r, f.y0+r, 270)
		add(f.x1, f.y1-r, 1, 0)
		vertex(f.x1, f.y1, 1, 1)
		arc(f.x1-r, f.y1-r, 0)
		add(f.x0+r, f.y1, 0, 1)
		vertex(f.x0, f.y1, -1, 1)
		arc(f.x0+r, f.y1-r, 90)
		add(f.x0, f.y0+r, -1, 0)
		vertex(f.x0, f.y0, -1, -1)
		arc(f.x0+r, f.y0+r, 180)
	}
	pl := polyline{pts: pts, cum: make([]float64, len(pts)+1)}
	for i := range pts {
		q := pts[(i+1)%len(pts)]
		pl.cum[i+1] = pl.cum[i] + math.Hypot(q.x-pts[i].x, q.y-pts[i].y)
	}
	return pl
}

// at — точка на длине дуги s (по кругу), нормаль пересчитана в единичную.
func (pl *polyline) at(s float64) cpt {
	l := pl.length()
	s = math.Mod(s, l)
	if s < 0 {
		s += l
	}
	i := sort.SearchFloat64s(pl.cum, s)
	if i > 0 {
		i--
	}
	i = min(i, len(pl.pts)-1)
	a, b := pl.pts[i], pl.pts[(i+1)%len(pl.pts)]
	seg := pl.cum[i+1] - pl.cum[i]
	t := 0.0
	if seg > 0 {
		t = (s - pl.cum[i]) / seg
	}
	nx, ny := a.nx+(b.nx-a.nx)*t, a.ny+(b.ny-a.ny)*t
	if n := math.Hypot(nx, ny); n > 0 {
		nx, ny = nx/n, ny/n
	}
	return cpt{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t, nx, ny}
}

// piece — точки контура от длины s0 до s1 (s1 > s0, в пределах периметра): концы и вершины между.
func (pl *polyline) piece(s0, s1 float64) []paint.Pt {
	a, b := pl.at(s0), pl.at(s1)
	out := []paint.Pt{{X: a.x, Y: a.y}}
	for i := sort.SearchFloat64s(pl.cum, s0); i < len(pl.pts); i++ {
		if pl.cum[i] > s1 {
			break
		}
		if pl.cum[i] > s0 {
			out = append(out, paint.Pt{X: pl.pts[i].x, Y: pl.pts[i].y})
		}
	}
	return append(out, paint.Pt{X: b.x, Y: b.y})
}
