package forms

// Quality — оценка раскладки внутри своего вида и полы допустимости (design §5). Оценка
// layout.score заполняет разворот и штрафует «мелко»; у формы с воздухом (лента, лесенка)
// это несправедливо, поэтому здесь заполнение считается по рамке блока, а «мелко» заменено
// полом по виду. Остальные члены score (разброс площадей, корешок, глубина) повторены —
// TestQualityMatchesScoreTerms сверяет копии с layout.

import (
	"math"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

const (
	// Веса членов оценки: 0.05·log₂ разброса площадей и 0.015 за уровень — как в layout.score.
	spreadW = 0.05
	depthW  = 0.015
	// bandW — штраф за каждую единицу доли рамки вне полосы вида.
	bandW = 1.5

	// foldEps — как spineEps в layout: фото, лишь касающееся корешка, его не пересекает.
	foldEps = 0.01

	// heroShare — доля главного в площади фото; у пары — каждого из двух (041). У «С главным» и
	// «Две крупных» на 17–40 фото со страницами проходит и главное ≥ heroManyRatio медиан прочих
	// (manyHeroOK, 072-4).
	heroShare = 0.30
	pairShare = 0.15
	// defaultFloor — мельчайшая сторона фото / высота страницы у видов без своего пола.
	defaultFloor = 0.12

	// ladderAirW — вес пустоты полосы ступеней лесенки s/(h+s) (055): блок лесенки засчитан
	// заполненным, а пустые углы идут отдельным членом. Шаг h/2 зашит в формат v9, так что член
	// равен 1/12 у любой лесенки: длинную лесенку он не наказывает за то, что она лесенка.
	ladderAirW = 0.25
	// emptyPageW — за каждую пустую страницу постраничного ключа (P[…|-]), всем видам (055): без
	// него лесенка на одной странице стояла выше крупных раскладок на весь разворот.
	emptyPageW = 0.15
)

// band — полоса доли рамки блока от разворота: вне неё вид теряет смысл (лента на весь
// разворот — уже мозаика, мозаика с воздухом — воздух).
type band struct{ lo, hi float64 }

var bands = map[Form]band{
	Mosaic: {0.6, 1}, Strip: {0.12, 0.45}, Rows: {0.3, 0.85}, Grid: {0.3, 0.85},
	Air: {0.25, 0.6}, Ladder: {0.25, 1}, Contrast: {0.3, 0.8}, Scatter: {0.25, 1},
}

// floors — пол мельчайшей стороны фото / высоты страницы по виду (таблица B, design §5).
var floors = map[Form]float64{
	Strip: 0.18, Rows: 0.14, Ladder: 0.18, Scatter: 0.18, Contrast: 0.12, Air: 0.14, Asym: 0.12, Grid: 0.12,
}

// paged — у ключа блоки по страницам: рамка и заполнение считаются на каждой странице.
func paged(key string) bool {
	return strings.HasPrefix(key, "P[") || strings.HasPrefix(key, "Q") || strings.HasPrefix(key, "B[")
}

// envelope — заполнение рамки блока фото и доля площади разворота, которую занимают рамки;
// air — пустота полосы ступеней лесенки s/(h+s) (наибольшая по сторонам), empty — пустых страниц.
// У разворота рамка одна, у P/Q — по странице. Лесенка по замыслу пуста в углах рамки
// (диагональ из ступеней), поэтому её блок считается заполненным: заполнение рамки 2/(k+1)
// наказывало бы длинную лесенку за длину, и 4 фото на разворот уступали бы лесенке на странице.
// Пустоту углов Quality вычитает отдельно — по полосе ступеней, одинаково при любом k (055).
func envelope(key string, f Form, rs []layout.Rect, p layout.Params) (env, share, air float64, empty int) {
	groups := [][]layout.Rect{rs}
	if paged(key) {
		var l, r []layout.Rect
		for _, x := range rs {
			if x.X+x.W/2 < p.PageW {
				l = append(l, x)
			} else {
				r = append(r, x)
			}
		}
		groups = [][]layout.Rect{l, r}
	}
	_, sides := shapeOf(key)
	var sum, box, shown float64
	for i, g := range groups {
		if len(g) == 0 {
			empty++
			continue
		}
		x0, y0, x1, y1 := bbox(g)
		area := 0.0
		for _, x := range g {
			area += x.W * x.H
		}
		sum += area
		shown += (x1 - x0) * (y1 - y0)
		if f == Ladder && i < len(sides) && isLadder(sides[i].root) {
			box += area
			air = max(air, stepAir(g, y1-y0))
		} else if f == Scatter {
			box += area // воздух — смысл вида (105): рамка блока не мера заполнения
		} else {
			box += (x1 - x0) * (y1 - y0)
		}
	}
	all := (2*p.PageW - 2*p.Margin) * (p.PageH - 2*p.Margin)
	if box <= 0 || all <= 0 {
		return 0, 0, 0, 0
	}
	return sum / box, shown / all, air, empty
}

// stepAir — пустая доля полосы ступеней лесенки s/(h+s): h — высота фото, s — шаг ступени по
// рамке (frameH − h)/(k−1); на порядок фото не опирается. Фото лесенки одной высоты (layout).
func stepAir(g []layout.Rect, frameH float64) float64 {
	if len(g) < 2 {
		return 0
	}
	h := g[0].H
	s := (frameH - h) / float64(len(g)-1)
	if !(h+s > 0) || s <= 0 {
		return 0
	}
	return s / (h + s)
}

func isLadder(root string) bool {
	return strings.HasPrefix(root, "L(") || strings.HasPrefix(root, "A(")
}

// spreadTerm — 0.05·log₂(max/min площадей) у фото без главных (у них разброс — смысл вида).
func spreadTerm(rs []layout.Rect, h HeroInfo) float64 {
	lo, hi, n := math.Inf(1), 0.0, 0
	for i, x := range rs {
		if h.isHero(i) {
			continue
		}
		a := x.W * x.H
		lo, hi, n = math.Min(lo, a), math.Max(hi, a), n+1
	}
	if n < 2 || lo <= 0 {
		return 0
	}
	return spreadW * math.Log2(hi/lo)
}

// spineTerm — штраф за фото через корешок, как в layout.score.
func spineTerm(rs []layout.Rect, p layout.Params) float64 {
	spine := p.PageW
	if p.Sheet {
		spine = 0
	}
	if spine <= 0 {
		return 0
	}
	s := 0.0
	for _, r := range rs {
		if r.X < spine-foldEps && r.X+r.W > spine+foldEps {
			if part := math.Min(spine-r.X, r.X+r.W-spine) / r.W; part < 0.2 {
				s += 0.4
			} else {
				s += 0.04 + 0.2*math.Max(0, 1-r.W/spine)
			}
		}
	}
	return s
}

// bandTerm — штраф доли рамки вне полосы вида.
func bandTerm(f Form, share float64) float64 {
	b, ok := bands[f]
	if !ok {
		b = band{0.5, 1}
	}
	return bandW * math.Max(0, math.Max(b.lo-share, share-b.hi))
}

// depthOf — глубина дерева ключа, как node.depth у перебора: лист 0, H/V/G/L/A — на уровень
// больше самого глубокого ребёнка (Z — как L/A); модификаторы и страницы уровня не добавляют.
func depthOf(key string) int {
	maxd, isOp := []int{0}, []bool{}
	for i := range len(key) {
		switch key[i] {
		case '(':
			isOp = append(isOp, i > 0 && strings.IndexByte("HVGLAZ", key[i-1]) >= 0)
			maxd = append(maxd, 0)
		case ')':
			if len(maxd) < 2 {
				continue
			}
			d := maxd[len(maxd)-1]
			if isOp[len(isOp)-1] {
				d++
			}
			maxd, isOp = maxd[:len(maxd)-1], isOp[:len(isOp)-1]
			maxd[len(maxd)-1] = max(maxd[len(maxd)-1], d)
		}
	}
	return maxd[0]
}

// Quality — оценка раскладки r вида f и допустимость: ok = false — нарушен пол вида (мелкое
// фото, главное меньше доли, фото через корешок не той формы). Оценки сравниваются в одном
// виде и между видами внутри допуска ряда «Авто» (variety.Row). В край — оценка layout.
func Quality(r layout.Result, f Form, p layout.Params) (q float64, ok bool) {
	return quality(r, f, p, 0)
}

// quality — Quality с полом мельчайшей стороны не строже relax мм (0 — полы вида как есть).
func quality(r layout.Result, f Form, p layout.Params, relax float64) (q float64, ok bool) {
	rs := r.Rects
	if len(rs) == 0 || !(p.PageH > 0) {
		return 0, false
	}
	if f == Bleed {
		return r.Score, true
	}
	h := Heroes(rs)
	env, share, air, empty := envelope(r.Key, f, rs, p)
	q = env - spreadTerm(rs, h) - spineTerm(rs, p) - depthW*float64(depthOf(r.Key)) - bandTerm(f, share)
	q -= ladderAirW * air
	q -= emptyPageW * float64(empty)
	return q, admissible(rs, f, p, h, relax)
}

// crosses — фото лежит по обе стороны корешка.
func crosses(x layout.Rect, p layout.Params) bool {
	return !p.Sheet && x.X < p.PageW-foldEps && x.X+x.W > p.PageW+foldEps
}

// admissible — правило сгиба (layout.FoldOK, одно для всех путей подбора, 064) и полы вида.
// Сгиб проверяется и у мозаики: прежде она обходила проверку, и узкое фото через корешок
// попадало в ряд мимо настройки. Рамки = кадры: «в край» Gen сам не строит (Bleed сюда не
// доходит — его ряд layout проверил по кадру). Мозаика дальше — как есть (оценена layout).
func admissible(rs []layout.Rect, f Form, p layout.Params, h HeroInfo, relax float64) bool {
	if !layout.FoldOK(rs, nil, p) {
		return false
	}
	if f == Mosaic {
		return true
	}
	heroForm := f == Hero || f == Pair || f == Spine
	if heroForm && h.N == 0 {
		return false
	}
	floor, ok := floors[f]
	if !ok {
		floor = defaultFloor
	}
	if relax > 0 {
		floor = math.Min(floor, relax/p.PageH)
	}
	sum := 0.0
	for _, x := range rs {
		sum += x.W * x.H
	}
	many := manyHeroOK(rs, f, p, h)
	for i, x := range rs {
		hero := heroForm && h.isHero(i)
		if hero {
			share := heroShare
			if h.N == 2 {
				share = pairShare
			}
			if quanta(x.W*x.H/sum) < quanta(share) && !many {
				return false
			}
			continue
		}
		// Пол вида с главным (decisions 0.15 №16): остальные — по сторонам, не через корешок,
		// при любом положении сгиба — это композиция вида, а не правило сгиба.
		if heroForm && crosses(x, p) || quanta(math.Min(x.W, x.H)/p.PageH) < quanta(floor) {
			return false
		}
	}
	return true
}
