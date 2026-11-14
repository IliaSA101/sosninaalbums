package grid

import (
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Вставка из буфера и «Дублировать» (099, design §4). Чистая функция над проектом: ядро зовёт её
// внутри try, ошибка (пределы, копия фото) — откат без шага, «всё или ничего» (099-15).

const (
	pasteStep  = 5.0 // сдвиг лесенки при повторе на том же листе, мм
	pasteSteps = 20  // ступеней лесенки не больше
	sameEps    = 0.5 // «такой же элемент на том же месте»: все стороны ближе, мм
)

// CopyFn — новая запись-копия фото по записи из снимка (098 model.AddCopy): ID копии.
type CopyFn func(p *model.Project, ref model.PhotoRef) (string, error)

// Twins — фото снимка, уже стоящие в местах альбома (099-6, 098): как узнать и как поставить.
// Ядро даёт свои (ops_clip.go); связка с 098 (PlacedFile, AddCopy, перенос семьи) меняет только их.
type Twins struct {
	Placed func(p *model.Project, id string) bool // фото стоит в месте альбома
	Copy   CopyFn                                 // «Поставить копию»
	Move   func(p *model.Project, id string)      // «Перенести сюда»: убрать фото с прежних мест
}

// Paste ставит снимок c на лист r.Sheet (обложка тоже) одной правкой: положение — по якорю
// (страница под курсором r.Hover, иначе страница источника; через корешок — доли листа), точка
// щелчка r.At — центр вставленного; на листе-источнике — лесенка 5 мм; группа — внутрь листа без
// смены размера. Фото, уже стоящие в местах альбома, без r.Repeat возвращаются в out.Repeat без
// правки (вопрос 098). Новые ID — сверху стека в порядке источника. Пустой снимок, неизвестный
// лист — холостая; сверх пределов листа — ошибка.
func Paste(p *model.Project, c *model.Clip, r model.PasteReq, tw Twins) (out model.PasteOut, err error) {
	t := p.Sheet(r.Sheet)
	if t == nil || c.Empty() {
		return out, nil
	}
	if r.Repeat == "" {
		for _, id := range c.PhotoIDs() {
			if tw.Placed(p, id) {
				out.Repeat = append(out.Repeat, id)
			}
		}
		if len(out.Repeat) > 0 {
			return out, nil // как ask у броска 042: проект не тронут, шага нет
		}
	}
	dx, dy := pasteShift(p, t, c, r)
	auto := len(t.Grid) == 0 && t.ID != model.CoverID
	if !auto || slices.ContainsFunc(c.Places, func(pl model.Place) bool { return pl.Photo == "" }) {
		p.Freeze(t) // как «+ Место» (grid.AddPlace); у ручной и обложки — холостой
		out.Froze, auto = auto, false
	}
	had := p.PlaceIDs() // до вставки: табличка едет только с вырезанным местом
	keepRefs(p, c)
	nid, err := pastePhotos(p, c, r.Repeat, tw)
	if err != nil {
		return out, err
	}
	fresh := map[string]string{} // ID в снимке → новый ID
	if auto {
		err = pasteAuto(t, c, nid, &out)
	} else {
		err = pastePlaces(p, t, c, nid, had, dx, dy, fresh, &out)
	}
	if err == nil {
		err = pasteDecor(p, t, c, dx, dy, fresh, &out)
	}
	if err != nil {
		return model.PasteOut{}, err
	}
	pasteOrder(t, c.Seq, fresh)
	return out, nil
}

// Duplicate — «Дублировать» (Ctrl+D): снимок выделенного и вставка на тот же лист одной правкой,
// со сдвигом лесенки; фото — копии сразу, без вопроса (0.23-q2). Буфер App не трогается.
func Duplicate(p *model.Project, sheetID string, sel model.ClipSel, tw Twins) (model.PasteOut, error) {
	c, ok := Take(p, sheetID, sel)
	if !ok {
		return model.PasteOut{}, nil
	}
	return Paste(p, &c, model.PasteReq{Sheet: sheetID, Repeat: model.RepeatCopy}, tw)
}

// pasteShift — сдвиг снимка c на лист t, мм (design §4, шаги 3–5): центр по r.At или якорю,
// лесенка на листе-источнике без r.At, затем группа внутрь листа.
func pasteShift(p *model.Project, t *model.Spread, c *model.Clip, r model.PasteReq) (dx, dy float64) {
	g := p.Geo(t)
	var cx, cy float64
	switch a := c.Anchor; {
	case r.At != nil:
		cx, cy = r.At.X, r.At.Y
	case a.Page >= 0:
		j := a.Page
		if r.Hover != nil {
			j = g.PageOf(r.Hover.X)
		}
		pg := g.Page(j)
		cx, cy = pg.X+a.Fx*pg.W, pg.Y+a.Fy*pg.H
	default:
		cx, cy = a.Fx*g.W, a.Fy*g.H
	}
	b := c.Box
	dx, dy = cx-(b.X+b.W/2), cy-(b.Y+b.H/2)
	if r.At == nil && t.ID == c.Src {
		for k := 0; k < pasteSteps && clashes(t, c, dx, dy); k++ {
			dx, dy = dx+pasteStep, dy+pasteStep
		}
	}
	return geom.Round2(into(b.X, b.W, dx, g.W)), geom.Round2(into(b.Y, b.H, dy, g.H))
}

// into — сдвиг d отрезка [x, x+w] по оси листа длины size: отрезок внутрь [0, size]; длиннее
// листа — его центр внутрь листа.
func into(x, w, d, size float64) float64 {
	if w > size {
		c := x + w/2
		return geom.ClampF(c+d, 0, size) - c
	}
	return geom.ClampF(x+d, 0, size-w) - x
}

// clashes — какой-то элемент снимка, сдвинутый на (dx, dy), лежит на элементе листа t того же
// вида: все стороны описанных прямоугольников ближе sameEps.
func clashes(t *model.Spread, c *model.Clip, dx, dy float64) bool {
	hit := func(a layout.Rect, bs []layout.Rect) bool {
		a.X, a.Y = a.X+dx, a.Y+dy
		return slices.ContainsFunc(bs, func(b layout.Rect) bool {
			return near(a.X, b.X) && near(a.Y, b.Y) && near(a.X+a.W, b.X+b.W) && near(a.Y+a.H, b.Y+b.H)
		})
	}
	var places, texts, stickers []layout.Rect
	for _, pl := range t.Grid {
		places = append(places, model.PlaceAABB(pl))
	}
	for _, tb := range t.Texts {
		texts = append(texts, model.BoxOf(tb).AABB())
	}
	for _, st := range t.Stickers {
		stickers = append(stickers, st.Rect().AABB())
	}
	for i := range c.Places {
		if hit(model.PlaceAABB(&c.Places[i]), places) {
			return true
		}
	}
	for i := range c.Texts {
		if hit(model.BoxOf(&c.Texts[i]).AABB(), texts) {
			return true
		}
	}
	for i := range c.Stickers {
		if hit(c.Stickers[i].Rect().AABB(), stickers) {
			return true
		}
	}
	return false
}

// near — числа ближе sameEps.
func near(a, b float64) bool { return a-b < sameEps && b-a < sameEps }

// keepRefs — записи картинок стикеров и рамочек, которых в проекте уже нет (вырезаны и сняты
// PruneRefs), возвращаются из снимка с тем же ID (у своих — тот же ключ Lib).
func keepRefs(p *model.Project, c *model.Clip) {
	back := func(id string) {
		if _, ok := p.Photos[id]; !ok && id != "" {
			if r, ok := c.Refs[id]; ok {
				p.Photos[id] = r.Copy()
			}
		}
	}
	for _, st := range c.Stickers {
		back(st.Photo)
	}
	for _, pl := range c.Places {
		if pl.Border != nil {
			back(pl.Border.Photo)
		}
	}
}

// pastePhotos — какое фото встаёт вместо каждого фото снимка (ID снимка → ID в проекте). Файл не
// стоит ни одной записью: тот же ID с записью из снимка (кадр, тон, подпись на момент копирования).
// Стоит: RepeatCopy — новая запись-копия по записи из снимка (кадр «как у скопированного»);
// RepeatMove — все записи файла уходят с мест, встаёт запись с ID снимка: прежняя, если она есть
// (фото переезжает таким, каким его видно), иначе из снимка. Записи нет ни там, ни там — фото не
// встаёт (место пустое).
func pastePhotos(p *model.Project, c *model.Clip, repeat string, tw Twins) (map[string]string, error) {
	nid := map[string]string{}
	for _, id := range c.PhotoIDs() {
		ref, inSnap := c.Refs[id]
		switch placed := tw.Placed(p, id); {
		case placed && repeat == model.RepeatCopy && inSnap:
			n, err := tw.Copy(p, ref.Copy())
			if err != nil {
				return nil, err
			}
			nid[id] = n
		case placed && repeat == model.RepeatCopy:
		case placed:
			tw.Move(p, id)
			if _, here := p.Photos[id]; !here && inSnap {
				p.Photos[id] = ref.Copy()
				p.FitFile(id)
			}
			if _, here := p.Photos[id]; here {
				nid[id] = id
			}
		case inSnap:
			p.Photos[id] = ref.Copy()
			p.FitFile(id)
			nid[id] = id
		}
	}
	return nid, nil
}

// pasteAuto — фото снимка на «Авто» t: в конец раскладки, как бросок 042; положение не берётся.
func pasteAuto(t *model.Spread, c *model.Clip, nid map[string]string, out *model.PasteOut) error {
	for _, id := range c.PhotoIDs() {
		if n := nid[id]; n != "" {
			out.Refs = append(out.Refs, n)
		}
	}
	t.Insert(out.Refs, -1)
	if len(t.Photos) > model.MaxPhotos {
		return model.ErrTooMany()
	}
	return nil
}

// pastePlaces — места снимка и фото «Авто» (ячейкой) на ручной лист t со сдвигом: новые ID в конец
// Grid. Табличка остаётся только у перенесённого места, которого в проекте уже нет (had — ID мест
// до вставки), иначе обнуляется, как у ClonePlace; роль — "": на виньетке это ученик.
func pastePlaces(p *model.Project, t *model.Spread, c *model.Clip, nid map[string]string, had map[string]bool, dx, dy float64, fresh map[string]string, out *model.PasteOut) error {
	taken := p.PlaceIDs()
	put := func(src model.Place, photo string) {
		pl := src
		pl.ID, pl.Photo, pl.Role, pl.Border = model.NewPlaceID(taken), "", "", src.Border.Clone()
		pl.X, pl.Y = geom.Round2(pl.X+dx), geom.Round2(pl.Y+dy)
		if had[src.ID] || src.Photo != "" && nid[src.Photo] != src.Photo {
			pl.Person = model.Person{}
		}
		p.ClampPlace(t, &pl)
		t.Grid = append(t.Grid, &pl)
		if n := nid[photo]; n != "" {
			p.Put(t, &pl, n)
		}
		fresh[src.ID] = pl.ID
		out.Refs = append(out.Refs, pl.ID)
	}
	for _, src := range c.Places {
		put(src, src.Photo)
	}
	for _, a := range c.Auto {
		put(model.Place{ID: a.ID, X: a.Box.X, Y: a.Box.Y, W: a.Box.W, H: a.Box.H, Photo: a.ID}, a.ID)
	}
	if len(t.Grid) > model.MaxPlaces {
		return model.ErrTooManyPlaces()
	}
	return nil
}

// pasteDecor — надписи и стикеры снимка на лист t со сдвигом и новыми ID; стикер — ClampSticker
// (картинки нет — не встаёт). Сверх MaxTexts и MaxStickers листа — ошибка, как у AddText и AddSticker.
func pasteDecor(p *model.Project, t *model.Spread, c *model.Clip, dx, dy float64, fresh map[string]string, out *model.PasteOut) error {
	for _, src := range c.Texts {
		tb := src
		tb.ID, tb.X, tb.Y = model.NewID(), geom.Round2(tb.X+dx), geom.Round2(tb.Y+dy)
		t.Texts = append(t.Texts, &tb)
		fresh[src.ID], out.Texts = tb.ID, append(out.Texts, tb.ID)
	}
	if len(t.Texts) > model.MaxTexts {
		return fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
	}
	for i := range c.Stickers {
		st := c.Stickers[i].Clone()
		st.ID, st.X, st.Y = model.NewID(), st.X+dx, st.Y+dy
		if st = p.ClampSticker(t, st); st != nil {
			t.Stickers = append(t.Stickers, st)
			fresh[c.Stickers[i].ID], out.Stickers = st.ID, append(out.Stickers, st.ID)
		}
	}
	if len(t.Stickers) > model.MaxStickers {
		return model.ErrStickers
	}
	return nil
}

// pasteOrder — новые элементы одним куском на вершину стека листа t в порядке источника seq
// (как putOrder у копии сетки, только позиция — вершина): вставленное видно поверх всего.
func pasteOrder(t *model.Spread, seq []string, fresh map[string]string) {
	var chunk []string
	for _, id := range seq {
		if n, ok := fresh[id]; ok {
			chunk = append(chunk, n)
		}
	}
	rest := slices.DeleteFunc(t.Stack(), func(id string) bool { return slices.Contains(chunk, id) })
	t.SetStack(append(rest, chunk...))
}
