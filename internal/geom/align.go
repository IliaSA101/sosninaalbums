package geom

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Выравнивание и распределение рамок (спека 010, AlignItems): чистая геометрия.

// Sheet — лист для опор page/spread, мм (model.Geo): размер листа, поле и страницы.
// Разворот — две страницы, обложка — сторонки, лист виньетки — одна.
type Sheet struct {
	W, H, Margin float64
	Pages        []layout.Rect
}

// PageAt — номер страницы, где лежит x (правый край страницы — уже следующая); вне страниц
// (корешок обложки, край листа) — ближайшая. Без страниц — 0.
func (st Sheet) PageAt(x float64) int {
	best, bd := 0, math.Inf(1)
	for i, r := range st.Pages {
		if x >= r.X && x < r.X+r.W {
			return i
		}
		if d := math.Min(math.Abs(x-r.X), math.Abs(x-r.X-r.W)); d < bd {
			best, bd = i, d
		}
	}
	return best
}

// pageOf — страница листа, где центр рамки r.
func (st Sheet) pageOf(r layout.Rect) int { return st.PageAt(r.X + r.W/2) }

// Опоры (to) AlignItems.
const (
	AlignSel    = "sel"
	AlignPage   = "page"
	AlignSpread = "spread"
)

// AlignShifts — сдвиги (dx, dy) рамок rects (снизу вверх по слоям): чистая геометрия.
// over — распределение хотя бы в одной группе не поместилось без наложения.
func AlignShifts(st Sheet, rects []layout.Rect, op, to string) (out [][2]float64, over bool) {
	out = make([][2]float64, len(rects))
	horiz := op == "left" || op == "hcenter" || op == "right" || op == "distH"
	whole := unionRect(rects)
	if op == "distH" || op == "distV" {
		// Опора page — у каждой страницы своё поле, как у выравнивания: объекты
		// каждой страницы распределяются в её поле.
		groups := make([][]int, max(len(st.Pages), 1))
		for i, r := range rects {
			k := 0
			if to == AlignPage {
				k = st.pageOf(r)
			}
			groups[k] = append(groups[k], i)
		}
		for _, g := range groups {
			if len(g) > 1 && distribute(out, rects, g, horiz, to, alignField(st, to, whole, rects[g[0]])) {
				over = true
			}
		}
		return out, over
	}
	for i, r := range rects {
		a, b := axisOf(r, horiz), axisOf(alignField(st, to, whole, r), horiz)
		var d float64
		switch op {
		case "left", "top":
			d = b.lo - a.lo
		case "hcenter", "vcenter":
			d = (b.lo+b.hi)/2 - (a.lo+a.hi)/2
		default: // right, bottom
			d = b.hi - a.hi
		}
		if horiz {
			out[i][0] = d
		} else {
			out[i][1] = d
		}
	}
	return out, false
}

// span — отрезок рамки на оси.
type span struct{ lo, hi float64 }

func axisOf(r layout.Rect, horiz bool) span {
	if horiz {
		return span{r.X, r.X + r.W}
	}
	return span{r.Y, r.Y + r.H}
}

// alignField — опора: выделение целиком (sel), поле страницы, где стоит центр
// рамки at, или поле листа.
func alignField(st Sheet, to string, whole, at layout.Rect) layout.Rect {
	m := st.Margin
	switch to {
	case AlignSpread:
		return layout.Rect{X: m, Y: m, W: st.W - 2*m, H: st.H - 2*m}
	case AlignPage:
		pg := layout.Rect{W: st.W, H: st.H}
		if len(st.Pages) > 0 {
			pg = st.Pages[st.pageOf(at)]
		}
		return layout.Rect{X: pg.X + m, Y: pg.Y + m, W: pg.W - 2*m, H: pg.H - 2*m}
	}
	return whole
}

// unionRect — описанный прямоугольник набора рамок.
func unionRect(rs []layout.Rect) layout.Rect {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range rs {
		x0, y0 = math.Min(x0, r.X), math.Min(y0, r.Y)
		x1, y1 = math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
	}
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// distribute — рамки group (2+, индексы rects) вдоль оси. Порядок — по центрам, равные
// центры — по слою (group идёт снизу вверх, сортировка устойчивая). Опора sel: крайние
// (по центру) стоят; page/spread: крайние встают к краям поля field. Помещаются —
// равные промежутки; нет — первый у начала, последний у конца, центры остальных
// равным шагом между ними (равный отрицательный промежуток свалил бы всех к одному
// краю); крайние шире поля — все центры в середине поля. true — не поместились.
func distribute(out [][2]float64, rects []layout.Rect, group []int, horiz bool, to string, field layout.Rect) bool {
	idx := slices.Clone(group)
	centre := func(i int) float64 { a := axisOf(rects[i], horiz); return (a.lo + a.hi) / 2 }
	slices.SortStableFunc(idx, func(a, b int) int {
		switch ca, cb := centre(a), centre(b); {
		case ca < cb:
			return -1
		case ca > cb:
			return 1
		}
		return 0
	})
	n := len(idx)
	first, last := axisOf(rects[idx[0]], horiz), axisOf(rects[idx[n-1]], horiz)
	from, upto := axisOf(field, horiz).lo, axisOf(field, horiz).hi
	if to == AlignSel {
		from, upto = first.lo, last.hi
	}
	sum := 0.0
	for _, i := range idx {
		a := axisOf(rects[i], horiz)
		sum += a.hi - a.lo
	}
	put := func(i int, lo float64) {
		if horiz {
			out[i][0] = lo - rects[i].X
		} else {
			out[i][1] = lo - rects[i].Y
		}
	}
	// Допуск — полшага Round2: ширины (X+W)−X у посадки встык дают gap ≈ −1e-14,
	// иначе ложное «не помещаются».
	gap := (upto - from - sum) / float64(n-1)
	if gap >= -0.005 {
		at := from
		for _, i := range idx {
			a := axisOf(rects[i], horiz)
			put(i, at)
			at += a.hi - a.lo + gap
		}
		return false
	}
	c0, c1 := from+(first.hi-first.lo)/2, upto-(last.hi-last.lo)/2
	if c0 > c1 {
		// Половины крайних больше поля: шаг от c0 к c1 перевернул бы порядок, и
		// повторный щелчок скакал бы. Все центры — в середину поля: от порядка не
		// зависит (повтор холостой) и остаётся на странице группы.
		c0 = (from + upto) / 2
		c1 = c0
	}
	for k, i := range idx {
		a := axisOf(rects[i], horiz)
		put(i, c0+(c1-c0)*float64(k)/float64(n-1)-(a.hi-a.lo)/2)
	}
	return true
}
