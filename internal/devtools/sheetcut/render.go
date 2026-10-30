package sheetcut

import (
	"image"
	"math"
)

// render вырезает стикер gi: пиксели его областей и мягкий край (softPx вокруг) в обрезке по
// содержимому с полем o.Pad; мягкий край — только по пикселям вне любых областей (метка 0):
// чужие стикеры и мусор не попадают в вырез, даже при зазоре меньше softPx.
func render(img *image.NRGBA, labels []int, stickerOf []int, gi int, g group, o Options) Sticker {
	sw := img.Bounds().Dx()
	zone := g.Rect.Inset(-softPx).Intersect(img.Bounds())
	zw, zh := zone.Dx(), zone.Dy()
	mask := make([]uint8, zw*zh)
	for y := 0; y < zh; y++ {
		for x := 0; x < zw; x++ {
			if stickerOf[labels[(zone.Min.Y+y)*sw+zone.Min.X+x]] == gi {
				mask[y*zw+x] = 1
			}
		}
	}
	dilate(mask, zw, zh, softPx)
	for y := 0; y < zh; y++ {
		for x := 0; x < zw; x++ {
			if l := labels[(zone.Min.Y+y)*sw+zone.Min.X+x]; l != 0 && stickerOf[l] != gi {
				mask[y*zw+x] = 0
			}
		}
	}
	// Рамка видимого: пиксели маски с ненулевой альфой.
	var box image.Rectangle
	for y := 0; y < zh; y++ {
		for x := 0; x < zw; x++ {
			if mask[y*zw+x] != 0 && img.Pix[((zone.Min.Y+y)*sw+zone.Min.X+x)*4+3] != 0 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, box.Dx()+2*o.Pad, box.Dy()+2*o.Pad))
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if mask[y*zw+x] == 0 {
				continue
			}
			si := ((zone.Min.Y+y)*sw + zone.Min.X + x) * 4
			di := (y-box.Min.Y+o.Pad)*out.Stride + (x-box.Min.X+o.Pad)*4
			copy(out.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	s := Sticker{Pieces: len(g.Regs), Area: g.Area}
	s.Rect = [4]int{g.Rect.Min.X, g.Rect.Min.Y, g.Rect.Max.X, g.Rect.Max.Y}
	if g.Cell.X >= 0 {
		s.Cell = &[2]int{g.Cell.X, g.Cell.Y}
	}
	if nw, nh := fitSide(out.Bounds().Dx(), out.Bounds().Dy(), o.MaxSide); nw != out.Bounds().Dx() || nh != out.Bounds().Dy() {
		out = scaleDown(out, nw, nh)
		s.Scaled = true
	}
	s.Img = out
	s.W, s.H = out.Bounds().Dx(), out.Bounds().Dy()
	s.CmW, s.CmH = cm(s.W), cm(s.H)
	return s
}

// cm — сантиметры при 300 dpi, до десятых.
func cm(px int) float64 { return math.Round(float64(px)/300*2.54*10) / 10 }

// dilate расширяет маску на r px (квадратом, Чебышёв): по строкам, затем по столбцам.
func dilate(m []uint8, w, h, r int) {
	tmp := make([]uint8, len(m))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if m[y*w+x] == 0 {
				continue
			}
			for k := max(0, x-r); k <= min(w-1, x+r); k++ {
				tmp[y*w+k] = 1
			}
		}
	}
	clear(m)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if tmp[y*w+x] == 0 {
				continue
			}
			for k := max(0, y-r); k <= min(h-1, y+r); k++ {
				m[k*w+x] = 1
			}
		}
	}
}
