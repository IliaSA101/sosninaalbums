// Package forms — генераторы форм раскладок разворота (спека 051, design §5): лента, ряды,
// сетка, воздух, разные страницы, мелкое — крупное, лесенка, рамка. Ключи — грамматика 051,
// геометрию и оценку считает layout.Build; здесь они только собираются и оцениваются по виду.
// Ряд Layouts (мозаика) и раскладки с главным фото (variety/featured.go) сюда не входят.
package forms

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Cand — раскладка вида Form с оценкой Q внутри вида (Quality).
type Cand struct {
	layout.Result
	Form Form
	Q    float64
}

// minShift — доля высоты разворота (страницы), которую блок оставляет свободной: у U/D
// без заметного сдвига вид тот же, что по центру, и такой ключ генератор не берёт
// (Build его строит — порог не часть формата).
const minShift = 0.1

// Диапазоны фото на развороте у видов (design §5).
const (
	stripMin, stripMax       = 3, 10
	rowsMin, rowsMax         = 6, 40
	gridMin, gridMax         = 4, 30
	airMin, airMax           = 2, 16
	asymMin, asymMax         = 5, 40
	contrastMin, contrastMax = 3, 7
	ladderMin, ladderMax     = 3, 5
	frameMin, frameMax       = 7, 16
	gridSmallMax             = 12  // S(G…) — до стольких фото
	gridSameShare            = 0.8 // доля фото одной ориентации у ровной сетки
	maxGrid                  = 4   // рядов сетки и рядов G — не больше (больше — фото мельче пола)
)

// shiftOf — какую часть блока проверять на сдвиг у U/D.
type shiftOf int8

const (
	noShift    shiftOf = iota
	wholeShift         // рамка всех фото в высоте разворота
	leftShift          // рамка фото левой страницы в высоте страницы
	rightShift
)

// gen — генератор форм одного разворота.
type gen struct {
	as     []float64
	pre    []float64
	p      layout.Params
	k      int
	flat   bool // можно блок на весь разворот (layflat)
	pages  bool // можно страницы (не лист)
	seen   map[string]bool
	out    []Cand
	builds int
	relax  float64               // пол мельчайшей стороны не строже этого (мм); 0 — полы как есть
	parts  map[partKey][]int     // разрезы на ряды многих фото (part)
	tables map[[2]int]*partTable // таблицы DP разреза по началу и minN (part)
	others map[[2]int][]string   // страницы рядом с главным (otherPage)
	heroes map[heroKey][]string  // лучшие страницы с главным (heroPage)
}

// Gen — раскладки форм для фото aspects (пропорции ширина/высота) в порядке ленты: ключи без
// повторов, каждая строится Build и проходит полы своего вида. Без layflat — постраничные
// варианты (Q, P), у листа (Sheet) — только на весь разворот. Вне 2…MaxPhotos фото — пусто.
func Gen(aspects []float64, p layout.Params) []Cand { return GenMin(aspects, p, 0) }

// GenMin — Gen, где пол мельчайшей стороны фото у вида не строже minSide мм (> 0): форма не
// мельче раскладки, с которой её сравнивают (ряд «Авто» на многих фото, решение 0.13 №40).
func GenMin(aspects []float64, p layout.Params, minSide float64) []Cand {
	k := len(aspects)
	if k < 2 || k > layout.MaxPhotos || !(p.PageW > 0) || !(p.PageH > 0) {
		return nil
	}
	for _, a := range aspects {
		if !(a > 0) || math.IsInf(a, 0) {
			return nil
		}
	}
	g := newGen(aspects, p, minSide)
	g.run()
	return g.out
}

func newGen(as []float64, p layout.Params, relax float64) *gen {
	return &gen{as: as, pre: prefixSums(as), p: p, k: len(as), flat: p.Layflat, pages: !p.Sheet, seen: map[string]bool{}, relax: relax}
}

// run — все генераторы по очереди; на многих фото (17–40) — ещё «сетка + главное» и «две крупных».
func (g *gen) run() {
	g.strip()
	g.rows()
	g.grid()
	g.air()
	g.asym()
	g.contrast()
	g.ladder()
	g.frame()
	if g.many() {
		g.gridHero()
		g.gridPair()
	}
}

// try строит ключ и берёт его, если Build его принял, вид проходит полы, а у U/D есть сдвиг.
func (g *gen) try(key string, sh shiftOf) {
	if key == "" || g.seen[key] {
		return
	}
	g.seen[key] = true
	g.builds++
	r, ok := layout.Build(key, g.as, g.p)
	if !ok || sh != noShift && !g.shifted(r.Rects, sh) {
		return
	}
	f := Of(key, r.Rects, g.p)
	if q, ok := quality(r, f, g.p, g.relax); ok {
		g.out = append(g.out, Cand{Result: r, Form: f, Q: q})
	}
}

// shifted — блок оставляет по высоте не меньше minShift свободного места.
func (g *gen) shifted(rs []layout.Rect, sh shiftOf) bool {
	var part []layout.Rect
	for _, x := range rs {
		left := x.X+x.W/2 < g.p.PageW
		if sh == wholeShift || sh == leftShift && left || sh == rightShift && !left {
			part = append(part, x)
		}
	}
	if len(part) == 0 {
		return false
	}
	_, y0, _, y1 := bbox(part)
	room := g.p.PageH - 2*g.p.Margin
	return room > 0 && (room-(y1-y0))/room >= minShift
}

// Фото на странице / развороте с одной ориентацией: доля горизонтальных или вертикальных.
func (g *gen) sameOrientation() bool {
	land := 0
	for _, a := range g.as {
		if a >= 1 {
			land++
		}
	}
	return float64(max(land, g.k-land)) >= gridSameShare*float64(g.k)
}
