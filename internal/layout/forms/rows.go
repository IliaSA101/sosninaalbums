package forms

import "github.com/IliaSA101/sosninaalbums/internal/layout"

// Ряды одной высоты (rows) и ровная сетка (grid): корень G (spec 051 §2).

// rows — G из 2–4 рядов с равными суммами пропорций на разворот (layflat) или Q[G|G] (pageRows;
// с ManyFrom фото — manyRows: до 6 рядов разрезом DP, 072-5). В ряду — не меньше двух фото.
func (g *gen) rows() {
	k := g.k
	if k < rowsMin || k > rowsMax {
		return
	}
	if g.flat {
		for r := 2; r <= min(maxGrid, k/2); r++ {
			g.try(gEq(g.pre, 0, k, r), noShift)
		}
	}
	switch {
	case g.pages && k >= layout.ManyFrom:
		g.manyRows()
	case g.pages:
		g.pageRows()
	}
}

// pageRows — Q[G|G]: разрез у середины суммы пропорций, по 2–4 ряда на обеих страницах. Если ни
// один не прошёл полы — запасной перебор: разрез ±1 и на одной странице на ряд больше или меньше.
// У Q высота фото общая — самая малая из страниц, и равное число рядов при неровном разрезе
// (12 | 13) делало фото мельче пола, хотя соседний разрез или лишний ряд давал крупнее (решение
// 0.13 №40). Когда основной прошёл, запасной не нужен: лишние ряды плодят почти одинаковые плитки.
func (g *gen) pageRows() {
	k, had := g.k, len(g.out)
	c0 := cutNear(g.pre, 0, k, 0.5)
	for r := 2; r <= maxGrid; r++ {
		if c0 >= 2*r && k-c0 >= 2*r {
			g.try(pairKey("", gEq(g.pre, 0, c0, r), gEq(g.pre, c0, k, r)), noShift)
		}
	}
	if len(g.out) > had {
		return
	}
	for _, c := range []int{c0, c0 - 1, c0 + 1} {
		for rl := 2; rl <= maxGrid; rl++ {
			for rr := max(2, rl-1); rr <= min(maxGrid, rl+1); rr++ {
				// уже испробованные ключи g.try пропускает
				if l, r := g.pairRows(0, c, rl), g.pairRows(c, k, rr); l != "" && r != "" {
					g.try(pairKey("", l, r), noShift)
				}
			}
		}
	}
}

// pairRows — G из r рядов фото lo…hi−1 с равными суммами пропорций, в каждом ряду не меньше двух
// фото; "" — так не делится. Запасной перебор не ставит одиночный кадр рядом с рядами.
func (g *gen) pairRows(lo, hi, r int) string {
	cuts := cutsEq(g.pre, lo, hi, r)
	if cuts == nil {
		return ""
	}
	b := append(append([]int{lo}, cuts...), hi)
	for i := 1; i < len(b); i++ {
		if b[i]-b[i-1] < 2 {
			return ""
		}
	}
	return gKey(b)
}

// grid — ровная сетка r×c (последний ряд короче) у фото почти одной ориентации: G на
// разворот, S(G…) с воздухом при k ≤ 12, Q[G|G] с сеткой на странице.
func (g *gen) grid() {
	k := g.k
	if k < gridMin || k > gridMax || !g.sameOrientation() {
		return
	}
	if g.flat {
		for r := 2; r <= maxGrid; r++ {
			key := gGrid(0, k, r)
			g.try(key, noShift)
			if k <= gridSmallMax && r <= 3 && key != "" {
				g.try(wrap('S', key), noShift)
			}
		}
	}
	if g.pages {
		m := (k + 1) / 2
		for r := 2; r <= 3; r++ {
			if l, rr := gGrid(0, m, r), gGrid(m, k, r); l != "" && rr != "" {
				g.try(pairKey("", l, rr), noShift)
			}
		}
	}
}
