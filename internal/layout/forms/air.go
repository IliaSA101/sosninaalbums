package forms

// Воздух (air): блок поменьше или прижатый вверх/вниз; разные страницы (asym): редкая
// страница с 1–4 фото рядом с плотной.

import "slices"

// airTrees — до четырёх деревьев фото lo…hi−1 для блока с воздухом: ряд, два ряда друг под
// другом, два столбца, три ряда. Лучшими они не отбираются — отбирает Quality.
func (g *gen) airTrees(lo, hi int) []string {
	n := hi - lo
	if n < 2 {
		return nil
	}
	m := cutNear(g.pre, lo, hi, 0.5)
	trees := []string{
		hrow(lo, hi),
		list('V', []string{hrow(lo, m), hrow(m, hi)}),
		list('H', []string{vcol(lo, lo+n/2), vcol(lo+n/2, hi)}),
	}
	if cuts := cutsEq(g.pre, lo, hi, 3); n >= 6 && cuts != nil {
		trees = append(trees, list('V', []string{hrow(lo, cuts[0]), hrow(cuts[0], cuts[1]), hrow(cuts[1], hi)}))
	}
	return slices.Compact(trees)
}

// pageTree — дерево для одной страницы: фото, ряд до трёх фото, иначе два ряда.
func (g *gen) pageTree(lo, hi int) string {
	switch n := hi - lo; {
	case n <= 3:
		return hrow(lo, hi)
	default:
		m := cutNear(g.pre, lo, hi, 0.5)
		return list('V', []string{hrow(lo, m), hrow(m, hi)})
	}
}

// air — S(t), U(t), D(t) для деревьев разворота (layflat) и P[S(…)|S(…)] — обе страницы поменьше.
func (g *gen) air() {
	k := g.k
	if k < airMin || k > airMax {
		return
	}
	if g.flat {
		for _, t := range g.airTrees(0, k) {
			g.try(wrap('S', t), noShift)
			g.try(wrap('U', t), wholeShift)
			g.try(wrap('D', t), wholeShift)
		}
	}
	if g.pages {
		c := cutNear(g.pre, 0, k, 0.5)
		g.try(pagedKey(wrap('S', g.pageTree(0, c)), wrap('S', g.pageTree(c, k))), noShift)
	}
}

// denseRoot — плотная страница: G из 2–4 рядов (по ~4 фото в ряду), у малого числа фото — ряд.
func (g *gen) denseRoot(lo, hi int) string {
	n := hi - lo
	r := max(2, min(maxGrid, (n+3)/4))
	if key := gEq(g.pre, lo, hi, r); key != "" && n >= 2*r {
		return key
	}
	return g.pageTree(lo, hi)
}

// asym — редкая страница (1–3 фото, S/U/D) рядом с плотной (от 4 фото), с обеих сторон.
func (g *gen) asym() {
	k := g.k
	if k < asymMin || k > asymMax {
		return
	}
	for s := 1; s <= min(3, k-denseMin); s++ {
		for _, mod := range []byte("SUD") {
			l, r := noShift, noShift
			if mod != 'S' {
				l, r = leftShift, rightShift
			}
			g.try(pagedKey(wrap(mod, hrow(0, s)), g.denseRoot(s, k)), l)
			g.try(pagedKey(g.denseRoot(0, k-s), wrap(mod, hrow(k-s, k))), r)
		}
	}
}
