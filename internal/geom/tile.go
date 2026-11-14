package geom

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Плитка новых мест (спека 042 design §3): брошенные фото, которым не хватило мест ручного
// разворота, встают равными ячейками по центру страницы. Это заготовка под ручную раскладку,
// а не готовый дизайн, поэтому ячейки одинаковые, а не раскладка «Авто».

// tileTie — относительная разница сумм площадей, ниже которой варианты сетки равны.
const tileTie = 1e-6

// Tile — k мест пропорций aspects равными ячейками в area (мм), промежуток gap; ячейка не
// крупнее maxSide по длинной стороне. Порядок — по рядам слева направо, ряды и неполный
// последний ряд — по центру. Пусто, если k = 0 или area вырождена.
func Tile(area layout.Rect, aspects []float64, gap, maxSide float64) []layout.Rect {
	k := len(aspects)
	if k == 0 || !(area.W > 0) || !(area.H > 0) || !finite(area.X, area.Y, area.W, area.H) {
		return nil
	}
	if !(gap > 0) || !finite(gap) {
		gap = 0
	}
	if !(maxSide > 0) || !finite(maxSide) {
		maxSide = math.Inf(1)
	}
	asp := make([]float64, k)
	for i, a := range aspects {
		asp[i] = goodAspect(a)
	}
	ma := median(asp)
	best, bestC := tileCell{}, 0
	for c := 1; c <= k; c++ {
		cell, ok := tileCellOf(area, k, c, gap, ma, maxSide, asp)
		if !ok {
			continue
		}
		switch {
		case bestC == 0 || cell.sum > best.sum*(1+tileTie):
		case math.Abs(cell.sum-best.sum) <= best.sum*tileTie && cell.skew < best.skew:
		default:
			continue
		}
		best, bestC = cell, c
	}
	if bestC == 0 {
		return nil
	}
	return tilePlace(area, asp, bestC, gap, best)
}

// tileCell — ячейка варианта сетки c столбцов: размер, сумма площадей мест и насколько
// пропорции блока отличаются от области (|ln|).
type tileCell struct {
	w, h, sum, skew float64
}

// tileCellOf — ячейка для c столбцов и ⌈k/c⌉ рядов; не помещается — ok = false.
func tileCellOf(area layout.Rect, k, c int, gap, ma, maxSide float64, asp []float64) (tileCell, bool) {
	r := (k + c - 1) / c
	cw := (area.W - gap*float64(c-1)) / float64(c)
	ch := (area.H - gap*float64(r-1)) / float64(r)
	if !(cw > 0) || !(ch > 0) {
		return tileCell{}, false
	}
	if cw/ma > ch {
		cw = ch * ma
	}
	ch = cw / ma
	if long := max(cw, ch); long > maxSide {
		cw, ch = cw*maxSide/long, ch*maxSide/long
	}
	sum := 0.0
	for _, a := range asp {
		w := min(cw, ch*a)
		sum += w * w / a
	}
	bw := cw*float64(c) + gap*float64(c-1)
	bh := ch*float64(r) + gap*float64(r-1)
	return tileCell{w: cw, h: ch, sum: sum, skew: math.Abs(math.Log(bw / bh / (area.W / area.H)))}, true
}

// tilePlace — места по рядам: блок и неполный последний ряд по центру области, место — фото
// по центру своей ячейки. Края округляются до 0,01 мм каждый: соседи не налезают друг на друга.
func tilePlace(area layout.Rect, asp []float64, c int, gap float64, cell tileCell) []layout.Rect {
	k := len(asp)
	r := (k + c - 1) / c
	bh := cell.h*float64(r) + gap*float64(r-1)
	y0 := area.Y + (area.H-bh)/2
	out := make([]layout.Rect, k)
	for i, a := range asp {
		row, col := i/c, i%c
		n := min(c, k-row*c) // мест в этом ряду
		rw := cell.w*float64(n) + gap*float64(n-1)
		cx := area.X + (area.W-rw)/2 + float64(col)*(cell.w+gap)
		cy := y0 + float64(row)*(cell.h+gap)
		w := min(cell.w, cell.h*a)
		h := w / a
		x0, y1 := Round2(cx+(cell.w-w)/2), Round2(cy+(cell.h-h)/2)
		out[i] = layout.Rect{X: x0, Y: y1, W: Round2(Round2(cx+(cell.w+w)/2) - x0), H: Round2(Round2(cy+(cell.h+h)/2) - y1)}
	}
	return out
}

// goodAspect — пропорции фото; не число и не > 0 — 1:1 (как у «Места»).
func goodAspect(a float64) float64 {
	if !(a > 0) || math.IsInf(a, 0) {
		return 1
	}
	return a
}

// finite — все числа конечны (не NaN и не ±Inf).
func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// median — медиана (у чётного числа — среднее двух средних); a не пуст.
func median(a []float64) float64 {
	s := slices.Clone(a)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
