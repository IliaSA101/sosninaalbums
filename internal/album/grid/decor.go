package grid

import (
	"encoding/json"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Копировать оформление» (спека 049 Ф2, design §5): режим копирования сетки без мест.

// CopyDecor копирует оформление области from листа src на цели одной правкой: цвет страниц,
// узоры, фото фона, стикеры и при texts — надписи (шапка виньетки не в счёт ни у источника,
// ни у цели). У цели оно заменяется; места, фото, виньетка и «Авто» не трогаются, поэтому
// мест у источника может и не быть. mirror — стикеры на странице другой стороны отражаются.
// Цель, у которой оформление уже такое же, пропускается: повтор — холостая правка, а не шаг
// с новыми ID. Неизвестный src — холостая правка; обложка не находится (Find — развороты).
func CopyDecor(p *model.Project, srcID, from string, targets []model.GridTarget, texts, mirror bool) (model.GridPlan, error) {
	var plan model.GridPlan
	_, src := p.Find(srcID)
	if src == nil {
		return plan, nil
	}
	fa, ok := p.AreaOf(src, from)
	if !ok {
		return plan, model.ErrScope
	}
	if len(targets) > 3*model.MaxSpreads {
		return plan, errTargets
	}
	sd := designOf(p, src, from, fa.X)
	sd.texts = withoutHead(src, sd.texts)
	seq := areaStack(p, src, from, sd)
	tpl := model.TplPage
	if from == model.ScopeSpread {
		tpl = model.TplSpread
	}
	o := putOpt{texts: texts, decor: true}
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
		// Те же отказы области, что у «Копировать раскладку»: лист 031 — только весь лист,
		// весь лист — только на весь лист.
		if _, err := FracBoxes(p, s, tpl, t.Scope, nil); err != nil {
			return plan, err
		}
		o.flip = mirror && from != model.ScopeSpread && t.Scope != from
		o.own = s == src
		if sameDecor(p, s, t.Scope, sd, o) {
			continue
		}
		old := s.Kinds()
		n, k, err := putDesign(p, s, t.Scope, sd, o)
		if err != nil {
			return plan, err
		}
		plan.Texts += n
		plan.Stickers += k
		decorOrder(s, old, seq)
	}
	return plan, nil
}

// PlanCopyDecor — сухой прогон CopyDecor на копии проекта c (её делает обёртка под замком).
// Ошибка — пустой план (её покажет сам CopyDecor).
func PlanCopyDecor(c *model.Project, src, from string, targets []model.GridTarget, texts, mirror bool) model.GridPlan {
	plan, err := CopyDecor(c, src, from, targets, texts, mirror)
	if err != nil {
		return model.GridPlan{}
	}
	return plan
}

// withoutHead — надписи области без шапки виньетки листа s: шапка — часть виньетки (её место
// считает раскладка), на чужой лист она не едет и у цели не заменяется.
func withoutHead(s *model.Spread, ts []model.TextBlock) []model.TextBlock {
	if !s.IsVignette() || s.Vig.Head == "" {
		return ts
	}
	return slices.DeleteFunc(ts, func(t model.TextBlock) bool { return t.ID == s.Vig.Head })
}

// sameDecor — оформление области scope цели s уже равно копии sd: цель пропускается. ID не в
// счёт; порядок стикеров между собой и надписей между собой учитывается (срезы идут по стеку),
// их положение в стеке относительно мест — нет.
func sameDecor(p *model.Project, s *model.Spread, scope string, sd srcDesign, o putOpt) bool {
	m := placed(p, s, scope, sd, o)
	cur := designOf(p, s, scope, m.x0)
	cur.texts = withoutHead(s, cur.texts)
	if !o.texts {
		cur.texts, m.texts = nil, nil
	}
	return decorKey(cur, scope) == decorKey(m, scope)
}

// decorKey — оформление области для сравнения: JSON без ID текстов и стикеров (у копии они
// новые); у страницы второй цвет не входит (designOf кладёт цвет страницы в bgL).
func decorKey(sd srcDesign, scope string) string {
	k := struct {
		Texts                 []model.TextBlock
		Stickers              []model.Sticker
		BgL, BgR              string
		Photo, PhotoL, PhotoR *model.Backdrop
		Pat, PatL, PatR       *model.Pattern
	}{BgL: sd.bgL, Photo: sd.photo, PhotoL: sd.photoL, PhotoR: sd.photoR, Pat: sd.pat, PatL: sd.patL, PatR: sd.patR}
	if scope == model.ScopeSpread {
		k.BgR = sd.bgR
	}
	for _, t := range sd.texts {
		t.ID = ""
		k.Texts = append(k.Texts, t)
	}
	for _, st := range sd.stickers {
		c := *st.Clone()
		c.ID = ""
		k.Stickers = append(k.Stickers, c)
	}
	b, err := json.Marshal(k)
	if err != nil { // NaN в модели не живёт (Marshal проекта паникует раньше); на всякий — «не равно»
		return "!" + err.Error()
	}
	return string(b)
}

// decorOrder ставит копии стикеров и надписей (чьих ID нет в old) в стек цели s в порядке
// источника seq: лежавшие у источника под всеми его местами (фоновый орнамент) — под первое
// место листа, чтобы не закрыть фото; остальные — наверх. Места цели не двигаются.
func decorOrder(s *model.Spread, old map[string]string, seq []string) {
	fresh := map[string][]string{} // вид → новые ID в порядке Stickers / Texts — как у источника
	for _, st := range s.Stickers {
		if old[st.ID] == "" {
			fresh[model.LayerSticker] = append(fresh[model.LayerSticker], st.ID)
		}
	}
	for _, t := range s.Texts {
		if old[t.ID] == "" {
			fresh[model.LayerText] = append(fresh[model.LayerText], t.ID)
		}
	}
	first := slices.Index(seq, model.LayerPlace) // мест у источника нет — всё наверх
	var below, above []string
	for i, k := range seq {
		if ids := fresh[k]; len(ids) > 0 {
			if i < first {
				below = append(below, ids[0])
			} else {
				above = append(above, ids[0])
			}
			fresh[k] = ids[1:]
		}
	}
	above = append(append(above, fresh[model.LayerSticker]...), fresh[model.LayerText]...) // страховка: seq короче
	moved := make(map[string]bool, len(below)+len(above))
	for _, id := range slices.Concat(below, above) {
		moved[id] = true
	}
	kind := s.Kinds()
	rest := make([]string, 0, len(s.Grid)+len(s.Stickers)+len(s.Texts))
	at := -1
	for _, id := range s.Stack() {
		if moved[id] {
			continue
		}
		if at < 0 && kind[id] == model.LayerPlace {
			at = len(rest)
		}
		rest = append(rest, id)
	}
	at = max(at, 0)
	s.SetStack(slices.Concat(rest[:at], below, rest[at:], above))
}
