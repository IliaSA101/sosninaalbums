package render

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Узоры в рендере (048 G5): после заливки страниц, до фото фона; область в px — от тех же
// px(), что заливка; рисует тот же gen, что превью /media/gen.

// patOpt — 254 dpi: ровно 10 px на мм, страница 100×50 мм — 1000×500 px.
var patOpt = Options{PageW: 100, PageH: 50, DPI: 254}

var (
	patLeft  = layout.Rect{W: 100, H: 50}
	patRight = layout.Rect{X: 100, W: 100, H: 50}
)

func renderPat(t *testing.T, s Spread) *image.RGBA {
	t.Helper()
	img, err := Render(s, patOpt)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func samePix(a, b *image.RGBA) bool { return a.Rect == b.Rect && string(a.Pix) == string(b.Pix) }

// Две страницы с разными узорами — пиксель в пиксель как gen.DrawPattern по областям в px
// поверх заливки; узор правой страницы не заходит за шов на левую.
func TestRenderPatternPlace(t *testing.T) {
	bg := [2]color.RGBA{bdGray, bdCream}
	pats := []PatternDraw{
		{Area: patLeft, Kind: "stripes", Inks: []string{"#203040"}, Scale: 1},
		{Area: patRight, Kind: "dots", Inks: []string{"#a03020"}, Scale: 1.5, Fade: 0.5},
	}
	got := renderPat(t, Spread{Bg: bg, Patterns: pats})

	plain := renderPat(t, Spread{Bg: bg})
	want := image.NewRGBA(plain.Rect)
	copy(want.Pix, plain.Pix)
	for _, pd := range pats {
		r := image.Rect(px(pd.Area.X, 254), px(pd.Area.Y, 254), px(pd.Area.X+pd.Area.W, 254), px(pd.Area.Y+pd.Area.H, 254))
		gen.DrawPattern(want, r, gen.Spec{Kind: pd.Kind, Inks: pd.Inks, Scale: pd.Scale}, 10, pd.Fade)
	}
	if samePix(want, plain) {
		t.Fatal("узоры ничего не нарисовали")
	}
	if !samePix(got, want) {
		t.Fatal("рендер узоров разошёлся с gen.DrawPattern по областям")
	}

	right := renderPat(t, Spread{Bg: bg, Patterns: pats[1:]})
	mid := px(patOpt.PageW, 254)
	for y := 0; y < right.Rect.Dy(); y += 7 {
		for x := range mid {
			if c := right.RGBAAt(x, y); c != bdGray {
				t.Fatalf("узор правой страницы на левой: (%d, %d) %v", x, y, c)
			}
		}
	}
}

// Экспорт и холст рисуют одно: область страницы в рендере — как превью /media/gen той же
// области в мм (масштаб и привязка мотива к углу), поверх того же цвета.
func TestRenderPatternMatchesPreview(t *testing.T) {
	spec := gen.Spec{Kind: "dots", Inks: []string{"#a03020"}, Scale: 1.5}
	got := renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray},
		Patterns: []PatternDraw{{Area: patLeft, Kind: spec.Kind, Inks: spec.Inks, Scale: spec.Scale}}})

	prev := gen.Request{Spec: spec, W: patLeft.W, H: patLeft.H, Px: 1000}.Draw()
	want := image.NewRGBA(prev.Rect)
	draw.Draw(want, want.Rect, image.NewUniform(bdGray), image.Point{}, draw.Src)
	draw.Draw(want, want.Rect, prev, image.Point{}, draw.Over)
	differ := 0
	for y := range want.Rect.Dy() {
		for x := range want.Rect.Dx() {
			if !near(got.RGBAAt(x, y), want.RGBAAt(x, y), 3) {
				differ++
			}
		}
	}
	if differ > 0 {
		t.Fatalf("рендер и превью разошлись в %d px из %d", differ, want.Rect.Dx()*want.Rect.Dy())
	}
}

// Не рисуются: прозрачность 100 %, неизвестный генератор (файл из новой версии), пустой ключ;
// узор под непрозрачным фото фона не виден.
func TestRenderPatternHidden(t *testing.T) {
	bg := [2]color.RGBA{bdGray, bdCream}
	plain := renderPat(t, Spread{Bg: bg})
	for _, pd := range []PatternDraw{
		{Area: patLeft, Kind: "stripes", Fade: 1},
		{Area: patLeft, Kind: "future-kind", Inks: []string{"#000000"}, Scale: 1},
		{},
	} {
		if got := renderPat(t, Spread{Bg: bg, Patterns: []PatternDraw{pd}}); !samePix(got, plain) {
			t.Errorf("%+v: узор нарисован", pd)
		}
	}

	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	area := layout.Rect{W: 200, H: 50}
	bd := Backdrop{Area: area, Box: area, Img: layout.Rect{Y: -50, W: 200, H: 200}, Path: solid}
	pat := []PatternDraw{{Area: area, Kind: "stripes", Inks: []string{"#203040"}}}
	under := renderPat(t, Spread{Bg: bg, Patterns: pat, Backdrops: []Backdrop{bd}})
	if over := renderPat(t, Spread{Bg: bg, Backdrops: []Backdrop{bd}}); !samePix(under, over) {
		t.Fatal("узор виден поверх непрозрачного фото фона")
	}
}

// Экспорт с «Прозрачностью» 0,5 = превью /media/gen (fade 0) с opacity 0,5 поверх заливки, как
// на холсте UI: перекрытия волн и конфетти не темнеют на печати.
func TestRenderPatternFadeAsPreview(t *testing.T) {
	bg := [2]color.RGBA{bdGray, bdCream}
	pats := []PatternDraw{
		{Area: patLeft, Kind: "waves", Fade: 0.5},
		{Area: patRight, Kind: "confetti", Seed: 3, Fade: 0.5},
	}
	got := renderPat(t, Spread{Bg: bg, Patterns: pats})
	want := renderPat(t, Spread{Bg: bg})
	for _, pd := range pats {
		r := image.Rect(px(pd.Area.X, 254), px(pd.Area.Y, 254), px(pd.Area.X+pd.Area.W, 254), px(pd.Area.Y+pd.Area.H, 254))
		pre := image.NewRGBA(r)
		gen.DrawPattern(pre, r, gen.Spec{Kind: pd.Kind, Seed: pd.Seed}, 10, 0)
		draw.DrawMask(want, r, pre, r.Min, image.NewUniform(color.Alpha{A: 128}), image.Point{}, draw.Over)
	}
	for y := range got.Rect.Dy() {
		for x := range got.Rect.Dx() {
			a, b := got.RGBAAt(x, y), want.RGBAAt(x, y)
			for _, d := range [3]int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
				if d > 2 || d < -2 {
					t.Fatalf("(%d, %d): печать %v, превью × 0,5 — %v", x, y, a, b)
				}
			}
		}
	}
}

// Прозрачность фото фона (спека 017, design §5).

var fadeRed = color.RGBA{200, 30, 30, 255}

func whiteJPEG(t *testing.T) string {
	t.Helper()
	w := color.RGBA{255, 255, 255, 255}
	return quadrants(t, testkit.TempDir(t), [4]color.RGBA{w, w, w, w})
}

// fadeMix — белое фото с прозрачностью fade поверх цвета bg по формуле fadeAlpha.
func fadeMix(bg color.RGBA, fade float64) color.RGBA {
	a := math.Round(255 * (1 - fade))
	ch := func(c uint8) uint8 { return uint8(math.Round((255*a + float64(c)*(255-a)) / 255)) }
	return color.RGBA{ch(bg.R), ch(bg.G), ch(bg.B), 255}
}

func wantExact(t *testing.T, what string, img *image.RGBA, dpi int, x, y float64, want color.RGBA) {
	t.Helper()
	if got := img.RGBAAt(px(x, dpi), px(y, dpi)); !near(got, want, 1) {
		t.Errorf("%s: (%v, %v) мм — %v, ждали %v ±1", what, x, y, got, want)
	}
}

// Белое на красном (200, 30, 30) при 40 % — (233, 165, 165) ±1 во всех трёх путях.
func TestRenderBackdropFade(t *testing.T) {
	white := whiteJPEG(t)
	want := color.RGBA{233, 165, 165, 255}
	if fadeMix(fadeRed, 0.4) != want {
		t.Fatalf("формула: %v", fadeMix(fadeRed, 0.4))
	}
	left := layout.Rect{W: 100, H: 100}
	cases := []struct {
		name string
		bd   Backdrop
		x, y float64
		outX float64 // вне рамки — чистый цвет страницы
		outY float64
	}{
		{"путь 1, без поворота", Backdrop{Area: left, Box: left, Img: left}, 50, 50, 150, 50},
		{"путь 1, рамка под 90°", Backdrop{Area: left, Box: layout.Rect{X: 80, Y: 10, W: 60, H: 40}, Rot: 90,
			Img: layout.Rect{W: 60, H: 40}}, 60, 40, 20, 80},
		{"путь 3, рамка под 30°", Backdrop{Area: left, Box: layout.Rect{X: 20, Y: 30, W: 60, H: 40}, Rot: 30,
			Img: layout.Rect{W: 60, H: 40}}, 36, 62, 90, 10},
	}
	for _, c := range cases {
		c.bd.Path, c.bd.Fade = white, 0.4
		img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, fadeRed}, Backdrops: []Backdrop{c.bd}}, 100)
		wantExact(t, c.name, img, 100, c.x, c.y, want)
		wantExact(t, c.name+": вне рамки", img, 100, c.outX, c.outY, fadeRed)
	}
	// Путь 2 у фона не встречается (рамка без поворота — путь 1, с поворотом — 3), но
	// prepareRot общий: наклон фото в прямой ячейке с Fade смешивается так же.
	cell := layout.Rect{X: 30, Y: 30, W: 40, H: 40}
	ph := framedPhoto(cell, 1, 1.5, 0, 0, white)
	ph.ImgRot, ph.Fade = 5, 0.4
	img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, fadeRed}, Photos: []Photo{ph}}, 100)
	wantExact(t, "путь 2", img, 100, 50, 50, want)

	// Разворот по страницам: левая 30 % над своим цветом, правая непрозрачна; фон
	// «весь разворот» смешивается с цветом каждой половины.
	blue := color.RGBA{20, 40, 160, 255}
	right := layout.Rect{X: 100, W: 100, H: 100}
	s := Spread{Bg: [2]color.RGBA{fadeRed, blue}, Backdrops: []Backdrop{
		{Area: left, Box: left, Img: layout.Rect{W: 100, H: 100}, Path: white, Fade: 0.3},
		{Area: right, Box: right, Img: layout.Rect{W: 100, H: 100}, Path: white},
	}}
	img = renderBd(t, s, 100)
	wantExact(t, "левая 30 %", img, 100, 50, 50, fadeMix(fadeRed, 0.3))
	wantExact(t, "правая 0 %", img, 100, 150, 50, color.RGBA{255, 255, 255, 255})
	all := layout.Rect{W: 200, H: 100}
	s.Backdrops = []Backdrop{{Area: all, Box: all, Img: layout.Rect{Y: -50, W: 200, H: 200}, Path: white, Fade: 0.5}}
	img = renderBd(t, s, 100)
	wantExact(t, "разворот над левой", img, 100, 50, 50, fadeMix(fadeRed, 0.5))
	wantExact(t, "разворот над правой", img, 100, 150, 50, fadeMix(blue, 0.5))
}

// Fade 0 (и всё, что fadeAlpha считает нулём) — те же байты, что прежний вывод.
func TestRenderBackdropFadeZero(t *testing.T) {
	q := quadsBd(t)
	area := layout.Rect{W: 200, H: 100}
	box := layout.Rect{X: 20, Y: 10, W: 120, H: 70}
	img := framedPhoto(layout.Rect{W: 120, H: 70}, 1, 1.2, 0.3, 0, q).Img
	for _, rot := range []float64{0, 90, 30} {
		base := Backdrop{Area: area, Box: box, Rot: rot, Img: img, Path: q, FlipH: true}
		ref := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdCream}, Backdrops: []Backdrop{base}}, 100)
		for _, f := range []float64{-0.5, math.NaN(), 0.001} {
			b := base
			b.Fade = f
			got := renderBd(t, Spread{Bg: [2]color.RGBA{bdGray, bdCream}, Backdrops: []Backdrop{b}}, 100)
			if !bytes.Equal(ref.Pix, got.Pix) {
				t.Errorf("rot %v, fade %v: вывод изменился", rot, f)
			}
		}
	}
}

// Fade 1: фон не рисуется и файл не открывается (несуществующий путь — не ошибка).
func TestRenderBackdropFadeFull(t *testing.T) {
	calls := 0
	old := prepareFn
	t.Cleanup(func() { prepareFn = old })
	prepareFn = func(ctx context.Context, ph Photo, dpi int, clip image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
		calls++
		return old(ctx, ph, dpi, clip)
	}
	missing := filepath.Join(testkit.TempDir(t), "нет.jpg")
	left := layout.Rect{W: 100, H: 100}
	for _, rot := range []float64{0, 90, 30} {
		bd := Backdrop{Area: left, Box: left, Rot: rot, Img: left, Path: missing, Fade: 1}
		if rot == 90 { // поворот вокруг угла: иначе рамка ушла бы за область
			bd.Box.X = 100
		}
		img := renderBd(t, Spread{Bg: [2]color.RGBA{fadeRed, bdGray}, Backdrops: []Backdrop{bd}}, 100)
		wantExact(t, "только цвет страницы", img, 100, 50, 50, fadeRed)
		bd.Fade = 0.99
		if _, err := Render(Spread{Backdrops: []Backdrop{bd}}, Options{PageW: 100, PageH: 100, DPI: 100}); err == nil {
			t.Errorf("rot %v: при 99 %% файл не читался", rot)
		}
	}
	if calls != 2 { // путь 1 (rot 0 и 90) при 99 %
		t.Fatalf("prepareFn вызван %d раз", calls)
	}
}

// fadeAlpha: 0 — без изменений, 1 — альфа 0, подизображение — только свои пиксели.
func TestFadeAlpha(t *testing.T) {
	mk := func() *image.NRGBA {
		im := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		for i := range im.Pix {
			im.Pix[i] = 255
		}
		im.Pix[3] = 101 // 101·153/255 = 60,6: без округления вышло бы 60
		return im
	}
	im := mk()
	fadeAlpha(im, 0)
	fadeAlpha(im, math.NaN())
	fadeAlpha(nil, 0.5)
	if !bytes.Equal(im.Pix, mk().Pix) {
		t.Fatal("fade 0 изменил пиксели")
	}
	fadeAlpha(im, 0.4)
	if im.Pix[3] != 61 || im.Pix[7] != 153 || im.Pix[4] != 255 {
		t.Fatalf("0.4: альфа %d и %d, цвет %d", im.Pix[3], im.Pix[7], im.Pix[4])
	}
	im = mk()
	sub, _ := im.SubImage(image.Rect(1, 1, 3, 3)).(*image.NRGBA)
	if sub.Stride == 4*sub.Rect.Dx() {
		t.Fatal("подизображение без своего Stride")
	}
	fadeAlpha(sub, 1)
	for y := range 4 {
		for x := range 4 {
			in := x >= 1 && x < 3 && y >= 1 && y < 3
			if a := im.NRGBAAt(x, y).A; (a == 0) != in {
				t.Errorf("(%d, %d): альфа %d", x, y, a)
			}
		}
	}
}
