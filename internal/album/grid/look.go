package grid

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Макет альбома (131 design §4): шаг серии встаёт на разворот тем же ApplySpreadTpl, что и
// готовый разворот 067, но места берутся по LookBoxes — композиция шага целиком вписана в область
// формата (0.27-131-20). Серию, порядок и виньетки собирает ядро (области друг друга не знают).

// PutLookStep — только серия по несобранному альбому (разворот заменяется целиком, свои места снимаются
// без вопроса); шаг на лист собранного альбома — PutLookTile / ApplySpreadTpl.
// PutLookStep ставит шаг st макета на разворот spreadID целиком: места по LookBoxes, рамочки,
// оформление, надписи и заготовки; Spread.Name — глава шага, Spread.LookStep — n (1…MaxLookSteps).
// Повтор того же шага — холостая правка (ID мест и предметов прежние). Свои из библиотеки — из
// libs, как у ApplySpreadTpl; missing — ключи, которых на этом ПК нет (слой пропущен), по порядку.
// Разворот заменяется целиком: у шага нет надписей и заготовок — снимаются прежние (заготовка
// прошлого макета не переезжает в новый; шапка виньетки остаётся), нет мест (или они на этом
// листе не ставятся) — снимаются свои места, таблички имён мест (071) тоже. Отказ (пустой шаг,
// область) — до любой правки. У листа 031 (виньетка) ставится только оформление; шаблон
// разворота на нём становится страницей, у шага без оформления остаются имя и номер. Неизвестный
// разворот — холостая, обложка — ErrScope (как у ApplySpreadTpl).
func PutLookStep(p *model.Project, spreadID string, st model.LookStep, n int, libs map[string]model.PhotoInfo) (plan model.GridPlan, missing []string, err error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return plan, nil, nil
	}
	t, ok := lookTpl(p, s, st.Tpl)
	if !ok && !s.IsVignette() {
		return plan, nil, model.ErrTplEmpty
	}
	if len(st.Tpl.Texts)+len(st.Tpl.Drafts) == 0 {
		s.Texts = slices.DeleteFunc(s.Texts, func(tx *model.TextBlock) bool { return !s.IsVignette() || tx.ID != s.Vig.Head })
	}
	if !s.IsVignette() && !lookSetsPlaces(p, s, t) {
		s.Grid, s.Order = nil, nil // свои места шаг не заменил бы — разворот без мест, как шаг
	}
	for _, pl := range s.Grid {
		pl.Person = model.Person{}
	}
	if ok {
		var gone map[string]bool
		if plan, gone, err = applySpreadTpl(p, spreadID, model.ScopeSpread, t, libs); err != nil {
			return model.GridPlan{}, nil, err
		}
		missing = slices.Sorted(maps.Keys(gone))
	}
	s.Name, s.LookStep = model.CleanName(st.Name), n
	return plan, missing, nil
}

// lookSetsPlaces — шаг t ставит места на лист s (как ApplySpreadTpl): места есть, у листа две
// страницы, это не лист 031.
func lookSetsPlaces(p *model.Project, s *model.Spread, t model.SpreadTpl) bool {
	_, two := p.AreaOf(s, model.ScopeLeft)
	return len(t.Places) > 0 && two && !s.IsVignette()
}

// lookTpl — шаблон шага для листа s: места по LookBoxes области шага (разворот — весь лист,
// страница — левая страница; у листа 031 страниц нет — весь лист). false — на листе 031 нечего
// ставить.
func lookTpl(p *model.Project, s *model.Spread, t model.SpreadTpl) (model.SpreadTpl, bool) {
	if s.IsVignette() {
		t = vigPage(t)
	}
	scope := model.ScopeSpread
	if _, two := p.AreaOf(s, model.ScopeLeft); two && t.Pages == 1 {
		scope = model.ScopeLeft
	}
	if a, ok := p.AreaOf(s, scope); ok && a.H > 0 {
		t.Places = model.LookBoxes(t, a.W/a.H)
	}
	c, ok := model.ClampSpreadTpl(t)
	return c, ok
}

// vigPage — шаблон разворота как страница для листа 031: страницы у него нет, места строит
// виньетка. Цвет левой страницы, узор и фото фона всего разворота (если у страницы своих нет)
// переходят на страницу; места и рамочки отбрасываются.
func vigPage(t model.SpreadTpl) model.SpreadTpl {
	t.Places, t.Borders = nil, nil
	if t.Pages != 2 || len(t.Page) == 0 {
		return t
	}
	pg := t.Page[0]
	if w := t.Whole; w != nil {
		if pg.Pat == nil {
			pg.Pat = w.Pat
		}
		if pg.Lib == "" {
			pg.Lib, pg.Fade = w.Lib, w.Fade
		}
	}
	t.Pages, t.Page, t.Whole = 1, []model.TplBg{pg}, nil
	return t
}

// LookUntouched — разворот spreadID копии c нетронут: повтор шага st (по номеру n) его не меняет,
// на нём нет табличек имён (071) и чужих надписей (у виньетки шапка не считается); свои места на
// шаге без мест повтор снимает, и сравнение это видит. Копия c
// меняется — её делает вызывающий для сверки всех разворотов разом.
func LookUntouched(c *model.Project, spreadID string, st model.LookStep, n int, libs map[string]model.PhotoInfo) bool {
	_, s := c.Find(spreadID)
	if s == nil {
		return false
	}
	for _, pl := range s.Grid {
		if pl.Person != (model.Person{}) {
			return false
		}
	}
	if len(st.Tpl.Texts)+len(st.Tpl.Drafts) == 0 {
		for _, tx := range s.Texts {
			if !s.IsVignette() || tx.ID != s.Vig.Head {
				return false // своя надпись шаблон не снимает — а замена макета её унесёт
			}
		}
	}
	before, err1 := json.Marshal(s)
	_, _, err2 := PutLookStep(c, spreadID, st, n, libs)
	after, err3 := json.Marshal(s)
	return err1 == nil && err2 == nil && err3 == nil && bytes.Equal(before, after)
}

// PutLookTile ставит шаг st на лист spreadID как плитку «Развороты» 067 («Поставить на разворот»,
// 131 design §4): места по LookBoxes, рамочки, оформление; фото — по порядку мест, лишние — в
// ленту (plan.Left). В отличие от PutLookStep разворот не заменяется целиком: надписи, которых нет
// в шаге, остаются (как у плитки 067), названия и таблички имён не трогаются. Spread.LookStep = n
// (0 — шаг чужого макета, номера нет). Повтор — холостая правка; обложка — ErrScope; неизвестный
// лист — холостая; шаг без мест и оформления — ErrTplEmpty (у листа 031 — холостая).
func PutLookTile(p *model.Project, spreadID string, st model.LookStep, n int, libs map[string]model.PhotoInfo) (plan model.GridPlan, missing []string, err error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return plan, nil, nil
	}
	t, ok := lookTpl(p, s, st.Tpl)
	if !ok {
		if s.IsVignette() {
			return plan, nil, nil
		}
		return plan, nil, model.ErrTplEmpty
	}
	var gone map[string]bool
	if plan, gone, err = applySpreadTpl(p, spreadID, model.ScopeSpread, t, libs); err != nil {
		return model.GridPlan{}, nil, err
	}
	s.LookStep = n
	return plan, slices.Sorted(maps.Keys(gone)), nil
}
