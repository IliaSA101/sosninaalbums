package packprep

import (
	"image"
	"math/bits"
)

const (
	thumbSide  = 512 // миниатюра предмета для листов проверки и dHash
	hashGrid   = 32  // серая сетка dHash до 9×8
	SimilarMax = 16  // dHash: расстояние не больше — подсказка «похоже на №N» (0.27-129-19, порог 16: решение 0.27-129-60)
)

// fit — картинка, вписанная в квадрат side по длинной стороне: усреднение по площади, цвет с весом
// альфы (прозрачное не подмешивается). Не увеличивает. Возвращает src, если уменьшать не нужно.
func fit(src *image.NRGBA, side int) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	m := max(sw, sh)
	if m <= side {
		return src
	}
	nw, nh := max(1, sw*side/m), max(1, sh*side/m)
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := range nh {
		y0, y1 := y*sh/nh, max((y+1)*sh/nh, y*sh/nh+1)
		for x := range nw {
			x0, x1 := x*sw/nw, max((x+1)*sw/nw, x*sw/nw+1)
			var r, g, b, a, cnt uint64
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					al := uint64(p[3])
					r += uint64(p[0]) * al
					g += uint64(p[1]) * al
					b += uint64(p[2]) * al
					a += al
					cnt++
				}
			}
			if a == 0 {
				continue
			}
			o := dst.Pix[y*dst.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(min(255, r/a)), uint8(min(255, g/a)), uint8(min(255, b/a)), uint8(min(255, a/cnt))
		}
	}
	return dst
}

// Hash — dHash предмета в 8 положениях (4 поворота × отражение), 64 бита каждое.
type Hash [8]uint64

// grayGrid — яркость миниатюры на сером фоне (128), сжатая до n×n независимо от пропорций: поворот на
// 90° тогда переставляет клетки, а не меняет сетку.
func grayGrid(img *image.NRGBA, n int) [][]float64 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	g := make([][]float64, n)
	for y := range n {
		g[y] = make([]float64, n)
		y0 := y * h / n
		y1 := min(h, max((y+1)*h/n, y0+1))
		for x := range n {
			x0 := x * w / n
			x1 := min(w, max((x+1)*w/n, x0+1))
			var sum float64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					p := img.Pix[sy*img.Stride+sx*4 : sy*img.Stride+sx*4+4]
					a := float64(p[3]) / 255
					lum := (0.299*float64(p[0]) + 0.587*float64(p[1]) + 0.114*float64(p[2]))
					sum += lum*a + 128*(1-a)
				}
			}
			g[y][x] = sum / float64((y1-y0)*(x1-x0))
		}
	}
	return g
}

// turn — сетка, повёрнутая на 90°; flip — отражение по горизонтали.
func turn(g [][]float64) [][]float64 {
	n := len(g)
	out := make([][]float64, n)
	for y := range n {
		out[y] = make([]float64, n)
		for x := range n {
			out[y][x] = g[n-1-x][y]
		}
	}
	return out
}

func flip(g [][]float64) [][]float64 {
	n := len(g)
	out := make([][]float64, n)
	for y := range n {
		out[y] = make([]float64, n)
		for x := range n {
			out[y][x] = g[y][n-1-x]
		}
	}
	return out
}

// bitsOf — dHash 64 бита: сетка сжимается до 9×8, бит — «левый темнее правого».
func bitsOf(g [][]float64) uint64 {
	n := len(g)
	var cell [8][9]float64
	for y := range 8 {
		for x := range 9 {
			y0, y1 := y*n/8, max((y+1)*n/8, y*n/8+1)
			x0, x1 := x*n/9, max((x+1)*n/9, x*n/9+1)
			var s float64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					s += g[sy][sx]
				}
			}
			cell[y][x] = s / float64((y1-y0)*(x1-x0))
		}
	}
	var h uint64
	for y := range 8 {
		for x := range 8 {
			h <<= 1
			if cell[y][x] < cell[y][x+1] {
				h |= 1
			}
		}
	}
	return h
}

// hashOf — dHash по альфе на сером в 8 положениях (0.27-129-19). img — миниатюра предмета.
func hashOf(img *image.NRGBA) Hash {
	g := grayGrid(img, hashGrid)
	var out Hash
	for f := range 2 {
		cur := g
		if f == 1 {
			cur = flip(g)
		}
		for r := range 4 {
			out[f*4+r] = bitsOf(cur)
			cur = turn(cur)
		}
	}
	return out
}

// Distance — наименьшее расстояние Хэмминга между a и любым из 8 положений b.
func Distance(a, b Hash) int {
	best := 64
	for _, h := range b {
		best = min(best, bits.OnesCount64(a[0]^h))
	}
	return best
}
