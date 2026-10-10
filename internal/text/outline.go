package text

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/vector"
)

// pen рисует контуры глифов, повёрнутые вокруг (ox, oy) — px холста.
// Растеризатор, маска и буфер точек переиспользуются от глифа к глифу.
//
// Глиф растеризуется в свою маску и ложится через DrawMask: Rasterizer.Draw
// прямо в холст не обрезает по dst и паникует, когда глиф у края (design §5).
type pen struct {
	dst      *image.RGBA
	src      image.Image
	sin, cos float64
	ox, oy   float64
	z        vector.Rasterizer
	mask     image.Alpha
	pts      []float32 // повёрнутые точки сегментов текущего глифа, по 2 на точку
}

func newPen(dst *image.RGBA, src image.Image, deg, ox, oy float64) *pen {
	sin, cos := math.Sincos(deg * math.Pi / 180)
	return &pen{dst: dst, src: src, sin: sin, cos: cos, ox: ox, oy: oy}
}

// rot поворачивает точку (px холста) по часовой вокруг (ox, oy): ось Y смотрит вниз.
func (p *pen) rot(x, y float64) (float64, float64) {
	dx, dy := x-p.ox, y-p.oy
	return p.ox + dx*p.cos - dy*p.sin, p.oy + dx*p.sin + dy*p.cos
}

// segArgs — сколько точек у сегмента.
func segArgs(op sfnt.SegmentOp) int {
	switch op {
	case sfnt.SegmentOpQuadTo:
		return 2
	case sfnt.SegmentOpCubeTo:
		return 3
	}
	return 1
}

// glyph рисует контур segs (px, ось Y вниз) с началом в (gx, gy) до поворота.
func (p *pen) glyph(segs sfnt.Segments, gx, gy float64) {
	p.pts = p.pts[:0]
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, s := range segs {
		for i := range segArgs(s.Op) {
			x, y := p.rot(gx+float64(s.Args[i].X)/64, gy+float64(s.Args[i].Y)/64)
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			p.pts = append(p.pts, float32(x), float32(y))
		}
	}
	if len(p.pts) == 0 {
		return
	}
	// Прямоугольник по самим точкам контура (с запасом в px): контур кривых лежит
	// внутри оболочки своих точек, так что растеризатор не выходит за маску.
	r := image.Rect(int(math.Floor(minX))-1, int(math.Floor(minY))-1, int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1)
	if !r.Overlaps(p.dst.Bounds()) {
		return
	}
	w, h := r.Dx(), r.Dy()
	p.z.Reset(w, h)
	p.z.DrawOp = draw.Src // Reset возвращает Over; маска переиспользуется — пишем каждый пиксель
	fx, fy := float32(r.Min.X), float32(r.Min.Y)
	pt := func(k int) (float32, float32) { return p.pts[2*k] - fx, p.pts[2*k+1] - fy }
	k := 0
	for j, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			if j > 0 {
				p.z.ClosePath()
			}
			p.z.MoveTo(pt(k))
		case sfnt.SegmentOpLineTo:
			p.z.LineTo(pt(k))
		case sfnt.SegmentOpQuadTo:
			x1, y1 := pt(k)
			x2, y2 := pt(k + 1)
			p.z.QuadTo(x1, y1, x2, y2)
		case sfnt.SegmentOpCubeTo:
			x1, y1 := pt(k)
			x2, y2 := pt(k + 1)
			x3, y3 := pt(k + 2)
			p.z.CubeTo(x1, y1, x2, y2, x3, y3)
		}
		k += segArgs(s.Op)
	}
	p.z.ClosePath()
	if cap(p.mask.Pix) < w*h {
		p.mask.Pix = make([]uint8, w*h)
	}
	p.mask.Pix, p.mask.Stride, p.mask.Rect = p.mask.Pix[:w*h], w, image.Rect(0, 0, w, h)
	p.z.Draw(&p.mask, p.mask.Rect, image.Opaque, image.Point{})
	draw.DrawMask(p.dst, r, p.src, image.Point{}, &p.mask, image.Point{}, draw.Over)
}
