package export

import (
	"image"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Фирменные стикеры в экспорте (спека 116): едут в задание как процедурные, на 300 dpi рисуются —
// чёрный «sosnina» и красный «films» видны; белый на белом листе даёт только красное; зеркальный
// и повёрнутый стикер на листе тоже рисуются. Несколько стикеров на одном листе.
func TestExportBrandStickers(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}}
	e := mustEditor(t, p, "")
	var ids []string
	for _, kind := range []string{"brand-albums-black", "brand-films-white", "brand-films-black"} {
		_, id, err := e.AddSticker("s1", 0, model.StickerSrc{Art: &model.Art{Kind: kind}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		e.SetStickerBox("s1", id, 10, 10+float64(len(ids)-1)*45, 80, 34, 0, "") // друг под другом: рамки не пересекаются
	}
	e.FlipSticker("s1", ids[2], "h")
	pp, v := e.Snapshot()
	got := FromView(pp, v)
	if len(got) != 1 || len(got[0].Arts) != 3 || len(got[0].Stickers) != 0 {
		t.Fatalf("задание: %+v", got)
	}
	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 300}
	img, err := render.Render(got[0], opt)
	if err != nil {
		t.Fatal(err)
	}
	count := func(box image.Rectangle, pred func(r, g, b uint8) bool) int {
		n := 0
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				if c := img.RGBAAt(x, y); pred(c.R, c.G, c.B) {
					n++
				}
			}
		}
		return n
	}
	for i, sv := range v.Spreads[0].Stickers {
		k := 300 / 25.4
		box := image.Rect(int(sv.X*k), int(sv.Y*k), int((sv.X+sv.W)*k)+1, int((sv.Y+sv.H)*k)+1).Intersect(img.Rect)
		red := count(box, func(r, g, b uint8) bool { return r > 200 && g < 60 && b < 60 })
		black := count(box, func(r, g, b uint8) bool { return r < 20 && g < 20 && b < 20 })
		if red < 300 {
			t.Errorf("стикер %d: красных px %d", i, red)
		}
		if white := got[0].Arts[i].Kind == "brand-films-white"; white == (black > 300) {
			t.Errorf("стикер %d (%s): чёрных px %d", i, got[0].Arts[i].Kind, black)
		}
	}
}
