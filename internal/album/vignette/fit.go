package vignette

import "math"

// eps гасит ошибку округления на границе «ровно влезает» (191/38.2 = 4.999…).
const eps = 1e-9

// grid — параметры сетки портретов: промежутки, пропорция, полоса подписи.
type grid struct {
	gx, gy, a, hs float64
}

// cellH — высота ячейки: портрет и полоса подписи под ним.
func (g grid) cellH(pw float64) float64 { return pw/g.a + g.hs }

// dims — сколько столбцов и рядов портрета шириной pw входит в r.
func (g grid) dims(r Rect, pw float64) (cols, rows int) {
	if r.W <= 0 || r.H <= 0 || pw <= 0 {
		return 0, 0
	}
	cols = int(math.Floor((r.W+g.gx)/(pw+g.gx) + eps))
	rows = int(math.Floor((r.H+g.gy)/(g.cellH(pw)+g.gy) + eps))
	return max(cols, 0), max(rows, 0)
}

// capacity — сколько портретов шириной pw помещается во все области.
func (g grid) capacity(rs []Rect, pw float64) int {
	n := 0
	for _, r := range rs {
		c, k := g.dims(r, pw)
		n += c * k
	}
	return n
}

// maxPW — наибольшая ширина портрета, при которой n помещаются в rs (двоичный поиск, 40 шагов:
// вместимость не растёт с размером). 0 — не помещаются ни при каком размере.
func (g grid) maxPW(n int, rs []Rect) float64 {
	hi := 0.0
	for _, r := range rs {
		hi = math.Max(hi, math.Min(r.W, (r.H-g.hs)*g.a))
	}
	if n <= 0 || hi <= 0 {
		return 0
	}
	lo := hi * 1e-4
	if g.capacity(rs, lo) < n {
		return 0
	}
	if g.capacity(rs, hi) >= n {
		return hi
	}
	for range 40 {
		mid := (lo + hi) / 2
		if g.capacity(rs, mid) >= n {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// fit — размер портрета для n мест. Полоса подписи не помещается даже при крошечном
// портрете (область ниже полосы) — полоса уменьшается вдвое до нуля: места важнее подписи,
// а «!» у подписи покажет, что ей тесно.
func (g grid) fit(n int, rs []Rect) (grid, float64) {
	for range 4 {
		if pw := g.maxPW(n, rs); pw > 0 || n <= 0 {
			return g, pw
		}
		g.hs /= 2
	}
	g.hs = 0
	return g, g.maxPW(n, rs)
}

// share делит n мест по областям пропорционально вместимости (не больше её), остаток —
// по порядку областей: на развороте страницы заполнены ровно, а не «левая доверху».
func share(n int, caps []int) []int {
	total := 0
	for _, c := range caps {
		total += c
	}
	out := make([]int, len(caps))
	if total == 0 || n <= 0 {
		return out
	}
	n = min(n, total)
	left := n
	for i, c := range caps {
		out[i] = n * c / total
		left -= out[i]
	}
	for i := 0; left > 0; i = (i + 1) % len(caps) {
		if out[i] < caps[i] {
			out[i]++
			left--
		}
	}
	return out
}

// place раскладывает n портретов шириной pw по областям, построчно в порядке областей.
// В области — сбалансированная сетка (столбцов не больше вместимости, ряды по возможности
// полные), блок по центру области, неполный последний ряд — по центру ряда.
func (g grid) place(n int, rs []Rect, pw float64) []Rect {
	if n <= 0 || pw <= 0 {
		return nil
	}
	caps := make([]int, len(rs))
	cols := make([]int, len(rs))
	for i, r := range rs {
		c, k := g.dims(r, pw)
		caps[i], cols[i] = c*k, c
	}
	ks := share(n, caps)
	out := make([]Rect, 0, n)
	for i, r := range rs {
		out = append(out, g.block(ks[i], cols[i], r, pw)...)
	}
	return out
}

// block — k портретов в области r при вместимости cols столбцов.
func (g grid) block(k, cols int, r Rect, pw float64) []Rect {
	if k <= 0 || cols <= 0 {
		return nil
	}
	rows := (k + cols - 1) / cols
	cols = (k + rows - 1) / rows
	ph, ch := pw/g.a, g.cellH(pw)
	blockH := float64(rows)*ch + float64(rows-1)*g.gy
	y := r.Y + (r.H-blockH)/2
	out := make([]Rect, 0, k)
	for row := range rows {
		inRow := min(cols, k-row*cols)
		rowW := float64(inRow)*pw + float64(inRow-1)*g.gx
		x := r.X + (r.W-rowW)/2
		for c := range inRow {
			out = append(out, Rect{x + float64(c)*(pw+g.gx), y, pw, ph})
		}
		y += ch + g.gy
	}
	return out
}
