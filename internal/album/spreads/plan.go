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

// PlanJob — снимок входа автосборки: фото без сеток и фонов, их действующие пропорции и
// оценки, геометрия и число разворотов альбома. Снимается под замком, считается вне его.
type PlanJob struct {
	infos []model.PhotoInfo
	asp   []float64
	rate  []int // действующие оценки infos (nil — нули): любимые развороты ч/б не берёт
	prm   layout.Params
	have  int
	tail  int // пустые развороты в конце: сборка займёт их первыми (035 U1)
	// drop — похожие кадры, которых сборка не берёт (078, Without); их ищет ядро (078-15).
	drop map[string]bool
}

// PlanOf — вход автосборки из проекта; photos уже без повторов. Пропорции — как их увидит
// AutoBuild после Remember: размеры из photos, поворот — у фото, уже стоящего в проекте.
// rate — действующие оценки (nil — нет).
func PlanOf(p *model.Project, photos []model.PhotoInfo, rate Rating) PlanJob {
	inGrid, onBg := gridPhotoSet(p), p.OnDecor()
	infos := slices.DeleteFunc(slices.Clone(photos), func(ph model.PhotoInfo) bool { return inGrid[ph.ID] || onBg[ph.ID] })
	tmp := &model.Project{Photos: make(map[string]model.PhotoRef, len(infos))}
	for _, ph := range infos {
		if r, ok := p.Photos[ph.ID]; ok {
			tmp.Photos[ph.ID] = r
		}
	}
	j := PlanJob{infos: infos, asp: tmp.Aspects(model.Remember(tmp, infos)), prm: p.Params(), have: len(p.Spreads), tail: blankTail(p)}
	if rate != nil {
		j.rate = make([]int, len(infos))
		for i, ph := range infos {
			j.rate[i] = rate(ph.ID)
		}
	}
	return j
}

// Infos, Asp, Rates — вход сборки по порядку (без сеток и фонов): по ним ядро ищет похожие
// кадры (078-15). Только чтение, без копий.
func (j PlanJob) Infos() []model.PhotoInfo { return j.infos }
func (j PlanJob) Asp() []float64           { return j.asp }
func (j PlanJob) Rates() []int             { return j.rate }

// Without — вход без кадров drop (ID): план, сборка и предпросмотр идут по оставшимся, а
// отброшенные не попадают в проект и не снимаются со своих разворотов. Пусто — вход тот же.
func (j PlanJob) Without(drop map[string]bool) PlanJob {
	if len(drop) == 0 {
		return j
	}
	k := PlanJob{prm: j.prm, have: j.have, tail: j.tail, drop: drop}
	for i, ph := range j.infos {
		if drop[ph.ID] {
			continue
		}
		k.infos, k.asp = append(k.infos, ph), append(k.asp, j.asp[i])
		if j.rate != nil {
			k.rate = append(k.rate, j.rate[i])
		}
	}
	return k
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
	ap := model.AutoPlan{Photos: n, Similar: len(j.drop)}
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
		pl = plan.PlanRange(j.asp, j.prm, plan.PlanOpts{Spreads: N, MinPer: from, MaxPer: to, Cuts: cuts, Phase: phase(o)})
	} else {
		ap.To = m
		pl = plan.PlanGroups(j.asp, j.prm, N, m)
	}
	ap.Spreads, ap.Avg, ap.Min, ap.Max = pl.Spreads, pl.Avg, pl.Min, pl.Max
	ap.Reuse = min(j.tail, ap.Spreads)
	ap.Gray = grayCount(j.grayOf(pl, o.Gray))
	return ap, pl, nil
}

// phase — сдвиг волны размеров для варианта o.Variant (078 §7): варианты 0 и 1 — без сдвига.
func phase(o model.AutoOpts) int { return max(o.Variant, 1) - 1 }

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
	b, err := build(p, pre, at, rate, nil)
	return b.made, err
}

// Rebuild — AutoBuild, где новые развороты по порядку берут ID из reuse (лишним — новые; ID,
// уже стоящий в проекте, не берётся): пересборка (078 §8) тем же вариантом даёт те же байты, а
// выделение и переход на разворот не прыгают. Возвращает ID собранных разворотов по порядку.
func Rebuild(p *model.Project, pre Prep, at int, rate Rating, reuse []string) ([]string, error) {
	b, err := build(p, pre, at, rate, reuse)
	return b.ids, err
}

// built — итог build: позиция первого собранного разворота, их число и ID; флаги запасных
// (разрешённых семейств нет — лучшая; nil — фильтра нет) и ч/б — их читает предпросмотр (Preview).
type built struct {
	at, made int
	ids      []string
	fb, gray []bool
}

// build — тело AutoBuild и Rebuild (reuse — ID для новых разворотов, nil — все новые).
// Похожие кадры (pre.job.drop) не берутся: тот же отбор, что у плана до замка.
func build(p *model.Project, pre Prep, at int, rate Rating, reuse []string) (built, error) {
	j := PlanOf(p, pre.photos, rate).Without(pre.job.drop)
	pl, ready, err := pre.planFor(j)
	if err != nil || len(pl.Groups) == 0 {
		return built{}, err
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
		return built{}, errSpreads() // вставка в середину: план считал хвост свободным
	}
	b := built{at: at, made: len(pl.Groups), ids: newIDs(p, len(pl.Groups), reuse), gray: j.grayOf(pl, pre.o.Gray)}
	out := make([]*model.Spread, len(pl.Groups))
	for i, g := range pl.Groups {
		out[i] = &model.Spread{ID: b.ids[i], Photos: slices.Clone(ids[g[0]:g[1]])}
	}
	p.Spreads = slices.Replace(p.Spreads, at, at+use, out...)
	b.fb = varyBuilt(p, at, len(out), pre.o, rate, ready, dead)
	for i, s := range out {
		if i < len(b.gray) && b.gray[i] {
			paintGray(p, s)
		}
	}
	return b, nil
}

// newIDs — n ID новых разворотов: сначала из reuse по порядку (кроме уже стоящих в p, пустых
// и повторов), остальные — новые.
func newIDs(p *model.Project, n int, reuse []string) []string {
	taken := make(map[string]bool, len(p.Spreads)+n)
	for _, s := range p.Sheets() {
		taken[s.ID] = true
	}
	out := make([]string, n)
	for i := range out {
		if i < len(reuse) && reuse[i] != "" && !taken[reuse[i]] {
			out[i] = reuse[i]
		} else {
			out[i] = model.NewID()
		}
		taken[out[i]] = true
	}
	return out
}
