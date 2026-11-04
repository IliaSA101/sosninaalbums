package forms

// «Разброс» (спека 105, design §4): свободные композиции для «Других вариантов» — центр крупнее
// с боками покрупнее (n) и по одной плитке на профиль уровней Z. В Gen не входят: ряд «Авто» и
// автосборка их не берут (variety.Cand.Manual), поэтому эталоны forms*.sha не меняются.

import (
	"math"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Фото на развороте у Free.
const freeMin, freeMax = 3, 5

// freeProfiles — уровни Z по фото слева направо на 3, 4 и 5 фото (design §4.1): `\` вниз,
// `/` вверх, V, Λ, `_/`, `\_` и ровный ряд `—`. Порядок — порядок плиток.
var freeProfiles = [7][3]string{
	{"adg", "aceg", "abdfg"},
	{"gda", "geca", "gfdba"},
	{"aga", "agga", "adgda"},
	{"gag", "gaag", "gdadg"},
	{"gga", "ggga", "gggga"},
	{"agg", "aggg", "agggg"},
	{"ddd", "dddd", "ddddd"},
}

// zKey — Z(…) из фото lo… с уровнями lv (по букве на фото).
func zKey(lv string, lo int) string {
	kids := make([]string, len(lv))
	for i := range len(lv) {
		kids[i] = lv[i:i+1] + leaf(lo+i)
	}
	return "Z(" + strings.Join(kids, ",") + ")"
}

// tryFree строит ключ и берёт его, если вид проходит полы; folded — отказ только из-за фото на
// сгибе при «Только широкие»: с «Любые фото» раскладка прошла бы (число для подсказки, 105 В1).
func (g *gen) tryFree(key string) (c Cand, ok, folded bool) {
	g.builds++
	r, ok := layout.Build(key, g.as, g.p)
	if !ok {
		return Cand{}, false, false
	}
	f := Of(key, r.Rects, g.p)
	q, ok := quality(r, f, g.p, 0)
	if ok {
		return Cand{Result: r, Form: f, Q: q}, true, false
	}
	if g.p.Layflat && !g.p.FoldAny && !g.p.Sheet {
		pa := g.p
		pa.FoldAny = true
		_, folded = quality(r, f, pa, 0)
	}
	return Cand{}, false, folded
}

// centerN — «центр крупнее» с боками n: лучший по Q из центральных фото, равных самому широкому
// (как contrastBigs). Только layflat: крупное стоит на корешке.
func (g *gen) centerN() (best Cand, ok bool, hidden int) {
	k := g.k
	w := g.widest(0, k)
	mids := []int{k / 2}
	if k%2 == 0 {
		mids = []int{k/2 - 1, k / 2}
	}
	for _, m := range mids {
		if quanta(g.as[m]) != quanta(g.as[w]) {
			continue
		}
		kids := make([]string, 0, k)
		for i := range k {
			if i == m {
				kids = append(kids, leaf(i))
			} else {
				kids = append(kids, "n"+leaf(i))
			}
		}
		c, good, folded := g.tryFree("H(" + strings.Join(kids, ",") + ")")
		switch {
		case folded:
			hidden++
		case good && (!ok || quanta(c.Q) > quanta(best.Q)):
			best, ok = c, true
		}
	}
	return best, ok, hidden
}

// Free — свободные композиции разворота для «Других вариантов»: сначала центр с боками n (если
// есть), затем по плитке на профиль «Разброса» в порядке таблицы. Только 3–5 фото. Профиль
// строится на разворот (layflat) Z(…), иначе — по страницам P[Z(левые)|Z(правые)]: сначала
// ⌈k/2⌉ фото слева, затем ⌊k/2⌋. Правило сгиба — как у всех форм (064).
func Free(aspects []float64, p layout.Params) []Cand {
	out, _ := FreeFold(aspects, p)
	return out
}

// FreeFold — Free и число раскладок на разворот, скрытых только правилом сгиба («Только
// широкие»): с «Любые фото» они встали бы (подсказка в галерее, 105 В1).
func FreeFold(aspects []float64, p layout.Params) (out []Cand, hidden int) {
	k := len(aspects)
	if k < freeMin || k > freeMax || !(p.PageW > 0) || !(p.PageH > 0) {
		return nil, 0
	}
	for _, a := range aspects {
		if !(a > 0) || math.IsInf(a, 0) {
			return nil, 0
		}
	}
	return newGen(aspects, p, 0).free()
}

// free — Free на готовом gen (builds считает Build для бюджета ≤ 24 на разворот, TestFreeBudget).
func (g *gen) free() (out []Cand, hidden int) {
	k := g.k
	if g.flat {
		c, ok, h := g.centerN()
		hidden += h
		if ok {
			out = append(out, c)
		}
	}
	for _, pr := range freeProfiles {
		lv := pr[k-freeMin]
		if g.flat {
			c, ok, folded := g.tryFree(zKey(lv, 0))
			if ok {
				out = append(out, c)
				continue
			}
			if folded {
				hidden++
			}
		}
		if !g.pages {
			continue
		}
		cuts := []int{(k + 1) / 2, k / 2}
		if k%2 == 0 {
			cuts = cuts[:1]
		}
		for _, j := range cuts {
			if c, ok, _ := g.tryFree(pagedKey(zKey(lv[:j], 0), zKey(lv[j:], j))); ok {
				out = append(out, c)
				break
			}
		}
	}
	return out, hidden
}
