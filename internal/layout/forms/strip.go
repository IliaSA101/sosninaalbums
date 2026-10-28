package forms

// Лента (strip): фото в одну линию через весь разворот; по центру, вверху или внизу.

// strip — Q[ряд|ряд] с разрезом у середины суммы пропорций (±1), QU и QD; при layflat ещё
// один ряд через корешок: H(…), U(H(…)), D(H(…)).
func (g *gen) strip() {
	k := g.k
	if k < stripMin || k > stripMax {
		return
	}
	if g.pages {
		c := cutNear(g.pre, 0, k, 0.5)
		for _, cc := range []int{c, c - 1, c + 1} {
			if cc >= 1 && cc < k {
				g.try(pairKey("", hrow(0, cc), hrow(cc, k)), noShift)
			}
		}
		g.try(pairKey("U", hrow(0, c), hrow(c, k)), wholeShift)
		g.try(pairKey("D", hrow(0, c), hrow(c, k)), wholeShift)
	}
	if g.flat {
		row := hrow(0, k)
		g.try(row, noShift)
		g.try(wrap('U', row), wholeShift)
		g.try(wrap('D', row), wholeShift)
	}
}
