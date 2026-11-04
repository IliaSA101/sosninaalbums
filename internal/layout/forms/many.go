package forms

// Формы многих фото (спека 072, 17–40 на развороте со страницами): «Ряды» с разрезом DP до 6
// рядов, «сетка + главное», «две крупных» и отбор самого крупного из почти одинаковых вариантов.
// Ключи — прежняя грамматика (Q[G|G], P[V(H…)|…]): layout.Build не меняется.

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

const (
	manyMax       = rowsMax // до стольких фото — генераторы многих фото (= предел рядов)
	manyRowsMax   = 6       // рядов на странице у «Рядов» и у страницы рядом с главным
	rowsPageMin   = 4       // фото на странице «Рядов»: 2 ряда по 2
	heroBlockMax  = 4       // фото в каждом из двух рядов рядом с главным
	heroRowsMax   = 5       // рядов под (над) блоком главного
	heroPlans     = 3       // страниц с главным в Build у «сетки + главное»
	pairPlans     = 2       // то же у «две крупных» на левой странице (на правой — лучшая)
	heroPageMin   = 6       // фото на странице с главным и рядом с ней — не меньше
	heroManyRatio = 3.0     // главное на многих фото: площадь ≥ стольких медиан прочих (072-4)
	keepShare     = 0.97    // из вариантов генератора — не мельче этой доли самого крупного (072-6)
)

// many — генераторы многих фото: нужны страницы и ManyFrom…manyMax фото.
func (g *gen) many() bool {
	return g.pages && g.k >= layout.ManyFrom && g.k <= manyMax
}

// manyRows — «Ряды» на многих фото (072-5): Q[G|G] с разрезом страниц у середины ± 2 и рядами DP,
// 2–6 рядов, на страницах поровну или на один больше. Общая высота фото обеих страниц (Q) — ряд
// читается через разворот. Из вариантов остаются самые крупные. Разрез — не ближе rowsPageMin к
// краю ленты: панорама длиннее остальных первым или последним фото ставит c0 у края (ревью 072).
func (g *gen) manyRows() {
	k := g.k
	from := len(g.out)
	c0 := cutNear(g.pre, 0, k, 0.5)
	for c := max(c0-2, rowsPageMin); c <= min(c0+2, k-rowsPageMin); c++ {
		for rl := 2; rl <= manyRowsMax; rl++ {
			for rr := max(2, rl-1); rr <= min(manyRowsMax, rl+1); rr++ {
				if l, r := g.part(0, c, rl, 2), g.part(c, k, rr, 2); l != nil && r != nil {
					g.try(pairKey("", gKey(l), gKey(r)), noShift)
				}
			}
		}
	}
	g.keepLargest(from)
}

// area — область страницы внутри полей.
func (g *gen) area() (w, h float64) {
	return g.p.PageW - 2*g.p.Margin, g.p.PageH - 2*g.p.Margin
}

// minIn — мельчайшая сторона фото lo…hi−1 высотой rh (ширина = rh·a).
func (g *gen) minIn(lo, hi int, rh float64) float64 {
	m := math.Inf(1)
	for j := lo; j < hi; j++ {
		m = math.Min(m, rh*math.Min(1, g.as[j]))
	}
	return m
}

// rowH — высота ряда фото lo…hi−1 во всю ширину w.
func (g *gen) rowH(lo, hi int, w float64) float64 {
	return (w - g.p.Gap*float64(hi-lo-1)) / (g.pre[hi] - g.pre[lo])
}

// estV — мельчайшая сторона у V(H…) из рядов b в области w×h, без Build: ряды во всю ширину,
// столбец сжимается по высоте.
func (g *gen) estV(b []int, w, h float64) float64 {
	tot := g.p.Gap * float64(len(b)-2)
	for i := 1; i < len(b); i++ {
		tot += g.rowH(b[i-1], b[i], w)
	}
	sc := math.Min(1, h/tot)
	m := math.Inf(1)
	for i := 1; i < len(b); i++ {
		m = math.Min(m, g.minIn(b[i-1], b[i], g.rowH(b[i-1], b[i], w)*sc))
	}
	return m
}

// estG — то же у G: высота всех рядов общая — меньшая из «поровну по высоте» и самого длинного ряда.
func (g *gen) estG(b []int, w, h float64) float64 {
	r := float64(len(b) - 1)
	hh := (h - g.p.Gap*(r-1)) / r
	for i := 1; i < len(b); i++ {
		hh = math.Min(hh, g.rowH(b[i-1], b[i], w))
	}
	return g.minIn(b[0], b[len(b)-1], hh)
}

// otherPage — страница рядом со страницей с главным: лучшие по мельчайшей стороне G и V(H…) из
// 2–6 рядов (кэш по lo, hi — нужна многим планам главного).
func (g *gen) otherPage(lo, hi int) []string {
	key := [2]int{lo, hi}
	if v, ok := g.others[key]; ok {
		return v
	}
	w, h := g.area()
	var bg, bv string
	mg, mv := 0.0, 0.0
	for r := 2; r <= manyRowsMax; r++ {
		if b := g.part(lo, hi, r, 2); b != nil {
			if e := quanta(g.estG(b, w, h)); e > mg {
				mg, bg = e, gKey(b)
			}
		}
		if b := g.part(lo, hi, r, 1); b != nil {
			if e := quanta(g.estV(b, w, h)); e > mv {
				mv, bv = e, list('V', rowsOf(b))
			}
		}
	}
	var out []string
	for _, s := range []string{bg, bv} {
		if s != "" {
			out = append(out, s)
		}
	}
	if g.others == nil {
		g.others = map[[2]int][]string{}
	}
	g.others[key] = out
	return out
}

// heroPlan — план страницы с главным: оценка без Build, ключ строится только у отобранных (строки
// ключей всех планов — заметная доля времени Gen). Блок рядом с главным — фото b1…b2−1 и b2…b3−1.
type heroPlan struct {
	est        float64
	b1, b2, b3 int
	rows       []int // границы рядов под (над) блоком
}

// heroKey — страница с главным: фото lo…hi−1, главное первое или последнее.
type heroKey struct {
	lo, hi int
	first  bool
}

// heroAt — главное страницы lo…hi−1: первое (first) или последнее её фото.
func heroAt(lo, hi int, first bool) int {
	if first {
		return lo
	}
	return hi - 1
}

// heroPage — до top (≤ heroPlans) лучших страниц lo…hi−1 с главным первым (first) или последним
// фото страницы; запоминаются в gen — «две крупных» берут те же страницы, что «сетка + главное».
func (g *gen) heroPage(lo, hi int, first bool, top int) []string {
	key := heroKey{lo, hi, first}
	out, ok := g.heroes[key]
	if !ok {
		out = g.heroPages(lo, hi, first)
		if g.heroes == nil {
			g.heroes = map[heroKey][]string{}
		}
		g.heroes[key] = out
	}
	return out[:min(top, len(out))]
}

// heroPages — heroPlans лучших страниц: V(H(главное, V(H(a), H(b))), ряды…) — главное высотой в
// блок из двух рядов, ниже ряды; у последнего — зеркально (ряды сверху, блок слева от главного).
// Оценка — мельчайшая сторона фото без главного, ничья — план раньше в переборе.
func (g *gen) heroPages(lo, hi int, first bool) []string {
	hp := heroAt(lo, hi, first)
	var best topPlans
	for a := 1; a <= heroBlockMax; a++ {
		for b := 1; b <= heroBlockMax; b++ {
			bl, ok := g.heroBlockOf(lo, hi, hp, first, a, b)
			if !ok {
				continue
			}
			for r := 1; r <= heroRowsMax; r++ {
				if rb := g.part(bl.rlo, bl.rhi, r, 1); rb != nil {
					if est, ok := g.heroEst(bl, rb); ok {
						best.add(heroPlan{est, bl.b1, bl.b2, bl.b3, rb})
					}
				}
			}
		}
	}
	out := make([]string, best.n)
	for i, pl := range best.p[:best.n] {
		blk := list('V', []string{hrow(pl.b1, pl.b2), hrow(pl.b2, pl.b3)})
		rows := rowsOf(pl.rows)
		if first {
			out[i] = list('V', append([]string{"H(" + leaf(hp) + "," + blk + ")"}, rows...))
		} else {
			out[i] = list('V', append(rows, "H("+blk+","+leaf(hp)+")"))
		}
	}
	return out
}

// topPlans — heroPlans лучших планов по убыванию оценки; равная — после прежних (как у стабильной
// сортировки всех планов).
type topPlans struct {
	p [heroPlans]heroPlan
	n int
}

func (t *topPlans) add(pl heroPlan) {
	j := t.n
	for j > 0 && t.p[j-1].est < pl.est {
		j--
	}
	if j < len(t.p) {
		copy(t.p[j+1:], t.p[j:])
		t.p[j] = pl
		t.n = min(t.n+1, len(t.p))
	}
}

// heroBlock — главное hp и блок из двух рядов рядом (a и b фото) на странице; ряды — rlo…rhi−1.
type heroBlock struct {
	hp, b1, b2, b3, rlo, rhi int
	s1, s2, wb, hb           float64 // суммы пропорций рядов блока, ширина и высота блока
	hero                     float64 // площадь главного до масштаба страницы
	big                      top2    // две наибольшие площади блока
}

// heroBlockOf — блок главного; false — фото на ряды не хватает или панорама-главное не оставляет
// блоку ширины.
func (g *gen) heroBlockOf(lo, hi, hp int, first bool, a, b int) (heroBlock, bool) {
	if hi-lo-1-a-b < 1 {
		return heroBlock{}, false
	}
	bl := heroBlock{hp: hp, b1: hp + 1, b2: hp + 1 + a, b3: hp + 1 + a + b, rlo: hp + 1 + a + b, rhi: hi}
	if !first {
		bl.b1, bl.b2, bl.b3, bl.rlo, bl.rhi = hp-a-b, hp-b, hp, lo, hp-a-b
	}
	w, _ := g.area()
	gap, ah := g.p.Gap, g.as[hp]
	bl.s1, bl.s2 = g.pre[bl.b2]-g.pre[bl.b1], g.pre[bl.b3]-g.pre[bl.b2]
	inv := 1/bl.s1 + 1/bl.s2
	// главное и блок в одну высоту: ah·hb + gap + wb = w, hb = wb·inv + gap
	bl.wb = (w - gap - ah*gap) / (1 + ah*inv)
	if !(bl.wb > 0) {
		return heroBlock{}, false
	}
	bl.hb = bl.wb*inv + gap
	// площади до масштаба страницы: сравнение главного с прочими от него не зависит
	bl.hero = ah * bl.hb * bl.hb
	g.addAreas(&bl.big, bl.b1, bl.b2, bl.wb/bl.s1)
	g.addAreas(&bl.big, bl.b2, bl.b3, bl.wb/bl.s2)
	return bl, true
}

// heroEst — оценка плана (блок и ряды rb): мельчайшая сторона фото без главного в квантах; false —
// главное по геометрии (Heroes) — не оно и не одно из двух крупных: ряд из одного-двух фото во всю
// ширину выходит крупнее, и главным становится он, а не угол страницы (heroLike).
func (g *gen) heroEst(bl heroBlock, rb []int) (float64, bool) {
	w, hgt := g.area()
	var hs [manyRowsMax]float64
	tot, big := bl.hb, bl.big
	for i := 1; i < len(rb); i++ {
		hs[i-1] = g.rowH(rb[i-1], rb[i], w)
		tot += hs[i-1] + g.p.Gap
		g.addAreas(&big, rb[i-1], rb[i], hs[i-1])
	}
	if !heroLike(bl.hero, big) {
		return 0, false
	}
	sc := math.Min(1, hgt/tot)
	m := math.Min(g.minIn(bl.b1, bl.b2, bl.wb/bl.s1*sc), g.minIn(bl.b2, bl.b3, bl.wb/bl.s2*sc))
	for i := 1; i < len(rb); i++ {
		m = math.Min(m, g.minIn(rb[i-1], rb[i], hs[i-1]*sc))
	}
	return quanta(m), true
}

// top2 — две наибольшие площади по убыванию.
type top2 [2]float64

// addAreas — площади фото lo…hi−1 высотой rh в t.
func (g *gen) addAreas(t *top2, lo, hi int, rh float64) {
	for j := lo; j < hi; j++ {
		switch a := rh * rh * g.as[j]; {
		case a > t[0]:
			t[0], t[1] = a, t[0]
		case a > t[1]:
			t[1] = a
		}
	}
}

// heroLike — главное площадью hero при прочих фото страницы others останется главным по Heroes:
// одно (≥ HeroRatio крупнейшего) или одно из двух крупных (оба ≥ HeroRatio третьего).
func heroLike(hero float64, others top2) bool {
	return hero >= HeroRatio*others[0] || math.Min(hero, others[0]) >= HeroRatio*others[1]
}

// tryHeroes — try; раскладка остаётся, если задуманное главное (у «две крупных» — хоть одно из
// задуманных) — главное и по геометрии (Heroes): одно или одно из пары. Оценка плана без Build и
// страница рядом (ряд из одного фото во всю ширину) могут дать фото крупнее задуманного, и тогда
// главное — не в углу страницы. Пара из главного и соседа по блоку (оба крупные, в углу) — вид
// «Две крупных», как его и назовёт Of.
func (g *gen) tryHeroes(key string, hs ...int) {
	n := len(g.out)
	if g.try(key, noShift); len(g.out) == n {
		return
	}
	h := Heroes(g.out[n].Rects)
	ok := false
	for _, i := range hs {
		ok = ok || h.isHero(i)
	}
	if h.N == 1 && !slices.Contains(hs, h.Idx[0]) {
		ok = false
	}
	if !ok {
		g.out = g.out[:n]
	}
}

// gridHero — «сетка + главное» (072-3): главное на два ряда в углу одной страницы (первое или
// последнее её фото), рядом ряды; другая страница — ряды. У страницы с главным на 0–2 фото меньше.
func (g *gen) gridHero() {
	from := len(g.out)
	c0 := cutNear(g.pre, 0, g.k, 0.5)
	for _, left := range []bool{true, false} {
		for _, first := range []bool{true, false} {
			for d := range 3 {
				c := c0 + d
				if left {
					c = c0 - d
				}
				if c >= heroPageMin && c <= g.k-heroPageMin {
					g.heroWith(c, left, first)
				}
			}
		}
	}
	g.keepLargest(from, Hero, Pair)
}

// heroWith — раскладки с главным на левой (left) или правой странице при разрезе c.
func (g *gen) heroWith(c int, left, first bool) {
	lo, hi, olo, ohi := 0, c, c, g.k
	if !left {
		lo, hi, olo, ohi = c, g.k, 0, c
	}
	for _, hp := range g.heroPage(lo, hi, first, heroPlans) {
		for _, op := range g.otherPage(olo, ohi) {
			key := pagedKey(hp, op)
			if !left {
				key = pagedKey(op, hp)
			}
			g.tryHeroes(key, heroAt(lo, hi, first))
		}
	}
}

// gridPair — «две крупных» (072-3): по главному на каждой странице — у внешних краёв, у корешка или
// по диагонали; разрез страниц — у середины ± 1. Планов слева pairPlans, справа — лучший: у 2×2
// (прототип) до 208 Build на разворот при бюджете 200, а формы ряда «Авто» те же. Замер 072 (32
// набора): 2 разреза вместо 3 — на 30 фото ≥ 4 форм 14 наборов вместо 18; 1×1 и 1×2 — на 28 фото
// ≥ 3 форм 29 вместо 32.
func (g *gen) gridPair() {
	k := g.k
	from := len(g.out)
	c0 := cutNear(g.pre, 0, k, 0.5)
	for _, d := range [...]int{0, -1, 1} {
		c := c0 + d
		if c < heroPageMin || c > k-heroPageMin {
			continue
		}
		for _, lf := range []bool{true, false} {
			for _, rf := range []bool{true, false} {
				rs := g.heroPage(c, k, rf, 1)
				for _, l := range g.heroPage(0, c, lf, pairPlans) {
					for _, r := range rs {
						g.tryHeroes(pagedKey(l, r), heroAt(0, c, lf), heroAt(c, k, rf))
					}
				}
			}
		}
	}
	g.keepLargest(from, Hero, Pair)
}

// sideOf — мельчайшая сторона фото раскладки; у «С главным» и «Две крупных» — без главных.
func sideOf(c Cand) float64 {
	h := Heroes(c.Rects)
	skip := c.Form == Hero || c.Form == Pair
	m := math.Inf(1)
	for i, r := range c.Rects {
		if !skip || !h.isHero(i) {
			m = math.Min(m, math.Min(r.W, r.H))
		}
	}
	return m
}

// keepLargest — из g.out[from:] остаются варианты видов only (нет — любого) не мельче keepShare
// самого крупного из них (072-6): Q внутри полосы доли разворота к размеру безразличен, и ряд «Авто»
// брал ряды «с воздухом» вдвое мельче рядов во всю высоту. Вид чужой генератору (у страницы с
// главным оно не вышло крупнее соседей — мозаика) отбрасывается: его мельчайшая сторона считается
// по всем фото и срезала бы настоящие раскладки с главным. Порядок остальных — прежний.
func (g *gen) keepLargest(from int, only ...Form) {
	if len(g.out) <= from {
		return
	}
	sides := make([]float64, len(g.out)-from)
	best := 0.0
	for i, c := range g.out[from:] {
		sides[i] = math.Inf(-1)
		if len(only) == 0 || slices.Contains(only, c.Form) {
			sides[i] = sideOf(c)
			best = math.Max(best, sides[i])
		}
	}
	floor := quanta(keepShare * best)
	n := from
	for i, c := range g.out[from:] {
		if !math.IsInf(sides[i], -1) && quanta(sides[i]) >= floor {
			g.out[n] = c
			n++
		}
	}
	g.out = g.out[:n]
}

// manyHeroOK — у «С главным» и «Две крупных» на ManyFrom…manyMax фото со страницами главное проходит
// и по соседям (072-4, design §2.5): площадь главного (у пары — второго крупного) ≥ heroManyRatio
// медиан прочих. Доля 0,30 площади там оставляла прочим меньше пола; «или» к доле — всё, что
// проходило, проходит. «Через корешок», лист и 41+ фото — только по доле: генераторов многих фото
// там нет, и правило лишь сдвигало бы прежние раскладки (ревью 072).
func manyHeroOK(rs []layout.Rect, f Form, p layout.Params, h HeroInfo) bool {
	if f != Hero && f != Pair || p.Sheet || len(rs) < layout.ManyFrom || len(rs) > manyMax || h.N == 0 {
		return false
	}
	as := make([]float64, 0, len(rs))
	for i, x := range rs {
		if !h.isHero(i) {
			as = append(as, x.W*x.H)
		}
	}
	slices.Sort(as)
	return quanta(h.Top[h.N-1]/as[len(as)/2]) >= quanta(heroManyRatio)
}
