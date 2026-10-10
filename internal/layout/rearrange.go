package layout

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Block — почему ряд с перестановкой пуст.
type Block string

const (
	BlockNone Block = ""     // перестановка возможна, но лучших раскладок не дала
	BlockSame Block = "same" // все фото одного класса пропорций
	BlockMany Block = "many" // фото больше, чем перебираем
)

const (
	maxPermN      = 7     // больше фото на развороте — перестановок нет
	maxPermPage   = 5     // больше фото на странице — только порядок ленты
	maxPermSpread = 5     // перестановки блока на весь разворот
	permBudget    = 15000 // деревьев на один вызов: при 7 разных пропорциях это ~40 мс
	// Раскладки с перестановкой показываем, только если они не хуже лучшей обычной больше чем на столько.
	rearrangeSlack = 0.03
)

// classes делит фото на классы пропорций (аспект с точностью 0.001, как в cacheKey):
// фото одного класса взаимозаменяемы. Классы нумеруются по первому появлению.
func classes(aspects []float64) (cls []int, rep []float64) {
	ids := map[float64]int{}
	cls = make([]int, len(aspects))
	for i, a := range aspects {
		r := math.Round(a * 1000)
		c, ok := ids[r]
		if !ok {
			c = len(rep)
			ids[r] = c
			rep = append(rep, a)
		}
		cls[i] = c
	}
	return cls, rep
}

// permSearch — состояние одного перебора: классы, бюджет, блоки по последовательностям.
type permSearch struct {
	g       geom
	rep     []float64 // аспект представителя каждого класса
	used    int       // деревьев перебрано
	stopped bool      // бюджет кончился, часть перестановок не рассмотрена
	seqs    map[string][]pageBlock
}

// pageBlock — блок страницы для последовательности классов, посчитанный в левой странице.
type pageBlock struct {
	seq    []int // класс фото на каждой позиции блока
	key    string
	rects  []Rect // по позициям блока
	depth  int
	height float64
	score  float64 // оценка самой страницы
}

func (ps *permSearch) left() bool { return ps.used < permBudget }

// more — можно ли перебирать дальше; бюджет кончился — помечает перебор неполным.
func (ps *permSearch) more() bool {
	if !ps.left() {
		ps.stopped = true
		return false
	}
	return true
}

// aspects — аспекты представителей по последовательности классов.
func (ps *permSearch) aspects(seq []int) []float64 {
	asp := make([]float64, len(seq))
	for i, c := range seq {
		asp[i] = ps.rep[c]
	}
	return asp
}

func seqKey(seq []int) string {
	var b strings.Builder
	for _, c := range seq {
		b.WriteString(strconv.Itoa(c))
		b.WriteByte(',')
	}
	return b.String()
}

// blocks — лучшие perSide деревьев над последовательностью классов в box, по убыванию
// оценки страницы; бюджет считает все перебранные деревья.
func (ps *permSearch) blocks(seq []int, box Rect) []pageBlock {
	cs, trees := bestBlocks(ps.aspects(seq), box, ps.g.gap, -1, perSide, false)
	ps.used += trees
	out := make([]pageBlock, len(cs))
	for i, c := range cs {
		out[i] = pageBlock{seq, c.Key, c.Rects, c.depth, c.height, c.Score}
	}
	return out
}

// top — лучшие perSide блоков последовательности; левая и правая страницы
// отличаются только сдвигом по x, поэтому считаем один раз в левой.
func (ps *permSearch) top(seq []int) []pageBlock {
	k := seqKey(seq)
	if r, ok := ps.seqs[k]; ok {
		return r
	}
	r := ps.blocks(seq, ps.g.pages[0])
	r = r[:min(len(r), perSide)]
	ps.seqs[k] = r
	return r
}

// eachSeq перебирает различные последовательности мультимножества классов
// (cnt[c] фото класса c) в лексикографическом порядке; fn → false — стоп.
func eachSeq(cnt []int, fn func(seq []int) bool) {
	total := 0
	for _, c := range cnt {
		total += c
	}
	cnt = slices.Clone(cnt)
	seq := make([]int, 0, total)
	var rec func() bool
	rec = func() bool {
		if len(seq) == total {
			return fn(slices.Clone(seq))
		}
		for c := range cnt {
			if cnt[c] == 0 {
				continue
			}
			cnt[c]--
			seq = append(seq, c)
			ok := rec()
			seq = seq[:len(seq)-1]
			cnt[c]++
			if !ok {
				return false
			}
		}
		return true
	}
	rec()
}

// assign — какие фото встают на позиции блока: j-е по счёту место класса c
// получает j-е фото из picks[c] (фото класса идут в порядке ленты — меньше инверсий).
func assign(seq []int, picks [][]int) []int {
	occ := make([]int, len(picks))
	idx := make([]int, len(seq))
	for j, c := range seq {
		idx[j] = picks[c][occ[c]]
		occ[c]++
	}
	return idx
}

// Rearranged — раскладки, где фото стоят не в порядке ленты: без тождественной
// перестановки, без дублей по форме и без форм, которые уже есть в Layouts(limit).
// Оценка не хуже лучшей Layouts − rearrangeSlack, Rects — в порядке входа.
func Rearranged(aspects []float64, p Params, limit int) ([]Result, Block) {
	r, b, _ := rearranged(aspects, p, limit)
	return r, b
}

func rearranged(aspects []float64, p Params, limit int) ([]Result, Block, *permSearch) {
	n := len(aspects)
	if n == 0 || n > MaxPhotos || p.PageW <= 0 || p.PageH <= 0 {
		return nil, BlockNone, nil
	}
	cls, rep := classes(aspects)
	if len(rep) == 1 {
		return nil, BlockSame, nil
	}
	if n > maxPermN {
		return nil, BlockMany, nil
	}
	base := Cached(aspects, p, limit)
	if len(base) == 0 {
		return nil, BlockNone, nil
	}
	ps := &permSearch{g: newGeom(p), rep: rep, seqs: map[string][]pageBlock{}}
	run := newPermRun(ps, p, cls, len(rep))

	// Порядок важен для бюджета: сначала страницы, потом блок на весь разворот.
	subs := leftSets(run.cnt)
	run.full = run.searchPages(subs)
	all := run.paged(subs)
	if p.Layflat && n <= maxPermSpread {
		all = append(all, run.spread()...)
	}

	// В край: два фото разных пропорций меняются страницами.
	if n == 2 {
		g := ps.g
		all = appendBleed(all, aspects, []Rect{g.bleed[1], g.bleed[0]}, g, "B[pages:1,0]")
	}

	by := func(r Result) float64 { return r.Score }
	return finish(all, base, aspects, p, limit, by, IsRearranged), BlockNone, ps
}

// permRun — один вызов rearranged: фото по классам и перебранные блоки страниц.
type permRun struct {
	ps      *permSearch
	p       Params
	n       int
	cls     []int   // класс каждого фото
	byClass [][]int // фото каждого класса в порядке ленты
	cnt     []int   // число фото в классе
	// Перебранные блоки по мультимножеству классов страницы (ключ — seqKey).
	full map[string][]pageBlock
}

func newPermRun(ps *permSearch, p Params, cls []int, nc int) *permRun {
	byClass := make([][]int, nc)
	for i, c := range cls {
		byClass[c] = append(byClass[c], i)
	}
	cnt := make([]int, nc)
	for c := range nc {
		cnt[c] = len(byClass[c])
	}
	return &permRun{ps: ps, p: p, n: len(cls), cls: cls, byClass: byClass, cnt: cnt}
}

// msize — число фото в мультимножестве классов.
func msize(m []int) int {
	s := 0
	for _, k := range m {
		s += k
	}
	return s
}

// leftSets — мультимножества классов для левой страницы (правая — дополнение),
// от меньших к большим: при нехватке бюджета хоть маленькие страницы переберём.
// При равном размере — лексикографически по числу фото каждого класса.
func leftSets(cnt []int) [][]int {
	var subs [][]int
	var rec func(c int, m []int)
	rec = func(c int, m []int) {
		if c == len(cnt) {
			subs = append(subs, slices.Clone(m))
			return
		}
		for k := 0; k <= cnt[c]; k++ {
			rec(c+1, append(m, k))
		}
	}
	rec(0, nil)
	sort.SliceStable(subs, func(i, j int) bool { return msize(subs[i]) < msize(subs[j]) })
	return subs
}

// searchPages — лучшие блоки по всем последовательностям каждого мультимножества,
// пока хватает бюджета. Пустые и больше maxPermPage не перебираются.
func (r *permRun) searchPages(subs [][]int) map[string][]pageBlock {
	full := map[string][]pageBlock{}
	for _, m := range subs {
		if k := msize(m); k == 0 || k > maxPermPage {
			continue
		}
		var acc []pageBlock
		eachSeq(m, func(seq []int) bool {
			if !r.ps.more() {
				return false
			}
			acc = append(acc, r.ps.top(seq)...)
			return true
		})
		full[seqKey(m)] = acc
	}
	return full
}

// pageTop — блоки страницы с мультимножеством m и конкретными фото picks:
// всё перебранное плюс блоки в порядке ленты (их считаем всегда).
func (r *permRun) pageTop(m []int, picks [][]int) []pageBlock {
	var inOrder []int
	for _, ids := range picks {
		inOrder = append(inOrder, ids...)
	}
	slices.Sort(inOrder)
	seq := make([]int, len(inOrder))
	for i, id := range inOrder {
		seq[i] = r.cls[id]
	}
	acc := append(slices.Clone(r.full[seqKey(m)]), r.ps.top(seq)...)
	slices.SortStableFunc(acc, func(a, b pageBlock) int { return desc(a.score, b.score) })
	var out []pageBlock
	seen := map[string]bool{}
	for _, b := range acc {
		k := seqKey(b.seq) + b.key
		if !seen[k] {
			seen[k] = true
			out = append(out, b)
		}
		if len(out) == perSide {
			break
		}
	}
	return out
}

// split — фото страниц при мультимножестве m слева: левая берёт самые ранние
// фото каждого класса, правая — остальные (rm — её мультимножество).
func (r *permRun) split(m []int) (lp, rp [][]int, rm []int) {
	lp, rp = make([][]int, len(r.cnt)), make([][]int, len(r.cnt))
	rm = make([]int, len(r.cnt))
	for c := range r.cnt {
		lp[c] = r.byClass[c][:m[c]]
		rp[c] = r.byClass[c][m[c]:]
		rm[c] = r.cnt[c] - m[c]
	}
	return lp, rp, rm
}

// paged — постраничные раскладки по всем делениям фото между страницами.
func (r *permRun) paged(subs [][]int) []Result {
	var all []Result
	for _, m := range subs {
		k := msize(m)
		if (k == 0 || k == r.n) && r.n > maxPermPage {
			continue // вся лента на одной странице без перебора — только порядок ленты
		}
		all = append(all, r.pairs(m, k)...)
	}
	return all
}

// pairs — все пары блоков левой и правой страниц при k фото слева.
func (r *permRun) pairs(m []int, k int) []Result {
	lp, rp, rm := r.split(m)
	ls := []pageBlock{{key: "-"}}
	if k > 0 {
		ls = r.pageTop(m, lp)
	}
	rs := []pageBlock{{key: "-"}}
	if k < r.n {
		rs = r.pageTop(rm, rp)
	}
	var out []Result
	for _, lb := range ls {
		for _, rb := range rs {
			rects := make([]Rect, r.n)
			lk, _ := putPage(rects, lb, lp)
			rk, ridx := putPage(rects, rb, rp)
			for _, id := range ridx {
				rects[id].X += r.p.PageW
			}
			s := pairScore(rects, r.ps.g, max(lb.depth, rb.depth), lb.height, rb.height, k == 0 || k == r.n)
			out = append(out, Result{rects, s, "P[" + lk + "|" + rk + "]"})
		}
	}
	return out
}

// putPage ставит блок страницы (посчитанный в левой) на фото picks в rects;
// возвращает ключ страницы и номера фото по позициям блока. Пустая страница — «-».
func putPage(rects []Rect, b pageBlock, picks [][]int) (string, []int) {
	if b.seq == nil {
		return "-", nil
	}
	idx := assign(b.seq, picks)
	for j, id := range idx {
		rects[id] = b.rects[j]
	}
	return relabel(b.key, idx), idx
}

// spread — перестановки блока на весь разворот, пока хватает бюджета.
func (r *permRun) spread() []Result {
	ps, g := r.ps, r.ps.g
	var all []Result
	eachSeq(r.cnt, func(seq []int) bool {
		if !ps.more() {
			return false
		}
		cs, trees := blockCandidates(ps.aspects(seq), g.spread, r.p.Gap)
		ps.used += trees
		idx := assign(seq, r.byClass)
		for _, c := range cs {
			rects := make([]Rect, r.n)
			for j, id := range idx {
				rects[id] = c.Rects[j]
			}
			all = append(all, Result{rects, score(rects, g.spread, g.spine, c.depth), relabel(c.Key, idx)})
		}
		return true
	})
	return all
}

// finish отбирает перестановки: убирает тождественные и формы из обычного ряда,
// из дублей по форме оставляет меньше всего отходящий от порядка ленты, пересчитывает
// точно через Build (перебор шёл по представителям классов), фильтрует по
// rearrangeSlack и сортирует. by — оценка для фильтра и сортировки, perm — ключ
// действительно переставляет фото.
func finish(all, base []Result, aspects []float64, p Params, limit int, by func(Result) float64, perm func(string) bool) []Result {
	if len(base) == 0 {
		return nil
	}
	shapes := map[string]bool{}
	best := math.Inf(-1)
	for _, r := range base {
		shapes[shapeKey(r.Key, aspects)] = true
		best = math.Max(best, by(r))
	}
	type pick struct {
		r   Result
		inv int
	}
	byShape := map[string]int{}
	var picks []pick
	for _, r := range all {
		// Грубый отсев до точного пересчёта: оценка по представителям классов почти точная.
		if !perm(r.Key) || by(r) < best-rearrangeSlack-0.01 {
			continue
		}
		sh := shapeKey(r.Key, aspects)
		if shapes[sh] {
			continue
		}
		inv := inversions(r.Key)
		if i, ok := byShape[sh]; ok {
			if inv < picks[i].inv || inv == picks[i].inv && by(r) > by(picks[i].r) {
				picks[i] = pick{r, inv}
			}
			continue
		}
		byShape[sh] = len(picks)
		picks = append(picks, pick{r, inv})
	}
	var out []Result
	for _, pk := range picks {
		r, ok := Build(pk.r.Key, aspects, p)
		if !ok {
			continue
		}
		if by(r) >= best-rearrangeSlack {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b Result) int { return desc(by(a), by(b)) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
