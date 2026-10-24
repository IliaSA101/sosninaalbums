package grid

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Шаблоны сеток (спека 007, design §2, §4–5): только геометрия в долях области
// (страницы или разворота), живут в программе (templates.json), а не в проекте.

const (
	// BuiltinLimit — сколько встроенных сеток отдаёт GridChoices (длина ряда «Из раскладок»).
	BuiltinLimit = 24
	// maxBuiltinN — вкладка «7+»: встроенные сетки — до 7 мест (027: предел склейки
	// поднят до MaxPhotos, а вкладки «1…7+» не меняются; виньетка — 0.11).
	maxBuiltinN = 7
)

// TplCells — ячейки разворота в порядке мест: у сетки — места (пустые тоже), у
// «Авто» — прямоугольники действующей раскладки; filled — в ячейке есть фото.
func TplCells(p *model.Project, s *model.Spread) (rs []model.Box, filled []bool) {
	if len(s.Grid) > 0 {
		for _, pl := range s.Grid {
			rs = append(rs, model.Box{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Rot: pl.Rot, Shape: pl.Shape, Round: pl.Round})
			filled = append(filled, pl.Photo != "")
		}
		return rs, filled
	}
	if cur, _, ok := p.Layout(s); ok {
		for _, r := range cur.Rects {
			rs = append(rs, rectBox(r))
			filled = append(filled, true)
		}
	}
	return rs, filled
}

// PlaceBoxes — места области scope разворота в долях области: для «Сохранить как
// шаблон». Второе значение — область шаблона (page для left/right, spread для
// spread). Нет мест — ErrNoPlaces. Только чтение.
func PlaceBoxes(p *model.Project, spreadID, scope string) ([]model.Box, string, error) {
	if scope != model.ScopeLeft && scope != model.ScopeRight && scope != model.ScopeSpread {
		return nil, "", errors.New("неизвестная область")
	}
	_, s := p.Find(spreadID)
	if s == nil {
		return nil, "", errors.New("разворот не найден")
	}
	a, ok := p.AreaOf(s, scope)
	if !ok {
		return nil, "", errors.New("неизвестная область") // страница листа 031
	}
	cells, _ := TplCells(p, s)
	tplScope := model.TplSpread
	if scope != model.ScopeSpread {
		tplScope = model.TplPage
	}
	var boxes []model.Box
	for _, r := range cells {
		if InArea(p, s, r.X, r.W, scope) {
			boxes = append(boxes, model.Box{X: (r.X - a.X) / a.W, Y: (r.Y - a.Y) / a.H, W: r.W / a.W, H: r.H / a.H, Rot: r.Rot, Shape: r.Shape, Round: r.Round})
		}
	}
	if boxes = model.ClampBoxes(boxes); len(boxes) == 0 {
		return nil, "", noPlacesIn(scope)
	}
	return boxes, tplScope, nil
}

func noPlacesIn(scope string) error {
	switch scope {
	case model.ScopeLeft:
		return model.NoPlacesError("На левой странице нет мест")
	case model.ScopeRight:
		return model.NoPlacesError("На правой странице нет мест")
	}
	return model.NoPlacesError("На развороте нет мест")
}

// GridSource — всё, что нужно GridChoicesOf от проекта: снимается под замком
// (GridInput), дальше layout считается уже вне него.
type GridSource struct {
	found   bool
	n       int
	prm     layout.Params
	asp     []float64
	inScope int
}

// GridInput читает из проекта всё для GridChoicesOf; n < 1 заменяется числом фото
// разворота. Неизвестный разворот — GridSource без found.
func GridInput(p *model.Project, spreadID string, n int, scope string) GridSource {
	_, s := p.Find(spreadID)
	if s == nil {
		return GridSource{n: n}
	}
	if n < 1 {
		n = len(s.Photos)
	}
	asp, prm := p.LayoutIn(s.Photos)
	return GridSource{found: true, n: n, prm: prm, asp: asp, inScope: scopePhotos(p, s, scope)}
}

// scopePhotos — сколько фото в области scope разворота.
func scopePhotos(p *model.Project, s *model.Spread, scope string) int {
	if scope != model.ScopeLeft && scope != model.ScopeRight {
		return len(s.Photos)
	}
	cells, filled := TplCells(p, s)
	cnt := 0
	for i, r := range cells {
		if filled[i] && InArea(p, s, r.X, r.W, scope) {
			cnt++
		}
	}
	return cnt
}

// GridChoicesOf считает галерею для разворота: встроенные сетки на n фото (n зажимается
// в 1..7, меньше 1 — число фото разворота) и шаблоны tpls с n местами (7 — от 7).
// scope — куда ставят ("left"/"right"/"spread"), по нему считается Drop. Только чтение;
// layout считается вне замка — in снят заранее (GridInput). Неизвестный разворот — пустые ряды.
func GridChoicesOf(in GridSource, scope string, tpls []model.Template) model.GridChoices {
	n := max(1, min(in.n, maxBuiltinN))
	c := model.GridChoices{N: n, Builtin: []model.GridChoice{}, Mine: []model.GridChoice{}}
	if !in.found {
		return c
	}
	for _, g := range builtinGrids(in.prm, in.asp, n) {
		g.Drop = dropCount(in.inScope, len(g.Boxes), g.Scope, scope)
		c.Builtin = append(c.Builtin, g)
	}
	for _, t := range tpls {
		if t.Scope == model.TplPage && in.prm.Sheet || t.Kind == model.TplVignette {
			continue // у листа 031 страниц нет; шаблоны виньетки — в её окне и галерее (038)
		}
		if len(t.Boxes) == n || (n == maxBuiltinN && len(t.Boxes) > n) {
			c.Mine = append(c.Mine, model.GridChoice{ID: t.ID, Name: t.Name, Scope: t.Scope, Boxes: t.Boxes,
				Drop: dropCount(in.inScope, len(t.Boxes), t.Scope, scope)})
		}
	}
	return c
}

// dropCount — фото области минус места шаблона (не меньше 0); шаблон страницы
// на разворот ставит места на обе страницы.
func dropCount(photos, places int, tplScope, scope string) int {
	if tplScope == model.TplPage && scope == model.ScopeSpread {
		places *= 2
	}
	return max(0, photos-places)
}

// builtinGrids — встроенные сетки на n фото: наборы пропорций считаются параллельно
// через layout.Cached, дубли по форме убираются. Сначала набор фото разворота
// (если их ровно n), затем остальные по оценке; не больше builtinLimit.
func builtinGrids(prm layout.Params, photoAsp []float64, n int) []model.GridChoice {
	if prm.PageW <= 0 || prm.PageH <= 0 {
		return nil
	}
	same := func(a float64) []float64 {
		s := make([]float64, n)
		for i := range s {
			s[i] = a
		}
		return s
	}
	mixed := make([]float64, n)
	for i := range mixed {
		mixed[i] = []float64{3.0 / 2, 2.0 / 3}[i%2]
	}
	var sets [][]float64
	if len(photoAsp) == n {
		sets = append(sets, photoAsp)
	}
	for _, a := range [][]float64{same(3.0 / 2), same(2.0 / 3), same(1), mixed} {
		if !slices.ContainsFunc(sets, func(b []float64) bool { return slices.Equal(a, b) }) {
			sets = append(sets, a)
		}
	}
	results := make([][]layout.Result, len(sets))
	var wg sync.WaitGroup
	for i, a := range sets {
		wg.Go(func() { results[i] = safeLayouts(a, prm) })
	}
	wg.Wait()

	type cand struct {
		res   layout.Result
		first bool // набор фото разворота
	}
	var all []cand
	for i, rs := range results {
		for _, r := range rs {
			all = append(all, cand{r, i == 0 && len(photoAsp) == n})
		}
	}
	// Набор фото разворота первым, остальные по оценке; sort устойчивый: равные — в порядке наборов.
	slices.SortStableFunc(all, func(a, b cand) int {
		if a.first != b.first {
			if a.first {
				return -1
			}
			return 1
		}
		if a.first {
			return 0 // внутри набора порядок Cached уже по оценке
		}
		return cmp.Compare(b.res.Score, a.res.Score)
	})
	out := make([]model.GridChoice, 0, BuiltinLimit)
	seenShape, seenKey := map[string]bool{}, map[string]int{}
	for _, c := range all {
		if len(out) >= BuiltinLimit {
			break
		}
		shape := ShapeKey(c.res.Rects)
		if seenShape[shape] {
			continue
		}
		seenShape[shape] = true
		id := c.res.Key
		if seenKey[id]++; seenKey[id] > 1 { // один ключ у разных наборов — разные формы
			id = fmt.Sprintf("%s#%d", id, seenKey[id])
		}
		out = append(out, model.GridChoice{ID: id, Scope: model.TplSpread, Boxes: spreadFractions(c.res.Rects, prm)})
	}
	return out
}

// layoutsImpl — перебор раскладок; в тестах подменяется, чтобы имитировать панику.
var layoutsImpl = layout.Cached

// safeLayouts — layout.Cached под recover: паника в горутине роняет весь процесс,
// а здесь она лишь убирает один набор из галереи.
func safeLayouts(aspects []float64, prm layout.Params) (rs []layout.Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ПАНИКА в раскладках сеток: %v\n%s", r, debug.Stack())
			rs = nil
		}
	}()
	return layoutsImpl(aspects, prm, BuiltinLimit)
}

// ShapeKey — форма раскладки без порядка мест: прямоугольники, округлённые до 0.5 мм.
func ShapeKey(rs []layout.Rect) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = fmt.Sprintf("%.0f,%.0f,%.0f,%.0f", r.X*2, r.Y*2, r.W*2, r.H*2)
	}
	slices.Sort(parts)
	return strings.Join(parts, ";")
}

// spreadFractions — прямоугольники в мм разворота → доли разворота (внутри 0..1).
func spreadFractions(rs []layout.Rect, prm layout.Params) []model.Box {
	out := make([]model.Box, len(rs))
	w, h := 2*prm.PageW, prm.PageH
	for i, r := range rs {
		out[i] = model.Box{X: r.X / w, Y: r.Y / h, W: r.W / w, H: r.H / h}
	}
	return model.ClampBoxes(out)
}
