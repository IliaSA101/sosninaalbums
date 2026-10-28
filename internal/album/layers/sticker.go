package layers

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Стикеры (спека 048 Ф2, design §1, §3): добавление, замена, рамка, зеркало, прозрачность,
// цвета. Слой стека между местами и текстами (model/order.go); пропорцию своего рисунка держит
// FitStickers в конвейере правки, зажим — ClampSticker.

// AddSticker — тело Editor.AddSticker: стикер id из src по центру страницы page листа (0 —
// левая, 1 — правая; у обложки — сторонки) шириной ¼ страницы, не выше ½. Неверный рисунок —
// ErrSticker, ключ своего — ErrLibKey, больше MaxStickers — ErrStickers; неизвестный лист —
// холостая.
func AddSticker(p *model.Project, spreadID string, page int, id string, src model.StickerSrc) error {
	st, err := fromSrc(p, src)
	if err != nil {
		return err
	}
	s := p.Sheet(spreadID)
	if s == nil {
		return nil
	}
	if len(s.Stickers) >= model.MaxStickers {
		return model.ErrStickers
	}
	pg := p.Geo(s).Page(page)
	a := p.StartAspect(st)
	w := pg.W / 4
	if w/a > pg.H/2 {
		w = pg.H / 2 * a
	}
	st.ID, st.W, st.H = id, w, w/a
	st.X, st.Y = pg.X+(pg.W-w)/2, pg.Y+(pg.H-st.H)/2
	if c := p.ClampSticker(s, st); c != nil {
		s.Stickers = append(s.Stickers, c)
	}
	return nil
}

// ReplaceSticker — тело Editor.ReplaceSticker («Заменить…»): другой рисунок или своё фото;
// центр, ширина, поворот, зеркало и прозрачность стоят, высота — по новой пропорции
// (FitStickers). Растяжимая фигура рамку не меняет. Тот же источник — холостая.
func ReplaceSticker(p *model.Project, spreadID, id string, src model.StickerSrc) error {
	n, err := fromSrc(p, src)
	if err != nil {
		return err
	}
	s, i := p.FindSticker(id)
	if s == nil || s != p.Sheet(spreadID) {
		return nil
	}
	st := s.Stickers[i].Clone()
	st.Photo, st.Art = n.Photo, n.Art
	if a, ok := p.StickerAspect(st); ok {
		cy := st.Y + st.H/2
		st.H = st.W / a
		st.Y = cy - st.H/2
	}
	if c := p.ClampSticker(s, st); c != nil {
		s.Stickers[i] = c
	}
	return nil
}

// fromSrc — стикер без рамки из src: рисунок зажат ClampArt (ключ узора или не по грамматике —
// ErrSticker), своё фото — в Photos (Remember), ключ проверен, как у фона.
func fromSrc(p *model.Project, src model.StickerSrc) (*model.Sticker, error) {
	if src.Art != nil {
		a := model.ClampArt(src.Art)
		if a == nil {
			return nil, model.ErrSticker
		}
		return &model.Sticker{Art: a}, nil
	}
	ph := src.Photo
	if ph.ID == "" || ph.Lib != "" && (!model.ValidLibKey(ph.Lib) || ph.ID != model.LibPhotoID(ph.Lib)) {
		return nil, model.ErrLibKey
	}
	if !p.OnBackdrop(ph.ID) { // пропорция у копии та же: стикеру запись фона годится
		model.Remember(p, []model.PhotoInfo{ph})
	}
	return &model.Sticker{Photo: ph.ID}, nil
}

// SetStickerBox — тело Editor.SetStickerBox: перенос, размер, поворот (мм листа, угол рамки до
// поворота, поворот вокруг центра); зажим — ClampSticker, пропорция — FitStickers.
func SetStickerBox(p *model.Project, spreadID, id string, x, y, w, h, rot float64) {
	editSticker(p, spreadID, id, func(st *model.Sticker) {
		st.X, st.Y, st.W, st.H, st.Rot = x, y, w, h, rot
	})
}

// FlipSticker — тело Editor.FlipSticker: зеркало по оси "h" | "v" (ось проверена).
func FlipSticker(p *model.Project, spreadID, id, axis string) {
	editSticker(p, spreadID, id, func(st *model.Sticker) {
		if axis == "h" {
			st.FlipH = !st.FlipH
		} else {
			st.FlipV = !st.FlipV
		}
	})
}

// SetStickerFade — тело Editor.SetStickerFade: прозрачность 0..1, шаг 0.01.
func SetStickerFade(p *model.Project, spreadID, id string, fade float64) {
	editSticker(p, spreadID, id, func(st *model.Sticker) { st.Fade = fade })
}

// SetStickerInks — тело Editor.SetStickerInks: цвета процедурного стикера (ключ стоит); зажим —
// ClampArt. У своего — холостая.
func SetStickerInks(p *model.Project, spreadID, id string, inks []string) {
	editSticker(p, spreadID, id, func(st *model.Sticker) {
		if st.Art != nil {
			st.Art.Inks = inks
		}
	})
}

// editSticker — правка копии стикера id листа spreadID и зажим; стикер другого листа или
// неизвестный — холостая.
func editSticker(p *model.Project, spreadID, id string, fn func(st *model.Sticker)) {
	s, i := p.FindSticker(id)
	if s == nil || s != p.Sheet(spreadID) {
		return
	}
	c := s.Stickers[i].Clone()
	fn(c)
	if n := p.ClampSticker(s, c); n != nil {
		s.Stickers[i] = n
	}
}
