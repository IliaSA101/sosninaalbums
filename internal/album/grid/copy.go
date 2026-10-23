package grid

import (
	"errors"
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
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
	boxes, err := FracBoxes(p, s, tplScope, scope, frac)
	if err != nil {
		return 0, err
	}
	return applyGrid(p, s, scope, boxes)
}

// srcDesign — тексты, стикеры и фон (цвет, фото, узор) области источника для копирования
// «с текстом и фоном».
type srcDesign struct {
	texts    []model.TextBlock // копии: координаты источника
	stickers []*model.Sticker  // копии (048, решение №101): координаты источника
	x0       float64           // левый край области источника
	bgL      string            // фон левой страницы области (у источника-страницы — её фон)
	bgR      string
	// Фото фона области: весь разворот / левая (у источника-страницы — её) / правая.
	photo, photoL, photoR *model.Backdrop
	pat, patL, patR       *model.Pattern // узоры области (048) — по тем же слотам, что фото
	head                  string         // ID шапки виньетки-источника среди texts ("" — нет)
}

// CopyGrid копирует места области from разворота src на цели одной правкой:
// источник из «Авто» сначала закрепляется (эталон не разойдётся с копиями),
// цель «та же область того же разворота» пропускается. design — ещё тексты и фон
// области; mirror — копия со страницы на страницу другой стороны отражается (места и стикеры).
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
	fa, ok := p.AreaOf(src, from)
	if !ok {
		return plan, model.ErrScope
	}
	a0 := fa.X
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
	var seq []string // виды копируемых мест, стикеров и текстов в стеке источника снизу вверх
	if design {
		sd = designOf(p, src, from, a0)
		seq = areaStack(p, src, from, sd)
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
		// Копия на страницу другой стороны отражается: места и (с оформлением) стикеры.
		flip := mirror && from != model.ScopeSpread && t.Scope != from
		f := frac
		if flip {
			f = mirrorFrac(frac)
		}
		boxes, err := FracBoxes(p, s, tpl, t.Scope, f)
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
			// Стикеры отражаются вместе с местами — как в «Копировать оформление».
			n, k, err := putDesign(p, s, t.Scope, sd, putOpt{texts: true, flip: flip, own: s == src})
			if err != nil {
				return plan, err
			}
			plan.Texts += n
			plan.Stickers += k
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
		sd.pat, sd.patL, sd.patR = s.Pat.Clone(), s.PatL.Clone(), s.PatR.Clone()
	case model.ScopeLeft:
		sd.photoL, sd.patL = s.BgPhotoL.Clone(), pageOrSheet(s.PatL, s.Pat)
	case model.ScopeRight:
		sd.bgL, sd.photoL, sd.patL = s.BgR, s.BgPhotoR.Clone(), pageOrSheet(s.PatR, s.Pat)
	}
	if scope != model.ScopeSpread {
		// Фото всего листа лежит и на странице, но не едет (фото не разрезать): только для
		// сверки повтора — у цели другого листа putDesign его снимает.
		sd.photo = s.BgPhoto.Clone()
	}
	for _, t := range s.Texts {
		if a := model.BoxOf(t).AABB(); InArea(p, s, a.X, a.W, scope) {
			sd.texts = append(sd.texts, *t)
			if s.IsVignette() && t.ID == s.Vig.Head {
				sd.head = t.ID
			}
		}
	}
	for _, st := range s.Stickers {
		if stickerIn(p, s, st, scope) {
			sd.stickers = append(sd.stickers, st.Clone())
		}
	}
	return sd
}

// pageOrSheet — узор, видный на странице: её свой, иначе узор всего листа (они взаимоисключающие).
func pageOrSheet(page, sheet *model.Pattern) *model.Pattern {
	if page != nil {
		return page.Clone()
	}
	return sheet.Clone()
}

// stickerIn — стикер лежит в области scope листа s (по описанному прямоугольнику, как текст).
func stickerIn(p *model.Project, s *model.Spread, st *model.Sticker, scope string) bool {
	a := st.Rect().AABB()
	return InArea(p, s, a.X, a.W, scope)
}

// putOpt — что и как кладёт putDesign. texts — надписи области заменяются; flip — стикеры
// отражаются в области цели (зеркальная копия на страницу другой стороны); decor — режим
// «Копировать оформление» (049 Ф2): шапка виньетки цели стоит, стикеры копии зажимаются;
// own — цель на листе-источнике: фото всего листа у неё общее с источником и остаётся.
type putOpt struct{ texts, flip, decor, own bool }

// putDesign заменяет оформление области scope разворота s на источник: фон, узоры, фото
// фона, стикеры и (при o.texts) тексты — новые ID, сдвиг на страницу. n, k — сколько текстов
// и стикеров цели заменено. Больше MaxTexts или MaxStickers на развороте — ошибка всей правки.
func putDesign(p *model.Project, s *model.Spread, scope string, sd srcDesign, o putOpt) (n, k int, err error) {
	m := placed(p, s, scope, sd, o)
	if o.texts {
		head := ""
		if o.decor && s.IsVignette() {
			head = s.Vig.Head // шапка — часть виньетки, а не оформления
		}
		before := len(s.Texts)
		s.Texts = slices.DeleteFunc(s.Texts, func(t *model.TextBlock) bool {
			a := model.BoxOf(t).AABB()
			return t.ID != head && InArea(p, s, a.X, a.W, scope)
		})
		n = before - len(s.Texts)
		for _, t := range m.texts {
			c := t
			c.ID = model.NewID()
			s.Texts = append(s.Texts, &c)
			// Шапка источника едет с раскладкой шапкой цели, как у «Дублировать»: иначе
			// «Применить» поставит вторую. Своя шапка цели, оставшаяся вне области, — важнее.
			if !o.decor && sd.head != "" && t.ID == sd.head && s.IsVignette() && s.VigHead(s.Vig.Head) == nil {
				v := *s.Vig
				v.Head = c.ID
				s.Vig = &v
			}
		}
		if len(s.Texts) > model.MaxTexts {
			return n, k, fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
		}
	}
	before := len(s.Stickers)
	s.Stickers = slices.DeleteFunc(s.Stickers, func(st *model.Sticker) bool { return stickerIn(p, s, st, scope) })
	k = before - len(s.Stickers)
	for _, st := range m.stickers {
		c := st.Clone()
		c.ID = model.NewID()
		s.Stickers = append(s.Stickers, c)
	}
	if len(s.Stickers) > model.MaxStickers {
		return n, k, model.ErrStickers
	}
	if len(s.Stickers) == 0 {
		s.Stickers = nil
	}
	switch scope {
	case model.ScopeLeft:
		s.BgL, s.BgPhotoL = m.bgL, m.photoL
		putPattern(s, scope, m.patL)
	case model.ScopeRight:
		s.BgR, s.BgPhotoR = m.bgL, m.photoL
		putPattern(s, scope, m.patL)
	default:
		s.BgL, s.BgR = m.bgL, m.bgR
		s.Pat, s.PatL, s.PatR = m.pat, m.patL, m.patR
		s.BgPhoto, s.BgPhotoL, s.BgPhotoR = m.photo, m.photoL, m.photoR
	}
	if scope != model.ScopeSpread && (!o.own || s.BgPhotoL != nil || s.BgPhotoR != nil) {
		// Фото чужого листа лежит и на этой странице — снимается, как у dropPhotos (048). Своё
		// (соседняя страница источника) остаётся: оно и есть фон источника. Фото страниц и
		// листа взаимоисключающие.
		s.BgPhoto = nil
	}
	if len(s.Texts) == 0 {
		s.Texts = nil
	}
	return n, k, nil
}

// placed — оформление sd в координатах области scope листа s (ID — источника): сдвиг на
// страницу; у o.flip стикеры отражаются в области (центр, FlipH, угол −Rot). В режиме decor
// стикеры зажимаются, как при постановке: отражённый край — новые числа, а сверка повтора
// (sameDecor) и запись должны видеть одно и то же.
func placed(p *model.Project, s *model.Spread, scope string, sd srcDesign, o putOpt) srcDesign {
	a := p.BdArea(s, scope)
	x0, dx := a.X, a.X-sd.x0
	m := srcDesign{x0: x0, bgL: sd.bgL, bgR: sd.bgR}
	for _, t := range sd.texts {
		c := t
		c.X = t.X - sd.x0 + x0
		m.texts = append(m.texts, c)
	}
	for _, st := range sd.stickers {
		c := st.Clone()
		// Округление, как у ClampSticker: сумма двух чисел с двумя знаками бывает 120.75999999999999 —
		// круг записи (clampStickers при чтении) поменял бы её.
		c.X = geom.Round2(st.X + dx)
		if o.flip {
			c.X, c.FlipH, c.Rot = geom.Round2(2*a.X+a.W-c.X-c.W), !c.FlipH, geom.NormDeg(-c.Rot)
		}
		if o.decor {
			if c = p.ClampSticker(s, c); c == nil {
				continue
			}
		}
		m.stickers = append(m.stickers, c)
	}
	if scope == model.ScopeSpread {
		m.photo = shifted(p, s, sd.photo, dx, scope)
		m.photoL, m.photoR = shifted(p, s, sd.photoL, dx, model.ScopeLeft), shifted(p, s, sd.photoR, dx, model.ScopeRight)
		m.pat, m.patL, m.patR = sd.pat.Clone(), sd.patL.Clone(), sd.patR.Clone()
		return m
	}
	m.photoL, m.patL = shifted(p, s, sd.photoL, dx, scope), sd.patL.Clone()
	if o.own && m.photoL == nil {
		m.photo = sd.photo.Clone() // фото листа-источника остаётся у цели (putDesign)
	}
	return m
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
		newID := make(map[string]string, len(s.Grid)+len(s.Stickers)+len(s.Texts)) // ID источника → ID копии
		for j, pl := range s.Grid {
			ns.Grid[j] = model.ClonePlace(pl, taken)
			newID[pl.ID] = ns.Grid[j].ID
		}
		if design {
			ns.BgL, ns.BgR = s.BgL, s.BgR
			ns.BgPhoto, ns.BgPhotoL, ns.BgPhotoR = s.BgPhoto.Clone(), s.BgPhotoL.Clone(), s.BgPhotoR.Clone()
			ns.Pat, ns.PatL, ns.PatR = s.Pat.Clone(), s.PatL.Clone(), s.PatR.Clone()
			if s.Caps != nil { // стиль подписей листа (030) — часть оформления
				c := *s.Caps
				ns.Caps = &c
			}
			if s.Vars != nil { // поля шапки (031) — вместе с шапкой-надписью
				v := *s.Vars
				ns.Vars = &v
			}
			for _, t := range s.Texts {
				c := *t
				c.ID = model.NewID()
				ns.Texts = append(ns.Texts, &c)
				newID[t.ID] = c.ID
			}
			for _, st := range s.Stickers {
				c := st.Clone()
				c.ID = model.NewID()
				ns.Stickers = append(ns.Stickers, c)
				newID[st.ID] = c.ID
			}
		}
		if s.IsVignette() { // копия виньетки — виньетка с теми же ролями мест (031); шапка — своя копия
			v := *s.Vig
			v.Head = newID[v.Head]
			ns.Kind, ns.Vig = s.Kind, &v
		}
		// Порядок слоёв — по карте ID (текст под фото остаётся под ним); без
		// design тексты и стикеры не копируются и выпадают, лишнее syncOrder сведёт к nil.
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

// areaStack — виды (place | sticker | text) мест области from, стикеров и текстов sd
// (копий из designOf) в стеке источника снизу вверх: по ним putOrder ставит копии
// в том же порядке, что у источника. Места области идут в порядке Grid — как у
// areaFrac, стикеры и тексты — как в sd.
func areaStack(p *model.Project, src *model.Spread, from string, sd srcDesign) []string {
	inText := make(map[string]bool, len(sd.texts))
	for _, t := range sd.texts {
		inText[t.ID] = true
	}
	inSticker := make(map[string]bool, len(sd.stickers))
	for _, st := range sd.stickers {
		inSticker[st.ID] = true
	}
	kind := src.Kinds()
	var seq []string
	for _, id := range src.Stack() {
		switch {
		case kind[id] == model.LayerPlace:
			if _, pl := src.FindPlace(id); InArea(p, src, pl.X, pl.W, from) {
				seq = append(seq, model.LayerPlace)
			}
		case inSticker[id]:
			seq = append(seq, model.LayerSticker)
		case inText[id]:
			seq = append(seq, model.LayerText)
		}
	}
	return seq
}

// putOrder ставит новые места, стикеры и тексты копии (чьих ID нет в old) в стек цели s
// одним куском в порядке источника seq — на позицию первого нового места;
// остальное — как было. Без этого текст, лежавший в источнике под фото, в копии
// всплыл бы наверх (syncOrder кладёт недостающий текст наверх).
func putOrder(s *model.Spread, old map[string]string, seq []string) {
	fresh := map[string][]string{} // вид → новые ID в порядке Grid / Stickers / Texts — как у источника
	add := func(kind, id string) {
		if old[id] == "" {
			fresh[kind] = append(fresh[kind], id)
		}
	}
	for _, pl := range s.Grid {
		add(model.LayerPlace, pl.ID)
	}
	for _, st := range s.Stickers {
		add(model.LayerSticker, st.ID)
	}
	for _, t := range s.Texts {
		add(model.LayerText, t.ID)
	}
	places := fresh[model.LayerPlace]
	chunk := make([]string, 0, len(fresh[model.LayerPlace])+len(fresh[model.LayerSticker])+len(fresh[model.LayerText]))
	for _, k := range seq {
		if ids := fresh[k]; len(ids) > 0 {
			chunk = append(chunk, ids[0])
			fresh[k] = ids[1:]
		}
	}
	for _, k := range []string{model.LayerPlace, model.LayerSticker, model.LayerText} { // страховка: seq короче
		chunk = append(chunk, fresh[k]...)
	}
	rest := make([]string, 0, len(s.Grid)+len(s.Stickers)+len(s.Texts))
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

// putPattern — узор страницы scope цели из источника (nil — без узора). Узор всего листа цели
// остаётся другой странице (SplitPat), как у dropPatterns (048): и при пустом узоре источника —
// иначе страница цели так и осталась бы в узоре листа.
func putPattern(s *model.Spread, scope string, pt *model.Pattern) {
	s.SplitPat(scope)
	*s.PatSlot(scope) = pt.Clone()
}

// shifted — копия фона, сдвинутая по x на dx (перенос на другую страницу листа s).
func shifted(p *model.Project, s *model.Spread, b *model.Backdrop, dx float64, scope string) *model.Backdrop {
	if b == nil {
		return nil
	}
	c := b.Clone()
	c.X += dx
	return p.ClampBackdrop(s, c, scope)
}
