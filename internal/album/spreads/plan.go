package spreads

// Автосборка ровно на N разворотов (спека 027 §5): подсказка окна (AutoPlan) и сама
// сборка (AutoBuild) идут через один PlanJob — иначе подсказка разошлась бы со сборкой.

import (
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
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
	inGrid, onBg := gridPhotoSet(p), p.OnBackdrops()
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

// Plan — подсказка окна: spreads зажимается в [⌈n/m⌉, min(n, MaxSpreads − непустых)],
// maxPer — в 1…MaxPhotos; пустой хвост считается свободным местом (Reuse). Фото нет — пустой план без ошибки; места нет — ошибка.
func (j PlanJob) Plan(spreads, maxPer int) (model.AutoPlan, error) {
	ap, _, err := j.plan(spreads, maxPer)
	return ap, err
}

func (j PlanJob) plan(spreads, maxPer int) (model.AutoPlan, layout.Plan, error) {
	n := len(j.asp)
	if n == 0 {
		return model.AutoPlan{}, layout.Plan{}, nil
	}
	maxPer = max(1, min(maxPer, model.MaxPhotos))
	lo, hi := layout.SpreadRange(n, maxPer)
	hi = min(hi, model.MaxSpreads-(j.have-j.tail))
	ap := model.AutoPlan{Photos: n, MinSpreads: lo, MaxSpreads: max(hi, 0)}
	if hi < lo {
		return ap, layout.Plan{}, errSpreads()
	}
	pl := layout.PlanGroups(j.asp, j.prm, max(lo, min(spreads, hi)), maxPer)
	ap.Spreads, ap.Avg, ap.Min, ap.Max = pl.Spreads, pl.Avg, pl.Min, pl.Max
	ap.Reuse = min(j.tail, ap.Spreads)
	return ap, pl, nil
}

// AutoBuild — операция над проектом; pre — вход, план и прогрев до замка (Prepare), photos
// там уже без повторов. Ровно o.Spreads разворотов (зажим — как у Plan) по не больше o.MaxPer
// фото, начиная с позиции at. Фото из сеток и с фонов не берутся. Сборка в конец (at вне
// альбома или в пустом хвосте) занимает пустые развороты хвоста вместо новых, лишние пустые
// остаются после собранных (035 U1). Раскладки — по o.Variety с учётом соседей (041);
// «Меньше» — лучшие, как 0.10. rate — оценки фото для звезды разворота.
// Возвращает число новых разворотов.
func AutoBuild(p *model.Project, pre Prep, at int, rate Rating) (made int, err error) {
	j := PlanOf(p, pre.photos)
	pl, ready, err := pre.planFor(j)
	if err != nil || len(pl.Groups) == 0 {
		return 0, err
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
		return 0, errSpreads() // вставка в середину: план считал хвост свободным
	}
	out := make([]*model.Spread, len(pl.Groups))
	for i, g := range pl.Groups {
		out[i] = &model.Spread{ID: model.NewID(), Photos: slices.Clone(ids[g[0]:g[1]])}
	}
	p.Spreads = slices.Replace(p.Spreads, at, at+use, out...)
	varyBuilt(p, at, len(out), pre.o, rate, ready, dead)
	return len(out), nil
}
