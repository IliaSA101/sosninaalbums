package paint

import (
	"image"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Raster — готовая картинка внутри генератора (фирменные стикеры): цепочка копий в
// premultiplied RGBA, каждая вдвое меньше предыдущей. При печати берётся исходник, на экране —
// копия, ближайшая по размеру: мелкий превью-квадрат не мерцает от прореженных пикселей.
type Raster struct {
	mips []*image.RGBA
}

// minMipSide — копии короче этой стороны (px) не строятся.
const minMipSide = 16

// NewRaster готовит картинку к рисованию; nil и пустая — nil (Canvas.Raster ничего не рисует).
func NewRaster(img image.Image) *Raster {
	if img == nil || img.Bounds().Empty() {
		return nil
	}
	b := img.Bounds()
	m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(m, m.Rect, img, b.Min, draw.Src)
	r := &Raster{mips: []*image.RGBA{m}}
	for m.Rect.Dx() >= 2*minMipSide && m.Rect.Dy() >= 2*minMipSide {
		m = halve(m)
		r.mips = append(r.mips, m)
	}
	return r
}

// halve — копия вдвое меньше: среднее блока 2×2 (premultiplied, поэтому края не темнеют).
func halve(m *image.RGBA) *image.RGBA {
	w, h := m.Rect.Dx()/2, m.Rect.Dy()/2
	n := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		r0 := m.Pix[2*y*m.Stride:]
		r1 := m.Pix[(2*y+1)*m.Stride:]
		out := n.Pix[y*n.Stride:]
		for x := range w {
			for ch := range 4 {
				s := uint32(r0[8*x+ch]) + uint32(r0[8*x+4+ch]) + uint32(r1[8*x+ch]) + uint32(r1[8*x+4+ch])
				out[4*x+ch] = uint8((s + 2) / 4) //nolint:gosec // среднее четырёх байт: 0..255
			}
		}
	}
	return n
}

// Size — размер исходника, px.
func (r *Raster) Size() (w, h int) {
	if r == nil {
		return 0, 0
	}
	return r.mips[0].Rect.Dx(), r.mips[0].Rect.Dy()
}

// level — копия, у которой пиксель не мельче пикселя холста (k — px холста на px исходника):
// остаток уменьшения не больше 2×, дальше справляется бикубическая выборка.
func (r *Raster) level(k float64) *image.RGBA {
	i := 0
	for i+1 < len(r.mips) && k*float64(int(2)<<i) <= 1 {
		i++
	}
	return r.mips[i]
}

// Raster кладёт картинку на прямоугольник (0..w)×(0..h) координат генератора: перенос,
// масштаб, поворот и зеркало берутся из c.M, как у фигур. Прозрачность холста у стикера
// всегда 1 (withCanvas рисует «слоем»), поэтому здесь не учитывается.
func (c *Canvas) Raster(r *Raster, w, h float64) {
	if r == nil || !(w > 0) || !(h > 0) {
		return
	}
	sw, sh := r.Size()
	// Рамка картинки в px холста ∩ обрезка — куда Transform вообще может писать.
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range [4][2]float64{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		q := c.at(p[0], p[1])
		x, y := float64(q[0]), float64(q[1])
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	if math.IsNaN(minX+minY+maxX+maxY) || math.IsInf(minX+minY+maxX+maxY, 0) {
		return
	}
	dr := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX)), int(math.Ceil(maxY))).Intersect(c.Clip)
	if dr.Empty() {
		return
	}
	// px холста на px исходника: корень площади образа (зеркало и поворот её не меняют).
	m := &c.M
	k := math.Sqrt(math.Abs(m[0]*m[4]-m[1]*m[3])) * math.Sqrt(w*h) / math.Sqrt(float64(sw*sh))
	src := r.level(k)
	// Без поворота (почти всегда) — separable-фильтр на целый прямоугольник: тонкая линия не
	// рвётся при уменьшении, и это быстрее. Иначе — билинейно из копии по уровню.
	if m[1] == 0 && m[3] == 0 && m[0] != 0 && m[4] != 0 && c.scaleAxis(src, w, h) {
		return
	}
	lw, lh := float64(src.Rect.Dx()), float64(src.Rect.Dy())
	s2d := f64.Aff3{
		m[0] * w / lw, m[1] * h / lh, m[2],
		m[3] * w / lw, m[4] * h / lh, m[5],
	}
	// Подкартинка по обрезке: Transform сам пишет только в её границы.
	xdraw.ApproxBiLinear.Transform(c.Dst.SubImage(dr).(*image.RGBA), s2d, src, src.Rect, xdraw.Over, nil)
}

// maxFlipPx — предел временной копии зеркальной картинки; больше — билинейный путь.
const maxFlipPx = 1 << 26

// scaleAxis кладёт src на прямоугольник без поворота; зеркало — перевёрнутой копией. false —
// не вышло (пустой или слишком большой прямоугольник): зови общий путь.
func (c *Canvas) scaleAxis(src *image.RGBA, w, h float64) bool {
	m := &c.M
	x0, x1 := m[2], m[2]+m[0]*w
	y0, y1 := m[5], m[5]+m[4]*h
	full := image.Rect(int(math.Round(min(x0, x1))), int(math.Round(min(y0, y1))),
		int(math.Round(max(x0, x1))), int(math.Round(max(y0, y1))))
	if full.Empty() {
		return false
	}
	flipX, flipY := m[0] < 0, m[4] < 0
	if !flipX && !flipY {
		xdraw.CatmullRom.Scale(c.Dst.SubImage(c.Clip).(*image.RGBA), full, src, src.Rect, xdraw.Over, nil)
		return true
	}
	if full.Dx()*full.Dy() > maxFlipPx {
		return false
	}
	tmp := image.NewRGBA(image.Rect(0, 0, full.Dx(), full.Dy()))
	xdraw.CatmullRom.Scale(tmp, tmp.Rect, src, src.Rect, xdraw.Src, nil)
	flip(tmp, flipX, flipY)
	r := full.Intersect(c.Clip)
	draw.Draw(c.Dst, r, tmp, r.Min.Sub(full.Min), draw.Over)
	return true
}

// flip переворачивает картинку на месте: по горизонтали и/или вертикали.
func flip(m *image.RGBA, h, v bool) {
	w, ht := m.Rect.Dx(), m.Rect.Dy()
	if h {
		for y := range ht {
			row := m.Pix[y*m.Stride : y*m.Stride+4*w]
			for i, j := 0, w-1; i < j; i, j = i+1, j-1 {
				for ch := range 4 {
					row[4*i+ch], row[4*j+ch] = row[4*j+ch], row[4*i+ch]
				}
			}
		}
	}
	if v {
		for i, j := 0, ht-1; i < j; i, j = i+1, j-1 {
			ri, rj := m.Pix[i*m.Stride:i*m.Stride+4*w], m.Pix[j*m.Stride:j*m.Stride+4*w]
			for k := range ri {
				ri[k], rj[k] = rj[k], ri[k]
			}
		}
	}
}
