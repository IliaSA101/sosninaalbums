package layers_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// AddStickers: ids в порядке пачки, ошибка — nil ids, пустая пачка ничего не меняет; пределы листа и альбома.
func TestAddStickersBatch(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.NewSpread(e, -1, edtest.Photos("b"))
	sp := edtest.SpreadIDs(e.View())
	p := edtest.P(e)
	star := model.StickerSrc{Art: &model.Art{Kind: "star"}}
	put := func(sheet string, size string) layers.StickerPut {
		return layers.StickerPut{Spread: sheet, Src: star, Anchor: layers.AnchorCenter, Size: size}
	}

	if ids, err := layers.AddStickers(p, nil); err != nil || len(ids) != 0 {
		t.Fatalf("пустая пачка: %v, %v", ids, err)
	}
	ids, err := layers.AddStickers(p, []layers.StickerPut{put(sp[0], "s"), put(sp[1], ""), put(sp[0], "l")})
	if err != nil || len(ids) != 3 || len(p.Spreads[0].Stickers) != 2 || len(p.Spreads[1].Stickers) != 1 {
		t.Fatalf("пачка: %v, %v, стикеров %d и %d", ids, err, len(p.Spreads[0].Stickers), len(p.Spreads[1].Stickers))
	}
	if p.Spreads[0].Stickers[0].ID != ids[0] || p.Spreads[1].Stickers[0].ID != ids[1] || p.Spreads[0].Stickers[1].ID != ids[2] {
		t.Fatalf("ids не в порядке пачки: %v", ids)
	}
	for name, c := range map[string]struct {
		put  layers.StickerPut
		want error
	}{
		"нет листа": {put("нет", "m"), layers.ErrNoSheet},
		"обложка":   {put(model.CoverID, "m"), layers.ErrNoSheet},
		"размер":    {put(sp[0], "xl"), layers.ErrAnchor},
		"место":     {layers.StickerPut{Spread: sp[0], Src: star, Anchor: "середина"}, layers.ErrAnchor},
	} {
		if ids, err := layers.AddStickers(p, []layers.StickerPut{c.put}); !errors.Is(err, c.want) || ids != nil {
			t.Errorf("%s: %v, %v", name, ids, err)
		}
	}
	// Лист: предел MaxStickers.
	full := make([]layers.StickerPut, model.MaxStickers)
	for i := range full {
		full[i] = put(sp[1], "s")
	}
	if ids, err := layers.AddStickers(p, full); !errors.Is(err, model.ErrStickers) || ids != nil {
		t.Errorf("больше предела на листе: %v, %v", ids, err)
	}
	// Альбом: предел MaxTotalStickers отказывает до первого стикера.
	n := len(p.Spreads[0].Stickers) + len(p.Spreads[1].Stickers)
	over := make([]layers.StickerPut, model.MaxTotalStickers)
	if ids, err := layers.AddStickers(p, over); !errors.Is(err, model.ErrStickersTotal) || ids != nil || len(p.Spreads[0].Stickers)+len(p.Spreads[1].Stickers) != n {
		t.Errorf("больше предела в альбоме: %v, %v", ids, err)
	}
}
