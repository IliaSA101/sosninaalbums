package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Плитка обложки макета (G6a): две стороны рядом, фон по CoverPageBg, места в зоне стороны,
// рамочки, заготовки, надписи и стикеры стороны (в окне видно то, что поставит «Применить»).

// coverLook — макет с обложкой: задняя красная, лицевая синяя (свои цвета), фон листа зелёный;
// на лицевой — угловое место и (sticker) звезда по центру.
func coverLook(sticker bool) model.AlbumTpl {
	l := testLook("#00ff00")
	place := []model.Box{{X: 0.02, Y: 0.02, W: 0.1, H: 0.1}}
	back := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: place, Page: []model.TplBg{{Color: "#ff0000"}}}
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: place, Page: []model.TplBg{{Color: "#0000ff"}}}
	if sticker {
		front.Stickers = []model.TplItem{{Art: &model.Art{Kind: "star"}, W: 0.4, H: 0.4}}
		front.Drafts = []model.TplText{{Text: "[Имена]", W: 0.5, Style: model.TextStyle{Size: 20}}}
	}
	l.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#00ff00"}, Back: &back, Front: &front}
	return l
}

func saveCoverLook(t *testing.T, a *App, name string, l model.AlbumTpl) string {
	t.Helper()
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.SaveLook("mine", name, l)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimPrefix(it.ID, "my:")
}

func lookPNG(t *testing.T, a *App, url string) image.Image {
	t.Helper()
	rec := getTpl(a, http.MethodGet, url)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d", url, rec.Code)
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	return img
}

// isHex — цвет пикселя с точностью до округления сжатия.
func isHex(c color.Color, hex string) bool {
	r, g, b, _ := c.RGBA()
	want, _ := strconv.ParseUint(hex[1:], 16, 32)
	d := func(got uint32, w uint64) bool { return int(got>>8)-int(w) < 6 && int(w)-int(got>>8) < 6 }
	return d(r, want>>16&255) && d(g, want>>8&255) && d(b, want&255)
}

// Лист обложки: ширина ровно px (в том числе нечётная), слева фон задней, справа — лицевой;
// f=1 — одна лицевая в пропорции r.
func TestMediaLookCoverSides(t *testing.T) {
	a := lookApp(t)
	hex := saveCoverLook(t, a, "Обложка", coverLook(false))
	for _, px := range []int{64, 65, 255, 256} {
		img := lookPNG(t, a, "/media/look/"+hex+"?v=1&i=0&px="+strconv.Itoa(px)+"&r=1.5&bg=0")
		b := img.Bounds()
		if b.Dx() != px || b.Dy() < px/2 || b.Dy() > px {
			t.Errorf("px=%d: %v, ждали ширину %d и высоту ≈ %d", px, b, px, px*2/3)
		}
		if c := img.At(b.Min.X+1, b.Min.Y+b.Dy()-2); !isHex(c, "#ff0000") {
			t.Errorf("px=%d: слева — задняя красная: %v", px, c)
		}
		if c := img.At(b.Max.X-2, b.Min.Y+b.Dy()-2); !isHex(c, "#0000ff") {
			t.Errorf("px=%d: справа — лицевая синяя: %v", px, c)
		}
	}
	// Лицевая: одна сторона, r — её отношение (0,75 → высота 4/3 ширины).
	img := lookPNG(t, a, "/media/look/"+hex+"?v=1&i=0&px=256&r=0.75&bg=0&f=1")
	b := img.Bounds()
	if b.Dx() != 256 || b.Dy() < 340 || b.Dy() > 343 {
		t.Errorf("лицевая: %v", b)
	}
	for _, p := range []image.Point{{1, 1}, {b.Dx() - 2, b.Dy() - 2}, {b.Dx() - 2, 1}} {
		if c := img.At(p.X, p.Y); !isHex(c, "#0000ff") {
			t.Errorf("лицевая вся синяя %v: %v", p, c)
		}
	}
	// Фон листа (bg=1): две страницы с цветами сторон.
	bgImg := lookPNG(t, a, "/media/look/"+hex+"?v=1&i=0&px=256&r=2&bg=1")
	bb := bgImg.Bounds()
	if !isHex(bgImg.At(5, 5), "#ff0000") || !isHex(bgImg.At(bb.Max.X-5, 5), "#0000ff") {
		t.Errorf("фон листа: %v %v", bgImg.At(5, 5), bgImg.At(bb.Max.X-5, 5))
	}
}

// Края пропорции: отношение листа на пределах (сторона r/2 уже минимума) — картинка, не 404 и не паника.
func TestMediaLookCoverRatioEdges(t *testing.T) {
	a := lookApp(t)
	hex := saveCoverLook(t, a, "Обложка", coverLook(true))
	for _, q := range []string{"r=0.2&bg=0", "r=5&bg=0", "r=0.2&bg=1", "r=0.2&bg=0&f=1", "r=5&bg=0&f=1"} {
		img := lookPNG(t, a, "/media/look/"+hex+"?v=1&i=0&px=64&"+q)
		if img.Bounds().Dx() != 64 {
			t.Errorf("%s: %v", q, img.Bounds())
		}
	}
}

// Стикер и заготовка лицевой видны в плитке; без них центр стороны — только фон.
func TestMediaLookCoverDecor(t *testing.T) {
	a := lookApp(t)
	with := saveCoverLook(t, a, "Со стикером", coverLook(true))
	without := saveCoverLook(t, a, "Без стикера", coverLook(false))
	centre := func(hex string) (other int) {
		img := lookPNG(t, a, "/media/look/"+hex+"?v=1&i=0&px=256&r=0.75&bg=0&f=1")
		b := img.Bounds()
		for y := b.Dy() * 4 / 10; y < b.Dy()*6/10; y++ {
			for x := b.Dx() * 4 / 10; x < b.Dx()*6/10; x++ {
				if !isHex(img.At(x, y), "#0000ff") {
					other++
				}
			}
		}
		return other
	}
	if n := centre(without); n != 0 {
		t.Errorf("без стикера центр — фон, чужих пикселей %d", n)
	}
	if n := centre(with); n == 0 {
		t.Error("стикер лицевой в плитке не виден")
	}
}

// Шаблоны плитки несут всё оформление стороны, а не только места; общий помощник — тот же, что у library-verify.
func TestLookCoverTilesContent(t *testing.T) {
	l := coverLook(true)
	l.Cover.Front.Borders = []model.TplBorder{{I: 0, Kind: "line"}}
	l.Cover.Front.Texts = []model.TplText{{Text: "Свадьба", W: 0.5}}
	tp, ok := lookTile(l, 0, false, false, 1.5)
	if !ok || len(tp) != 2 {
		t.Fatalf("%v %d", ok, len(tp))
	}
	f := tp[1]
	if len(f.Places) != 1 || len(f.Borders) != 1 || len(f.Drafts) != 1 || len(f.Texts) != 1 || len(f.Stickers) != 1 {
		t.Errorf("оформление лицевой: %+v", f)
	}
	if tp[0].Page[0].Color != "#ff0000" || f.Page[0].Color != "#0000ff" {
		t.Errorf("фон сторон — их цвета: %v %v", tp[0].Page[0], f.Page[0])
	}
	// bg: оформление сторон снято, цвета остались.
	bgT, _ := lookTile(l, 0, true, true, 0.75)
	g := bgT[0]
	if len(g.Places)+len(g.Borders)+len(g.Drafts)+len(g.Texts)+len(g.Stickers) != 0 || g.Page[0].Color != "#0000ff" {
		t.Errorf("bg лицевой: %+v", g)
	}
	// Источник макета не изменён плиткой.
	if len(l.Cover.Front.Places) != 1 || l.Cover.Front.Places[0].X != 0.02 {
		t.Errorf("макет изменён: %+v", l.Cover.Front.Places)
	}
	// Ключи библиотеки — по обеим сторонам.
	l.Cover.Back.Stickers = []model.TplItem{{Lib: "my:00000000000000aa", W: 0.2, H: 0.2}}
	l.Cover.Front.Page[0].Lib = "my:00000000000000bb"
	tp, _ = lookTile(l, 0, false, false, 1.5)
	if keys := lookKeys(tp); len(keys) != 2 {
		t.Errorf("ключи: %v", keys)
	}
}
