package grid

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// fillProject — проект со случайными местами: обложка (если cover) и четыре разворота; часть мест занята.
func fillProject(r *rand.Rand, cover bool) *model.Project {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	n := 0
	mk := func(maxX float64) []*model.Place {
		var out []*model.Place
		for range r.IntN(7) {
			n++
			pl := &model.Place{ID: fmt.Sprintf("%012x", n), X: r.Float64()*maxX - 40, Y: r.Float64() * 200, W: 20 + r.Float64()*60, H: 30}
			if r.IntN(3) == 0 {
				pl.Photo = fmt.Sprintf("old%d", n)
			}
			out = append(out, pl)
		}
		return out
	}
	for i := range 4 {
		s := &model.Spread{ID: fmt.Sprintf("s%d", i+1), Grid: mk(500)}
		if r.IntN(3) == 0 { // виньетка: «Заполнить места» берёт её места как обычные (131-10)
			s.Kind, s.Vig = model.KindVignette, &model.Vignette{Tpl: "t", Teachers: 1, Pupils: 2}
		}
		p.Spreads = append(p.Spreads, s)
	}
	if cover {
		pr, _ := model.CoverPresetOf(model.CoverDefault)
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Grid: mk(700)}}
		if r.IntN(4) == 0 {
			p.Cover.Spec.Safe = 500 // зон нет
		}
	}
	return p
}

// Порядок мест обложки в model.CoverSeats делит места на сторонки так же, как coverSides (по InArea):
// лицевая, задняя, остальные; в том числе когда зоны нет (места не относятся к сторонке).
func TestCoverSeatsMatchInArea(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 132))
	for it := range 300 {
		p := fillProject(r, true)
		cs := coverSides(p)
		var want []*model.Place
		taken := map[*model.Place]bool{}
		for _, i := range []int{1, 0} { // лицевая, задняя
			if cs[i] == nil {
				continue
			}
			for _, pl := range cs[i].places {
				if !taken[pl] {
					taken[pl] = true
					want = append(want, pl)
				}
			}
		}
		for _, pl := range p.Cover.Sheet.Grid {
			if !taken[pl] {
				want = append(want, pl)
			}
		}
		if got := p.CoverSeats(); !slices.Equal(got, want) {
			t.Fatalf("набор %d: CoverSeats %v, по InArea %v (зон %d)", it, placeIDs(got), placeIDs(want), len(p.CoverSides()))
		}
	}
}

func placeIDs(pls []*model.Place) (out []string) {
	for _, pl := range pls {
		out = append(out, pl.ID)
	}
	return out
}

// fillRef — прежний обход FillPlaces (только p.Spreads с fromSpread): образец для сверки побитово.
func fillRef(p *model.Project, ids []string, from string) (left int) {
	start, _ := p.Find(from)
	k := 0
	for _, s := range p.Spreads[max(start, 0):] {
		for _, pl := range s.Grid {
			if k < len(ids) && pl.Photo == "" {
				p.Put(s, pl, ids[k])
				k++
			}
		}
	}
	return len(ids) - k
}

func freshInfos(n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, n)
	for i := range out {
		out[i] = model.PhotoInfo{ID: fmt.Sprintf("new%02d", i), Path: fmt.Sprintf("/x/new%02d.jpg", i), W: 300, H: 200}
	}
	return out
}

// С разворота (и с неизвестного ID) FillPlaces даёт ровно то, что давал прежний обход: тот же JSON и left.
func TestFillPlacesFromSpreadSame(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 132))
	for it := range 300 {
		a := fillProject(r, it%2 == 0)
		b := a.Clone()
		from := []string{"s1", "s2", "s3", "s4", "", "нет такого"}[r.IntN(6)]
		photos := freshInfos(r.IntN(14))
		left := FillPlaces(a, photos, from)
		ids := model.Remember(b, photos)
		if want := fillRef(b, ids, from); left != want || string(model.Marshal(a)) != string(model.Marshal(b)) {
			t.Fatalf("набор %d, с %q: left %d ≠ %d или проект разошёлся", it, from, left, want)
		}
	}
}

// Свойство 132-6: фото ≥ мест N — остаток = фото − N, фото меньше — остатка нет; с любого листа,
// число — FillCounts (то, что покажет кнопка). После посадки отсюда остаётся ровно N − фото
// (не досталось), а при фото ≥ N — 0 мест; в проекте есть виньетки.
func TestFillCountMatchesFill(t *testing.T) {
	r := rand.New(rand.NewPCG(13, 132))
	for it := range 200 {
		p := fillProject(r, true)
		from, cover := p.FillCounts()
		sheets := map[string]int{model.CoverID: cover}
		for i, s := range p.Spreads {
			sheets[s.ID] = from[i]
		}
		for id, n := range sheets {
			m := n + r.IntN(4) - 1 // от N−1 до N+2
			if m < 0 {
				m = 0
			}
			q := p.Clone()
			if got, want := FillPlaces(q, freshInfos(m), id), max(m-n, 0); got != want {
				t.Fatalf("набор %d, с %q: мест %d, фото %d: остаток %d, ждали %d", it, id, n, m, got, want)
			}
			qf, qc := q.FillCounts()
			after := qc
			if id != model.CoverID {
				i, _ := q.Find(id)
				after = qf[max(i, 0)]
			}
			if want := max(n-m, 0); after != want {
				t.Fatalf("набор %d, с %q: мест %d, фото %d: отсюда после посадки %d, ждали %d", it, id, n, m, after, want)
			}
		}
	}
}
