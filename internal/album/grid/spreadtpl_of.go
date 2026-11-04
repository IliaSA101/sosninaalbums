package grid

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Снимок области в шаблон разворота (спека 067, design §2): обратное ApplySpreadTpl.

// SpreadTplOfSkip — шаблон из области scope разворота spreadID: места (places; «Авто» — как их
// закрепила бы правка, проект не трогается) в долях области, цвет, узоры, фото фона и свои
// стикеры — только из библиотеки (с ключом Lib: кадр съёмки в шаблон не идёт, решение 6),
// рисунки, надписи (texts; шапка виньетки — часть виньетки, не входит). Геометрия — якорями от
// краёв области (design §1). Обложка и неизвестная область — ErrScope; нечего снять — ErrNothing.
// Рамочки мест (встроенные и свои из библиотеки) идут в borders вместе с местами; заготовки
// (надписи с Draft) — в drafts, а не в texts. Свои стикеры, для которых skip(ключ) верно (код
// ссылки клиента не должен уйти в шаблон, решение 0.28-131-157), пропускаются; skip nil — берутся все.
func SpreadTplOfSkip(p *model.Project, spreadID, scope string, places, texts bool, skip func(lib string) bool) (model.SpreadTpl, error) {
	return spreadTplOf(p, spreadID, scope, tplSnap{places: places, texts: texts, skip: skip})
}

// SpreadTplOfAs — снимок без skip, с флажком «Надписи — заготовками» (131, «Сохранить альбом как
// макет»): asDrafts кладёт в drafts все надписи, а не только помеченные Draft.
func SpreadTplOfAs(p *model.Project, spreadID, scope string, places, texts, asDrafts bool) (model.SpreadTpl, error) {
	return spreadTplOf(p, spreadID, scope, tplSnap{places: places, texts: texts, asDrafts: asDrafts})
}

// tplSnap — что снимает spreadTplOf: места, надписи (все заготовками — asDrafts), какие свои стикеры
// пропустить (skip), и frozen — «Авто» уже закреплена в p (копия вызывающего): места берутся как
// есть, без копии проекта на каждый разворот.
type tplSnap struct {
	places, texts, asDrafts bool
	skip                    func(lib string) bool
	frozen                  bool
}

func spreadTplOf(p *model.Project, spreadID, scope string, o tplSnap) (model.SpreadTpl, error) {
	if spreadID == model.CoverID {
		return model.SpreadTpl{}, model.ErrScope
	}
	_, s := p.Find(spreadID)
	if s == nil {
		return model.SpreadTpl{}, ErrNothing
	}
	a, ok := pageArea(p, s, scope)
	if !ok || !model.ValidScope(scope) {
		return model.SpreadTpl{}, model.ErrScope
	}
	t := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: a.W / a.H, W: a.W, H: a.H}
	_, two := p.AreaOf(s, model.ScopeLeft)
	switch {
	case scope == model.ScopeSpread && two:
		t.Pages = 2
		t.Page = []model.TplBg{bgOf(p, s.BgL, s.PatL, s.BgPhotoL), bgOf(p, s.BgR, s.PatR, s.BgPhotoR)}
		if w := bgOf(p, "", s.Pat, s.BgPhoto); w.Pat != nil || w.Lib != "" {
			t.Whole = &w
		}
	case scope == model.ScopeSpread: // лист 031: цвет, узор и фото — всего листа
		t.Page = []model.TplBg{bgOf(p, s.BgL, s.Pat, s.BgPhoto)}
	case scope == model.ScopeLeft:
		t.Page = []model.TplBg{bgOf(p, s.BgL, pageOrSheet(s.PatL, s.Pat), s.BgPhotoL)}
	default:
		t.Page = []model.TplBg{bgOf(p, s.BgR, pageOrSheet(s.PatR, s.Pat), s.BgPhotoR)}
	}
	if o.places {
		t.Places = placesOf(p, s, scope, o.frozen)
		t.Borders = bordersOf(p, t.Places)
		for i := range t.Places {
			t.Places[i].Border = nil // в памяти шаблона рамочек у коробок нет: они в Borders
		}
	}
	tplLayers(p, s, scope, a, &t, o)
	c, ok := model.ClampSpreadTpl(t)
	if !ok {
		return model.SpreadTpl{}, ErrNothing
	}
	return c, nil
}

// tplLayers дописывает в t стикеры (свои — только из библиотеки) и надписи области scope листа s,
// якорями от краёв области a; общая часть снимка разворота и сторонки обложки макета.
func tplLayers(p *model.Project, s *model.Spread, scope string, a layout.Rect, t *model.SpreadTpl, o tplSnap) {
	sd := designOf(p, s, scope, a.X)
	under := underPlaces(p, s, scope)
	for _, st := range sd.stickers {
		if it, ok := itemOf(p, st, a, under[st.ID]); ok && (it.Lib == "" || o.skip == nil || !o.skip(it.Lib)) {
			t.Stickers = append(t.Stickers, it)
		}
	}
	if !o.texts {
		return
	}
	for _, tx := range withoutHead(s, sd.texts) {
		if tx.Draft || o.asDrafts {
			t.Drafts = append(t.Drafts, textOf(&tx, a))
		} else {
			t.Texts = append(t.Texts, textOf(&tx, a))
		}
	}
}

// placesOf — места области в долях, с рамочками (SpreadTplOfAs переносит их в Borders и
// обнуляет Box.Border: в памяти шаблона они не живут); у «Авто» — закреплённые на копии проекта.
func placesOf(p *model.Project, s *model.Spread, scope string, frozen bool) []model.Box {
	if len(s.Grid) > 0 || frozen {
		return AreaFrac(p, s, scope)
	}
	c := p.Clone()
	_, cs := c.Find(s.ID)
	c.Freeze(cs)
	return AreaFrac(c, cs, scope)
}

// bgOf — фон страницы шаблона: цвет, узор и фото фона, если оно своё из библиотеки.
func bgOf(p *model.Project, color string, pat *model.Pattern, b *model.Backdrop) model.TplBg {
	g := model.TplBg{Color: color, Pat: pat.Clone()}
	if b != nil {
		if key := p.Photos[b.Photo].Lib; key != "" {
			g.Lib, g.Fade = key, b.Fade
		}
	}
	return g
}

// underPlaces — стикеры листа, лежащие в стеке под всеми местами области (фоновый орнамент):
// в шаблоне Under, при постановке — снова под местами.
func underPlaces(p *model.Project, s *model.Spread, scope string) map[string]bool {
	kind := s.Kinds()
	out := map[string]bool{}
	for _, id := range s.Stack() {
		switch kind[id] {
		case model.LayerPlace:
			if _, pl := s.FindPlace(id); InArea(p, s, pl.X, pl.W, scope) {
				return out
			}
		case model.LayerSticker:
			out[id] = true
		}
	}
	return map[string]bool{} // мест в области нет — «под местами» не бывает
}

// anchorOf — якорь центра (cx, cy) в области a: треть области по каждой оси — край или центр,
// сдвиг от него в долях S (у растяжимого — ширины и высоты области, anchorPoint).
func anchorOf(a layout.Rect, cx, cy float64, stretch bool) model.Anchor {
	sx, sy := min(a.W, a.H), min(a.W, a.H)
	if stretch {
		sx, sy = a.W, a.H
	}
	third := func(c, lo, n float64) (int8, float64) {
		switch f := (c - lo) / n; {
		case f < 1.0/3:
			return -1, lo
		case f > 2.0/3:
			return 1, lo + n
		}
		return 0, lo + n/2
	}
	ax, rx := third(cx, a.X, a.W)
	ay, ry := third(cy, a.Y, a.H)
	return model.Anchor{AX: ax, AY: ay, DX: (cx - rx) / sx, DY: (cy - ry) / sy}
}

// itemOf — стикер в шаблон: рисунок или своё с ключом библиотеки (иначе false).
func itemOf(p *model.Project, st *model.Sticker, a layout.Rect, under bool) (model.TplItem, bool) {
	it := model.TplItem{Rot: st.Rot, Fade: st.Fade, FlipH: st.FlipH, FlipV: st.FlipV, Under: under}
	if st.Art != nil {
		it.Art = &model.Art{Kind: st.Art.Kind, Inks: slices.Clone(st.Art.Inks), Seed: st.Art.Seed}
	} else if it.Lib = p.Photos[st.Photo].Lib; it.Lib == "" {
		return it, false
	}
	s, stretch := min(a.W, a.H), model.StretchArt(it.Art)
	it.Anchor = anchorOf(a, st.X+st.W/2, st.Y+st.H/2, stretch)
	it.W, it.H = st.W/s, st.H/s
	if stretch {
		it.W, it.H = st.W/a.W, st.H/a.H
	}
	return it, true
}

// textOf — надпись в шаблон: центр рамки якорем, ширина в долях S.
func textOf(t *model.TextBlock, a layout.Rect) model.TplText {
	cx, cy := model.BoxOf(t).Center()
	return model.TplText{Anchor: anchorOf(a, cx, cy, false), W: t.W / min(a.W, a.H), Rot: t.Rot, Auto: t.Auto, Text: t.Text, Style: t.Style}
}
