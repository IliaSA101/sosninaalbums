package grid

import (
	"maps"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Обложка по макету альбома (131 design §5): фон на весь лист (ApplyLookCoverBg), затем задняя и
// лицевая сторонки — шаблон страницы на безопасную зону сторонки (coverSides, 127). Места шага
// встают по LookBoxes внутри зоны, фото обложки садятся по порядку, лишние уходят в ленту.
// Корешок не трогается. Обложки нет — не создаётся.

// CoverRes — итог ApplyLookCover.
type CoverRes struct {
	Changed  bool     // обложка изменилась (повтор — нет)
	Replaced bool     // изменился свой фон: он у обложки был
	Left     int      // фото, ушедшие в ленту
	Texts    int      // надписей сторонок заменено (GridPlan.Texts)
	Stickers int      // стикеров сторонок заменено (GridPlan.Stickers)
	Own      int      // своего заменено всего: места, надписи, стикеры сторонок
	Missing  []string // ключи библиотеки без файла на этом ПК
}

// coverSideTplZone — безопасная зона сторонки scope обложки s для шаблона страницы t; false — это
// не сторонка обложки (или шаблон не страницы, или зоны нет): tplParts берёт область листа.
func coverSideTplZone(p *model.Project, s *model.Spread, scope string, t model.SpreadTpl) (layout.Rect, bool) {
	if c, ok := coverSideOf(p, scope); ok && t.Pages == 1 && p.IsCover(s) {
		return c.box, true
	}
	return layout.Rect{}, false
}

// ApplyLookCover ставит оформление обложки макета c: фон (Bg и цвет, узор, фото сторон —
// CoverPageBg), затем сторонки (Back — задняя, Front — лицевая): места, рамочки, стикеры, надписи и
// заготовки. Сторонка без шаблона не трогается; фон листа ставит только эта функция, сторонка его
// не меняет. Обложки нет или c пуст — холостая правка. Повтор того же — холостая (Changed =
// false). Свои из библиотеки — из libs; ключа нет — слой пропускается, ключ в Missing. Отказ
// (стикеров или мест больше предела) — ошибка, вызывающий откатывает правку целиком (Editor.try).
func ApplyLookCover(p *model.Project, c *model.CoverTpl, libs map[string]model.PhotoInfo) (res CoverRes, err error) {
	if c == nil || p.Cover == nil || p.Cover.Sheet == nil {
		return res, nil
	}
	gone := map[string]bool{}
	bg := ApplyLookCoverBg(p, c, libs)
	res.Changed, res.Replaced = bg.Changed, bg.Replaced
	for _, k := range bg.Missing {
		gone[k] = true
	}
	for i, side := range []*model.SpreadTpl{c.Back, c.Front} {
		if side == nil {
			continue
		}
		scope := model.ScopeLeft
		if i == 1 {
			scope = model.ScopeRight
		}
		r, err := applyCoverSide(p, scope, *side, libs, gone)
		if err != nil {
			return CoverRes{}, err
		}
		res.Changed = res.Changed || r.changed
		res.Left += r.left
		res.Texts += r.texts
		res.Stickers += r.stickers
		res.Own += r.own
	}
	res.Missing = slices.Sorted(maps.Keys(gone))
	return res, nil
}

// sideRes — итог одной сторонки.
type sideRes struct {
	changed  bool // места или оформление изменились
	left     int  // фото в ленту
	texts    int  // надписей заменено
	stickers int  // стикеров заменено
	own      int  // своего заменено: места (если шаблон ставит места), надписи, стикеры
}

// applyCoverSide ставит шаблон страницы t на сторонку scope обложки: места по LookBoxes в зоне
// (рамочки шаблона — на места), стикеры и надписи; фон не трогается (его ставит ApplyLookCover).
// Зоны нет или шаблон негоден — ничего. gone пополняется ключами без файла.
func applyCoverSide(p *model.Project, scope string, t model.SpreadTpl, libs map[string]model.PhotoInfo, gone map[string]bool) (res sideRes, err error) {
	s := p.Cover.Sheet
	c, ok := coverSideOf(p, scope)
	t, valid := model.ClampSpreadTpl(t)
	if !ok || !valid || t.Pages != 1 {
		return res, nil
	}
	t.Page = []model.TplBg{{}}           // фон стороны уже поставлен ApplyLookCoverBg: ключи его картинок здесь не считаются
	parts, _ := tplParts(p, s, scope, t) // зона и Pages = 1 проверены выше: отказа области нет
	r := &tplRes{p: p, s: s, libs: libs, gone: gone}
	sd, seq := r.design(t, scope, parts)
	// Фон сторонки уже поставлен: сверка и запись берут текущий. Фото всего листа остаётся у сторонки
	// (putOpt.own): без него сверка видела бы его как лишнее и повтор не был бы холостым.
	cur := designOf(p, s, scope, sd.x0)
	sd.bgL, sd.patL, sd.photoL, sd.photo = cur.bgL, cur.patL, cur.photoL, cur.photo
	o := putOpt{decor: true, own: true, texts: len(t.Texts)+len(t.Drafts) > 0}
	var boxes []model.Box
	places := len(t.Places) > 0
	if places {
		r.borders(&t)
		boxes = c.boxes(model.LookBoxes(t, c.box.W/c.box.H))
	}
	moved := places && !c.sameAt(boxes) // места встают заново; те же остаются со своими ID
	if !moved && sameDecor(p, s, scope, sd, o) {
		return res, nil
	}
	res.changed = true
	if moved {
		res.own = len(c.places)
		if res.left, err = applyGrid(p, s, scope, boxes); err != nil {
			return sideRes{}, err
		}
	}
	old, keep := s.Kinds(), *s
	if res.texts, res.stickers, err = putDesign(p, s, scope, sd, o); err != nil {
		return sideRes{}, err
	}
	res.own += res.texts + res.stickers
	// Фон листа putDesign переписал бы (узор листа распался бы на страницы): возвращается как был.
	s.BgL, s.BgR, s.Pat, s.PatL, s.PatR = keep.BgL, keep.BgR, keep.Pat, keep.PatL, keep.PatR
	s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.BgPhotoS = keep.BgPhoto, keep.BgPhotoL, keep.BgPhotoR, keep.BgPhotoS
	decorOrder(s, old, seq)
	return res, nil
}

// sameAt — места сторонки уже такие, как boxes: геометрия в пределах idemTol (обложку двигает
// Reflow, шум округления не должен делать повтор правкой), рамочки те же, без «Вписать».
func (c *coverSide) sameAt(boxes []model.Box) bool {
	if !c.sameAs(boxes) {
		return false
	}
	for i, pl := range c.places {
		if pl.Fit != "" || !sameBorder(pl.Border, boxes[i].Border) {
			return false
		}
	}
	return true
}

// CoverExtras — на сторонках обложки есть своё, чего шаблон c не ставит: места там, где у шаблона
// мест нет, надписи там, где у него нет надписей и заготовок. Повтор c этого не заменит, а другой
// макет заменит: сверка «обложка от c» (lookCoverIs) без этого их не видит, как grid.LookUntouched
// у разворотов. Корешок не входит. Обложки нет — false.
func CoverExtras(p *model.Project, c *model.CoverTpl) bool {
	if c == nil || p.Cover == nil || p.Cover.Sheet == nil {
		return false
	}
	s := p.Cover.Sheet
	for i, side := range []*model.SpreadTpl{c.Back, c.Front} {
		scope := model.ScopeLeft
		if i == 1 {
			scope = model.ScopeRight
		}
		sd, ok := coverSideOf(p, scope)
		if side == nil || !ok {
			continue
		}
		if len(side.Places) == 0 && len(sd.places) > 0 {
			return true
		}
		if len(side.Texts)+len(side.Drafts) > 0 {
			continue
		}
		for _, tx := range s.Texts {
			if a := model.BoxOf(tx).AABB(); InArea(p, s, a.X, a.W, scope) {
				return true
			}
		}
	}
	return false
}
