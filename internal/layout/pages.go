package layout

import (
	"math"
	"slices"
)

// pageCtx — постраничный ключ, у которого листаем одну страницу.
type pageCtx struct {
	g     geom
	p     Params
	side  int
	own   []int  // фото страницы в порядке ленты
	other string // запись второй страницы в ключе как есть
	rects []Rect // прямоугольники второй страницы (остальные нули)
	og    pageGeo
	empty bool // вторая страница пуста
	many  bool // на развороте ManyFrom+ фото: его страницы из рядов (как в manyLayouts)
}

func newPageCtx(key string, aspects []float64, p Params, side int) (*pageCtx, bool) {
	n := len(aspects)
	if side != 0 && side != 1 || n == 0 || n > MaxPhotos || p.PageW <= 0 || p.PageH <= 0 || p.Sheet {
		return nil, false
	}
	k, ok := pageForm(key)
	if !ok || !leavesOK(k.leaves(), n) {
		return nil, false
	}
	pc := &pageCtx{g: newGeom(p), p: p, side: side, rects: make([]Rect, n), many: n >= ManyFrom}
	pc.own = k.pages[side].leaves(nil)
	if len(pc.own) == 0 {
		return nil, false
	}
	slices.Sort(pc.own)
	ot, om := k.pages[1-side], k.mods[1-side]
	pc.other = pageString(om, ot)
	if ot == nil {
		pc.empty = true
		return pc, true
	}
	pc.og, ok = pc.g.page(1-side, om, ot, aspects, pc.rects)
	return pc, ok
}

// key — ключ разворота, где эта страница записана как own, а вторая как есть.
func (pc *pageCtx) key(own string) string {
	if pc.side == 1 {
		return joinPages(pc.other, own)
	}
	return joinPages(own, pc.other)
}

// result собирает разворот из блока страницы (rects по позициям блока, idx — их фото).
func (pc *pageCtx) result(c candidate, idx []int) Result {
	rects := slices.Clone(pc.rects)
	for j, id := range idx {
		rects[id] = c.Rects[j]
	}
	var pg [2]pageGeo
	pg[pc.side] = pageGeo{h: c.height, depth: c.depth}
	pg[1-pc.side] = pc.og
	return Result{Rects: rects, Score: pagedScore(rects, pc.g, pg, pc.empty), Key: pc.key(relabel(c.Key, idx))}
}

// pageRow — блок страницы, собранный в целый разворот.
type pageRow struct {
	res   Result
	own   string  // запись страницы в ключе
	score float64 // оценка самой страницы
	perm  bool    // фото страницы стоят не в порядке ленты
}

// rows — блоки страницы для одной последовательности её фото, по убыванию оценки
// страницы; первые keep (keep < 0 — все). На развороте из многих фото страница больше
// maxFullTree — ряды и столбцы manyBlocks: в них есть блок, который поставил manyLayouts
// (у перебора lines рядов не больше 4), а страница > 16 фото перебором не считается.
// Страница поменьше — mixBlocks: в первых keep перебора блока manyLayouts может не быть.
func (pc *pageCtx) rows(asp []float64, idx []int, keep int) ([]pageRow, int) {
	box := pc.g.pages[pc.side]
	var cs []candidate
	var trees int
	switch {
	case pc.many && len(asp) > maxFullTree:
		cs, trees = manyBlocks(asp, box, pc.g.gap, keep)
	case pc.many && len(asp) > 1 && keep >= 0:
		cs, trees = mixBlocks(asp, box, pc.g.gap, keep)
	default:
		cs, trees = bestBlocks(asp, box, pc.g.gap, -1, keep, false)
	}
	out := make([]pageRow, len(cs))
	for i, c := range cs {
		own := relabel(c.Key, idx)
		out[i] = pageRow{pc.result(c, idx), own, c.Score, IsRearranged(own)}
	}
	return out, trees
}

// mixBlocks — страница до maxFullTree фото на развороте из многих: все ряды и столбцы
// manyBlocks (среди них блок, который поставил manyLayouts) и лучшие деревья перебора, всего
// keep − 1: одно место — «поменьше» из variants, иначе он вытеснил бы из ряда блок manyLayouts.
// Перебор оставлен ради разнообразия: рядов и столбцов на странице 2–7 фото меньше 14.
func mixBlocks(asp []float64, box Rect, gap float64, keep int) ([]candidate, int) {
	out, _ := manyBlocks(asp, box, gap, -1)
	room := max(keep-1, len(out))
	has := make(map[string]bool, room)
	for _, c := range out {
		has[c.Key] = true
	}
	full, trees := bestBlocks(asp, box, gap, -1, room, false)
	for _, c := range full {
		if len(out) >= room {
			break
		}
		if !has[c.Key] {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b candidate) int { return desc(tieQ(a.Score), tieQ(b.Score)) })
	return out[:min(len(out), keep)], trees
}

// smallRank — насколько «поменьше» в ряду страницы ниже своего блока. По честной
// оценке (заполнение) оно уходит в конец ряда и при многих деревьях отсекается limit.
const smallRank = 0.15

// minShift — меньший сдвиг к краю на глаз не отличить от вписанного по центру.
const minShift = 0.05

// variants — та же страница с модификаторами, от лучшего блока best: у одного
// фото в край, к внешнему краю, к корешку и поменьше; у нескольких — только поменьше.
// Прижатые к краю строятся, только если сдвиг от центра заметен (minShift ширины страницы).
func (pc *pageCtx) variants(aspects []float64, best pageRow) []pageRow {
	mods := "S"
	if len(pc.own) == 1 {
		mods = pageMods
	}
	box := pc.g.pages[pc.side]
	var out []pageRow
	for _, m := range []byte(mods) {
		own := string(m) + "(" + best.own + ")"
		r, ok := Build(pc.key(own), aspects, pc.p)
		if !ok {
			continue
		}
		ps := best.score
		switch m {
		case 'S':
			ps -= smallRank
		case 'E':
			id := pc.own[0]
			ps = score([]Rect{r.Rects[id]}, box, -1, 0) - bleedPenalty(cropOf(r.Rects[id], aspects[id]))
		case 'O', 'I':
			if id := pc.own[0]; math.Abs(r.Rects[id].X-best.res.Rects[id].X) < minShift*box.W {
				continue
			}
		}
		out = append(out, pageRow{r, own, ps, best.perm})
	}
	return out
}

// PageChoices — варианты одной страницы (side 0 — левая, 1 — правая) при текущем
// постраничном ключе: целые развороты, где вторая страница взята из key как есть.
// Ряды отсортированы по оценке самой страницы, а не разворота: порядок ряда не
// прыгает, когда листают другую страницу. В ряду «в порядке ленты» есть и
// модификаторы страницы (variants). ok=false, если у key нет страниц (IsPaged) или страница пуста.
func PageChoices(key string, aspects []float64, p Params, side, limit int) (inOrder, rearranged []Result, block Block, ok bool) {
	// Подрезка (104): ряды и порядок — как без неё, Rects/Score/Crop строк — из Build(p).
	orig, crop := p, fitCrop(p) > 0
	if crop {
		p = noCrop(p)
	}
	pc, ok := newPageCtx(key, aspects, p, side)
	if !ok {
		return nil, nil, BlockNone, false
	}
	base := pc.inOrder(aspects, limit)
	r, b := pc.rearranged(aspects, p, limit, base)
	in := results(base)
	if crop {
		return cropRows(in, aspects, orig), cropRows(r, aspects, orig), b, true
	}
	return in, r, b, true
}

func results(rows []pageRow) []Result {
	out := make([]Result, len(rows))
	for i, r := range rows {
		out[i] = r.res
	}
	return out
}

func (pc *pageCtx) inOrder(aspects []float64, limit int) []pageRow {
	asp := make([]float64, len(pc.own))
	for i, id := range pc.own {
		asp[i] = aspects[id]
	}
	// Ряд дальше limit в выдачу не войдёт: перед ним limit рядов с оценкой не ниже,
	// а модификаторы при равенстве встают за ними.
	keep := limit
	if keep <= 0 {
		keep = -1
	}
	rows, _ := pc.rows(asp, pc.own, keep)
	if len(rows) > 0 {
		rows = append(rows, pc.variants(aspects, rows[0])...)
		// Ничьи (tieQ) — порядок построения, а не последние биты (решение 95).
		slices.SortStableFunc(rows, func(a, b pageRow) int { return desc(tieQ(a.score), tieQ(b.score)) })
	}
	return rows[:min(len(rows), limit)]
}

func (pc *pageCtx) rearranged(aspects []float64, p Params, limit int, base []pageRow) ([]Result, Block) {
	own := make([]float64, len(pc.own))
	for i, id := range pc.own {
		own[i] = aspects[id]
	}
	cls, rep := classes(own)
	if len(rep) == 1 {
		return nil, BlockSame
	}
	if len(pc.own) > maxPermPage {
		return nil, BlockMany
	}
	picks := make([][]int, len(rep))
	cnt := make([]int, len(rep))
	for i, c := range cls {
		picks[c] = append(picks[c], pc.own[i])
		cnt[c]++
	}
	pageScore := map[string]float64{}
	perm := map[string]bool{}
	var all []Result
	used := 0
	eachSeq(cnt, func(seq []int) bool {
		if used >= permBudget {
			return false
		}
		asp := make([]float64, len(seq))
		for i, c := range seq {
			asp[i] = rep[c]
		}
		// Все: в finish форма с меньшим числом инверсий может вытеснить лучшую по оценке.
		rows, trees := pc.rows(asp, assign(seq, picks), -1)
		used += trees
		for _, r := range rows {
			pageScore[r.res.Key] = r.score
			perm[r.res.Key] = r.perm
			all = append(all, r.res)
		}
		return true
	})
	var baseRes []Result
	for _, r := range base {
		pageScore[r.res.Key] = r.score
		baseRes = append(baseRes, r.res)
	}
	// Форма и инверсии — по ключу всего разворота: вторая страница у всех строк одна,
	// так что различаются они только этой страницей. Тождественность — по самой странице.
	by := func(r Result) float64 {
		if s, ok := pageScore[r.Key]; ok {
			return s
		}
		return math.Inf(-1)
	}
	out := finish(all, baseRes, aspects, p, limit, by, func(k string) bool { return perm[k] })
	return out, BlockNone
}
