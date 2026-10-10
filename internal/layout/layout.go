// Package layout раскладывает фотографии по развороту альбома.
//
// Идея: фото идут в заданном порядке, и каждую раскладку можно описать
// деревом. Лист — фото, внутренний узел ставит детей либо в ряд, либо
// в столбик. Для любого такого дерева размеры считаются точно, без
// обрезки фото: высота узла линейно зависит от его ширины (h = a*w + b),
// и эти коэффициенты складываются снизу вверх. Перебираем деревья,
// оцениваем каждое и возвращаем лучшие варианты.
package layout

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Rect — прямоугольник в миллиметрах от левого верхнего угла разворота.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Params — геометрия разворота.
type Params struct {
	PageW   float64 // ширина одной страницы, мм
	PageH   float64 // высота страницы, мм
	Margin  float64 // поле по краю страницы, мм
	Gap     float64 // промежуток между фото, мм
	Layflat bool    // фото может идти через корешок
}

// Result — один вариант раскладки. Rects идут в порядке входных фото.
type Result struct {
	Rects []Rect
	Score float64
	Key   string
}

// Сколько фото перебираем всеми деревьями. Дальше число деревьев
// растёт слишком быстро, и используем только ряды и колонки.
const maxFullTree = 7

// MaxPhotos — больше фото на один разворот не раскладываем: Layouts на 16 фото
// ≈ 35 мс (худший набор, холодный кэш; до 020 — 100–125), на 20 до 020 было ≈ 350 мс, дальше растёт
// быстро, а View считает раскладку каждого разворота. Выше предела Layouts,
// Rearranged, PageChoices и Build отказывают сразу, без счёта.
const MaxPhotos = 16

// Layouts возвращает до limit лучших раскладок в порядке ленты, лучшая первая.
// aspects — отношение ширины к высоте каждого фото.
func Layouts(aspects []float64, p Params, limit int) []Result {
	n := len(aspects)
	if n == 0 || n > MaxPhotos || p.PageW <= 0 || p.PageH <= 0 {
		return nil
	}
	g := newGeom(p)
	var all []Result

	// 1. Блок на весь разворот — только с layflat: иначе фото не должны касаться корешка.
	// Нужны только блоки, способные войти в первые limit различных ключей; в all они
	// идут в порядке перебора, как раньше, — тогда итоговая сортировка та же.
	// Равенство с прежним порядком доказано для конечных оценок; для конечных пропорций
	// они конечны: aspectOf > 0 и place ≥ 1 мм.
	if p.Layflat {
		keep := limit
		if keep <= 0 {
			keep = -1 // topUnique при limit 0 отдаёт все
		}
		cs, _ := bestBlocks(aspects, g.spread, p.Gap, g.spine, keep, true)
		slices.SortFunc(cs, func(a, b candidate) int { return cmp.Compare(a.tree, b.tree) })
		for _, c := range cs {
			all = append(all, c.Result)
		}
	}

	// 2. Отдельно левая и правая страница: первые k фото слева, остальные справа.
	for k := 0; k <= n; k++ {
		ls := topBlocks(aspects[:k], g.pages[0], p.Gap, perSide)
		rs := topBlocks(aspects[k:], g.pages[1], p.Gap, perSide)
		for i := range rs {
			// Позиции в ключе сквозные по развороту.
			rs[i].Key = mapLeaves(rs[i].Key, func(j int) string { return strconv.Itoa(j + k) })
		}
		for _, l := range ls {
			for _, r := range rs {
				rects := append(append([]Rect{}, l.Rects...), r.Rects...)
				s := pairScore(rects, g, max(l.depth, r.depth), l.height, r.height, k == 0 || k == n)
				all = append(all, Result{rects, s, "P[" + l.Key + "|" + r.Key + "]"})
			}
		}
	}

	// 3. Фото в край, с обрезкой: на весь разворот или по странице.
	if n == 1 {
		if p.Layflat {
			all = appendBleed(all, aspects, []Rect{g.full}, g, "B[full:0]")
		}
		all = appendBleed(all, aspects, []Rect{g.bleed[1]}, g, "B[right:0]")
	}
	if n == 2 {
		all = appendBleed(all, aspects, []Rect{g.bleed[0], g.bleed[1]}, g, "B[pages:0,1]")
	}
	return topUnique(all, limit)
}

// perSide — сколько лучших блоков каждой страницы идёт в пары.
const perSide = 4

// topUnique сортирует по оценке и оставляет limit лучших без повторов ключа.
func topUnique(all []Result, limit int) []Result {
	slices.SortStableFunc(all, func(a, b Result) int { return desc(a.Score, b.Score) })
	seen := map[string]bool{}
	out := make([]Result, 0, limit)
	for _, r := range all {
		if seen[r.Key] {
			continue
		}
		seen[r.Key] = true
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out
}

func appendBleed(all []Result, aspects []float64, rects []Rect, g geom, key string) []Result {
	if r, ok := bleedResult(rects, aspects, g, key); ok {
		all = append(all, r)
	}
	return all
}

// candidate — вариант блока до общей оценки.
type candidate struct {
	Result
	depth  int
	height float64
	tree   int // номер дерева в переборе
}

// topBlocks — лучшие блоки для одной страницы. Пустая страница — один пустой вариант.
func topBlocks(aspects []float64, box Rect, gap float64, limit int) []candidate {
	if len(aspects) == 0 {
		return []candidate{{Result: Result{Key: "-"}}}
	}
	cs, _ := bestBlocks(aspects, box, gap, -1, limit, false)
	return cs[:min(len(cs), limit)]
}

// desc — сравнение для сортировки по убыванию. Равные и NaN дают 0 — ровно less
// из прежнего sort.SliceStable, а алгоритм у slices.SortStableFunc тот же, так что
// и порядок тот же.
func desc(a, b float64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	}
	return 0
}

// bestBlocks — то же, что blockCandidates + score(box, spine) + устойчивая сортировка
// по убыванию оценки, но прямоугольники и ключ строятся только у первых keep
// (keep < 0 — у всех; unique — первых keep различных ключей). Деревья перебора
// десятки тысяч, а нужны первые 4–24. Числа те же: fitTree и score те же.
// При NaN в оценке отдаёт все: порядок сортировки с NaN зависит от всего списка,
// и отсекать нельзя (из настоящих пропорций NaN не бывает).
func bestBlocks(aspects []float64, box Rect, gap, spine float64, keep int, unique bool) (out []candidate, trees int) {
	n := len(aspects)
	roots := newEnum(aspects, gap).roots(n)
	type scored struct { // без указателей: сортировка дешёвая
		s, h float64
		i    int
	}
	buf := make([]Rect, n)
	list := make([]scored, 0, len(roots))
	for i, t := range roots {
		h, ok := fitTree(t, box, gap, buf)
		if !ok {
			continue
		}
		s := score(buf, box, spine, t.depth)
		if math.IsNaN(s) {
			keep, unique = -1, false
		}
		list = append(list, scored{s, h, i})
	}
	slices.SortStableFunc(list, func(a, b scored) int { return desc(a.s, b.s) })
	var seen map[string]bool
	if unique {
		seen = map[string]bool{}
	}
	for _, x := range list {
		if len(out) == keep {
			break
		}
		t := roots[x.i]
		key := t.keyStr()
		if unique {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		rects := make([]Rect, n)
		fitTree(t, box, gap, rects)
		out = append(out, candidate{Result{rects, x.s, key}, t.depth, x.h, x.i})
	}
	return out, len(roots)
}

func newEnum(aspects []float64, gap float64) *enumerator {
	e := &enumerator{gap: gap, memo: map[[3]int][]*node{}}
	for i, a := range aspects {
		e.leaves = append(e.leaves, &node{leaf: i, a: 1 / a, key: strconv.Itoa(i)})
	}
	return e
}

// roots — все деревья перебора над n фото.
func (e *enumerator) roots(n int) []*node {
	switch {
	case n == 1:
		return e.leaves // иначе один и тот же лист приходит и как ряд, и как столбец
	case n <= maxFullTree:
		return append(e.trees(0, n, true), e.trees(0, n, false)...)
	default:
		return append(e.lines(n, false, 4), e.lines(n, true, 6)...)
	}
}

// blockCandidates перебирает деревья и вписывает каждое в box по центру.
// trees — сколько деревьев перебрано (на это опирается бюджет перестановок).
func blockCandidates(aspects []float64, box Rect, gap float64) (out []candidate, trees int) {
	n := len(aspects)
	roots := newEnum(aspects, gap).roots(n)
	out = make([]candidate, 0, len(roots))
	for i, t := range roots {
		rects := make([]Rect, n)
		h, ok := fitTree(t, box, gap, rects)
		if !ok {
			continue
		}
		out = append(out, candidate{Result{Rects: rects, Key: t.keyStr()}, t.depth, h, i})
	}
	return out, len(roots)
}

type node struct {
	leaf  int // индекс фото; -1 у составного узла
	horiz bool
	kids  []*node
	a, b  float64 // высота при ширине w: h = a*w + b
	depth int
	key   string // у составного узла — пусто до keyStr
}

// keyStr — ключ узла. Строится по требованию: у большинства деревьев перебора он не нужен.
func (n *node) keyStr() string {
	if n.key == "" {
		var b strings.Builder
		n.writeKey(&b)
		n.key = b.String()
	}
	return n.key
}

func (n *node) writeKey(b *strings.Builder) {
	if n.key != "" {
		b.WriteString(n.key)
		return
	}
	if n.horiz {
		b.WriteString("H(")
	} else {
		b.WriteString("V(")
	}
	for i, k := range n.kids {
		if i > 0 {
			b.WriteByte(',')
		}
		k.writeKey(b)
	}
	b.WriteByte(')')
}

func compose(horiz bool, kids []*node, gap float64) *node {
	n := &node{leaf: -1, horiz: horiz, kids: kids}
	g := gap * float64(len(kids)-1)
	for _, k := range kids {
		n.depth = max(n.depth, k.depth+1)
	}
	if horiz {
		// Все дети одной высоты h, ширина ребёнка (h-b)/a.
		var inv, sb float64
		for _, k := range kids {
			inv += 1 / k.a
			sb += k.b / k.a
		}
		n.a = 1 / inv
		n.b = n.a * (sb - g)
	} else {
		// Все дети одной ширины w, высоты складываются.
		for _, k := range kids {
			n.a += k.a
			n.b += k.b
		}
		n.b += g
	}
	return n
}

// place раскладывает узел в заданный прямоугольник. false — если фото
// получились меньше миллиметра (слишком много промежутков в тесном месте).
func (n *node) place(x, y, w, h, gap float64, out []Rect) bool {
	if w < 1 || h < 1 {
		return false
	}
	if n.leaf >= 0 {
		out[n.leaf] = Rect{x, y, w, h}
		return true
	}
	for _, k := range n.kids {
		if n.horiz {
			kw := (h - k.b) / k.a
			if !k.place(x, y, kw, h, gap, out) {
				return false
			}
			x += kw + gap
		} else {
			kh := k.a*w + k.b
			if !k.place(x, y, w, kh, gap, out) {
				return false
			}
			y += kh + gap
		}
	}
	return true
}

type enumerator struct {
	leaves []*node
	gap    float64
	memo   map[[3]int][]*node
}

// trees возвращает все деревья над фото [i, j) с корнем заданной ориентации.
// Ориентация чередуется по уровням: ряд внутри ряда — это просто более длинный ряд.
func (e *enumerator) trees(i, j int, horiz bool) []*node {
	if j-i == 1 {
		return []*node{e.leaves[i]}
	}
	key := [3]int{i, j, 0}
	if horiz {
		key[2] = 1
	}
	if r, ok := e.memo[key]; ok {
		return r
	}
	var res []*node
	var parts [][]*node
	var rec func(start int)
	rec = func(start int) {
		if start == j {
			if len(parts) >= 2 {
				e.product(parts, horiz, &res)
			}
			return
		}
		for end := start + 1; end <= j; end++ {
			if start == i && end == j {
				continue // одна часть — это не разбиение
			}
			parts = append(parts, e.trees(start, end, !horiz))
			rec(end)
			parts = parts[:len(parts)-1]
		}
	}
	rec(i)
	e.memo[key] = res
	return res
}

func (e *enumerator) product(parts [][]*node, horiz bool, res *[]*node) {
	idx := make([]int, len(parts))
	for {
		kids := make([]*node, len(parts))
		for p, i := range idx {
			kids[p] = parts[p][i]
		}
		*res = append(*res, compose(horiz, kids, e.gap))
		p := len(idx) - 1
		for p >= 0 {
			idx[p]++
			if idx[p] < len(parts[p]) {
				break
			}
			idx[p] = 0
			p--
		}
		if p < 0 {
			return
		}
	}
}

// lines — для большого числа фото: только ряды (или колонки) подряд идущих фото.
func (e *enumerator) lines(n int, columns bool, maxLines int) []*node {
	var res []*node
	var cuts []int
	var rec func(start int)
	rec = func(start int) {
		if start == n {
			var groups []*node
			prev := 0
			for _, c := range append(cuts, n) {
				if c-prev == 1 {
					groups = append(groups, e.leaves[prev])
				} else {
					groups = append(groups, compose(!columns, e.leaves[prev:c], e.gap))
				}
				prev = c
			}
			if len(groups) == 1 {
				res = append(res, groups[0])
			} else {
				res = append(res, compose(columns, groups, e.gap))
			}
			return
		}
		if len(cuts) >= maxLines-1 {
			rec(n)
			return
		}
		for end := start + 1; end <= n; end++ {
			if end < n {
				cuts = append(cuts, end)
				rec(end)
				cuts = cuts[:len(cuts)-1]
			} else {
				rec(n)
			}
		}
	}
	rec(0)
	return res
}

// spineEps — допуск на корешок, мм: фото, зашедшее за него меньше, не считается пересекающим.
const spineEps = 0.01

// score: чем больше, тем лучше. Главное — насколько плотно фото заполняют
// разворот; штрафы за сильный разброс размеров, мелкие фото, узкие полоски
// фото на другой стороне корешка и лишнюю вложенность.
func score(rects []Rect, box Rect, spine float64, depth int) float64 {
	if len(rects) == 0 {
		return -1
	}
	var sum float64
	minA, maxA := math.Inf(1), 0.0
	minSide := math.Inf(1)
	for _, r := range rects {
		a := r.W * r.H
		sum += a
		minA = math.Min(minA, a)
		maxA = math.Max(maxA, a)
		minSide = math.Min(minSide, math.Min(r.W, r.H))
	}
	s := sum / (box.W * box.H)
	s -= 0.05 * math.Log2(maxA/minA)
	if want := 0.22 * box.H; minSide < want && len(rects) > 1 {
		s -= 2 * (want - minSide) / box.H
	}
	if spine > 0 {
		for _, r := range rects {
			if r.X < spine-spineEps && r.X+r.W > spine+spineEps {
				part := math.Min(spine-r.X, r.X+r.W-spine) / r.W
				if part < 0.2 {
					s -= 0.4 // узкая полоска фото по ту сторону корешка
				} else {
					// Через корешок хорошо идут широкие кадры, узким — плохо:
					// сгиб попадает в середину сюжета. spine = ширине страницы.
					s -= 0.04 + 0.2*math.Max(0, 1-r.W/spine)
				}
			}
		}
	}
	s -= 0.015 * float64(depth)
	return s
}
