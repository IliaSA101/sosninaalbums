package spreads

// Ч/б развороты автосборки (078 design §6, решение 078-7): доля разворотов целиком, наименее
// цветные, не подряд; первый и последний цветные. Выбор по группам плана — один для подсказки
// окна (AutoPlan.Gray), «Что соберётся» и самой сборки.

import (
	"cmp"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// grayShares — доля ч/б разворотов по уровню окна: 0 нет, 1 — 10 %, 2 — 20 %, 3 — 30 %.
var grayShares = [...]float64{0, 0.10, 0.20, 0.30}

// grayFav — разворот с фото такой оценки и выше остаётся цветным, пока хватает других.
const grayFav = 4

// grayGroup — что выбору ч/б известно о развороте плана: цветность фото (без подписи — NaN)
// и есть ли среди них любимое (4–5★).
type grayGroup struct {
	colors []float32
	fav    bool
}

// grayPick — какие из групп плана станут ч/б. g = round(N · доля), при доле > 0 и N ≥ 3 — не
// меньше 1; кандидаты — развороты 1…N−2. Оценка — средняя цветность (фото без подписи — медиана
// альбома), разворот с исходно ч/б фото — первым; меньше — раньше, ничья — меньший индекс.
// Первый проход пропускает соседей выбранных и любимые развороты; не набрали — второй без
// этих запретов. nil — ч/б нет.
func grayPick(groups []grayGroup, share int) []bool {
	n := len(groups)
	if share <= 0 || n < 3 {
		return nil
	}
	g := int(math.Round(float64(n) * grayShares[min(share, len(grayShares)-1)]))
	g = min(max(g, 1), n-2)
	med := grayMedian(groups)
	type cand struct {
		i     int
		mono  bool
		score float64
	}
	cs := make([]cand, 0, n-2)
	for i := 1; i < n-1; i++ {
		c := cand{i: i}
		sum, k := 0.0, 0
		for _, v := range groups[i].colors {
			x := med
			if !math.IsNaN(float64(v)) {
				x = float64(v)
				c.mono = c.mono || v < imgsig.ColorGray
			}
			sum, k = sum+x, k+1
		}
		if k > 0 {
			c.score = sum / float64(k)
		}
		cs = append(cs, c)
	}
	slices.SortStableFunc(cs, func(a, b cand) int {
		if a.mono != b.mono {
			return flipBool(a.mono)
		}
		return cmp.Or(cmp.Compare(a.score, b.score), cmp.Compare(a.i, b.i))
	})
	out := make([]bool, n)
	took := 0
	for pass := range 2 {
		for _, c := range cs {
			free := pass == 1 || !out[c.i-1] && !out[c.i+1] && !groups[c.i].fav
			if took < g && !out[c.i] && free {
				out[c.i] = true
				took++
			}
		}
	}
	return out
}

// flipBool — признак у a ставит его раньше.
func flipBool(a bool) int {
	if a {
		return -1
	}
	return 1
}

// grayMedian — медиана цветности всех фото с подписью; подписей нет — 0 (все развороты равны).
func grayMedian(groups []grayGroup) float64 {
	var all []float64
	for _, gr := range groups {
		for _, v := range gr.colors {
			if !math.IsNaN(float64(v)) {
				all = append(all, float64(v))
			}
		}
	}
	if len(all) == 0 {
		return 0
	}
	slices.Sort(all)
	m := len(all) / 2
	if len(all)%2 == 1 {
		return all[m]
	}
	return (all[m-1] + all[m]) / 2
}

// grayOf — выбор ч/б по группам плана pl входа j (share — уровень окна, AutoOpts.Gray).
func (j PlanJob) grayOf(pl plan.Plan, share int) []bool {
	if share <= 0 {
		return nil
	}
	groups := make([]grayGroup, len(pl.Groups))
	for gi, g := range pl.Groups {
		gr := grayGroup{colors: make([]float32, 0, g[1]-g[0])}
		for k := g[0]; k < g[1]; k++ {
			c := float32(math.NaN())
			if s := j.infos[k].Sig; s.Ok {
				c = s.Color
			}
			gr.colors = append(gr.colors, c)
			gr.fav = gr.fav || k < len(j.rate) && j.rate[k] >= grayFav
		}
		groups[gi] = gr
	}
	return grayPick(groups, share)
}

// grayCount — сколько разворотов выбрано.
func grayCount(gray []bool) int {
	k := 0
	for _, g := range gray {
		if g {
			k++
		}
	}
	return k
}

// paintGray ставит «Ч/б» всем фото разворота s; яркость и контраст фото остаются, уже ч/б не
// трогаются (078 §6).
func paintGray(p *model.Project, s *model.Spread) {
	for _, id := range s.Photos {
		r, ok := p.Photos[id]
		if !ok || r.Adj != nil && r.Adj.Gray {
			continue
		}
		t := model.Tone{Gray: true}
		if r.Adj != nil {
			t.Bright, t.Contrast = r.Adj.Bright, r.Adj.Contrast
		}
		r.Adj = model.ClampTone(&t)
		p.Photos[id] = r
	}
}
