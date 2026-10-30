package layers

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Стикеры (спека 048 Ф2, design §1, §3): добавление, замена, рамка, зеркало, прозрачность,
// цвета. Слой стека между местами и текстами (model/order.go); пропорцию своего рисунка держит
// FitStickers в конвейере правки, зажим — ClampSticker.

// AddSticker — тело Editor.AddSticker: стикер id из src по центру страницы page листа (0 —
// левая, 1 — правая; у обложки — сторонки) шириной ¼ страницы, не выше ½. Неверный рисунок —
// ErrSticker, ключ своего — ErrLibKey, больше MaxStickers — ErrStickers; неизвестный лист —
// холостая.
func AddSticker(p *model.Project, spreadID string, page int, id string, src model.StickerSrc) error {
	return AddStickerAt(p, spreadID, page, id, src, AnchorCenter, FracM)
}

// Якоря и доли ширины стикера агента (151, 0.34-151-9). Центр и ¼ — прежний щелчок по плитке.
const (
	AnchorCenter      = "center"
	AnchorTopLeft     = "top-left"
	AnchorTopRight    = "top-right"
	AnchorBottomLeft  = "bottom-left"
	AnchorBottomRight = "bottom-right"

	FracS = 1.0 / 6 // s
	FracM = 0.25    // m
	FracL = 1.0 / 3 // l
)

// ErrAnchor — неизвестное место или размер стикера.
var ErrAnchor = errors.New("стикер не поставлен: неизвестное место или размер")

// ErrNoSide — на этой сторонке обложки нет безопасной зоны (или она меньше мельчайшего стикера).
var ErrNoSide = errors.New("на этой сторонке обложки нет места")

// SizeFrac — доля ширины страницы по размеру s | m | l ("" — m).
func SizeFrac(size string) (float64, bool) {
	switch size {
	case "s":
		return FracS, true
	case "m", "":
		return FracM, true
	case "l":
		return FracL, true
	}
	return 0, false
}

// AddStickerAt — AddSticker с якорем и долей ширины страницы frac (не выше ½ высоты): центр или
// угол страницы, угол — на расстоянии max(Settings.Safe, Margin) от края (стикер остаётся на
// странице). AddSticker — тот же ход с центром и ¼. Неизвестный якорь или доля — ErrAnchor;
// неизвестный лист и стикер, не прошедший ClampSticker, — холостая (проверяет вызывающий).
func AddStickerAt(p *model.Project, spreadID string, page int, id string, src model.StickerSrc, anchor string, frac float64) error {
	return addStickerIn(p, spreadID, page, nil, id, src, anchor, frac)
}

// addStickerIn — тело AddStickerAt и пачки агента на обложку. Размер и центр — всегда от страницы
// листа (у обложки — сторонка с заворотом и расставом), как у щелчка человека: агентский center + m
// равен щелчку. zone nil — углы от страницы с отступом max(Safe, Margin) (разворот). zone — безопасная
// зона сторонки обложки (CoverSide.Box): углы от неё без отступа (Box уже безопасная), а рамка,
// вышедшая за зону, сдвигается внутрь; шире или выше зоны — уменьшается до неё (пропорция та же).
// Зона уже MinSticker после уменьшения — ErrNoSide: ClampSticker вернул бы рамку за зону.
func addStickerIn(p *model.Project, spreadID string, page int, zone *layout.Rect, id string, src model.StickerSrc, anchor string, frac float64) error {
	if !okFrac(frac) || !slices.Contains([]string{AnchorCenter, AnchorTopLeft, AnchorTopRight, AnchorBottomLeft, AnchorBottomRight}, anchor) {
		return ErrAnchor
	}
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
	w := pg.W * frac
	if w/a > pg.H/2 {
		w = pg.H / 2 * a
	}
	h := w / a
	if zone == nil {
		st.X, st.Y = anchorXY(pg, w, h, anchor, max(p.Settings.Safe, p.Settings.Margin))
	} else {
		if k := min(zone.W/w, zone.H/h, 1); k < 1 {
			w, h = w*k, h*k
		}
		if w < model.MinSticker || h < model.MinSticker {
			return ErrNoSide
		}
		if anchor == AnchorCenter {
			st.X, st.Y = anchorXY(pg, w, h, anchor, 0)
		} else {
			st.X, st.Y = anchorXY(*zone, w, h, anchor, 0)
		}
		st.X = geom.ClampF(st.X, zone.X, zone.X+zone.W-w)
		st.Y = geom.ClampF(st.Y, zone.Y, zone.Y+zone.H-h)
	}
	st.ID, st.W, st.H = id, w, h
	if c := p.ClampSticker(s, st); c != nil {
		s.Stickers = append(s.Stickers, c)
	}
	return nil
}

// okFrac — доля ширины страницы осмысленна (NaN не проходит).
func okFrac(f float64) bool { return f > 0 && f <= 1 }

// anchorXY — угол рамки w × h в странице pg по якорю; отступ inset от краёв не больше, чем
// остаётся свободного места (рамка не выходит за страницу).
func anchorXY(pg layout.Rect, w, h float64, anchor string, inset float64) (x, y float64) {
	x, y = pg.X+(pg.W-w)/2, pg.Y+(pg.H-h)/2
	ix, iy := min(inset, max(0, (pg.W-w)/2)), min(inset, max(0, (pg.H-h)/2))
	switch anchor {
	case AnchorTopLeft, AnchorBottomLeft:
		x = pg.X + ix
	case AnchorTopRight, AnchorBottomRight:
		x = pg.X + pg.W - ix - w
	}
	switch anchor {
	case AnchorTopLeft, AnchorTopRight:
		y = pg.Y + iy
	case AnchorBottomLeft, AnchorBottomRight:
		y = pg.Y + pg.H - iy - h
	}
	return x, y
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
