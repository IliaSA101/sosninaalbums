package sheetcut

import (
	"image"
	"math"
	"sort"
)

// region — связная область (8-связность) пикселей с альфой не ниже порога.
type region struct {
	Rect image.Rectangle
	Area int
}

// labelRegions размечает связные области: labels[i] = номер области (с 1) или 0; regs[id-1].
func labelRegions(img *image.NRGBA, thr uint8) ([]int, []region) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	labels := make([]int, w*h)
	var regs []region
	var queue []int
	on := func(i int) bool { return img.Pix[i*4+3] >= thr }
	for start := range labels {
		if labels[start] != 0 || !on(start) {
			continue
		}
		id := len(regs) + 1
		labels[start] = id
		queue = append(queue[:0], start)
		var r region
		for k := 0; k < len(queue); k++ {
			i := int(queue[k])
			x, y := i%w, i/w
			r.Area++
			r.Rect = r.Rect.Union(image.Rect(x, y, x+1, y+1))
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					if q := ny*w + nx; labels[q] == 0 && on(q) {
						labels[q] = id
						queue = append(queue, q)
					}
				}
			}
		}
		regs = append(regs, r)
	}
	return labels, regs
}

// group — будущий стикер: номера областей (с 0 — индекс в regs) и общая рамка.
type group struct {
	Regs []int
	Rect image.Rectangle
	Area int
	Cell image.Point // колонка, ряд; (-1,-1) без сетки
}

func (g *group) add(id int, r region) {
	if len(g.Regs) == 0 {
		g.Rect = r.Rect
	} else {
		g.Rect = g.Rect.Union(r.Rect)
	}
	g.Regs = append(g.Regs, id)
	g.Area += r.Area
}

// byGrid — склейка по сетке cols×rows: область относится к ячейке по центру своей рамки,
// все области ячейки — один стикер. Пустые ячейки возвращаются списком (колонка, ряд).
// Порядок результата — слева направо, сверху вниз по ячейкам.
func byGrid(regs []region, sheet image.Point, cols, rows int) (groups []group, empty []image.Point) {
	cells := make([]group, cols*rows)
	for i := range cells {
		cells[i].Cell = image.Pt(i%cols, i/cols)
	}
	for id, r := range regs {
		c := cellOf(r.Rect, sheet, cols, rows)
		cells[c.Y*cols+c.X].add(id, r)
	}
	for _, g := range cells {
		if len(g.Regs) == 0 {
			empty = append(empty, g.Cell)
			continue
		}
		groups = append(groups, g)
	}
	return groups, empty
}

// cellOf — ячейка сетки, в которую попал центр рамки.
func cellOf(r image.Rectangle, sheet image.Point, cols, rows int) image.Point {
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	return image.Pt(min(cols-1, max(0, cx*cols/sheet.X)), min(rows-1, max(0, cy*rows/sheet.Y)))
}

// spillsIntoEmpty — рамка заметно (≥ 15 % своей площади) заходит в пустую ячейку сетки: похоже,
// один кусок задел две ячейки (стикеры слиплись). Заход в занятую соседнюю ячейку не в счёт:
// широкий стикер (лента) может быть шире ячейки.
func spillsIntoEmpty(r image.Rectangle, sheet image.Point, cols, rows int, empty map[image.Point]bool) bool {
	share := func(lo, hi, i, n, total int) float64 {
		c0, c1 := i*total/n, (i+1)*total/n
		return float64(max(0, min(hi, c1)-max(lo, c0))) / float64(max(1, hi-lo))
	}
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cols; cx++ {
			if empty[image.Pt(cx, cy)] && share(r.Min.X, r.Max.X, cx, cols, sheet.X)*share(r.Min.Y, r.Max.Y, cy, rows, sheet.Y) >= 0.15 {
				return true
			}
		}
	}
	return false
}

// byGlue — запасной путь без сетки: крупные области (площадь ≥ largeFrac от наибольшей) — стикеры;
// мелкие приклеиваются к ближайшей крупной, если расстояние между рамками ≤ glue px, иначе
// отбрасываются (число — второй результат). Порядок — по рядам центров, внутри ряда слева направо.
func byGlue(regs []region, glue int, largeFrac float64) (groups []group, dropped int) {
	biggest := 0
	for _, r := range regs {
		biggest = max(biggest, r.Area)
	}
	var small []int
	for id, r := range regs {
		if float64(r.Area) >= largeFrac*float64(biggest) {
			g := group{Cell: image.Pt(-1, -1)}
			g.add(id, r)
			groups = append(groups, g)
		} else {
			small = append(small, id)
		}
	}
	// Мелкие клеим к исходным рамкам крупных: порядок приклейки не двигает границы цели.
	orig := make([]image.Rectangle, len(groups))
	for i, g := range groups {
		orig[i] = g.Rect
	}
	for _, id := range small {
		best, bd := -1, math.MaxFloat64
		for i := range groups {
			if d := rectGap(regs[id].Rect, orig[i]); d < bd {
				best, bd = i, d
			}
		}
		if best < 0 || bd > float64(glue) {
			dropped++
			continue
		}
		groups[best].add(id, regs[id])
	}
	return orderRows(groups), dropped
}

// rectGap — расстояние между прямоугольниками (0, если пересекаются).
func rectGap(a, b image.Rectangle) float64 {
	dx := max(0, a.Min.X-b.Max.X, b.Min.X-a.Max.X)
	dy := max(0, a.Min.Y-b.Max.Y, b.Min.Y-a.Max.Y)
	return math.Hypot(float64(dx), float64(dy))
}

// orderRows упорядочивает группы по рядам: сортировка по центру Y, новый ряд — когда центр
// ушёл от среднего центра ряда больше чем на половину медианной высоты; внутри ряда — по X.
func orderRows(groups []group) []group {
	if len(groups) < 2 {
		return groups
	}
	hs := make([]int, len(groups))
	for i, g := range groups {
		hs[i] = g.Rect.Dy()
	}
	sort.Ints(hs)
	tol := float64(hs[len(hs)/2]) / 2
	cy := func(g group) float64 { return float64(g.Rect.Min.Y+g.Rect.Max.Y) / 2 }
	sort.SliceStable(groups, func(i, j int) bool { return cy(groups[i]) < cy(groups[j]) })
	var rows [][]group
	var sum float64
	for _, g := range groups {
		if n := len(rows); n > 0 && math.Abs(cy(g)-sum/float64(len(rows[n-1]))) <= tol {
			rows[n-1] = append(rows[n-1], g)
			sum += cy(g)
			continue
		}
		rows = append(rows, []group{g})
		sum = cy(g)
	}
	out := groups[:0:0]
	for _, row := range rows {
		sort.SliceStable(row, func(i, j int) bool { return row[i].Rect.Min.X+row[i].Rect.Max.X < row[j].Rect.Min.X+row[j].Rect.Max.X })
		out = append(out, row...)
	}
	return out
}
