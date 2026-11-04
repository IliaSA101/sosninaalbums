package imgsig

import "image"

// planes — яркость и цветоразности (0…255, нейтраль 128) в сетке w×h; cb и cr — nil, если не нужны.
type planes struct {
	y, cb, cr []float64
}

// rowFn кладёт строку sy картинки (от верха Bounds) в буферы; cb и cr — nil, если не нужны.
type rowFn func(sy int, y, cb, cr []float64)

// gridOf уменьшает картинку до сетки w×h средним по площади клетки. Если картинка меньше сетки,
// клетка берёт ближайший пиксель. Тип плоскостей (JPEG YCbCr, Gray, RGBA/NRGBA) читается без At:
// на миниатюре 400 px это в разы быстрее и не выделяет память на пиксель.
func gridOf(img image.Image, w, h int, chroma bool) planes {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	rd := rowReader(img)
	out := planes{y: make([]float64, w*h)}
	yb := make([]float64, sw)
	var cbb, crb []float64
	if chroma {
		out.cb, out.cr = make([]float64, w*h), make([]float64, w*h)
		cbb, crb = make([]float64, sw), make([]float64, sw)
	}
	x0, x1 := cells(sw, w)
	y0, y1 := cells(sh, h)
	for ty := range h {
		for sy := y0[ty]; sy < y1[ty]; sy++ {
			rd(sy, yb, cbb, crb)
			for tx := range w {
				i := ty*w + tx
				for x := x0[tx]; x < x1[tx]; x++ {
					out.y[i] += yb[x]
					if chroma {
						out.cb[i] += cbb[x]
						out.cr[i] += crb[x]
					}
				}
			}
		}
		for tx := range w {
			i := ty*w + tx
			cnt := float64((y1[ty] - y0[ty]) * (x1[tx] - x0[tx]))
			out.y[i] /= cnt
			if chroma {
				out.cb[i] /= cnt
				out.cr[i] /= cnt
			}
		}
	}
	return out
}

// cells — границы [lo, hi) n клеток по size пикселям; клетка не пустая и при size < n.
func cells(size, n int) (lo, hi []int) {
	lo, hi = make([]int, n), make([]int, n)
	for i := range n {
		lo[i] = i * size / n
		hi[i] = max((i+1)*size/n, lo[i]+1)
	}
	return lo, hi
}

func rowReader(img image.Image) rowFn {
	b := img.Bounds()
	sw := b.Dx()
	switch m := img.(type) {
	case *image.YCbCr:
		return func(sy int, y, cb, cr []float64) {
			py := b.Min.Y + sy
			yo := m.YOffset(b.Min.X, py)
			for x := range sw {
				y[x] = float64(m.Y[yo+x])
			}
			if cb != nil {
				for x := range sw {
					ci := m.COffset(b.Min.X+x, py)
					cb[x], cr[x] = float64(m.Cb[ci]), float64(m.Cr[ci])
				}
			}
		}
	case *image.Gray:
		return func(sy int, y, cb, cr []float64) {
			o := m.PixOffset(b.Min.X, b.Min.Y+sy)
			for x := range sw {
				y[x] = float64(m.Pix[o+x])
			}
			neutral(cb, cr, sw)
		}
	case *image.RGBA:
		return func(sy int, y, cb, cr []float64) {
			o := m.PixOffset(b.Min.X, b.Min.Y+sy)
			for x := range sw {
				p := m.Pix[o+4*x:]
				put(x, float64(p[0]), float64(p[1]), float64(p[2]), y, cb, cr)
			}
		}
	case *image.NRGBA:
		return func(sy int, y, cb, cr []float64) {
			o := m.PixOffset(b.Min.X, b.Min.Y+sy)
			for x := range sw {
				p := m.Pix[o+4*x:]
				a := float64(p[3]) / 255 // как RGBA: прозрачное — тёмное
				put(x, float64(p[0])*a, float64(p[1])*a, float64(p[2])*a, y, cb, cr)
			}
		}
	}
	return func(sy int, y, cb, cr []float64) {
		for x := range sw {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+sy).RGBA()
			put(x, float64(r>>8), float64(g>>8), float64(bl>>8), y, cb, cr)
		}
	}
}

func neutral(cb, cr []float64, n int) {
	if cb == nil {
		return
	}
	for x := range n {
		cb[x], cr[x] = 128, 128
	}
}

// put переводит RGB в YCbCr (формулы JPEG) и кладёт в x-й отсчёт буферов.
func put(x int, r, g, b float64, y, cb, cr []float64) {
	y[x] = 0.299*r + 0.587*g + 0.114*b
	if cb != nil {
		cb[x] = 128 - 0.168736*r - 0.331264*g + 0.5*b
		cr[x] = 128 + 0.5*r - 0.418688*g - 0.081312*b
	}
}
