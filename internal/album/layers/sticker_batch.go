package layers

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// ErrNoSheet — лист стикера не найден.
var ErrNoSheet = errors.New("такого разворота нет")

// StickerPut — один стикер пачки AddStickers: лист, страница (0 — левая, 1 — правая), рисунок,
// якорь Anchor… и размер s | m | l ("" — m). У обложки Page — сторонка: 0 — задняя, 1 — лицевая.
type StickerPut struct {
	Spread string
	Page   int
	Src    model.StickerSrc
	Anchor string
	Size   string
}

// AddStickers — тело Editor.AddStickersIf: пачка стикеров агента (151, design §1.5). Пачка целиком
// или никак (откат — try вызывающего): неизвестный лист — ErrNoSheet, сторонка обложки без
// безопасной зоны — ErrNoSide, место или размер — ErrAnchor, больше MaxStickers на листе — model.ErrStickers, больше MaxTotalStickers в
// альбоме — model.ErrStickersTotal, стикер, не прошедший зажим, — model.ErrSticker. Пустая пачка —
// холостая. ids — новые стикеры в порядке puts.
func AddStickers(p *model.Project, puts []StickerPut) (ids []string, err error) {
	total := 0
	for _, s := range p.Sheets() {
		total += len(s.Stickers)
	}
	if total+len(puts) > model.MaxTotalStickers {
		return nil, model.ErrStickersTotal
	}
	for _, in := range puts {
		s := p.Sheet(in.Spread)
		if s == nil {
			return nil, ErrNoSheet
		}
		page := min(max(in.Page, 0), 1)
		var zone *layout.Rect
		if p.IsCover(s) {
			if zone = coverZone(p, page); zone == nil {
				return nil, ErrNoSide
			}
		}
		frac, ok := SizeFrac(in.Size)
		if !ok {
			return nil, ErrAnchor
		}
		id, n := model.NewID(), len(s.Stickers)
		if err := addStickerIn(p, in.Spread, page, zone, id, in.Src, in.Anchor, frac); err != nil {
			return nil, err
		}
		if len(s.Stickers) == n {
			return nil, model.ErrSticker
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// coverZone — безопасная зона сторонки обложки для страницы page (0 — задняя, 1 — лицевая); nil —
// у сторонки зоны нет (CoverSides такие пропускает).
func coverZone(p *model.Project, page int) *layout.Rect {
	scope := model.ScopeLeft
	if page == 1 {
		scope = model.ScopeRight
	}
	for _, sd := range p.CoverSides() {
		if sd.Scope == scope {
			return &sd.Box
		}
	}
	return nil
}
