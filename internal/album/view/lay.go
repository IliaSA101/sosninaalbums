package view

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// lay — действующая раскладка разворота «Авто», её место в ряду и листание страниц.
// Только чтение p и кэша layout — можно звать из воркеров.
func lay(p *model.Project, s *model.Spread) Laid {
	cur, best, ok := p.Layout(s)
	if !ok {
		return Laid{done: true}
	}
	l := Laid{cur: cur, variant: model.IndexOf(best, cur.Key), variants: len(best), ok: true, done: true}
	asp, prm := p.Aspects(s.Photos), p.Params()
	// Ряды с перестановкой считаются, только если текущая раскладка из них.
	if l.variant < 0 && layout.IsRearranged(cur.Key) {
		if rr, _ := layout.CachedRearranged(asp, prm, model.VariantLimit); model.IndexOf(rr, cur.Key) >= 0 {
			l.variant, l.variants, l.rearranged = model.IndexOf(rr, cur.Key), len(rr), true
		}
	}
	if !layout.IsPaged(cur.Key) {
		return l
	}
	l.pages = make([]model.PageView, 2)
	for side := range 2 {
		pv := model.PageView{Variant: -1}
		for _, r := range cur.Rects {
			if (r.X < prm.PageW) == (side == 0) {
				pv.Photos++
			}
		}
		if in, ok := layout.CachedPagesInOrder(cur.Key, asp, prm, side, model.VariantLimit); ok {
			pv.Variant, pv.Variants = model.IndexOf(in, cur.Key), len(in)
			if pv.Variant < 0 && PagePermuted(cur.Key, side) {
				if _, rr, _, _ := layout.CachedPages(cur.Key, asp, prm, side, model.VariantLimit); model.IndexOf(rr, cur.Key) >= 0 {
					pv.Variant, pv.Variants, pv.Rearranged = model.IndexOf(rr, cur.Key), len(rr), true
				}
			}
		}
		l.pages[side] = pv
	}
	return l
}

// PagePermuted — фото страницы в ключе P[…|…] стоят не в порядке ленты.
func PagePermuted(key string, side int) bool {
	parts := strings.SplitN(strings.TrimSuffix(strings.TrimPrefix(key, "P["), "]"), "|", 2)
	return len(parts) == 2 && layout.IsRearranged(parts[side])
}

// Laid — раскладочная часть вида разворота «Авто», посчитанная заранее.
// done = false — не считалась (сетка, пусто, паника): spreadView посчитает сам.
type Laid struct {
	cur                  layout.Result
	variant, variants    int
	rearranged, ok, done bool
	pages                []model.PageView
}

// layWorker — счёт разворота в воркере; тест подменяет его, чтобы имитировать панику.
var layWorker = lay

// layAll — lay для разворотов «Авто» с фото, параллельно. Только чтение p (вызов под e.mu).
func layAll(p *model.Project, spreads []*model.Spread) []Laid {
	ls := make([]Laid, len(spreads))
	if !model.ParallelLayouts() {
		return ls
	}
	var jobs []int
	for i, s := range spreads {
		if len(s.Grid) == 0 && len(s.Photos) > 0 {
			jobs = append(jobs, i)
		}
	}
	var pl model.PanicLog
	model.Each(len(jobs), func(j int) {
		s := spreads[jobs[j]]
		pl.Safely(s.ID, func() { ls[jobs[j]] = layWorker(p, s) })
	})
	pl.Summary()
	return ls
}
