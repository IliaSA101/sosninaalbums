package layout

// Быстрые раскладки для ManyFrom…MaxPhotos фото (спека 027): ряды и столбцы с
// выравниванием, разрез на ряды — DP по префиксам. Перебора нет: число деревьев
// ограничено построением (бюджет числом, а не временем), поэтому «лучшая» не зависит
// от скорости машины. Деревья — обычные узлы compose: ключ, Build и score прежние.

import (
	"math"
	"slices"
)

const (
	// ManyFrom — с этого числа фото на развороте вместо перебора работают генераторы.
	// До 16 включительно — прежний перебор: иначе изменился бы вид собранных альбомов.
	ManyFrom = 17
	// ManyBatch — порция ряда «Авто» на многих фото; первая — из основных семейств.
	ManyBatch = 12
	// ManyCap — длиннее ряд не досчитывается.
	ManyCap = 48
)

// tieEps — ничьи решает порядок построения, а не шум округления: сборка с FMA (arm64 в
// macOS universal, GOAMD64=v3) меняет последние биты оценок, и первый вариант менялся бы.
// С 0.10 так и до 16 фото (order, решение 95); прежний порядок — LegacyLayouts.
const tieEps = 1e-9

// tieQ — оценка с точностью tieEps для сортировки.
func tieQ(s float64) float64 { return math.Round(s / tieEps) }

// lineBreaks делит фото [lo, hi) на r рядов подряд (cols — на r столбцов) так, чтобы
// высоты рядов при заполнении box были ближе к равным: DP по префиксам, O(r·m²).
// Возвращает границы рядов (r+1 чисел от lo до hi); nil — r вне 1…m или нет места.
func lineBreaks(asp []float64, lo, hi int, box Rect, gap float64, r int, cols bool) []int {
	if r < 1 || r > hi-lo {
		return nil
	}
	return breakDP(asp, lo, hi, box, gap, r, cols).cuts(lo, hi-lo)
}

// breakTable — DP lineBreaks над фото [lo, lo+m): best[k][j] — лучшая сумма для первых j
// фото в k рядах, from — начало ряда k. Клетка не зависит от того, где кончается страница,
// поэтому таблица до конца ленты годится всем страницам с тем же lo — с теми же битами.
type breakTable struct {
	m    int
	best []float64
	from []int
}

// breakDP — таблица для r рядов над [lo, hi); nil — нет места.
func breakDP(asp []float64, lo, hi int, box Rect, gap float64, r int, cols bool) *breakTable {
	m := hi - lo
	w, h := box.W, box.H
	if cols {
		w, h = h, w
	}
	want := (h - float64(r-1)*gap) / float64(r)
	if !(want > 0) || !(w > 0) {
		return nil
	}
	pre := make([]float64, m+1)
	for i := range m {
		a := asp[lo+i]
		if cols {
			a = 1 / a
		}
		pre[i+1] = pre[i] + a
	}
	inf := math.Inf(1)
	t := &breakTable{m, make([]float64, (r+1)*(m+1)), make([]int, (r+1)*(m+1))}
	for i := range t.best {
		t.best[i] = inf
	}
	t.best[0] = 0
	for k := 1; k <= r; k++ {
		prev, cur, fr := t.best[(k-1)*(m+1):], t.best[k*(m+1):], t.from[k*(m+1):]
		// Ряд k кончается на j; остальным r−k рядам нужно хотя бы по фото.
		for j := k; j <= m-(r-k); j++ {
			for i := k - 1; i < j; i++ {
				pv := prev[i]
				if pv == inf {
					continue
				}
				rh := (w - float64(j-i-1)*gap) / (pre[j] - pre[i])
				d := (rh - want) / want
				if v := pv + d*d; v < cur[j]-tieEps {
					cur[j], fr[j] = v, i
				}
			}
		}
	}
	return t
}

// cuts — границы r рядов для первых m фото таблицы (r — число рядов таблицы).
func (t *breakTable) cuts(lo, m int) []int {
	if t == nil {
		return nil
	}
	r := len(t.best)/(t.m+1) - 1
	out := make([]int, r+1)
	for k, j := r, m; k > 0; k-- {
		out[k] = lo + j
		j = t.from[k*(t.m+1)+j]
	}
	out[0] = lo
	return out
}

// breakKey — таблица breakDP в manyLayouts: левые страницы всех разрезов начинаются с
// первого фото, у них одна таблица на r до конца ленты.
type breakKey struct {
	r    int
	cols bool
	box  Rect
}

// lineBreaks — как lineBreaks, но страница с первого фото берёт общую таблицу breakKey
// (breaks != nil — только manyLayouts: в quickScore разрезов нет, таблица была бы лишней).
// Правые страницы так не делятся: таблица задом наперёд меняет выбор из равных разбиений.
func (e *enumerator) lineBreaks(asp []float64, lo, hi int, box Rect, r int, cols bool) []int {
	if e.breaks == nil || lo > 0 {
		return lineBreaks(asp, lo, hi, box, e.gap, r, cols)
	}
	if r < 1 || r > hi {
		return nil
	}
	k := breakKey{r, cols, box}
	t, ok := e.breaks[k]
	if !ok {
		t = breakDP(asp, 0, len(asp), box, e.gap, r, cols)
		e.breaks[k] = t
	}
	return t.cuts(0, hi)
}

// lineCenter — число рядов (столбцов), при котором равные ряды заполняют box: r² = Σa·H/W.
func lineCenter(asp []float64, box Rect, cols bool) int {
	var s float64
	for _, a := range asp {
		if cols {
			a = 1 / a
		}
		s += a
	}
	w, h := box.W, box.H
	if cols {
		w, h = h, w
	}
	return int(math.Round(math.Sqrt(s * h / w)))
}

// linesTree — дерево по границам рядов: V(H(…), …) или для столбцов H(V(…), …).
func (e *enumerator) linesTree(cuts []int, cols bool) *node {
	var groups []*node
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if b-a == 1 {
			groups = append(groups, e.leaves[a])
		} else {
			groups = append(groups, compose(!cols, e.leaves[a:b], e.gap))
		}
	}
	if len(groups) == 1 {
		return groups[0]
	}
	return compose(cols, groups, e.gap)
}

// lineNodes — деревья рядов (и столбцов, если both) над [lo, hi) с числом рядов r*+d,
// d0 ≤ |d| ≤ d1, без повторов ключа (один ряд — то же, что столбцы по одному фото).
func (e *enumerator) lineNodes(asp []float64, lo, hi int, box Rect, d0, d1 int, both bool) []*node {
	if hi-lo == 1 {
		if d0 == 0 {
			return []*node{e.leaves[lo]}
		}
		return nil
	}
	var out []*node
	seen := map[string]bool{}
	for _, cols := range []bool{false, true} {
		if cols && !both {
			break
		}
		c := lineCenter(asp[lo:hi], box, cols)
		for d := -d1; d <= d1; d++ {
			if max(d, -d) < d0 {
				continue
			}
			cuts := e.lineBreaks(asp, lo, hi, box, c+d, cols)
			if cuts == nil {
				continue
			}
			t := e.linesTree(cuts, cols)
			// Ряды разного числа — разные деревья; повтор бывает только между рядами и столбцами.
			if !both {
				out = append(out, t)
			} else if k := t.keyStr(); !seen[k] {
				seen[k] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// mblock — дерево над фото [lo, hi), вписанное в свою область.
type mblock struct {
	t      *node
	lo, hi int
	rects  []Rect // фото lo…hi−1
	h      float64
}

// Семейства кандидатов: каждое, пока его не было в ряду, получает место в порции
// своим лучшим вариантом (pick).
const (
	famPair   = iota // постранично, разрез у середины
	famHero1         // крупное первое на левой странице
	famSpread        // ряды и столбцы на весь разворот
	famHeroS         // крупное первое слева на развороте
	famFar           // разрез дальше от середины
	famR3            // r*±3 на среднем разрезе
	famHeroN         // крупное последнее на правой странице
	famCenter        // крупное фото в середине разворота
	famCount
)

// manyCand — кандидат: пара страниц или (r == nil) дерево на весь разворот.
type manyCand struct {
	s    float64
	l, r *mblock
	fam  int
	key  string // строится по требованию
}

func (c *manyCand) keyStr() string {
	if c.key == "" {
		if c.r == nil {
			c.key = c.l.t.keyStr()
		} else {
			c.key = "P[" + c.l.t.keyStr() + "|" + c.r.t.keyStr() + "]"
		}
	}
	return c.key
}

type blockSet struct {
	lo, hi, d0, d1 int
	box            Rect
}

// manyGen — один расчёт: семейства складывают кандидатов, pick выдаёт порции.
type manyGen struct {
	asp    []float64
	p      Params
	g      geom
	e      *enumerator
	buf    []Rect
	sets   map[blockSet][]*mblock
	fam    int // семейство, которое сейчас складывает кандидатов
	cands  []manyCand
	sorted bool
	seen   map[string]bool // ключи, уже выданные в ряд
	shown  [famCount]bool  // семейства, уже бывшие в ряду
}

func newManyGen(asp []float64, p Params) *manyGen {
	return &manyGen{asp: asp, p: p, g: newGeom(p), e: newEnum(asp, p.Gap),
		buf: make([]Rect, len(asp)), sets: map[blockSet][]*mblock{}, seen: map[string]bool{}}
}

func (m *manyGen) fit(t *node, lo, hi int, box Rect) (*mblock, bool) {
	h, ok := fitTree(t, box, m.p.Gap, m.buf)
	if !ok {
		return nil, false
	}
	return &mblock{t, lo, hi, slices.Clone(m.buf[lo:hi]), h}, true
}

func (m *manyGen) fitAll(ts []*node, lo, hi int, box Rect) []*mblock {
	var out []*mblock
	for _, t := range ts {
		if b, ok := m.fit(t, lo, hi, box); ok {
			out = append(out, b)
		}
	}
	return out
}

// blocks — ряды и столбцы над [lo, hi) в box (запоминаются: семейства их делят).
func (m *manyGen) blocks(lo, hi int, box Rect, d0, d1 int) []*mblock {
	k := blockSet{lo, hi, d0, d1, box}
	if bs, ok := m.sets[k]; ok {
		return bs
	}
	bs := m.fitAll(m.e.lineNodes(m.asp, lo, hi, box, d0, d1, true), lo, hi, box)
	m.sets[k] = bs
	return bs
}

func (m *manyGen) add(s float64, l, r *mblock) {
	m.cands = append(m.cands, manyCand{s: s, l: l, r: r, fam: m.fam})
	m.sorted = false
}

// pairs — постраничные кандидаты: каждый блок левой страницы с каждым блоком правой.
func (m *manyGen) pairs(ls, rs []*mblock) {
	for _, l := range ls {
		copy(m.buf[l.lo:l.hi], l.rects)
		for _, r := range rs {
			copy(m.buf[r.lo:r.hi], r.rects)
			m.add(pairScore(m.buf, m.g, max(l.t.depth, r.t.depth), l.h, r.h, false), l, r)
		}
	}
}

// cut — первые k фото на левой странице, ряды r*±d при d0 ≤ |d| ≤ d1.
func (m *manyGen) cut(k, d0, d1 int) {
	n := len(m.asp)
	if k < 1 || k >= n {
		return
	}
	m.pairs(m.blocks(0, k, m.g.pages[0], d0, d1), m.blocks(k, n, m.g.pages[1], d0, d1))
}

// spread — кандидаты на весь разворот (только layflat): оценка со штрафом за корешок.
func (m *manyGen) spread(bs []*mblock) {
	for _, b := range bs {
		m.add(score(b.rects, m.g.spread, m.g.spine, b.t.depth), b, nil)
	}
}

// hcat — ряд из частей; ряд внутри ряда раскрывается: ориентация чередуется по уровням.
func (m *manyGen) hcat(parts ...*node) *node {
	var kids []*node
	for _, t := range parts {
		if t.leaf < 0 && t.horiz {
			kids = append(kids, t.kids...)
		} else {
			kids = append(kids, t)
		}
	}
	return compose(true, kids, m.p.Gap)
}

// mainCut — разрезы первой порции: n/2 ± mainCut. При ±2 первый вариант уступал лучшему
// разрезу больше 0,05 оценки в 11 % наборов без layflat, при ±5 — в 0,2 % (проба sa-critic 027).
const mainCut = 5

// main — семейства первой порции: постранично с разрезом у середины, крупное первое
// на левой странице, при layflat — ряды на весь разворот и крупное первое слева.
func (m *manyGen) main() {
	n, h, g := len(m.asp), len(m.asp)/2, m.g
	if !m.p.Sheet { // у листа страниц нет
		m.fam = famPair
		for k := h - mainCut; k <= h+mainCut; k++ {
			m.cut(k, 0, 2)
		}
		m.fam = famHero1
		if l, ok := m.fit(m.e.leaves[0], 0, 1, g.pages[0]); ok {
			m.pairs([]*mblock{l}, m.blocks(1, n, g.pages[1], 0, 2))
		}
	}
	if !m.p.Layflat {
		return
	}
	m.fam = famSpread
	m.spread(m.blocks(0, n, g.spread, 0, 2))
	// Первое фото во всю высоту, остальные — в оставшейся части разворота.
	m.fam = famHeroS
	w0 := m.asp[0] * g.spread.H
	rest := Rect{g.spread.X + w0 + m.p.Gap, g.spread.Y, g.spread.W - w0 - m.p.Gap, g.spread.H}
	if rest.W >= 1 {
		var ts []*node
		for _, t := range m.e.lineNodes(m.asp, 1, n, rest, 0, 2, true) {
			ts = append(ts, m.hcat(m.e.leaves[0], t))
		}
		m.spread(m.fitAll(ts, 0, n, g.spread))
	}
}

// more — семейства порций 2–4: разрез дальше от середины, r*±3 на среднем разрезе,
// крупное последнее на правой странице, при layflat — крупное фото в середине разворота.
func (m *manyGen) more() {
	n, h, g := len(m.asp), len(m.asp)/2, m.g
	if !m.p.Sheet {
		m.morePages(n, h, g)
	}
	if !m.p.Layflat {
		return
	}
	m.fam = famCenter
	wi := m.asp[h] * g.spread.H
	side := Rect{g.spread.X, g.spread.Y, (g.spread.W-wi)/2 - m.p.Gap, g.spread.H}
	if side.W < 1 {
		return
	}
	var ts []*node
	for _, l := range m.e.lineNodes(m.asp, 0, h, side, 0, 2, true) {
		for _, r := range m.e.lineNodes(m.asp, h+1, n, side, 0, 2, true) {
			ts = append(ts, m.hcat(l, m.e.leaves[h], r))
		}
	}
	m.spread(m.fitAll(ts, 0, n, g.spread))
}

// morePages — постраничные семейства порций 2–4 (у листа 031 их нет).
func (m *manyGen) morePages(n, h int, g geom) {
	m.fam = famFar
	for d := mainCut + 1; d <= mainCut+3; d++ {
		m.cut(h-d, 0, 2)
		m.cut(h+d, 0, 2)
	}
	m.fam = famR3
	l2, l3 := m.blocks(0, h, g.pages[0], 0, 2), m.blocks(0, h, g.pages[0], 3, 3)
	r2, r3 := m.blocks(h, n, g.pages[1], 0, 2), m.blocks(h, n, g.pages[1], 3, 3)
	m.pairs(l3, slices.Concat(r2, r3))
	m.pairs(l2, r3)
	m.fam = famHeroN
	if r, ok := m.fit(m.e.leaves[n-1], n-1, n, g.pages[1]); ok {
		m.pairs(m.blocks(0, n-1, g.pages[0], 0, 2), []*mblock{r})
	}
}

// pick — следующая порция: до ManyBatch новых ключей по убыванию оценки. Семейство,
// которого ещё не было в ряду, получает место своим лучшим вариантом: иначе на многих
// фото весь ряд занимают пары страниц, а «крупное первое» и ряды на весь разворот
// (их оценка ниже) не видны. Лучший кандидат берётся всегда, так что первый вариант
// ряда — лучший. Внутри порции — по оценке.
func (m *manyGen) pick() []Result {
	if !m.sorted {
		slices.SortStableFunc(m.cands, func(a, b manyCand) int { return desc(tieQ(a.s), tieQ(b.s)) })
		m.sorted = true
	}
	var chosen []int
	take := func(i int) {
		c := &m.cands[i]
		m.seen[c.keyStr()] = true
		m.shown[c.fam] = true
		chosen = append(chosen, i)
	}
	for i := range m.cands {
		if len(chosen) == ManyBatch {
			break
		}
		if c := &m.cands[i]; !m.shown[c.fam] && !m.seen[c.keyStr()] {
			take(i)
		}
	}
	for i := range m.cands {
		if len(chosen) == ManyBatch {
			break
		}
		if !m.seen[m.cands[i].keyStr()] {
			take(i)
		}
	}
	slices.Sort(chosen)
	out := make([]Result, len(chosen))
	for j, i := range chosen {
		c := &m.cands[i]
		rects := make([]Rect, len(m.asp))
		copy(rects[c.l.lo:c.l.hi], c.l.rects)
		if c.r != nil {
			copy(rects[c.r.lo:c.r.hi], c.r.rects)
		}
		out[j] = Result{rects, c.s, c.keyStr()}
	}
	return out
}

// manyLayouts — ряд «Авто» для ManyFrom+ фото: первые limit элементов последовательности
// порций по ManyBatch (limit ≤ 0 или больше ManyCap — ManyCap). Первая порция — из
// основных семейств, следующие — из всех без уже выданных ключей. Порции считаются
// целиком и не зависят от limit, поэтому ряд с меньшим limit — префикс ряда с большим.
func manyLayouts(asp []float64, p Params, limit int) []Result {
	if limit <= 0 || limit > ManyCap {
		limit = ManyCap
	}
	m := newManyGen(asp, p)
	m.e.breaks = map[breakKey]*breakTable{}
	m.main()
	out := m.pick()
	if limit > ManyBatch {
		m.more()
		for len(out) < limit {
			b := m.pick()
			if len(b) == 0 {
				break
			}
			out = append(out, b...)
		}
	}
	return out[:min(len(out), limit)]
}

// manyBlocks — блоки страницы из многих фото для pageCtx.rows: ряды и столбцы r*±3 по
// убыванию оценки страницы, первые keep (keep < 0 — все). Как bestBlocks, но без перебора.
func manyBlocks(asp []float64, box Rect, gap float64, keep int) ([]candidate, int) {
	e := newEnum(asp, gap)
	ts := e.lineNodes(asp, 0, len(asp), box, 0, 3, true)
	buf := make([]Rect, len(asp))
	var out []candidate
	for i, t := range ts {
		h, ok := fitTree(t, box, gap, buf)
		if !ok {
			continue
		}
		out = append(out, candidate{Result{slices.Clone(buf), score(buf, box, -1, t.depth), t.keyStr()}, t.depth, h, i})
	}
	slices.SortStableFunc(out, func(a, b candidate) int { return desc(tieQ(a.Score), tieQ(b.Score)) })
	if keep >= 0 {
		out = out[:min(len(out), keep)]
	}
	return out, len(ts)
}
