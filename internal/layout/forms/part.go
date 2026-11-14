package forms

// Разрез ленты на ряды DP (072, design §2.1): общий у «Рядов», страницы рядом с главным и рядов
// под ним.

import "math"

// partKey — разрез фото lo…hi−1 на r рядов, в ряду ≥ minN фото.
type partKey struct{ lo, hi, r, minN int }

// part — partMin с запоминанием в генераторе: один и тот же разрез нужен «Рядам», странице рядом с
// главным и рядам под ним, а таблица DP от lo годится для любого hi и числа рядов до manyRowsMax.
// Кэш — только в gen: Gen зовут параллельно (ряды «Авто» разных разворотов). Отрезок вне ленты —
// nil: разрез страниц у края (панорама длиннее остальных) не должен ронять Gen (ревью 072).
func (g *gen) part(lo, hi, r, minN int) []int {
	if lo < 0 || hi > g.k || lo > hi {
		return nil
	}
	key := partKey{lo, hi, r, minN}
	if b, ok := g.parts[key]; ok {
		return b
	}
	if g.parts == nil {
		g.parts = map[partKey][]int{}
		g.tables = map[[2]int]*partTable{}
	}
	var b []int
	if r <= manyRowsMax {
		t := g.tables[[2]int{lo, minN}]
		if t == nil {
			t = newPartTable(g.pre, lo, g.k, manyRowsMax, minN)
			g.tables[[2]int{lo, minN}] = t
		}
		b = t.cut(r, hi)
	} else {
		b = partMin(g.pre, lo, hi, r, minN)
	}
	g.parts[key] = b
	return b
}

// partMin — границы разреза lo…hi−1 на r рядов (в ряду ≥ minN фото) с наименьшей наибольшей суммой
// пропорций ряда: при общей ширине у такого ряда самая малая высота, она и решает размер фото.
// Сравнения в квантах, ничья — разрез левее (первый в переборе): иначе шум 1e-12 меняет ключ.
// nil — так не делится. Границы: b[0] = lo, b[r] = hi.
func partMin(pre []float64, lo, hi, r, minN int) []int {
	if r < 1 || minN < 1 || lo < 0 || hi >= len(pre) || hi-lo < minN*r {
		return nil
	}
	return newPartTable(pre, lo, hi, r, minN).cut(r, hi)
}

// partTable — DP разреза от lo: f[j][i] — наименьшая наибольшая сумма ряда (в квантах) у фото
// lo…lo+i−1 в j рядах, from — начало последнего ряда. Строка i не зависит от того, где кончается
// разрез, — одна таблица отвечает за все hi ≤ lo+n.
type partTable struct {
	lo, n, rmax int
	f           []float64
	from        []int
}

func newPartTable(pre []float64, lo, hi, rmax, minN int) *partTable {
	n := hi - lo
	t := &partTable{lo: lo, n: n, rmax: rmax, f: make([]float64, (rmax+1)*(n+1)), from: make([]int, (rmax+1)*(n+1))}
	inf := math.Inf(1)
	for i := range t.f {
		t.f[i] = inf
	}
	t.f[0] = 0
	if minN < 1 {
		return t
	}
	w := n + 1
	for j := 1; j <= rmax; j++ {
		prev, cur := t.f[(j-1)*w:j*w], t.f[j*w:(j+1)*w]
		for i := minN * j; i <= n; i++ {
			for s := minN * (j - 1); s <= i-minN; s++ {
				if prev[s] == inf {
					continue
				}
				if v := math.Max(prev[s], quanta(pre[lo+i]-pre[lo+s])); v < cur[i] {
					cur[i], t.from[j*w+i] = v, s
				}
			}
		}
	}
	return t
}

// cut — границы разреза lo…hi−1 на r рядов; nil — так не делится.
func (t *partTable) cut(r, hi int) []int {
	n, w := hi-t.lo, t.n+1
	if r < 1 || r > t.rmax || n < 0 || n > t.n || math.IsInf(t.f[r*w+n], 1) {
		return nil
	}
	b := make([]int, r+1)
	b[r] = hi
	for j, i := r, n; j >= 1; j-- {
		i = t.from[j*w+i]
		b[j-1] = t.lo + i
	}
	return b
}

// rowsOf — ряды H по границам b.
func rowsOf(b []int) []string {
	rows := make([]string, 0, len(b)-1)
	for i := 1; i < len(b); i++ {
		rows = append(rows, hrow(b[i-1], b[i]))
	}
	return rows
}
