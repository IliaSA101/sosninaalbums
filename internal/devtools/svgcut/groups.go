package svgcut

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// group — один предмет листа: номера путей (в порядке рисования) и общая рамка.
type group struct {
	Paths []int
	Box   Box
	Base  int // номер группы до split/drop (с 1) — на него ссылаются split и drop
}

// SplitLine режет склеенную группу прямой: пути, чей центр левее X (выше Y), отходят в отдельный
// предмет. Group — номер группы до split/drop (по отчёту первого прогона). Ровно одна из X, Y задана.
type SplitLine struct {
	Group int      `json:"group"`
	X     *float64 `json:"x,omitempty"`
	Y     *float64 `json:"y,omitempty"`
}

// ParseGrid разбирает «6x6» (колонки×ряды); пусто — без сетки (0, 0).
func ParseGrid(s string) (cols, rows int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	a, b, ok := strings.Cut(strings.ToLower(s), "x")
	cols, e1 := strconv.Atoi(a)
	rows, e2 := strconv.Atoi(b)
	if !ok || e1 != nil || e2 != nil || cols < 1 || rows < 1 || cols > 100 || rows > 100 {
		return 0, 0, fmt.Errorf("сетка %q: нужно КОЛОНКИxРЯДЫ, например 6x6 (до 100)", s)
	}
	return cols, rows, nil
}

// ParseSplit разбирает «7:x=812» и «7:y=300» (группа до split : ось = положение в координатах листа).
func ParseSplit(s string) (SplitLine, error) {
	num, rest, ok := strings.Cut(s, ":")
	axis, val, ok2 := strings.Cut(rest, "=")
	n, e1 := strconv.Atoi(strings.TrimSpace(num))
	v, e2 := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if !ok || !ok2 || e1 != nil || e2 != nil || n < 1 || math.IsNaN(v) || math.IsInf(v, 0) {
		return SplitLine{}, fmt.Errorf("split %q: нужно ГРУППА:x=ЧИСЛО или ГРУППА:y=ЧИСЛО", s)
	}
	switch strings.TrimSpace(axis) {
	case "x":
		return SplitLine{Group: n, X: &v}, nil
	case "y":
		return SplitLine{Group: n, Y: &v}, nil
	}
	return SplitLine{}, fmt.Errorf("split %q: ось x или y", s)
}

// ParseKnock разбирает «812,640» — точку выреза в координатах листа.
func ParseKnock(s string) ([2]float64, error) {
	a, b, ok := strings.Cut(s, ",")
	x, e1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	y, e2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if !ok || e1 != nil || e2 != nil || math.IsNaN(x+y) || math.IsInf(x+y, 0) {
		return [2]float64{}, fmt.Errorf("knock %q: нужно X,Y в координатах листа", s)
	}
	return [2]float64{x, y}, nil
}

// ParseDrop разбирает «3,5,9» — номера групп до split/drop.
func ParseDrop(s string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("drop %q: нужны номера групп через запятую", s)
		}
		out = append(out, n)
	}
	return out, nil
}

// byGap склеивает пути, чьи рамки пересекаются или ближе gap: связные компоненты. Развёртка по X —
// не квадратична на листе из сотен путей.
func byGap(doc *Doc, idx []int, gap float64) []group {
	order := slices.Clone(idx)
	sort.SliceStable(order, func(a, b int) bool { return doc.Paths[order[a]].Box.X0 < doc.Paths[order[b]].Box.X0 })
	parent := make([]int, len(order))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for a := range order {
		ba := doc.Paths[order[a]].Box
		for b := a + 1; b < len(order); b++ {
			bb := doc.Paths[order[b]].Box
			if bb.X0 > ba.X1+gap {
				// Список отсортирован по X0: дальше только правее, пересечений с a нет.
				break
			}
			if ba.Near(bb, gap) {
				parent[find(b)] = find(a)
			}
		}
	}
	return collect(doc, order, find)
}

// collect собирает группы по корням объединения; пути группы — в порядке рисования.
func collect(doc *Doc, order []int, find func(int) int) []group {
	byRoot := map[int]*group{}
	var roots []int
	for i, pi := range order {
		r := find(i)
		g := byRoot[r]
		if g == nil {
			g = &group{Box: doc.Paths[pi].Box}
			byRoot[r] = g
			roots = append(roots, r)
		}
		g.Paths = append(g.Paths, pi)
		g.Box = g.Box.Union(doc.Paths[pi].Box)
	}
	out := make([]group, 0, len(roots))
	for _, r := range roots {
		g := byRoot[r]
		slices.Sort(g.Paths)
		out = append(out, *g)
	}
	return out
}

// byGrid относит путь к ячейке сетки по центру его рамки (как sheetcut): склеенное разделяется,
// разбросанное в ячейке собирается. Порядок — по рядам; пустые ячейки возвращаются отдельно.
func byGrid(doc *Doc, idx []int, cols, rows int) (groups []group, empty [][2]int) {
	cells := map[int]*group{}
	for _, pi := range idx {
		cx, cy := doc.Paths[pi].Box.Center()
		c := min(cols-1, max(0, int(cx/(doc.W/float64(cols)))))
		r := min(rows-1, max(0, int(cy/(doc.H/float64(rows)))))
		g := cells[r*cols+c]
		if g == nil {
			g = &group{Box: doc.Paths[pi].Box}
			cells[r*cols+c] = g
		}
		g.Paths = append(g.Paths, pi)
		g.Box = g.Box.Union(doc.Paths[pi].Box)
	}
	for k := 0; k < cols*rows; k++ {
		if g := cells[k]; g != nil {
			slices.Sort(g.Paths)
			groups = append(groups, *g)
		} else {
			empty = append(empty, [2]int{k%cols + 1, k/cols + 1})
		}
	}
	return groups, empty
}

// orderRows упорядочивает группы по рядам: сортировка по центру Y, новый ряд — когда центр ушёл от
// среднего центра ряда больше чем на половину медианной высоты; внутри ряда — по X (как sheetcut).
func orderRows(groups []group) []group {
	if len(groups) < 2 {
		return groups
	}
	hs := make([]float64, len(groups))
	for i, g := range groups {
		hs[i] = g.Box.H()
	}
	slices.Sort(hs)
	tol := hs[len(hs)/2] / 2
	cy := func(g group) float64 { return (g.Box.Y0 + g.Box.Y1) / 2 }
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
		sort.SliceStable(row, func(i, j int) bool { return row[i].Box.X0+row[i].Box.X1 < row[j].Box.X0+row[j].Box.X1 })
		out = append(out, row...)
	}
	return out
}

// applySplitDrop нумерует группы (Base), режет по split и снимает drop. Номера в обеих правках — до
// любых изменений, поэтому одно не сдвигает другое. Неверный номер — ошибка, а не молчаливый пропуск.
func applySplitDrop(doc *Doc, groups []group, split []SplitLine, drop []int) ([]group, error) {
	for i := range groups {
		groups[i].Base = i + 1
	}
	for _, n := range drop {
		if n < 1 || n > len(groups) {
			return nil, fmt.Errorf("svgcut: drop %d: групп только %d", n, len(groups))
		}
	}
	for _, s := range split {
		if s.Group < 1 || s.Group > len(groups) {
			return nil, fmt.Errorf("svgcut: split: группы %d нет (групп %d)", s.Group, len(groups))
		}
		if (s.X == nil) == (s.Y == nil) {
			return nil, fmt.Errorf("svgcut: split группы %d: нужна ровно одна из x, y", s.Group)
		}
	}
	var out []group
	for _, g := range groups {
		if slices.Contains(drop, g.Base) {
			continue
		}
		parts := []group{g}
		for _, s := range split {
			if s.Group == g.Base {
				parts = cutParts(doc, parts, s)
			}
		}
		out = append(out, parts...)
	}
	if len(out) == 0 {
		return nil, errors.New("svgcut: после drop не осталось групп")
	}
	return out, nil
}

// cutParts делит каждую часть прямой s; пустая половина отбрасывается. Части идут слева направо
// (сверху вниз): так номера результата не зависят от порядка путей в файле.
func cutParts(doc *Doc, parts []group, s SplitLine) []group {
	var out []group
	for _, p := range parts {
		var lo, hi group
		lo.Base, hi.Base = p.Base, p.Base
		for _, pi := range p.Paths {
			cx, cy := doc.Paths[pi].Box.Center()
			v, at := cx, 0.0
			if s.Y != nil {
				v, at = cy, *s.Y
			} else {
				at = *s.X
			}
			side := &hi
			if v < at {
				side = &lo
			}
			if len(side.Paths) == 0 {
				side.Box = doc.Paths[pi].Box
			} else {
				side.Box = side.Box.Union(doc.Paths[pi].Box)
			}
			side.Paths = append(side.Paths, pi)
		}
		for _, h := range []group{lo, hi} {
			if len(h.Paths) > 0 {
				out = append(out, h)
			}
		}
	}
	return out
}
