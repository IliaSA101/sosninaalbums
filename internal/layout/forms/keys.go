package forms

// Сборка ключей генераторов. Ключ — каноническая запись грамматики 051 (layout/keyparse.go):
// Build отвергает любую другую, так что здесь нельзя ни «H(0)» (узел — от двух детей),
// ни пробелов. Позиции — сквозные, в порядке ленты.

import (
	"math"
	"strconv"
	"strings"
)

func leaf(i int) string { return strconv.Itoa(i) }

// list — «op(a,b,…)»; у одного ребёнка узла нет — он сам.
func list(op byte, kids []string) string {
	if len(kids) == 1 {
		return kids[0]
	}
	return string(op) + "(" + strings.Join(kids, ",") + ")"
}

// leaves — позиции lo…hi−1 строками.
func leaves(lo, hi int) []string {
	out := make([]string, 0, hi-lo)
	for i := lo; i < hi; i++ {
		out = append(out, leaf(i))
	}
	return out
}

// hrow — ряд фото lo…hi−1: фото или H.
func hrow(lo, hi int) string { return list('H', leaves(lo, hi)) }

// vcol — столбец фото lo…hi−1: фото или V.
func vcol(lo, hi int) string { return list('V', leaves(lo, hi)) }

// wrap — «M(корень)».
func wrap(mod byte, root string) string { return string(mod) + "(" + root + ")" }

// pagedKey — P[левая|правая]; пустая страница — "-".
func pagedKey(l, r string) string { return "P[" + l + "|" + r + "]" }

// pairKey — Q[левый|правый] с выравниванием по высоте mod («» — по центру, «U», «D»).
func pairKey(mod, l, r string) string { return "Q" + mod + "[" + l + "|" + r + "]" }

// prefixSums — префиксные суммы пропорций: pre[i] = Σ a[0..i).
func prefixSums(as []float64) []float64 {
	pre := make([]float64, len(as)+1)
	for i, a := range as {
		pre[i+1] = pre[i] + a
	}
	return pre
}

// cutNear — разрез в (lo, hi), где сумма пропорций слева ближе всего к доле frac суммы
// lo…hi−1; ничья — левее. Расстояния в квантах: у одинаковых фото они равны точно, и
// последний бит (FMA на arm64) не должен решать, по какую сторону встанет разрез.
func cutNear(pre []float64, lo, hi int, frac float64) int {
	target := pre[lo] + frac*(pre[hi]-pre[lo])
	best, bestD := lo+1, math.Inf(1)
	for c := lo + 1; c < hi; c++ {
		if d := quanta(math.Abs(pre[c] - target)); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// cutsEq — r−1 разрезов lo…hi−1 на r частей с равными суммами пропорций; nil — части пустые.
func cutsEq(pre []float64, lo, hi, r int) []int {
	if hi-lo < r {
		return nil
	}
	cuts := make([]int, 0, r-1)
	prev := lo
	for j := 1; j < r; j++ {
		c := cutNear(pre, lo, hi, float64(j)/float64(r))
		if c <= prev || c >= hi {
			return nil
		}
		cuts = append(cuts, c)
		prev = c
	}
	return cuts
}

// gKey — G из рядов по границам bounds (первая — lo, последняя — hi); нужно ≥ 2 рядов.
func gKey(bounds []int) string {
	if len(bounds) < 3 {
		return ""
	}
	rows := make([]string, 0, len(bounds)-1)
	for i := 1; i < len(bounds); i++ {
		rows = append(rows, hrow(bounds[i-1], bounds[i]))
	}
	return "G(" + strings.Join(rows, ",") + ")"
}

// gEq — G из r рядов lo…hi−1 с равными суммами пропорций; "" — не вышло.
func gEq(pre []float64, lo, hi, r int) string {
	cuts := cutsEq(pre, lo, hi, r)
	if cuts == nil {
		return ""
	}
	return gKey(append(append([]int{lo}, cuts...), hi))
}

// gGrid — ровная сетка lo…hi−1 в r рядов: все ряды по c фото, последний — остаток;
// "" — меньше двух рядов или ряд из одного фото.
func gGrid(lo, hi, r int) string {
	n := hi - lo
	c := (n + r - 1) / r
	if r < 2 || c < 2 || n-(r-1)*c < 1 {
		return ""
	}
	bounds := []int{lo}
	for i := 1; i < r; i++ {
		bounds = append(bounds, lo+i*c)
	}
	return gKey(append(bounds, hi))
}
