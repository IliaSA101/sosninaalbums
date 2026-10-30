package view_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Стикеры в View (048 G11): StickerView, Z в общем стеке, LowDPI, обложка, круг Save → Load.

const (
	stKeyA = "my:0123456789abcdef"
	stKeyB = "my:fedcba9876543210"
)

func ownSrc(key string, w, h int) model.StickerSrc {
	return model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs(key[3:] + ".png"), Lib: key, W: w, H: h}}
}

func artSrc(kind string, inks ...string) model.StickerSrc {
	return model.StickerSrc{Art: &model.Art{Kind: kind, Inks: inks}}
}

func putSticker(t *testing.T, e *album.Editor, sid string, page int, src model.StickerSrc) string {
	t.Helper()
	_, id, err := e.AddSticker(sid, page, src)
	if err != nil || id == "" {
		t.Fatalf("AddSticker: %q, %v", id, err)
	}
	return id
}

// stickerOf — вид стикера id на листе sv.
func stickerOf(t *testing.T, sv model.SpreadView, id string) model.StickerView {
	t.Helper()
	for _, st := range sv.Stickers {
		if st.ID == id {
			return st
		}
	}
	t.Fatalf("стикера %s нет в виде: %+v", id, sv.Stickers)
	return model.StickerView{}
}

// Пусто — [] (UI не проверяет null).
func TestStickerViewsEmpty(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	v := e.View()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if v.Spreads[0].Stickers == nil || strings.Count(string(data), `"stickers":[]`) != len(v.Spreads) {
		t.Fatalf("стикеры пустых листов: %#v", v.Spreads[0].Stickers)
	}
}

// Свой, процедурный, растяжимый и неизвестный на двух листах: рамка, URL, имя, цвета, пикеры,
// пропорция, dpi, Known; порядок — Spread.Stickers. Круг Save → Load вид не меняет; зеркало —
// шаг, отмена его снимает; та же прозрачность — холостая.
func TestStickerViews(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t) // лист 600×300
	own := putSticker(t, e, s1, 0, ownSrc(stKeyA, 400, 200))
	heart := putSticker(t, e, s1, 1, artSrc("heart", "#102030"))
	plate := putSticker(t, e, s1, 1, artSrc("plate"))
	e.SetStickerBox(s1, plate, 20, 30, 120, 20, 15, "")
	future := putSticker(t, e, s2, 0, artSrc("zz-future", "#010203"))
	e.FlipSticker(s1, heart, "h")
	v := e.SetStickerFade(s1, own, 0.3, "")

	sv := v.Spreads[0]
	if got := []string{sv.Stickers[0].ID, sv.Stickers[1].ID, sv.Stickers[2].ID}; len(sv.Stickers) != 3 ||
		!slices.Equal(got, []string{own, heart, plate}) {
		t.Fatalf("стикеры листа 1: %+v", sv.Stickers)
	}
	o := stickerOf(t, sv, own)
	st := edtest.P(e).Spreads[0].Stickers[0]
	if o.X != st.X || o.Y != st.Y || o.W != st.W || o.H != st.H || o.Fade != 0.3 || o.Gen || o.Art != nil ||
		o.Photo != model.LibPhotoID(stKeyA) || o.Lib != stKeyA || o.URL != "/media/p/"+o.Photo || !o.Known ||
		o.Aspect != 2 || o.DPI != geom.FrameDPI(400, layout.Rect{W: o.W, H: o.H}) || o.DPI == 0 || o.Name != "" {
		t.Fatalf("свой: %+v (в модели %+v)", o, *st)
	}

	hm, _ := gen.Lookup("heart")
	h := stickerOf(t, sv, heart)
	if !h.Gen || !h.Known || h.Name != hm.Name || h.Inks != 2 || h.Stretch || h.Aspect != 1 || h.DPI != 0 ||
		!h.FlipH || h.FlipV || h.Photo != "" || h.Lib != "" || h.Art == nil ||
		!slices.Equal(h.Art.Inks, []string{"#102030", hm.Inks[1]}) {
		t.Fatalf("сердце: %+v %+v", h, h.Art)
	}
	if !strings.HasPrefix(h.URL, "/media/gen/art?k=heart&i=102030&") || strings.Contains(h.URL, "px=") || strings.Contains(h.URL, "a=") {
		t.Fatalf("URL сердца %q", h.URL)
	}
	path, query, _ := strings.Cut(h.URL+"&px=64", "?")
	if req, ok := gen.Parse(path, query); !ok || !req.Art {
		t.Fatalf("URL не разбирается /media/gen: %q", h.URL)
	}

	pl := stickerOf(t, sv, plate)
	if !pl.Stretch || pl.Aspect != 0 || pl.W != 120 || pl.H != 20 || pl.Rot != 15 || !pl.Known || pl.Inks != 2 {
		t.Fatalf("плашка: %+v", pl)
	}
	path, query, _ = strings.Cut(pl.URL+"&px=600", "?")
	if req, ok := gen.Parse(path, query); !ok || !strings.Contains(pl.URL, "&a=6&") {
		t.Fatalf("плашка — не в пропорции рамки: %q", pl.URL)
	} else if w, h := req.Size(); w != 600 || h != 100 {
		t.Fatalf("плашка превью %d×%d, ждали 600×100", w, h)
	}

	f := stickerOf(t, v.Spreads[1], future)
	if f.Known || f.Name != "" || f.URL != "" || f.Inks != 0 || !f.Gen || f.Art == nil || f.Art.Kind != "zz-future" ||
		!slices.Equal(f.Art.Inks, []string{"#010203"}) {
		t.Fatalf("неизвестный генератор: %+v %+v", f, f.Art)
	}

	q := edtest.SaveLoad(t, e)
	v2 := edtest.Open(t, q, "").View()
	for i := range v.Spreads {
		if !reflect.DeepEqual(v.Spreads[i].Stickers, v2.Spreads[i].Stickers) {
			t.Fatalf("лист %d после Save → Load:\n%+v\n%+v", i, v.Spreads[i].Stickers, v2.Spreads[i].Stickers)
		}
	}

	undo := edtest.UndoLen(e)
	if got := stickerOf(t, e.FlipSticker(s1, own, "v").Spreads[0], own); !got.FlipV {
		t.Fatalf("зеркало: %+v", got)
	}
	rev := edtest.Rev(e)
	e.SetStickerFade(s1, own, 0.3, "")
	edtest.Same(t, e, rev, undo+1, "та же прозрачность")
	if got := e.Undo().Spreads[0].Stickers; !reflect.DeepEqual(got, v.Spreads[0].Stickers) {
		t.Fatalf("после отмены: %+v", got)
	}
}

// Z стикеров — в общем стеке: сетка — места, стикеры, тексты; стикер под место — ниже места;
// «Авто» — ячейки, затем стикеры и тексты. Z всех элементов — 1..N без повторов.
func TestStickerViewZ(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(200, 10, 100, 100, "b")})
	e.SetCaption("a", "Подпись", "")
	tx := edtest.TextAt(t, e, "s1", 50, 50)
	a := putSticker(t, e, "s1", 0, artSrc("star"))
	b := putSticker(t, e, "s1", 1, artSrc("heart"))
	sv := e.View().Spreads[0]
	pz := map[string]int{}
	var all []int
	for _, p := range sv.Places {
		pz[p.Photo] = p.Z
		all = append(all, p.Z)
	}
	for _, c := range sv.Captions {
		all = append(all, c.Z)
	}
	sa, sb := stickerOf(t, sv, a), stickerOf(t, sv, b)
	all = append(all, sa.Z, sb.Z, sv.Texts[0].Z)
	slices.Sort(all)
	if !slices.Equal(all, []int{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("сетка: z %v", all)
	}
	if pz["b"] >= sa.Z || sa.Z >= sb.Z || sb.Z >= sv.Texts[0].Z || sv.Texts[0].ID != tx {
		t.Fatalf("порядок по умолчанию: места %v, стикеры %d %d, текст %d", pz, sa.Z, sb.Z, sv.Texts[0].Z)
	}
	sv = e.ArrangeLayers("s1", "back", []string{b}).Spreads[0]
	if sb := stickerOf(t, sv, b); sb.Z != 1 {
		t.Fatalf("стикер вниз стека: z %d, места %+v", sb.Z, sv.Places)
	}

	ea, id := edtest.AutoEd(t)
	edtest.AddText(t, ea, id, 0)
	k := putSticker(t, ea, id, 1, artSrc("star"))
	av := ea.View().Spreads[0]
	cells := len(av.Frames)
	if sk := stickerOf(t, av, k); cells == 0 || sk.Z != cells+1 || av.Texts[0].Z != cells+2 {
		t.Fatalf("«Авто»: ячеек %d, стикер %d, текст %d", cells, sk.Z, av.Texts[0].Z)
	}
	av = ea.ArrangeLayers(id, "front", []string{k}).Spreads[0]
	if sk := stickerOf(t, av, k); sk.Z != cells+2 || av.Texts[0].Z != cells+1 {
		t.Fatalf("«Авто», стикер наверх: стикер %d, текст %d", sk.Z, av.Texts[0].Z)
	}
}

// Свой стикер, растянутый мельче порога dpi, — в LowDPI листа и альбома; процедурный не
// считается; стикер обложки — в виде обложки и в её LowDPI.
func TestStickerViewLowDPICover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	v0 := e.View() // фото мест 300×200 px сами ниже порога: считаем прирост
	own := putSticker(t, e, "s1", 0, ownSrc(stKeyA, 400, 400))
	putSticker(t, e, "s1", 0, artSrc("star"))
	v := e.SetStickerBox("s1", own, 10, 10, 100, 100, 0, "") // 400 px на 100 мм — 102 dpi
	if o := stickerOf(t, v.Spreads[0], own); o.DPI != 102 || v.Spreads[0].LowDPI != v0.Spreads[0].LowDPI+1 || v.LowDPI != v0.LowDPI+1 {
		t.Fatalf("dpi %d, LowDPI листа %d, альбома %d", o.DPI, v.Spreads[0].LowDPI, v.LowDPI)
	}
	c := putSticker(t, e, model.CoverID, 1, ownSrc(stKeyB, 100, 100))
	v = e.SetStickerBox(model.CoverID, c, 400, 100, 50, 50, 0, "") // 51 dpi
	cs := v.Cover.Sheet
	if len(cs.Stickers) != 1 || cs.Stickers[0].ID != c || cs.LowDPI != v0.Cover.Sheet.LowDPI+1 || v.LowDPI != v0.LowDPI+2 {
		t.Fatalf("обложка: %+v, LowDPI %d / %d", cs.Stickers, cs.LowDPI, v.LowDPI)
	}
	if !slices.Contains(v.BackdropUsed, model.LibPhotoID(stKeyB)) || !slices.Contains(v.LibPhotos, model.LibPhotoID(stKeyB)) {
		t.Fatalf("фото стикера обложки: used %v, lib %v", v.BackdropUsed, v.LibPhotos)
	}
}
