package layers

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ErrNoSheet — лист стикера не найден (или это обложка: стикеры на обложку агенту — 0.35).
var ErrNoSheet = errors.New("такого разворота нет")

// StickerPut — один стикер пачки AddStickers: лист, страница (0 — левая, 1 — правая), рисунок,
// якорь Anchor… и размер s | m | l ("" — m).
type StickerPut struct {
	Spread string
	Page   int
	Src    model.StickerSrc
	Anchor string
	Size   string
}

// AddStickers — тело Editor.AddStickersIf: пачка стикеров агента (151, design §1.5). Пачка целиком
// или никак (откат — try вызывающего): неизвестный лист или обложка — ErrNoSheet, место или
// размер — ErrAnchor, больше MaxStickers на листе — model.ErrStickers, больше MaxTotalStickers в
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
		if s == nil || p.IsCover(s) {
			return nil, ErrNoSheet
		}
		frac, ok := SizeFrac(in.Size)
		if !ok {
			return nil, ErrAnchor
		}
		id, n := model.NewID(), len(s.Stickers)
		if err := AddStickerAt(p, in.Spread, min(max(in.Page, 0), 1), id, in.Src, in.Anchor, frac); err != nil {
			return nil, err
		}
		if len(s.Stickers) == n {
			return nil, model.ErrSticker
		}
		ids = append(ids, id)
	}
	return ids, nil
}
