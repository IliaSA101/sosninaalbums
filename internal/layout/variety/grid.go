package variety

// Сетки и деревья ключей генератора featured: сетка фото подряд — ряды или столбцы, разрезы у
// равных сумм пропорций; оценка — как у layout, без Build (его зовёт featured на готовом ключе).

import (
	"math"
	"strconv"
	"strings"
)

const (
	// gridRows — сетка — до стольких рядов (столбцов): больше — фото мельче пола.
	gridRows = 4
	// smallWant — порог мелких в оценке layout (доля высоты области): сетки выбираются так же.
	// Копия, как spreadPenalty: сверяет TestGridFitIsLayoutScore.
	smallWant = 0.22
)

// tn — дерево ключа: лист (leaf ≥ 0) или ряд H / столбец V.
type tn struct {
	leaf int
	h    bool
	kids []tn
}

func lf(i int) tn { return tn{leaf: i} }

// node — узел ориентации h. Дети той же ориентации раскрываются (H(H(a,b),c) — та же картинка,
// что H(a,b,c), но другой ключ и лишний уровень глубины), единственный ребёнок — он сам.
func node(h bool, kids ...tn) tn {
	var out []tn
	for _, k := range kids {
		if k.leaf < 0 && k.h == h {
			out = append(out, k.kids...)
		} else {
			out = append(out, k)
		}
	}
	if len(out) == 1 {
		return out[0]
	}
	return tn{leaf: -1, h: h, kids: out}
}

func (t tn) write(b *strings.Builder) {
	if t.leaf >= 0 {
		b.WriteString(strconv.Itoa(t.leaf))
		return
	}
	if t.h {
		b.WriteString("H(")
	} else {
		b.WriteString("V(")
	}
	for i, k := range t.kids {
		if i > 0 {
			b.WriteByte(',')
		}
		k.write(b)
	}
	b.WriteByte(')')
}

func (t tn) String() string {
	var b strings.Builder
	t.write(&b)
	return b.String()
}

// gridKey — запрос сетки: одна и та же сетка нужна многим ключам (другая страница у center).
type gridKey struct {
	lo, hi int
	w, h   float64
	keep   int
}

// grids — до keep лучших сеток фото lo…hi−1 в область w×h: ряды друг под другом V(H(…),…) или
// столбцы H(V(…),…), 1…gridRows частей подряд. Разрезы — у равных сумм пропорций частей (ряды
// одной высоты), каждый с ближайшим соседом слева: ≤ 15 разбиений на ориентацию вместо полного
// перебора. Оценка — как у layout: заполнение области, разброс площадей, мелкие; Build не нужен.
func (g *gen) grids(lo, hi int, w, h float64, keep int) []tn {
	if lo >= hi || w <= 0 || h <= 0 {
		return nil
	}
	gk := gridKey{lo, hi, w, h, keep}
	if ts, ok := g.memo[gk]; ok {
		return ts
	}
	top := gridTop{keep: min(keep, 2)}
	g.orient(lo, hi, true, w, h, &top)
	g.orient(lo, hi, false, h, w, &top) // столбцы — те же ряды в транспонированной области
	out := make([]tn, top.n)
	for i, c := range top.best[:top.n] {
		out[i] = g.tree(lo, hi, c.cuts[:c.nc], c.rows)
	}
	g.memo[gk] = out
	return out
}

// gridCand — разбиение сетки и его оценка.
type gridCand struct {
	q    float64
	cuts [gridRows - 1]int
	nc   int
	rows bool
}

// gridTop — до keep (≤ 2) лучших разбиений по убыванию оценки; ничья — раньше найденное.
type gridTop struct {
	best    [2]gridCand
	n, keep int
}

// add сравнивает оценки в квантах (как tieQ в layout): у симметричных разбиений ничьи точные, и
// без квантов их решал бы последний бит — FMA на arm64 давала бы на Mac другие сетки.
func (top *gridTop) add(q float64, cuts []int, rows bool) {
	at := top.n
	for at > 0 && quanta(top.best[at-1].q) < quanta(q) {
		at--
	}
	if at >= top.keep {
		return
	}
	c := gridCand{q: q, rows: rows}
	c.nc = copy(c.cuts[:], cuts)
	copy(top.best[at+1:], top.best[at:])
	top.best[at], top.n = c, min(top.n+1, top.keep)
}

// orient — разбиения фото lo…hi−1 на 1…gridRows рядов (rows) или столбцов в область w×h (для
// столбцов — уже транспонированную) в top.
func (g *gen) orient(lo, hi int, rows bool, w, h float64, top *gridTop) {
	n := hi - lo
	as, pre := g.bufA[:n], g.bufP[:n+1]
	for i := range n {
		as[i] = g.as[lo+i]
		if !rows {
			as[i] = 1 / as[i]
		}
		pre[i+1] = pre[i] + as[i]
	}
	// Повторов разбиений нет: разрез j — c_j или c_j − 1 по своему биту маски, а у разных r разное
	// число разрезов, так что разные (r, маска) дают разные разбиения.
	cuts := make([]int, 0, gridRows)
	for r := 1; r <= min(gridRows, n); r++ {
		for mask := range 1 << (r - 1) {
			var ok bool
			if cuts, ok = g.cutsAt(cuts[:0], pre, lo, r, mask); !ok {
				continue
			}
			if q, ok := g.fit(as, pre, cuts, lo, w, h); ok {
				top.add(q, cuts, rows)
			}
		}
	}
}

// cutsAt — разрезы на r частей по префиксным суммам pre: j-й — там, где сумма доходит до j/r
// всей; бит j−1 маски сдвигает его на фото раньше. false — части пустые или не по порядку.
// Суммы — в квантах: у одинаковых фото сумма попадает в цель точно, и последний бит решал бы,
// по какую сторону фото встанет разрез.
func (g *gen) cutsAt(cuts []int, pre []float64, lo, r, mask int) ([]int, bool) {
	n := len(pre) - 1
	prev := 0
	for j := 1; j < r; j++ {
		target := quanta(pre[n] * float64(j) / float64(r))
		c := 1
		for c < n && quanta(pre[c]) < target {
			c++
		}
		if mask>>(j-1)&1 == 1 {
			c--
		}
		if c <= prev || c >= n {
			return cuts, false
		}
		cuts = append(cuts, lo+c)
		prev = c
	}
	return cuts, true
}

// tree — сетка по разрезам cuts: ряды (rows) или столбцы.
func (g *gen) tree(lo, hi int, cuts []int, rows bool) tn {
	var parts []tn
	start := lo
	for j := range len(cuts) + 1 {
		end := hi
		if j < len(cuts) {
			end = cuts[j]
		}
		ls := make([]tn, 0, end-start)
		for i := start; i < end; i++ {
			ls = append(ls, lf(i))
		}
		parts = append(parts, node(rows, ls...))
		start = end
	}
	return node(!rows, parts...)
}

// fit — оценка разбиения as (пропорции частей подряд, pre — их префиксные суммы; разрезы cuts
// — со сдвигом lo) на ряды в области w×h, как score у layout: заполнение − разброс − мелкие;
// false — не помещается. Столбцы вызывающий передаёт как ряды в транспонированной области.
func (g *gen) fit(as, pre []float64, cuts []int, lo int, w, h float64) (float64, bool) {
	gap, n, parts := g.p.Gap, len(as), len(cuts)+1
	bound := func(j int) int { // начало части j (j = parts — конец)
		switch j {
		case 0:
			return 0
		case parts:
			return n
		}
		return cuts[j-1] - lo
	}
	// Высота стопки рядов при ширине W: A·W + C (ряд j — (W − (n_j−1)·gap)/S_j).
	a, c := 0.0, gap*float64(parts-1)
	for j := range parts {
		b, e := bound(j), bound(j+1)
		s := pre[e] - pre[b]
		a += 1 / s
		c -= gap * float64(e-b-1) / s
	}
	width := w
	if a*width+c > h {
		width = (h - c) / a
	}
	area, minA, maxA, minSide := 0.0, math.Inf(1), 0.0, math.Inf(1)
	for j := range parts {
		b, e := bound(j), bound(j+1)
		s := pre[e] - pre[b]
		rh := (width - gap*float64(e-b-1)) / s
		if !(rh >= 1) {
			return 0, false
		}
		lo, hi := as[b], as[b]
		for _, x := range as[b+1 : e] {
			lo, hi = min(lo, x), max(hi, x)
		}
		area += rh * rh * s
		minA, maxA = min(minA, rh*rh*lo), max(maxA, rh*rh*hi)
		minSide = min(minSide, rh, rh*lo)
	}
	q := area/(w*h) - spreadPenalty*math.Log2(maxA/minA)
	if want := smallWant * g.ch; minSide < want {
		q -= 2 * (want - minSide) / g.ch
	}
	return q, true
}
