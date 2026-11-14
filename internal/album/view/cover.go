package view

import (
	"cmp"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Вид обложки (032 §6): лист собирается тем же spreadView, что и развороты (подстановки,
// заглушки, Unfilled/Unnamed — тот же fields), плюс числа, направляющие и зоны броска.

// Guide — что Editor знает о файле-разметке (читается один раз на путь, не под каждый вид).
type Guide struct {
	Path string
	Info library.MarkupInfo
	OK   bool
}

// markupOf — вид разметки path по тому, что прочитал Editor (st — по тому же пути, иначе «не прочитан»).
func markupOf(path string, st Guide, w, h float64, where string) *model.GuideView {
	ok := st.OK && st.Path == path
	return model.MarkupView(path, st.Info, ok, w, h, where)
}

// coverView — вид обложки p или nil; лист считается в итоги вида v (LowDPI, пустые места, фоны).
func coverView(f *fields, st State, v *model.View) *model.CoverView {
	p := f.p
	c := p.Cover
	if c == nil || c.Sheet == nil {
		return nil
	}
	cg, g := p.CoverGeo(), p.Geo(c.Sheet)
	cv := &model.CoverView{
		Sheet: spreadView(f, c.Sheet, Laid{}), Spec: c.Spec, Preset: c.Spec.Preset,
		Spine: cg.Spine, N: cg.N, Over: cg.Over, Layout: model.CoverLayoutName(c.Layout), SpineBg: c.SpineBg,
		SpineText: p.SpineTextID(),
		Zones:     p.CoverZones(), Sides: p.CoverSides(),
		Drop: dropZones(p, c), Scopes: coverScopes(p, c), TextMaxW: p.TextMaxW(c.Sheet),
		PxW: px(g.W, p.Settings.DPI), PxH: px(g.H, p.Settings.DPI), Fix: model.CoverFixText(p), Missing: model.CoverMissing(c.Spec.Preset),
	}
	for i := range cv.Sides {
		sd := &cv.Sides[i]
		cv.Safe = append(cv.Safe, sd.Box)
		cs := st.CoverSides[0]
		if sd.Scope == model.ScopeRight {
			cs = st.CoverSides[1]
		}
		sd.Places, sd.Known, sd.Alone = cs.Places, cs.Known, cs.Alone
	}
	cv.Markup = markupOf(c.Guide, st.CoverGuide, g.W, g.H, model.MarkupCover)
	sv := &cv.Sheet
	sv.LowDPI = lowDPI(sv, v.MinDPI)
	v.LowDPI += sv.LowDPI
	for _, b := range sv.Backdrops {
		v.BackdropUsed = append(v.BackdropUsed, b.Photo)
	}
	for _, pl := range sv.Places {
		if pl.Photo == "" {
			v.EmptyPlaces++
		}
	}
	return cv
}

// px — сторона файла 00.jpg при dpi.
func px(mm float64, dpi int) int { return int(math.Round(mm / 25.4 * float64(dpi))) }

// coverScopes — области фото фона раскладки с подписями (061): одна точка для карточки фона,
// меню «На фон» и зон броска (dropZones). Корешок уже model.MinSpine — без своей области
// (decisions 31: фото на нём не видно, SetBackdrop отказывает).
func coverScopes(p *model.Project, c *model.Cover) []model.CoverScope {
	switch c.Layout {
	case model.CoverWrap:
		return []model.CoverScope{{Scope: model.ScopeSpread, Label: "Вся обложка"}}
	case model.CoverBoth:
		if !p.SpineOK(c.Sheet) {
			return []model.CoverScope{{Scope: model.ScopeLeft, Label: "Задняя"}, {Scope: model.ScopeRight, Label: "Лицевая"}}
		}
		return []model.CoverScope{{Scope: model.ScopeLeft, Label: "Задняя"}, {Scope: model.ScopeSpine, Label: "Корешок"},
			{Scope: model.ScopeRight, Label: "Лицевая"}}
	}
	return []model.CoverScope{{Scope: model.ScopeRight, Label: "Лицевая"}}
}

// dropZones — пустые зоны броска фото фона по раскладке обложки: область фона (с заворотом
// и расставом), у которой фото ещё нет. В both зоны не перекрываются (061): задняя [0, X2],
// корешок [X2, X5] (если он есть в coverScopes), лицевая [X5, W] — расставы отданы корешку только
// в броске, области фона сторонок прежние.
func dropZones(p *model.Project, c *model.Cover) []model.CoverZone {
	s := c.Sheet
	kinds := map[string]string{model.ScopeSpread: "wrap", model.ScopeLeft: "back", model.ScopeRight: "front", model.ScopeSpine: "spine"}
	label := map[string]string{model.ScopeSpread: "Одно фото через корешок"}
	out := []model.CoverZone{}
	for _, sc := range coverScopes(p, c) {
		if *s.Slot(sc.Scope) != nil {
			continue
		}
		r := p.BdArea(s, sc.Scope)
		if c.Layout == model.CoverBoth {
			x := p.CoverGeo().X
			switch sc.Scope {
			case model.ScopeLeft:
				r.W = x[2] - r.X
			case model.ScopeRight:
				r.W, r.X = r.X+r.W-x[5], x[5]
			}
		}
		out = append(out, model.CoverZone{Kind: kinds[sc.Scope], Rect: r, Label: cmp.Or(label[sc.Scope], sc.Label)})
	}
	return out
}
