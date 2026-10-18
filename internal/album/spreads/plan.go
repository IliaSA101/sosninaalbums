package spreads

// Автосборка ровно на N разворотов (спека 027 §5, диапазон и серии — 051 §3–4): подсказка
// окна (AutoPlan) и сама сборка (AutoBuild) идут через один PlanJob — иначе подсказка
// разошлась бы со сборкой.

import (
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/series"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
)

// PlanJob — снимок входа автосборки: фото без сеток и фонов, их действующие пропорции,
// геометрия и число разворотов альбома. Снимается под замком, считается вне его.
type PlanJob struct {
	infos []model.PhotoInfo
	asp   []float64
	prm   layout.Params
	have  int
	tail  int // пустые развороты в конце: сборка займёт их первыми (035 U1)
}

// PlanOf — вход автосборки из проекта; photos уже без повторов. Пропорции — как их увидит
// AutoBuild после Remember: размеры из photos, поворот — у фото, уже стоящего в проекте.
func PlanOf(p *model.Project, photos []model.PhotoInfo) PlanJob {
	inGrid, onBg := gridPhotoSet(p), p.OnDecor()
	infos := slices.DeleteFunc(slices.Clone(photos), func(ph model.PhotoInfo) bool { return inGrid[ph.ID] || onBg[ph.ID] })
	tmp := &model.Project{Photos: make(map[string]model.PhotoRef, len(infos))}
	for _, ph := range infos {
		if r, ok := p.Photos[ph.ID]; ok {
			tmp.Photos[ph.ID] = r
		}
	}
	return PlanJob{infos: infos, asp: tmp.Aspects(model.Remember(tmp, infos)), prm: p.Params(), have: len(p.Spreads), tail: blankTail(p)}
}

// blankTail — сколько разворотов подряд в конце альбома пусты (Spread.Empty): заготовки
// от «+ Новый разворот». Разворот с названием, цветом, сеткой или виньетка — не пустой.
func blankTail(p *model.Project) int {
	k := 0
	for k < len(p.Spreads) && p.Spreads[len(p.Spreads)-1-k].Empty() {
		k++
	}
	return k
}

// errSpreads — разворотов в альбоме не хватает даже на самую плотную сборку.
func errSpreads() error {
	return fmt.Errorf("автосборка даст больше %d разворотов: в альбоме их не может быть больше", model.MaxSpreads)
}

// Plan — подсказка окна: N зажимается в допустимое для пути (plan), верх ещё в MaxSpreads −
// непустых; пустой хвост считается свободным местом (Reuse). Фото нет — пустой план без
// ошибки; места нет — ошибка.
func (j PlanJob) Plan(o model.AutoOpts) (model.AutoPlan, error) {
	ap, _, err := j.plan(o)
	return ap, err
}

// plan — три пути (051 §3): «Авто» — диапазон AutoPer по N, N в [⌈n/64⌉, n]; свой
// диапазон (MinPer ≥ 1) — N в RangeSpreads; MinPer 0 — ровно, PlanGroups 0.12 побитово.
// Серии (o.Series) тянут разрезы только с диапазоном, но счёт границ виден на любом пути.
func (j PlanJob) plan(o model.AutoOpts) (model.AutoPlan, plan.Plan, error) {
	n := len(j.asp)
	if n == 0 {
		return model.AutoPlan{}, plan.Plan{}, nil
	}
	ap := model.AutoPlan{Photos: n}
	var cuts []float64
	if o.Series {
		cuts = series.Cuts(j.infos)
		ap.Cuts = strongCuts(cuts)
	}
	ranged := o.Auto || o.MinPer > 0
	m := max(1, min(o.MaxPer, model.MaxPhotos))
	var lo, hi int
	switch {
	case o.Auto:
		lo, hi = plan.SpreadRange(n, model.MaxPhotos)
	case ranged:
		lo, hi = plan.RangeSpreads(n, o.MinPer, m)
	default:
		lo, hi = plan.SpreadRange(n, m)
	}
	hi = min(hi, model.MaxSpreads-(j.have-j.tail))
	ap.MinSpreads, ap.MaxSpreads = lo, max(hi, 0)
	if hi < lo {
		return ap, plan.Plan{}, errSpreads()
	}
	N := max(lo, min(o.Spreads, hi))
	var pl plan.Plan
	if ranged {
		from, to := o.MinPer, m
		if o.Auto {
			from, to = plan.AutoPer(n, N)
		}
		ap.From, ap.To = plan.RangePer(n, N, from, to)
		pl = plan.PlanRange(j.asp, j.prm, plan.PlanOpts{Spreads: N, MinPer: from, MaxPer: to, Cuts: cuts})
	} else {
		ap.To = m
		pl = plan.PlanGroups(j.asp, j.prm, N, m)
	}
	ap.Spreads, ap.Avg, ap.Min, ap.Max = pl.Spreads, pl.Avg, pl.Min, pl.Max
	ap.Reuse = min(j.tail, ap.Spreads)
	return ap, pl, nil
}

// strongCuts — сколько границ серий окно покажет: сила от половины (смена папки, пауза от
// ~6 порогов T₀); слабые только подталкивают разрез.
func strongCuts(cuts []float64) int {
	k := 0
	for _, c := range cuts {
		if c >= 0.5 {
			k++
		}
	}
	return k
}

// AutoBuild — операция над проектом; pre — вход, план и прогрев до замка (Prepare), photos
// там уже без повторов. Ровно o.Spreads разворотов по тем же опциям и зажиму, что у Plan
// (диапазон, «Авто», серии), начиная с позиции at. Фото из сеток и с фонов не берутся. Сборка в конец (at вне
// альбома или в пустом хвосте) занимает пустые развороты хвоста вместо новых, лишние пустые
// остаются после собранных (035 U1). Раскладки — по o.Variety с учётом соседей (041);
// «Меньше» — лучшие, как 0.10. rate — оценки фото для звезды разворота.
// Семейства (o.Families, 065) сужают выбор раскладок. Возвращает число новых разворотов.
func AutoBuild(p *model.Project, pre Prep, at int, rate Rating) (made int, err error) {
	_, made, _, err = build(p, pre, at, rate)
	return made, err
}

// build — AutoBuild с позицией первого собранного разворота и флагами запасных (разрешённых
// семейств нет — лучшая; nil — фильтра нет): их читает предпросмотр на копии (Preview).
func build(p *model.Project, pre Prep, at int, rate Rating) (at0, made int, fb []bool, err error) {
	j := PlanOf(p, pre.photos)
	pl, ready, err := pre.planFor(j)
	if err != nil || len(pl.Groups) == 0 {
		return 0, 0, nil, err
	}
	// Хвост — по снимку до Detach, как в Plan: иначе подсказка разойдётся со сборкой.
	// Опустевшие от переезда фото не занимаются — их уберёт Normalize.
	start := len(p.Spreads) - j.tail
	ids := model.Remember(p, j.infos)
	had := make(map[*model.Spread]bool, len(p.Spreads))
	for _, s := range p.Spreads {
		had[s] = len(s.Photos) > 0
	}
	p.Detach(ids)
	// Опустевшие от переезда фото развороты Normalize удалит — соседями выбора они не бывают.
	dead := func(s *model.Spread) bool { return had[s] && s.Empty() }
	use := 0
	if at < 0 || at >= start { // в т.ч. за концом альбома
		at, use = start, min(j.tail, len(pl.Groups))
	} else if len(p.Spreads)+len(pl.Groups) > model.MaxSpreads {
		return 0, 0, nil, errSpreads() // вставка в середину: план считал хвост свободным
	}
	out := make([]*model.Spread, len(pl.Groups))
	for i, g := range pl.Groups {
		out[i] = &model.Spread{ID: model.NewID(), Photos: slices.Clone(ids[g[0]:g[1]])}
	}
	p.Spreads = slices.Replace(p.Spreads, at, at+use, out...)
	fb = varyBuilt(p, at, len(out), pre.o, rate, ready, dead)
	return at, len(out), fb, nil
}
