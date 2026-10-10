package grid

import (
	"errors"
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Копирование, размножение и постановка сетки (спека 007, design §3).

// MaxDuplicate — новых разворотов за одно «Размножить».
const MaxDuplicate = 100

var errTargets = errors.New("слишком много целей копирования")

// ApplyGrid ставит сетку по долям frac области шаблона tplScope (TplPage |
// TplSpread) на область scope разворота (шаблоны и встроенные сетки). Доли
// зажимаются (clampBoxes, NaN — отказ), угол коробки — угол места. left — фото,
// ушедшие в ленту.
func ApplyGrid(p *model.Project, spreadID, scope, tplScope string, frac []model.Box) (left int, err error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return 0, nil
	}
	boxes, err := FracBoxes(p, tplScope, scope, frac)
	if err != nil {
		return 0, err
	}
	return applyGrid(p, s, scope, boxes)
}

// srcDesign — тексты и фон области источника для копирования «с текстом и фоном».
type srcDesign struct {
	texts []model.TextBlock // копии: координаты источника
	x0    float64           // левый край области источника
	bgL   string            // фон левой страницы области (у источника-страницы — её фон)
	bgR   string
	// Фото фона области: весь разворот / левая (у источника-страницы — её) / правая.
	photo, photoL, photoR *model.Backdrop
}

// CopyGrid копирует места области from разворота src на цели одной правкой:
// источник из «Авто» сначала закрепляется (эталон не разойдётся с копиями),
// цель «та же область того же разворота» пропускается. design — ещё тексты и фон
// области; mirror — копия со страницы на страницу другой стороны отражается.
// left — фото, ушедшие в ленту (GridPlan.Left). Неизвестный src — холостая правка.
func CopyGrid(p *model.Project, srcID, from string, targets []model.GridTarget, design, mirror bool) (model.GridPlan, error) {
	return copyGrid(p, srcID, from, targets, design, mirror)
}

// copyGrid — тело CopyGrid; имя стоит в исключениях funlen/gocognit линтера.
func copyGrid(p *model.Project, srcID, from string, targets []model.GridTarget, design, mirror bool) (model.GridPlan, error) {
	var plan model.GridPlan
	_, src := p.Find(srcID)
	if src == nil {
		return plan, nil
	}
	a0, _, ok := p.Area(from)
	if !ok {
		return plan, model.ErrScope
	}
	if len(targets) > 3*model.MaxSpreads {
		return plan, errTargets
	}
	wasGrid := make(map[*model.Spread]bool, len(p.Spreads))
	for _, s := range p.Spreads {
		wasGrid[s] = len(s.Grid) > 0
	}
	p.Freeze(src)
	frac := AreaFrac(p, src, from)
	if len(frac) == 0 {
		return plan, model.ErrNoPlaces
	}
	var sd srcDesign
	var seq []string // виды копируемых мест и текстов в стеке источника снизу вверх
	if design {
		sd = designOf(p, src, from, a0)
		seq = areaStack(p, src, from, sd.texts)
	}
	seen := map[model.GridTarget]bool{}
	for _, t := range expandTargets(targets, from) {
		if seen[t] {
			continue
		}
		seen[t] = true
		_, s := p.Find(t.Spread)
		if s == nil || s == src && t.Scope == from {
			continue
		}
		tpl := model.TplPage
		if from == model.ScopeSpread {
			tpl = model.TplSpread
		}
		f := frac
		if mirror && from != model.ScopeSpread && t.Scope != from {
			f = mirrorFrac(frac)
		}
		boxes, err := FracBoxes(p, tpl, t.Scope, f)
		if err != nil {
			return plan, err
		}
		old := s.Kinds() // до правки: новые места и тексты — те, чьих ID тут нет
		left, err := applyGrid(p, s, t.Scope, boxes)
		if err != nil {
			return plan, err
		}
		plan.Left += left
		if design {
			n, err := putDesign(p, s, t.Scope, sd)
			if err != nil {
				return plan, err
			}
			plan.Texts += n
			putOrder(s, old, seq)
		}
	}
	for _, s := range p.Spreads {
		if len(s.Grid) > 0 && !wasGrid[s] {
			plan.Frozen++
		}
	}
	return plan, nil
}

// PlanCopyGrid — сухой прогон CopyGrid на копии проекта c (её делает обёртка под замком):
// без снимка, события и rev. Ошибка — пустой план (её покажет сам CopyGrid).
func PlanCopyGrid(c *model.Project, src, from string, targets []model.GridTarget, design, mirror bool) model.GridPlan {
	plan, err := CopyGrid(c, src, from, targets, design, mirror)
	if err != nil {
		return model.GridPlan{}
	}
	return plan
}

// expandTargets: со страницы-источника цель «разворот» — это обе его страницы.
func expandTargets(targets []model.GridTarget, from string) []model.GridTarget {
	if from == model.ScopeSpread {
		return targets
	}
	out := make([]model.GridTarget, 0, len(targets))
	for _, t := range targets {
		if t.Scope == model.ScopeSpread {
			out = append(out, model.GridTarget{Spread: t.Spread, Scope: model.ScopeLeft}, model.GridTarget{Spread: t.Spread, Scope: model.ScopeRight})
			continue
		}
		out = append(out, t)
	}
	return out
}

// designOf — тексты (центр описанного прямоугольника блока в области) и фон области scope разворота s.
func designOf(p *model.Project, s *model.Spread, scope string, x0 float64) srcDesign {
	sd := srcDesign{x0: x0, bgL: s.BgL, bgR: s.BgR}
	switch scope {
	case model.ScopeSpread:
		sd.photo, sd.photoL, sd.photoR = s.BgPhoto.Clone(), s.BgPhotoL.Clone(), s.BgPhotoR.Clone()
	case model.ScopeLeft:
		sd.photoL = s.BgPhotoL.Clone()
	case model.ScopeRight:
		sd.bgL, sd.photoL = s.BgR, s.BgPhotoR.Clone()
	}
	for _, t := range s.Texts {
		if a := model.BoxOf(t).AABB(); InArea(p, a.X, a.W, scope) {
			sd.texts = append(sd.texts, *t)
		}
	}
	return sd
}

// putDesign заменяет тексты и фон области scope разворота s на источник
// (новые ID, сдвиг на страницу). n — сколько блоков цели заменено. Больше
// MaxTexts на развороте — ошибка всей правки.
func putDesign(p *model.Project, s *model.Spread, scope string, sd srcDesign) (n int, err error) {
	x0, _, _ := p.Area(scope)
	before := len(s.Texts)
	s.Texts = slices.DeleteFunc(s.Texts, func(t *model.TextBlock) bool {
		a := model.BoxOf(t).AABB()
		return InArea(p, a.X, a.W, scope)
	})
	n = before - len(s.Texts)
	for _, t := range sd.texts {
		c := t
		c.ID, c.X = model.NewID(), t.X-sd.x0+x0
		s.Texts = append(s.Texts, &c)
	}
	if len(s.Texts) > model.MaxTexts {
		return n, fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
	}
	dx := x0 - sd.x0
	switch scope {
	case model.ScopeLeft:
		s.BgL = sd.bgL
		s.BgPhotoL = shifted(p, sd.photoL, dx, scope)
	case model.ScopeRight:
		s.BgR = sd.bgL
		s.BgPhotoR = shifted(p, sd.photoL, dx, scope)
	default:
		s.BgL, s.BgR = sd.bgL, sd.bgR
		s.BgPhoto = shifted(p, sd.photo, dx, scope)
		s.BgPhotoL, s.BgPhotoR = shifted(p, sd.photoL, dx, model.ScopeLeft), shifted(p, sd.photoR, dx, model.ScopeRight)
	}
	if scope != model.ScopeSpread && (s.BgPhotoL != nil || s.BgPhotoR != nil) {
		s.BgPhoto = nil // фоны страниц и всего разворота взаимоисключающие
	}
	if len(s.Texts) == 0 {
		s.Texts = nil
	}
	return n, nil
}

// DuplicateGrid — count (1..MaxDuplicate) новых разворотов сразу после src с
// местами src без фото; design — ещё тексты и фон. Источник из «Авто»
// закрепляется. Нет мест — ошибка; больше MaxSpreads разворотов — ошибка.
func DuplicateGrid(p *model.Project, src string, count int, design bool) error {
	count = min(count, MaxDuplicate)
	i, s := p.Find(src)
	if s == nil {
		return nil
	}
	if len(p.Spreads)+count > model.MaxSpreads {
		return fmt.Errorf("в альбоме не может быть больше %d разворотов", model.MaxSpreads)
	}
	p.Freeze(s)
	if len(s.Grid) == 0 {
		return model.ErrNoPlaces
	}
	taken := p.PlaceIDs()
	fresh := make([]*model.Spread, count)
	st := s.Stack()
	for k := range fresh {
		ns := &model.Spread{ID: model.NewID(), Photos: []string{}, Grid: make([]*model.Place, len(s.Grid))}
		newID := make(map[string]string, len(s.Grid)+len(s.Texts)) // ID источника → ID копии
		for j, pl := range s.Grid {
			ns.Grid[j] = model.ClonePlace(pl, taken)
			newID[pl.ID] = ns.Grid[j].ID
		}
		if design {
			ns.BgL, ns.BgR = s.BgL, s.BgR
			ns.BgPhoto, ns.BgPhotoL, ns.BgPhotoR = s.BgPhoto.Clone(), s.BgPhotoL.Clone(), s.BgPhotoR.Clone()
			for _, t := range s.Texts {
				c := *t
				c.ID = model.NewID()
				ns.Texts = append(ns.Texts, &c)
				newID[t.ID] = c.ID
			}
		}
		// Порядок слоёв — по карте ID (текст под фото остаётся под ним); без
		// design тексты не копируются и выпадают, лишнее syncOrder сведёт к nil.
		for _, id := range st {
			if n, ok := newID[id]; ok {
				ns.Order = append(ns.Order, n)
			}
		}
		fresh[k] = ns
	}
	p.Spreads = slices.Insert(p.Spreads, i+1, fresh...)
	return nil
}

// areaStack — виды (layerPlace | layerText) мест области from и текстов texts
// (копий из designOf) в стеке источника снизу вверх: по ним putOrder ставит копии
// в том же порядке, что у источника. Места области идут в порядке Grid — как у
// areaFrac, тексты — как в texts.
func areaStack(p *model.Project, src *model.Spread, from string, texts []model.TextBlock) []string {
	inText := make(map[string]bool, len(texts))
	for _, t := range texts {
		inText[t.ID] = true
	}
	kind := src.Kinds()
	var seq []string
	for _, id := range src.Stack() {
		switch {
		case kind[id] == model.LayerPlace:
			if _, pl := src.FindPlace(id); InArea(p, pl.X, pl.W, from) {
				seq = append(seq, model.LayerPlace)
			}
		case inText[id]:
			seq = append(seq, model.LayerText)
		}
	}
	return seq
}

// putOrder ставит новые места и тексты копии (чьих ID нет в old) в стек цели s
// одним куском в порядке источника seq — на позицию первого нового места;
// остальное — как было. Без этого текст, лежавший в источнике под фото, в копии
// всплыл бы наверх (syncOrder кладёт недостающий текст наверх).
func putOrder(s *model.Spread, old map[string]string, seq []string) {
	var places, texts []string // новые, в порядке Grid / Texts — как у источника
	for _, pl := range s.Grid {
		if old[pl.ID] == "" {
			places = append(places, pl.ID)
		}
	}
	for _, t := range s.Texts {
		if old[t.ID] == "" {
			texts = append(texts, t.ID)
		}
	}
	chunk := make([]string, 0, len(places)+len(texts))
	pi, ti := 0, 0
	for _, k := range seq {
		if k == model.LayerPlace && pi < len(places) {
			chunk = append(chunk, places[pi])
			pi++
		} else if k == model.LayerText && ti < len(texts) {
			chunk = append(chunk, texts[ti])
			ti++
		}
	}
	chunk = append(append(chunk, places[pi:]...), texts[ti:]...) // страховка: seq короче
	rest := make([]string, 0, len(s.Grid)+len(s.Texts))
	at := -1
	for _, id := range s.Stack() {
		if !slices.Contains(chunk, id) {
			rest = append(rest, id)
		} else if at < 0 && len(places) > 0 && id == places[0] {
			at = len(rest)
		}
	}
	if at < 0 {
		at = len(rest)
	}
	s.SetStack(slices.Insert(rest, at, chunk...))
}

// shifted — копия фона, сдвинутая по x на dx (перенос на другую страницу).
func shifted(p *model.Project, b *model.Backdrop, dx float64, scope string) *model.Backdrop {
	if b == nil {
		return nil
	}
	c := b.Clone()
	c.X += dx
	return p.ClampBackdrop(c, scope)
}
