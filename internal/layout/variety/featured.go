package variety

// Часть 2 спеки 041 (design §8, решения 0.11 №30–36): раскладки с главным фото для разворотов
// 8–16 фото; с 051 — 5–40 (5–7: новый диапазон, 17–40: сетки G вместо рядов V(H…)). Оценка layout
// штрафует разброс площадей, и главное фото проигрывает сетке 0.2–0.5, поэтому лучшие из них
// вставляет в ряд «Авто» Row (051); остальные — только в «Другие варианты».
// Ключи — нынешняя грамматика, в порядке ленты; геометрия и оценка — layout.Build. В Layout ""
// не попадают: ключ пишет выбор сборки или подборка.

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Kind — семейство генератора featured; NoKind — раскладка не из него (ряд «Авто», воздух).
type Kind int8

// Семейства генератора (h — главное фото, G — сетка рядов или столбцов, m — первое фото правой страницы).
const (
	NoKind    Kind = iota
	KindPage       // одно крупное на странице: P[0|G], P[G|k−1], то же «в край»
	KindJoint      // крупное у стыка страниц: P[G|V(h,G)], P[V(G,h)|G], то же с H — любая позиция h
	KindSpine      // крупное через корешок, фото по бокам: H(Gv,h,Gv), только layflat
	KindProps      // подпорки по внешним краям страниц: P[H(0,G)|H(G,k−1)]
	KindPair       // две крупных + лента: P[V(0,G)|V(m,G)], снизу и по диагонали
)

const (
	// FeatMin — от стольких фото главное фото из ряда «Авто» не бывает: от них «Другие варианты» и Pick
	// берут featured (решения 0.11); генератор умеет и меньше — featLo.
	FeatMin = 8
	// featLo, featHi — на скольких фото работает генератор (051 G5: 5–7 и 17–40 — новое).
	featLo, featHi = 5, 40
	// heroCap — потолок лучшей оценки в допуске (решение №30): у одинаковых 12 фото сетка даёт
	// 0.89, главное по Fair — 0.66–0.74; без потолка главного на них не бывает.
	heroCap = 0.72
	// otherMinSide — мельчайшая сторона не-главных фото / высота страницы (жёсткий пол, №31).
	otherMinSide = 0.12
	// heroShare — доля главного в площади фото; у пары — каждого из двух (pairShare).
	heroShare = 0.30
	pairShare = 0.15
	// foldEps — как spineEps в layout: фото, лишь касающееся корешка, его не пересекает.
	foldEps = 0.01
	// sideSlack — главное «близко к странице» по пропорциям, если отличается не больше чем в 1/0.75 раза.
	sideSlack = 0.75
	// minPart — область сетки рядом с главным — не меньше этой доли страницы.
	minPart = 0.3
)

// Feat — раскладка генератора с измерениями.
type Feat struct {
	layout.Result
	Info
	Kind Kind
}

// paged — ключ P[left|right] (страница «в край» — "E(i)").
func paged(l, r string) string { return "P[" + l + "|" + r + "]" }

// gen — генератор одного разворота.
type gen struct {
	as     []float64
	p      layout.Params
	cw, ch float64 // область страницы внутри полей, мм
	sw     float64 // область разворота внутри полей по ширине, мм
	seen   map[string]bool
	memo   map[gridKey][]tn
	bufA   []float64 // пропорции сетки (grids), по числу фото
	bufP   []float64 // их префиксные суммы
	out    []Feat
	builds int  // вызовов Build (бюджет design §8 — ≤ 150 на разворот при 8–16 фото)
	many   bool // с ManyFrom фото страница-сетка — G (ряды одной высоты), а не V(H…)
}

// featured — раскладки с главным фото для разворота из 5–40 фото (иначе nil), прошедшие жёсткие
// запреты (решение №31): главное — по геометрии, его доля в площади, не-главные не мельче пола и не
// на сгибе, через сгиб — только широкое главное. Звезду не знает: «у стыка» строится для любой
// позиции, и «Другие варианты» (кэш по пропорциям) годятся для любой звезды.
func featured(as []float64, p layout.Params) []Feat {
	k := len(as)
	if k < featLo || k > featHi || p.Sheet || p.PageW <= 2*p.Margin || p.PageH <= 2*p.Margin {
		return nil
	}
	for _, a := range as {
		if !(a > 0) || math.IsInf(a, 0) {
			return nil
		}
	}
	g := newGen(as, p)
	g.run()
	return g.out
}

func newGen(as []float64, p layout.Params) *gen {
	return &gen{as: as, p: p, cw: p.PageW - 2*p.Margin, ch: p.PageH - 2*p.Margin, sw: 2*p.PageW - 2*p.Margin,
		many: len(as) >= layout.ManyFrom, seen: map[string]bool{}, memo: map[gridKey][]tn{}, bufA: make([]float64, len(as)), bufP: make([]float64, len(as)+1)}
}

// run — все семейства по порядку (порядок выдачи — порядок генерации; подборка сортирует сама).
func (g *gen) run() {
	g.page()
	g.joint()
	g.center()
	if g.p.Layflat {
		g.spine()
	}
	g.props()
	g.pair()
}

// root — запись сетки как корня страницы: с ManyFrom фото ряды V(H…) — G (одной высоты фото; корень G
// бывает только у страницы целиком), иначе — как есть.
func (g *gen) root(t tn) string {
	if !g.many || t.h || t.leaf >= 0 || len(t.kids) < 2 {
		return t.String()
	}
	rows := make([]string, len(t.kids))
	for i, r := range t.kids {
		if r.leaf < 0 && (!r.h || slices.ContainsFunc(r.kids, func(x tn) bool { return x.leaf < 0 })) {
			return t.String() // не ряды из фото
		}
		rows[i] = r.String()
	}
	return "G(" + strings.Join(rows, ",") + ")"
}

// try строит ключ и берёт его, если он проходит запреты (Describe — только у прошедших: вид
// дорогой, а отсеивается больше половины).
func (g *gen) try(k Kind, key string) {
	if g.seen[key] {
		return
	}
	g.seen[key] = true
	g.builds++
	r, ok := layout.Build(key, g.as, g.p)
	if ok && g.allowed(r.Rects) {
		g.out = append(g.out, Feat{Result: r, Info: Describe(r, g.p), Kind: k})
	}
}

// allowed — жёсткие запреты (решение №31): главное по геометрии есть и занимает свою долю площади,
// не-главные не мельче пола и не на сгибе. Через сгиб — по общему правилу layout.FoldOK
// (064: своей копии правила больше нет, главное — как любое фото); запрет «не-главное на сгибе» —
// композиция вида, он при любом положении сгиба (decisions 0.15 №16).
func (g *gen) allowed(rs []layout.Rect) bool {
	h := forms.Heroes(rs)
	if h.N == 0 || !layout.FoldOK(rs, g.as, g.p) {
		return false
	}
	share := heroShare
	if h.N == 2 {
		share = pairShare
	} else {
		h.Idx[1] = -1
	}
	sum := 0.0
	for _, x := range rs {
		sum += x.W * x.H
	}
	// Пороги — в квантах долей: у одинаковых фото сторона ложится точно на пол, и последний бит
	// (FMA на arm64) решал бы, пройдёт ли раскладка.
	spine := g.p.PageW
	for i, x := range rs {
		if h.Idx[0] == i || h.Idx[1] == i {
			if quanta(x.W*x.H/sum) < quanta(share) {
				return false
			}
			continue
		}
		if x.X < spine-foldEps && x.X+x.W > spine+foldEps || quanta(math.Min(x.W, x.H)/g.p.PageH) < quanta(otherMinSide) {
			return false
		}
	}
	return true
}

// page — одно крупное на странице, сетка на другой: первое фото слева или последнее справа,
// обычное и «в край».
func (g *gen) page() {
	k := len(g.as)
	for _, t := range g.grids(1, k, g.cw, g.ch, 2) {
		g.try(KindPage, paged("0", g.root(t)))
		g.try(KindPage, paged("E(0)", g.root(t)))
	}
	last := strconv.Itoa(k - 1)
	for _, t := range g.grids(0, k-1, g.cw, g.ch, 2) {
		g.try(KindPage, paged(g.root(t), last))
		g.try(KindPage, paged(g.root(t), "E("+last+")"))
	}
}

// joint — главное h у стыка страниц: первым на правой (над сеткой или слева от неё) или последним
// на левой. Достижима любая позиция 1…k−2 — звезда в середине ленты тоже может стать главной.
func (g *gen) joint() {
	k := len(g.as)
	for h := 1; h <= k-2; h++ {
		v, hz := g.sides(h)
		if l := g.grid(0, h, g.cw, g.ch); l != nil {
			if t, ok := g.under(h, h+1, k); ok && v {
				g.try(KindJoint, paged(g.root(*l), node(false, lf(h), t).String()))
			}
			if t, ok := g.beside(h, h+1, k); ok && hz {
				g.try(KindJoint, paged(g.root(*l), node(true, lf(h), t).String()))
			}
		}
		if r := g.grid(h+1, k, g.cw, g.ch); r != nil {
			if t, ok := g.under(h, 0, h); ok && v {
				g.try(KindJoint, paged(node(false, t, lf(h)).String(), g.root(*r)))
			}
			if t, ok := g.beside(h, 0, h); ok && hz {
				g.try(KindJoint, paged(node(true, t, lf(h)).String(), g.root(*r)))
			}
		}
	}
}

// sides — с какой стороны от главного h на его странице ставить сетку: над/под (во всю ширину)
// — если главное не уже страницы, слева/справа (во всю высоту) — если не шире; у близких к
// странице пропорций — обе. Иначе главное сжимается, и раскладка заведомо проигрывает: Build
// на неё не тратим (бюджет design §8).
func (g *gen) sides(h int) (under, beside bool) {
	r := g.as[h] / (g.cw / g.ch)
	return r >= sideSlack, r <= 1/sideSlack
}

// center — главное h посередине своей страницы (страницы делятся пополам): сетки фото до и после
// него сверху и снизу или слева и справа; другая страница — сетка. Вместе с joint звезда
// почти в любом месте ленты может стать главной.
func (g *gen) center() {
	k := len(g.as)
	for _, m := range halves(k) {
		for h := 1; h < k-1; h++ {
			g.centerAt(h, m)
		}
	}
}

// centerAt — главное h посередине своей страницы при правой странице с фото m.
func (g *gen) centerAt(h, m int) {
	k := len(g.as)
	lo, hi, olo, ohi := 0, m, m, k
	if h >= m {
		lo, hi, olo, ohi = m, k, 0, m
	}
	if h == lo || h == hi-1 {
		return // у края страницы — это joint и page
	}
	other := g.grid(olo, ohi, g.cw, g.ch)
	if other == nil {
		return
	}
	nb, na := float64(h-lo), float64(hi-h-1)
	v, hz := g.sides(h)
	for _, horiz := range [2]bool{false, true} {
		var b, a *tn
		switch {
		case horiz && hz:
			restW := max(g.cw-g.ch*g.as[h]-2*g.p.Gap, minPart*g.cw)
			b, a = g.grid(lo, h, restW*nb/(nb+na), g.ch), g.grid(h+1, hi, restW*na/(nb+na), g.ch)
		case !horiz && v:
			restH := max(g.ch-g.cw/g.as[h]-2*g.p.Gap, minPart*g.ch)
			b, a = g.grid(lo, h, g.cw, restH*nb/(nb+na)), g.grid(h+1, hi, g.cw, restH*na/(nb+na))
		}
		if b == nil || a == nil {
			continue
		}
		page := node(horiz, *b, lf(h), *a).String()
		if h < m {
			g.try(KindJoint, paged(page, g.root(*other)))
		} else {
			g.try(KindJoint, paged(g.root(*other), page))
		}
	}
}

// spine — главное s через корешок, по бокам столбцы (≥ 2 фото с каждой стороны); только
// широкие кадры при любом положении сгиба: это вид (портрет посередине — не «главное»), и Build
// на узкие не тратим (бюджет design §8). Пропускает ли сгиб — решает allowed.
func (g *gen) spine() {
	k := len(g.as)
	for s := 2; s <= k-3; s++ {
		if g.as[s] < layout.FoldAspect {
			continue
		}
		side := (g.sw - g.ch*g.as[s] - 2*g.p.Gap) / 2
		if side <= 0 {
			continue
		}
		l, r := g.grid(0, s, side, g.ch), g.grid(s+1, k, side, g.ch)
		if l != nil && r != nil {
			g.try(KindSpine, node(true, *l, lf(s), *r).String())
		}
	}
}

// props — подпорки: первое и последнее фото крупно у внешних краёв, сетки между ними; постранично
// (на весь разворот мелкие ложатся на сгиб в 80–90 % случаев, решение №32).
func (g *gen) props() {
	k := len(g.as)
	for _, m := range halves(k) {
		l, ok1 := g.beside(0, 1, m)
		r, ok2 := g.beside(k-1, m, k-1)
		if ok1 && ok2 {
			g.try(KindProps, paged(node(true, lf(0), l).String(), node(true, r, lf(k-1)).String()))
		}
	}
}

// pair — две крупных, по одной на странице, над или под лентой: сверху, снизу и по диагонали.
func (g *gen) pair() {
	k := len(g.as)
	for _, m := range halves(k) {
		var left, right [2]string // [0] — крупное сверху, [1] — снизу
		if t, ok := g.under(0, 1, m); ok {
			left[0] = node(false, lf(0), t).String()
		}
		if t, ok := g.under(m-1, 0, m-1); ok {
			left[1] = node(false, t, lf(m-1)).String()
		}
		if t, ok := g.under(m, m+1, k); ok {
			right[0] = node(false, lf(m), t).String()
		}
		if t, ok := g.under(k-1, m, k-1); ok {
			right[1] = node(false, t, lf(k-1)).String()
		}
		for _, lr := range [][2]int{{0, 0}, {1, 1}, {0, 1}, {1, 0}} {
			if left[lr[0]] != "" && right[lr[1]] != "" {
				g.try(KindPair, paged(left[lr[0]], right[lr[1]]))
			}
		}
	}
}

// halves — где начинается правая страница у «подпорок» и пары: посередине (у нечётных — оба).
func halves(k int) []int {
	if k%2 == 0 {
		return []int{k / 2}
	}
	return []int{k / 2, k/2 + 1}
}

// under — сетка фото lo…hi−1 над или под главным h во всю ширину страницы. Область сетки — не
// меньше minPart высоты: у вертикального главного блок всё равно сожмётся, а сетку выбрать надо.
func (g *gen) under(h, lo, hi int) (tn, bool) {
	t := g.grid(lo, hi, g.cw, max(g.ch-g.cw/g.as[h]-g.p.Gap, minPart*g.ch))
	if t == nil {
		return tn{}, false
	}
	return *t, true
}

// beside — сетка фото lo…hi−1 слева или справа от главного h во всю высоту страницы (область —
// не уже minPart: у горизонтального главного блок сожмётся по высоте).
func (g *gen) beside(h, lo, hi int) (tn, bool) {
	t := g.grid(lo, hi, max(g.cw-g.ch*g.as[h]-g.p.Gap, minPart*g.cw), g.ch)
	if t == nil {
		return tn{}, false
	}
	return *t, true
}

// grid — лучшая сетка фото lo…hi−1 в область w×h; nil — фото нет или область пуста.
func (g *gen) grid(lo, hi int, w, h float64) *tn {
	if ts := g.grids(lo, hi, w, h, 1); len(ts) > 0 {
		return &ts[0]
	}
	return nil
}
