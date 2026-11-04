package colorpick

// Кадр листа для службы агентов (143): тот же рендер и те же превью фото, что у пипетки, но размер
// задаёт вызывающий, а кадр не кэшируется (размеры разные, читают редко).

import (
	"context"
	"image"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Sheet рисует лист (разворот или обложку) spreadID видом печати шириной не больше px: картинки —
// из превью кэша миниатюр, через Host.Photo (проверка пути и тома), недоступные места пусты.
// Ждёт слот рендера вместе с пипеткой; срок и отмена — ctx вызывающего. Нет листа — ErrNoSheet.
// Альбом не меняется.
func (pk *Picker) Sheet(ctx context.Context, spreadID string, px int) (*image.RGBA, error) {
	p, v := pk.h.Editor().Snapshot()
	sv, ok := viewSheet(v, spreadID)
	if !ok {
		return nil, ErrNoSheet
	}
	release, err := pk.pick.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	scene := export.SheetScene(p, sv)
	if v.Cover != nil && v.Cover.Sheet.ID == spreadID {
		if cs, ok := export.CoverScene(p, v); ok {
			scene = cs
		}
	}
	pk.pickPreviews(ctx, p.Photos, &scene)
	return render.RenderCtx(ctx, scene, render.Options{PageW: sv.W / 2, PageH: sv.H, DPI: sheetDPI(px, sv.W, sv.H)})
}

// sheetDPI — целый dpi кадра: большая сторона листа не больше px (округление вниз), но не меньше
// pickMinDPI. По одной ширине лист 50×1000 мм при px = 1600 давал кадр 1598×15984 (+100 МБ).
func sheetDPI(px int, w, h float64) int {
	return max(int(float64(px)*25.4/max(w, h, 1)), pickMinDPI)
}
