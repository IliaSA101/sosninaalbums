package forms

// Фигуры: мелкое — крупное — мелкое (contrast), лесенка (ladder), рамка (frame).

import "strings"

// widest — позиция самого широкого фото lo…hi−1 (ничья — левее).
func (g *gen) widest(lo, hi int) int {
	at := lo
	for i := lo + 1; i < hi; i++ {
		if quanta(g.as[i]) > quanta(g.as[at]) {
			at = i
		}
	}
	return at
}

// contrastRow — ряд фото lo…hi−1: самое широкое обычное, остальные поменьше (m).
func (g *gen) contrastRow(lo, hi int) string {
	if hi-lo == 1 {
		return leaf(lo)
	}
	big := g.widest(lo, hi)
	kids := make([]string, 0, hi-lo)
	for i := lo; i < hi; i++ {
		if i == big {
			kids = append(kids, leaf(i))
		} else {
			kids = append(kids, "m"+leaf(i))
		}
	}
	return "H(" + strings.Join(kids, ",") + ")"
}

// contrast — H(m0,1,m2) на разворот (layflat) и Q[…|…] с крупным на каждой странице.
func (g *gen) contrast() {
	k := g.k
	if k < contrastMin || k > contrastMax {
		return
	}
	if g.flat {
		g.try(g.contrastRow(0, k), noShift)
	}
	if g.pages {
		c := cutNear(g.pre, 0, k, 0.5)
		g.try(pairKey("", g.contrastRow(0, c), g.contrastRow(c, k)), noShift)
	}
}

// ladder — лесенка L (вниз) и A (вверх) на разворот (layflat) или на странице: 3 фото либо
// все фото ключа, остальные — на другой странице.
func (g *gen) ladder() {
	k := g.k
	if k < ladderMin || k > ladderMax {
		return
	}
	for _, op := range []byte("LA") {
		if g.flat {
			g.try(string(op)+"("+strings.Join(leaves(0, k), ",")+")", noShift)
		}
		if !g.pages {
			continue
		}
		ns := []int{ladderMin}
		if k > ladderMin {
			ns = append(ns, k)
		}
		for _, n := range ns {
			lad := func(lo, hi int) string { return string(op) + "(" + strings.Join(leaves(lo, hi), ",") + ")" }
			rest := func(lo, hi int) string {
				if lo == hi {
					return "-"
				}
				return hrow(lo, hi)
			}
			g.try(pagedKey(lad(0, n), rest(n, k)), noShift)
			g.try(pagedKey(rest(0, k-n), lad(k-n, k)), noShift)
		}
	}
}

// frame — H(V(…),крупное,V(…)): самое широкое фото посередине ленты в рамке из столбцов;
// только layflat (крупное стоит на корешке).
func (g *gen) frame() {
	k := g.k
	if k < frameMin || k > frameMax || !g.flat {
		return
	}
	b := g.widest(k/3, k-k/3)
	g.try("H("+vcol(0, b)+","+leaf(b)+","+vcol(b+1, k)+")", noShift)
	if b2 := k / 2; b2 != b {
		g.try("H("+vcol(0, b2)+","+leaf(b2)+","+vcol(b2+1, k)+")", noShift)
	}
}
