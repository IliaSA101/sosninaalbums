package grid

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Снимок «Копировать» и «Вырезать» (099, design §3). Снимок — копии: буфер переживает правку и
// удаление оригинала (Ctrl+C, Del, Ctrl+V), вставка его не меняет.

// anchorEps — допуск «группа внутри страницы», мм.
const anchorEps = 0.5

// Take — снимок выделенного sel на листе sheetID (обложка тоже): места «Ручной» (по ссылке места
// или фото), фото «Авто», надписи и стикеры этого листа; чужое и неизвестное отбрасывается.
// false — нечего копировать: лист не найден или ни одна ссылка не нашлась.
func Take(p *model.Project, sheetID string, sel model.ClipSel) (model.Clip, bool) {
	s := p.Sheet(sheetID)
	if s == nil {
		return model.Clip{}, false
	}
	c := model.Clip{Src: s.ID, Refs: map[string]model.PhotoRef{}}
	taken := map[string]bool{}
	ref := func(id string) {
		if r, ok := p.Photos[id]; ok && id != "" {
			c.Refs[id] = r.Copy()
		}
	}
	takePlaces(p, s, sel.Refs, &c, taken, ref)
	for _, id := range sel.Texts {
		if i := slices.IndexFunc(s.Texts, func(t *model.TextBlock) bool { return t.ID == id }); i >= 0 && !taken[id] {
			taken[id] = true
			c.Texts = append(c.Texts, *s.Texts[i])
		}
	}
	for _, id := range sel.Stickers {
		if i := slices.IndexFunc(s.Stickers, func(st *model.Sticker) bool { return st.ID == id }); i >= 0 && !taken[id] {
			taken[id] = true
			c.Stickers = append(c.Stickers, *s.Stickers[i].Clone())
			ref(s.Stickers[i].Photo)
		}
	}
	if c.Empty() {
		return model.Clip{}, false
	}
	for _, id := range s.Stack() {
		if taken[id] {
			c.Seq = append(c.Seq, id)
		}
	}
	c.Box = clipBox(&c)
	c.Anchor = clipAnchor(p.Geo(s), c.Box)
	return c, true
}

// takePlaces — места «Ручной» (копия с рамочкой; повторы ссылок схлопываются) или фото «Авто»
// (ячейка раскладки) по ссылкам refs; записи фото и рамочек — через ref. Фото «Авто» идут в Seq
// первыми: в стеке слоёв их нет.
func takePlaces(p *model.Project, s *model.Spread, refs []string, c *model.Clip, taken map[string]bool, ref func(string)) {
	if len(s.Grid) > 0 {
		for _, r := range refs {
			pl := s.PlaceRef(r)
			if pl == nil || taken[pl.ID] {
				continue
			}
			taken[pl.ID] = true
			cp := *pl
			cp.Border = pl.Border.Clone()
			c.Places = append(c.Places, cp)
			ref(pl.Photo)
			if pl.Border != nil {
				ref(pl.Border.Photo)
			}
		}
		return
	}
	cur, _, ok := p.Layout(s)
	if !ok || len(cur.Rects) != len(s.Photos) {
		return
	}
	for _, r := range refs {
		i := slices.Index(s.Photos, r)
		if i < 0 || taken[r] {
			continue
		}
		taken[r] = true
		c.Auto = append(c.Auto, model.ClipPhoto{ID: r, Box: cur.Rects[i]})
		c.Seq = append(c.Seq, r)
		ref(r)
	}
}

// clipBox — описанный прямоугольник всех элементов снимка, мм.
func clipBox(c *model.Clip) layout.Rect {
	var rs []layout.Rect
	for i := range c.Places {
		rs = append(rs, model.PlaceAABB(&c.Places[i]))
	}
	for _, a := range c.Auto {
		rs = append(rs, a.Box)
	}
	for i := range c.Texts {
		rs = append(rs, model.BoxOf(&c.Texts[i]).AABB())
	}
	for i := range c.Stickers {
		rs = append(rs, c.Stickers[i].Rect().AABB())
	}
	return union(rs)
}

// union — объединение прямоугольников; пусто — нулевой.
func union(rs []layout.Rect) layout.Rect {
	if len(rs) == 0 {
		return layout.Rect{}
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range rs {
		x0, y0 = min(x0, r.X), min(y0, r.Y)
		x1, y1 = max(x1, r.X+r.W), max(y1, r.Y+r.H)
	}
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// clipAnchor — якорь группы b на листе g: страница, внутри которой лежит группа (допуск anchorEps),
// и центр в её долях; через корешок, на весь лист и у листа с одной страницей — доли листа (Page −1).
func clipAnchor(g model.Geo, b layout.Rect) model.ClipAnchor {
	cx, cy := b.X+b.W/2, b.Y+b.H/2
	if len(g.Pages) >= 2 {
		for k, pg := range g.Pages {
			if b.X >= pg.X-anchorEps && b.Y >= pg.Y-anchorEps && b.X+b.W <= pg.X+pg.W+anchorEps && b.Y+b.H <= pg.Y+pg.H+anchorEps {
				return model.ClipAnchor{Page: k, Fx: (cx - pg.X) / pg.W, Fy: (cy - pg.Y) / pg.H}
			}
		}
	}
	return model.ClipAnchor{Page: -1, Fx: cx / g.W, Fy: cy / g.H}
}

// Cut — Take и удаление взятого в той же правке: надписи и стикеры снимаются, фото «Авто» уходят
// в ленту, места «Ручной» — вместе с фото (в отличие от DeleteSelection, где место остаётся
// пустым). Опустевший «Авто» убирает Normalize, как при Del. false — нечего вырезать, правки нет.
func Cut(p *model.Project, sheetID string, sel model.ClipSel) (model.Clip, bool) {
	c, ok := Take(p, sheetID, sel)
	if !ok {
		return c, false
	}
	for _, t := range c.Texts {
		p.DeleteText(t.ID)
	}
	if s := p.Sheet(sheetID); len(s.Texts) == 0 {
		s.Texts = nil // как у прочитанного файла: круг Save→Load не меняет лист
	}
	for _, st := range c.Stickers {
		p.DeleteSticker(st.ID)
	}
	auto := make([]string, len(c.Auto))
	for i, a := range c.Auto {
		auto[i] = a.ID
	}
	p.Detach(auto)
	places := make([]string, len(c.Places))
	for i, pl := range c.Places {
		places[i] = pl.ID
	}
	p.DeletePlaces(sheetID, places)
	return c, true
}
