package render

import (
	"image"
	"image/color"
	"image/draw"
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
