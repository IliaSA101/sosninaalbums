package spreads

// «Весь альбом заново» (107, design §4–5): пул — фото голых разворотов «Авто» по порядку
// альбома (плюс выбранные свободные), всё остальное — якоря. Якорь остаётся байт в байт со
// своими фото, а в плане стоит стеной: группа не пересекает позицию, где он стоял в пуле.
// План, подсказка окна, «Что соберётся» и сборка идут через один PlanJob.

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// anchor — разворот-якорь: s — в проекте, на котором снят вход (только для сборки на нём же),
// id и wall — для сверки входа до замка и под ним; wall — число фото пула перед якорем.
type anchor struct {
	s    *model.Spread
	id   string
	wall int
}

// same — тот же якорь на том же месте пула (s у входа до замка может быть чужим указателем).
func (a anchor) same(b anchor) bool { return a.id == b.id && a.wall == b.wall }

// bare — голый разворот: «Авто» с фото и ничего своего (ни сетки, ни виньетки, ни текстов,
// стикеров, фона, цвета, узора, названия, стиля подписей, полей шапки; раскладка не в счёт —
// пересборка её и меняет). Новое поле разворота с данными пользователя попадает в Spread.Empty
// и тем самым сюда.
func bare(s *model.Spread) bool {
	c := *s
	c.Photos, c.Layout, c.Variant = nil, "", 0
	return len(s.Photos) > 0 && c.Empty()
}

// BareIDs — ID голых разворотов по порядку альбома: новые развороты пересборки берут их же,
// чтобы переход и выделение не прыгали.
func BareIDs(p *model.Project) []string {
	var out []string
	for _, s := range p.Spreads {
		if bare(s) {
			out = append(out, s.ID)
		}
	}
	return out
}

// walls — стены якорей в пуле (по якорю, возрастают; повторы и края план приводит сам); nil — якорей нет.
func (j PlanJob) walls() []int {
	if len(j.anchors) == 0 {
		return nil
	}
	out := make([]int, len(j.anchors))
	for i, a := range j.anchors {
		out[i] = a.wall
	}
	return out
}

// albumPlan — состав альбома для AutoPlan.Album; nil вне режима альбома.
func (j PlanJob) albumPlan() *model.AlbumPlan {
	if !j.album {
		return nil
	}
	st := j.stat
	return &st
}

// AlbumOf — вход пересборки альбома: p и photos (фото ленты по порядку, уже без повторов).
// Голых разворотов нет — пул пуст, даже если свободные пришли: пересобирать нечего. Сведения о
// фото альбома берутся из photos по ID, а нет там (файл пропал) — из записи проекта; копия (098)
// наследует время съёмки и XMP от записи своего файла, но не подпись кадра: «убрать похожие» не
// должно выбросить сделанную нарочно копию. Свободные (не в листах, не на фонах, не отсеяны
// notForBuild) встают после ближайшего предыдущего в photos фото пула, без такого — в начало
// пула; стены сдвигаются на число вставленных перед ними (якорь в самом начале остаётся первым).
func AlbumOf(p *model.Project, photos []model.PhotoInfo, rate Rating) PlanJob {
	j := PlanJob{album: true, prm: p.Params()}
	var pool []string
	for _, s := range p.Spreads {
		if bare(s) {
			j.stat.Spreads++
			pool = append(pool, s.Photos...)
			continue
		}
		j.anchors = append(j.anchors, anchor{s: s, id: s.ID, wall: len(pool)})
		switch {
		case s.Kind == model.KindVignette:
			j.stat.Vignettes++
		case len(s.Grid) > 0:
			j.stat.Manual++
		case s.Empty():
			j.stat.Blank++
		default:
			j.stat.Decor++
		}
	}
	j.stat.Photos, j.have = len(pool), len(j.anchors)
	if len(pool) == 0 {
		return j
	}
	free := freeAfter(p, photos, pool)
	ids, end := mergePool(pool, free)
	j.stat.Free = len(ids) - len(pool)
	for i := range j.anchors {
		if w := j.anchors[i].wall; w > 0 {
			j.anchors[i].wall = end[w-1]
		}
	}
	j.infos = albumInfos(p, photos, ids)
	tmp := &model.Project{Photos: make(map[string]model.PhotoRef, len(ids))}
	for _, id := range ids {
		if r, ok := p.Photos[id]; ok {
			tmp.Photos[id] = r
		}
	}
	rem := model.Remember(tmp, j.infos)
	j.asp, j.own = tmp.Aspects(rem), tmp.OwnFrame(rem)
	if rate != nil {
		j.rate = make([]int, len(ids))
		for i, id := range ids {
			j.rate[i] = rate(id)
		}
	}
	return j
}

// freeAfter — свободные фото из photos: free[k] встают после pool[k], free[-1] (ключ -1) — в начало.
func freeAfter(p *model.Project, photos []model.PhotoInfo, pool []string) map[int][]string {
	at := make(map[string]int, len(pool))
	for i, id := range pool {
		at[id] = i
	}
	placed := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			placed[id] = true
		}
	}
	skip, onBg := notForBuild(p), p.OnDecor()
	free, cur := map[int][]string{}, -1
	for _, ph := range photos {
		if i, ok := at[ph.ID]; ok {
			cur = i
		} else if !placed[ph.ID] && !onBg[ph.ID] && !skip(ph.ID) {
			free[cur] = append(free[cur], ph.ID)
		}
	}
	return free
}

// mergePool — пул со вставленными свободными и end[k] — длина пула после pool[k] со своими свободными.
func mergePool(pool []string, free map[int][]string) (ids []string, end []int) {
	ids = append(make([]string, 0, len(pool)), free[-1]...)
	end = make([]int, len(pool))
	for k, id := range pool {
		ids = append(append(ids, id), free[k]...)
		end[k] = len(ids)
	}
	return ids, end
}

// albumInfos — сведения о фото пула ids по порядку (см. AlbumOf).
func albumInfos(p *model.Project, photos []model.PhotoInfo, ids []string) []model.PhotoInfo {
	by := make(map[string]model.PhotoInfo, len(photos))
	for _, ph := range photos {
		by[ph.ID] = ph
	}
	out := make([]model.PhotoInfo, len(ids))
	for i, id := range ids {
		if ph, ok := by[id]; ok {
			out[i] = ph
			continue
		}
		if base, ok := by[model.BaseID(id)]; ok && base.ID != id { // копия: свойства файла — от его записи
			out[i] = base
			out[i].ID, out[i].Sig = id, imgsig.Sig{}
			continue
		}
		r := p.Photos[id]
		out[i] = model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H, Size: r.Size, Lib: r.Lib}
	}
	return out
}

// RebuildAlbum — операция над проектом: пересборка альбома (107) по входу pre (AlbumOf, Prepare).
// Голые развороты заменяются новыми по плану, якоря стоят между теми же фото; новые развороты
// по порядку берут ID из reuse (лишним — новые). Возвращает ID новых разворотов по порядку.
// Пул пуст — проект не меняется (холостая правка).
func RebuildAlbum(p *model.Project, pre Prep, rate Rating, reuse []string) ([]string, error) {
	b, err := buildAlbum(p, pre, rate, reuse)
	return b.ids, err
}

// WithoutBare — копия p для сверки «до правки» (settle): ID голых разворотов заменены. Новые
// развороты пересборки берут ID прежних, и KeepLayouts, найдя тот же ID с другими фото, стёр бы
// выбранную раскладку; обложка и число разворотов в копии те же (Reflow корешка).
func WithoutBare(p *model.Project) *model.Project {
	c := p.Clone()
	for _, s := range c.Spreads {
		if bare(s) {
			s.ID = "~" + s.ID
		}
	}
	return c
}

// buildAlbum — тело build для режима альбома (pre.job.album): at не действует, reuse — ID для
// новых разворотов. Прежние голые развороты уходят из списка целиком, якоря и их фото не
// трогаются; фото, не вошедшие в сборку (похожие), остаются без разворота — в ленте.
func buildAlbum(p *model.Project, pre Prep, rate Rating, reuse []string) (built, error) {
	j := AlbumOf(p, pre.photos, rate).Without(pre.job.drop)
	pl, ready, err := pre.planFor(j)
	if err != nil || len(pl.Groups) == 0 {
		return built{}, err
	}
	ids := model.Remember(p, j.infos)
	kept := make([]*model.Spread, len(j.anchors))
	for i, a := range j.anchors {
		kept[i] = a.s
	}
	p.Spreads = slices.Clone(kept) // ID прежних голых освободились: newIDs их отдаёт
	b := built{made: len(pl.Groups), ids: newIDs(p, len(pl.Groups), reuse), gray: j.grayOf(pl, pre.o.Gray)}
	out := make([]*model.Spread, len(pl.Groups))
	for i, g := range pl.Groups {
		out[i] = &model.Spread{ID: b.ids[i], Photos: slices.Clone(ids[g[0]:g[1]])}
	}
	// Группа с началом до стены — перед якорем, с началом от стены — после (стены жёсткие).
	p.Spreads = make([]*model.Spread, 0, len(kept)+len(out))
	gi := 0
	for _, a := range j.anchors {
		for ; gi < len(out) && pl.Groups[gi][0] < a.wall; gi++ {
			b.pos = append(b.pos, len(p.Spreads))
			p.Spreads = append(p.Spreads, out[gi])
		}
		p.Spreads = append(p.Spreads, a.s)
	}
	for ; gi < len(out); gi++ {
		b.pos = append(b.pos, len(p.Spreads))
		p.Spreads = append(p.Spreads, out[gi])
	}
	b.fb = varyChains(p, b.pos, pre.o, rate, ready)
	for i, s := range out {
		if i < len(b.gray) && b.gray[i] {
			paintGray(p, s)
		}
	}
	return b, nil
}

// varyChains — раскладки новых разворотов (pos — их индексы по возрастанию, они же — номера групп
// плана): каждая цепочка подряд идущих выбирается с соседями по краям — якорями и уже
// выбранными цепочками. Возвращает флаги запасных по порядку групп (nil — фильтра нет).
func varyChains(p *model.Project, pos []int, o model.AutoOpts, rate Rating, ready [][]variety.Cand) []bool {
	var fb []bool
	for i := 0; i < len(pos); {
		j := i
		for j+1 < len(pos) && pos[j+1] == pos[j]+1 {
			j++
		}
		var rd [][]variety.Cand
		if i < len(ready) {
			rd = ready[i:min(j+1, len(ready))]
		}
		if f := varyBuilt(p, pos[i], j-i+1, o, rate, rd, nil); f != nil {
			if fb == nil {
				fb = make([]bool, len(pos))
			}
			copy(fb[i:], f)
		}
		i = j + 1
	}
	return fb
}
