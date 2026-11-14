package forms

// Фигуры: мелкое — крупное — мелкое (contrast), лесенка (ladder), рамка (frame).

import (
	"regexp"
	"strings"
)

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

// contrastBigs — кандидаты в крупные для ряда lo…hi−1: самое широкое; при равных пропорциях
// (в квантах) ещё и центральное — «центр крупнее» (для чётного числа — оба центральных).
func (g *gen) contrastBigs(lo, hi int) []int {
	w := g.widest(lo, hi)
	bigs := []int{w}
	n := hi - lo
	if n < 3 {
		return bigs
	}
	mids := []int{lo + n/2}
	if n%2 == 0 {
		mids = []int{lo + n/2 - 1, lo + n/2}
	}
	for _, m := range mids {
		if m != w && quanta(g.as[m]) == quanta(g.as[w]) {
			bigs = append(bigs, m)
		}
	}
	return bigs
}

// flatRow — плоский ряд H(…) без вложенных узлов.
var flatRow = regexp.MustCompile(`H\(([^()]*)\)`)

// CenterBig — ключ формы contrast с «центр крупнее»: в каждом ряду H от трёх фото обычное одно,
// и оно в середине (при чётном числе — в одной из двух центральных), прочие — малые (m; n — боками
// покрупнее, 105). Такие ряды ряд «Авто» берёт отдельным слотом: по Q они уступают левому крупному
// (центр на сгибе штрафуется spineTerm), но владелец просит именно их (0.23-q6).
func CenterBig(key string) bool {
	found := false
	for _, m := range flatRow.FindAllStringSubmatch(key, -1) {
		kids := strings.Split(m[1], ",")
		n := len(kids)
		if n < 3 {
			continue
		}
		big := -1
		for i, k := range kids {
			if !strings.HasPrefix(k, "m") && !strings.HasPrefix(k, "n") {
				if big >= 0 {
					return false
				}
				big = i
			}
		}
		if big != n/2 && (n%2 != 0 || big != n/2-1) {
			return false
		}
		found = true
	}
	return found
}

// contrastRow — ряд фото lo…hi−1: фото big обычное, остальные поменьше (m).
func (g *gen) contrastRow(lo, hi, big int) string {
	if hi-lo == 1 {
		return leaf(lo)
	}
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

// contrast — H(m0,1,m2) на разворот (layflat) и Q[…|…] с крупным на каждой странице; крупное —
// самое широкое, а при равных пропорциях пробуется и среднее.
func (g *gen) contrast() {
	k := g.k
	if k < contrastMin || k > contrastMax {
		return
	}
	if g.flat {
		for _, b := range g.contrastBigs(0, k) {
			g.try(g.contrastRow(0, k, b), noShift)
		}
	}
	if g.pages {
		c := cutNear(g.pre, 0, k, 0.5)
		for _, bl := range g.contrastBigs(0, c) {
			for _, br := range g.contrastBigs(c, k) {
				g.try(pairKey("", g.contrastRow(0, c, bl), g.contrastRow(c, k, br)), noShift)
			}
		}
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
