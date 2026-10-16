package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand/v2"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Стикеры в рендере (048 G11): процедурный — вектор gen.DrawArt в px печати (= превью
// /media/gen той же рамки), свой — фото целиком в рамке с альфой; оба в общем стеке по Z.

var stRed = color.RGBA{220, 30, 40, 255}

// halfPNG — свой стикер 100×100 px: левая половина красная, правая прозрачная.
func halfPNG(t *testing.T) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 50 {
			im.SetNRGBA(x, y, color.NRGBA{stRed.R, stRed.G, stRed.B, 255})
		}
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), "half.png"), im)
}

// at — пиксель холста в точке (x, y) мм при 10 px/мм.
func at(img *image.RGBA, x, y float64) color.RGBA {
	return img.RGBAAt(int(math.Floor(x*10)), int(math.Floor(y*10)))
}

// Превью = печать: рамка в рендере — как превью /media/gen той же рамки (px = ширина рамки
// в px печати) поверх того же фона; с прозрачностью — превью с opacity, как на холсте UI.
// Растяжимая плашка — в пропорции рамки. Несколько стикеров на листе сразу.
func TestRenderArtMatchesPreview(t *testing.T) {
	bg := [2]color.RGBA{bdGray, bdCream}
	arts := []ArtDraw{
		{Box: layout.Rect{X: 10, Y: 5, W: 40, H: 40}, Kind: "heart", Inks: []string{"#203040"}, Z: 1},
		{Box: layout.Rect{X: 60, Y: 20, W: 120, H: 20}, Kind: "plate", Z: 2},
		{Box: layout.Rect{X: 150, Y: 2, W: 16, H: 16}, Kind: "confetti-pile", Seed: 9, Fade: 0.5, Z: 3},
	}
	got := renderPat(t, Spread{Bg: bg, Arts: arts})
	want := renderPat(t, Spread{Bg: bg})
	plain := renderPat(t, Spread{Bg: bg})
	for _, a := range arts {
		pw := int(math.Round(a.Box.W * 10))
		prev := gen.Request{Art: true, Spec: gen.Spec{Kind: a.Kind, Inks: a.Inks, Seed: a.Seed, Aspect: a.Box.W / a.Box.H}, Px: pw}.Draw()
		r := prev.Rect.Add(image.Pt(int(a.Box.X*10), int(a.Box.Y*10)))
		if r.Dy() != int(math.Round(a.Box.H*10)) {
			t.Fatalf("%s: превью %v не в пропорции рамки", a.Kind, prev.Rect)
		}
		alpha := uint8(255)
		if a.Fade > 0 {
			alpha = 128
		}
		draw.DrawMask(want, r, prev, image.Point{}, image.NewUniform(color.Alpha{A: alpha}), image.Point{}, draw.Over)
	}
	if samePix(got, plain) {
		t.Fatal("стикеры ничего не нарисовали")
	}
	differ := 0
	for y := range want.Rect.Dy() {
		for x := range want.Rect.Dx() {
			if !near(got.RGBAAt(x, y), want.RGBAAt(x, y), 3) {
				differ++
			}
		}
	}
	if differ > 0 {
		t.Fatalf("печать и превью разошлись в %d px", differ)
	}
}

// Зеркало и поворот рамки: зеркало по горизонтали — отражённое превью; поворот на 90° по часовой
// вокруг центра — превью, повёрнутое так же (сетка px переходит в сетку px; покрытие края
// сглаживания в float32 после поворота расходится до 5 единиц цвета — допуск 6).
func TestRenderArtFlipRot(t *testing.T) {
	bg := [2]color.RGBA{bdGray, bdGray}
	box := layout.Rect{X: 20, Y: 5, W: 40, H: 20} // 400×200 px, центр (400, 150)
	spec := gen.Spec{Kind: "arrow", Aspect: 2}
	prev := gen.Request{Art: true, Spec: spec, Px: 400}.Draw()
	base := renderPat(t, Spread{Bg: bg})
	compose := func(fn func(x, y int) (int, int)) *image.RGBA {
		want := image.NewRGBA(base.Rect)
		copy(want.Pix, base.Pix)
		layer := image.NewRGBA(base.Rect)
		for y := range 200 {
			for x := range 400 {
				dx, dy := fn(x, y)
				layer.SetRGBA(dx, dy, prev.RGBAAt(x, y))
			}
		}
		draw.Draw(want, want.Rect, layer, image.Point{}, draw.Over)
		return want
	}
	for _, c := range []struct {
		name string
		a    ArtDraw
		want *image.RGBA
		tol  int
	}{
		{"зеркало", ArtDraw{Box: box, Kind: "arrow", FlipH: true}, compose(func(x, y int) (int, int) { return 200 + 399 - x, 50 + y }), 3},
		{"поворот 90", ArtDraw{Box: box, Kind: "arrow", Rot: 90}, compose(func(x, y int) (int, int) { return 300 + 199 - y, -50 + x }), 6},
	} {
		got := renderPat(t, Spread{Bg: bg, Arts: []ArtDraw{c.a}})
		differ := 0
		for y := range got.Rect.Dy() {
			for x := range got.Rect.Dx() {
				if !near(got.RGBAAt(x, y), c.want.RGBAAt(x, y), c.tol) {
					differ++
				}
			}
		}
		if samePix(got, base) || differ > 0 {
			t.Errorf("%s: разошлось %d px", c.name, differ)
		}
	}
}

// Процедурный стикер — вектор в px печати: край круга 100 мм при 300 dpi — не шире 2 px на
// сторону (растр превью, растянутый до печати, дал бы размытую полосу в десятки px).
func TestRenderArtSharp(t *testing.T) {
	opt := Options{PageW: 60, PageH: 120, DPI: 300}
	white := color.RGBA{255, 255, 255, 255}
	img, err := Render(Spread{Bg: [2]color.RGBA{white, white},
		Arts: []ArtDraw{{Box: layout.Rect{X: 10, Y: 10, W: 100, H: 100}, Kind: "circle", Inks: []string{"#000000"}}}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	y := px(60, 300)
	partial, ink := 0, 0
	for x := range img.Rect.Dx() {
		switch c := img.RGBAAt(x, y); {
		case c == white:
		case c.R == 0 && c.G == 0 && c.B == 0:
			ink++
		default:
			partial++
		}
	}
	if ink < px(90, 300) || partial > 4 {
		t.Fatalf("строка через центр: заливки %d px, переходных %d", ink, partial)
	}
}

// Свой стикер — PNG с альфой: прозрачная половина показывает страницу при любом повороте
// (прямо, 30°, 90°) и с зеркалом; прозрачность смешивает и непрозрачную половину.
func TestRenderStickerAlphaRot(t *testing.T) {
	path := halfPNG(t)
	box := layout.Rect{X: 20, Y: 10, W: 30, H: 30} // центр (35, 25)
	sin, cos := math.Sincos(30 * math.Pi / 180)
	for _, c := range []struct {
		name     string
		ph       Photo
		red, bg  [2]float64 // точки, мм
		redColor color.RGBA
	}{
		{"прямо", Photo{}, [2]float64{27.5, 25}, [2]float64{42.5, 25}, stRed},
		{"зеркало", Photo{FlipH: true}, [2]float64{42.5, 25}, [2]float64{27.5, 25}, stRed},
		{"90°", Photo{Rot: 90}, [2]float64{35, 17.5}, [2]float64{35, 32.5}, stRed},
		{"30°", Photo{Rot: 30}, [2]float64{35 - 7.5*cos, 25 - 7.5*sin}, [2]float64{35 + 7.5*cos, 25 + 7.5*sin}, stRed},
		{"прозрачность", Photo{Fade: 0.5}, [2]float64{27.5, 25}, [2]float64{42.5, 25},
			color.RGBA{uint8((int(stRed.R) + int(bdGray.R)) / 2), uint8((int(stRed.G) + int(bdGray.G)) / 2), uint8((int(stRed.B) + int(bdGray.B)) / 2), 255}},
	} {
		ph := c.ph
		ph.Cell, ph.Img, ph.Path = box, box, path
		img := renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Stickers: []Photo{ph}})
		if got := at(img, c.red[0], c.red[1]); !near(got, c.redColor, 3) {
			t.Errorf("%s: непрозрачная половина %v, ждали %v", c.name, got, c.redColor)
		}
		if got := at(img, c.bg[0], c.bg[1]); got != bdGray {
			t.Errorf("%s: прозрачная половина %v, ждали страницу", c.name, got)
		}
	}
}

// Стек: стикеры — по Z между фото мест и текстами; Z ниже фото — стикер под ним.
// Невидимые (прозрачность 100 %, неизвестный ключ, мусорная рамка) не рисуются, файл
// невидимого своего не читается; пропавший файл видимого — ошибка «стикера».
func TestRenderStickerStack(t *testing.T) {
	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	cell := layout.Rect{X: 10, Y: 10, W: 30, H: 30}
	photo := Photo{Cell: cell, Img: cell, Path: solid, Z: 2}
	art := ArtDraw{Box: layout.Rect{X: 20, Y: 20, W: 10, H: 10}, Kind: "circle", Inks: []string{"#000000"}}
	bg := [2]color.RGBA{bdGray, bdGray}
	for _, c := range []struct {
		z    int
		want color.RGBA
	}{{3, color.RGBA{0, 0, 0, 255}}, {1, bdCream}} {
		a := art
		a.Z = c.z
		img := renderPat(t, Spread{Bg: bg, Photos: []Photo{photo}, Arts: []ArtDraw{a}})
		if got := at(img, 25, 25); !near(got, c.want, 3) {
			t.Errorf("стикер z=%d над фото z=2: центр %v, ждали %v", c.z, got, c.want)
		}
	}

	plain := renderPat(t, Spread{Bg: bg})
	gone := filepath.Join(testkit.TempDir(t), "нет.png")
	for _, s := range []Spread{
		{Arts: []ArtDraw{{Box: art.Box, Kind: "circle", Fade: 1}}},
		{Arts: []ArtDraw{{Box: art.Box, Kind: "zz-future", Inks: []string{"#000000"}}}},
		{Arts: []ArtDraw{{Box: layout.Rect{X: math.NaN(), W: 10, H: 10}, Kind: "circle"}, {Box: layout.Rect{W: 0, H: 10}, Kind: "circle"}}},
		{Stickers: []Photo{{Cell: cell, Img: cell, Path: gone, Fade: 1}}},
	} {
		s.Bg = bg
		if got := renderPat(t, s); !samePix(got, plain) {
			t.Errorf("%+v: нарисовано", s)
		}
	}
	_, err := Render(Spread{Bg: bg, Stickers: []Photo{{Cell: cell, Img: cell, Path: gone}}}, patOpt)
	if err == nil || !strings.Contains(err.Error(), "нет.png") {
		t.Fatalf("пропавший свой стикер: %v", err)
	}
}

// blocksPNG — случайные цвета в сетке 16×12 px: стикер растягивается в десятки раз, и сдвиг окна
// на полпикселя оригинала — это десяток px печати, виден по краям блоков.
func blocksPNG(t *testing.T) string {
	t.Helper()
	rng := rand.New(rand.NewPCG(7, 7))
	im := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	for by := 0; by < 12; by++ {
		for bx := 0; bx < 16; bx++ {
			c := color.NRGBA{uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256)), 255}
			draw.Draw(im, image.Rect(bx, by, bx+1, by+1), image.NewUniform(c), image.Point{}, draw.Src)
		}
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), "blocks.png"), im)
}

// Аудит 048 Ф2: рамка своего стикера — до двойного размера листа, а ячейка пути 1 считалась
// целиком (1200×1200 мм при 150 dpi — 387 МБ при холсте 25 МБ, на 300 dpi гигабайты: fatal
// OOM). Готовая ячейка не больше холста при любом повороте кратном 90° и любом зеркале,
// несколько стикеров сразу; тот же путь у фото места, вылезающего за лист, и у фона.
// Выделение за рендер остаётся порядка холста.
func TestRenderStickerHugeFrameBounded(t *testing.T) {
	path := blocksPNG(t)
	opt := Options{PageW: 300, PageH: 300, DPI: 150} // холст 600×300 мм
	loop := opt
	loop.DPI = 75 // перебор поворотов и зеркал — на лёгком холсте, память меряем ниже на 150
	cw, ch := CanvasSize(loop)
	canvas := image.Rect(0, 0, cw, ch).Inset(-clipSlack)
	big := layout.Rect{X: -300, Y: -450, W: 1200, H: 1200}
	var mu sync.Mutex
	var cells []image.Rectangle
	old := prepareFn
	t.Cleanup(func() { prepareFn = old })
	prepareFn = func(ph Photo, dpi int, clip image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
		cell, r, err := old(ph, dpi, clip)
		if cell != nil {
			mu.Lock()
			cells = append(cells, cell.Bounds(), r)
			mu.Unlock()
		}
		return cell, r, err
	}
	for _, rot := range []float64{0, 90, 180, -90} {
		for _, fl := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			cells = cells[:0]
			sp := Spread{Stickers: []Photo{
				{Cell: big, Img: big, Path: path, Rot: rot, FlipH: fl[0], FlipV: fl[1]},
				{Cell: layout.Rect{X: 100, Y: -500, W: 900, H: 900}, Img: layout.Rect{X: 100, Y: -500, W: 900, H: 900}, Path: path, Rot: rot, FlipH: fl[1], FlipV: fl[0], Z: 1},
			}, Photos: []Photo{{Cell: big, Img: big, Path: path, Rot: rot, FlipH: fl[0], FlipV: fl[1]}},
				Backdrops: []Backdrop{{Area: layout.Rect{W: 300, H: 300}, Box: big, Img: layout.Rect{W: 1200, H: 1200}, Rot: rot, Path: path}}}
			if _, err := Render(sp, loop); err != nil {
				t.Fatalf("rot %v flip %v: %v", rot, fl, err)
			}
			if len(cells) < 6 { // два стикера и фото места (фон под поворотом может уйти из области)
				t.Fatalf("rot %v flip %v: подготовлено %d ячеек", rot, fl, len(cells)/2)
			}
			for i := 0; i < len(cells); i += 2 {
				b, r := cells[i], cells[i+1]
				if b.Dx() > canvas.Dx() || b.Dy() > canvas.Dy() || !r.In(canvas) || b.Dx() != r.Dx() || b.Dy() != r.Dy() {
					t.Errorf("rot %v flip %v: ячейка %v в %v вне холста %v", rot, fl, b, r, canvas)
				}
			}
		}
	}
	// Оригинал в один пиксель, раздутый на 1200 мм: расширенное окно вышло бы за холст —
	// остаётся прежнее округление, ячейка всё равно не больше холста.
	px1 := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	px1.SetNRGBA(0, 0, color.NRGBA{stRed.R, stRed.G, stRed.B, 255})
	one := writePNG(t, filepath.Join(testkit.TempDir(t), "one.png"), px1)
	cells = cells[:0]
	img, err := Render(Spread{Stickers: []Photo{{Cell: big, Img: big, Path: one, FlipV: true}}}, loop)
	if err != nil || len(cells) != 2 || cells[0].Dx() > canvas.Dx() || cells[0].Dy() > canvas.Dy() {
		t.Fatalf("оригинал 1×1: ячейки %v, ошибка %v", cells, err)
	}
	if got := img.RGBAAt(cw/2, ch/2); !near(got, stRed, 3) {
		t.Errorf("оригинал 1×1: центр %v", got)
	}
	prepareFn = old
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	sp := Spread{Stickers: []Photo{{Cell: big, Img: big, Path: path, FlipH: true}}}
	if _, err := Render(sp, opt); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&b)
	// Холст 25 МБ + ячейка не больше его + расширенное окно; целая ячейка была бы 387 МБ.
	if got := (b.TotalAlloc - a.TotalAlloc) >> 20; got > 160 {
		t.Fatalf("выделено %d МБ при холсте 25 МБ", got)
	}
}

// Превью = печать: стикер, вылезающий за лист (слева, справа, сверху, снизу), с зеркалом и
// поворотом кратным 90° выходит тот же, что вырезка из холста, где он целиком внутри:
// направление зеркала и обрезки не перепутаны, картинка не съехала.
func TestRenderStickerClippedMatchesWhole(t *testing.T) {
	path := blocksPNG(t)
	opt := Options{PageW: 100, PageH: 100, DPI: 254}     // 10 px/мм, холст 200×100 мм
	wide := Options{PageW: 150, PageH: 200, DPI: 254}    // тот же лист, сдвинутый на (50, 50) мм
	const sh = 500                                       // 50 мм = 500 px
	boxes := []layout.Rect{{X: -15, Y: 5, W: 40, H: 30}, // слева
		{X: 175, Y: 60, W: 40, H: 30}, // справа
		{X: 60, Y: -10, W: 40, H: 30}, // сверху
		{X: 120, Y: 85, W: 40, H: 30}} // снизу
	bg := [2]color.RGBA{bdGray, bdGray}
	for _, rot := range []float64{0, 90, 180, -90} {
		for _, fl := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			var cut, whole []Photo
			for i, b := range boxes {
				ph := Photo{Cell: b, Img: b, Path: path, Rot: rot, FlipH: fl[0], FlipV: fl[1], Z: i}
				cut = append(cut, ph)
				ph.Cell.X, ph.Cell.Y, ph.Img.X, ph.Img.Y = b.X+50, b.Y+50, b.X+50, b.Y+50
				whole = append(whole, ph)
			}
			got, err := Render(Spread{Bg: bg, Stickers: cut}, opt)
			if err != nil {
				t.Fatal(err)
			}
			full, err := Render(Spread{Bg: bg, Stickers: whole}, wide)
			if err != nil {
				t.Fatal(err)
			}
			bad, total := diffInked(got, full, sh)
			if total < 2000 || bad*1000 > total { // допуск — округление размера расширенного окна
				t.Errorf("rot %v flip %v: расходится %d из %d пикселей", rot, fl, bad, total)
			}
		}
	}
}

// diffInked — сколько пикселей got (из total, где что-то нарисовано) расходятся с full,
// сдвинутым на shift px; фон страниц не считается.
func diffInked(got, full *image.RGBA, shift int) (bad, total int) {
	for y := range got.Bounds().Dy() {
		for x := range got.Bounds().Dx() {
			g, w := got.RGBAAt(x, y), full.RGBAAt(x+shift, y+shift)
			if g == bdGray && w == bdGray {
				continue
			}
			total++
			if !near(g, w, 3) {
				bad++
			}
		}
	}
	return bad, total
}

// stripePNG — w×h px с полосами цвета по обеим осям: пикселям есть что смешивать при Lanczos.
func stripePNG(t *testing.T, name string, w, h int) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			im.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 3), uint8((x/5 + y/5) * 40), 255})
		}
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), name), im)
}

// Аудит 048, перепроверка: промежуточный буфер imaging.Resize (ширина результата × высота окна)
// при сильной анизотропии — сотни МБ при холсте в десятки (кадр в .salbum подделываем).
// Высокий узкий PNG в широкой низкой рамке и наоборот, несколько стикеров, фото места и
// обрезка по листу: выделение остаётся порядка холста.
func TestRenderStickerAnisotropyBounded(t *testing.T) {
	tall := stripePNG(t, "tall.png", 100, 20000)
	wide := stripePNG(t, "wide.png", 20000, 100)
	opt := Options{PageW: 600, PageH: 30, DPI: 300} // холст 7087×354 px, ~10 МБ
	rowFrame := layout.Rect{X: 0, Y: 10, W: 600, H: 5}
	colFrame := layout.Rect{X: 10, Y: 0, W: 5, H: 600}
	thin := layout.Rect{X: 0, Y: 20, W: 600, H: 4}
	over := layout.Rect{X: -300, Y: 10, W: 1200, H: 5}
	cases := []struct {
		name string
		sp   Spread
	}{
		{"высокий в низкой", Spread{Stickers: []Photo{{Cell: rowFrame, Img: rowFrame, Path: tall}}}},
		{"низкий в высокой (выходит за лист)", Spread{Stickers: []Photo{{Cell: colFrame, Img: colFrame, Path: wide}}}},
		{"несколько сразу, зеркало, поворот", Spread{Stickers: []Photo{
			{Cell: rowFrame, Img: rowFrame, Path: tall, FlipH: true},
			{Cell: thin, Img: thin, Path: tall, Rot: 180, Z: 1},
			{Cell: colFrame, Img: colFrame, Path: wide, Z: 2},
		}}},
		{"фото места", Spread{Photos: []Photo{{Cell: rowFrame, Img: rowFrame, Path: tall}}}},
		{"вылезает за лист", Spread{Stickers: []Photo{{Cell: over, Img: over, Path: tall}}}},
	}
	for _, c := range cases {
		runtime.GC()
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		img, err := Render(c.sp, opt)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		runtime.ReadMemStats(&b)
		// Холст 10 МБ + декод 8 МБ + ячейка; прежний буфер 7087×20000 был бы ~570 МБ.
		if got := (b.TotalAlloc - a.TotalAlloc) >> 20; got > 90 {
			t.Errorf("%s: выделено %d МБ при холсте 10 МБ", c.name, got)
		}
		if img.Bounds().Empty() {
			t.Errorf("%s: пустой холст", c.name)
		}
	}
}

// resizeBounded даёт ту же картинку, что прямой imaging.Resize: на умеренной анизотропии
// (рост по ширине и уменьшение по высоте — единственный случай со сменой порядка проходов)
// расхождение — только округление промежуточного 8-битного буфера; остальное — дословно.
func TestResizeBoundedMatchesPlain(t *testing.T) {
	cases := []struct {
		sw, sh, dw, dh int
		same           bool // порядок проходов прежний: байт в байт (картинка плавная, как фото)
	}{
		{120, 400, 200, 150, false}, // ширина растёт, высота падает: порядок меняется
		{150, 600, 220, 90, false},
		{300, 200, 150, 400, true}, // ширина падает, высота растёт
		{200, 200, 300, 500, true}, // обе растут
		{400, 300, 100, 50, true},  // обе падают
		{50, 50, 50, 50, true},     // без изменений
		{1, 1, 40, 30, true},       // оригинал в один пиксель
		{1, 500, 1000, 1, false},   // предел: ширина ×1000, высота в 1 px
	}
	for _, c := range cases {
		im := image.NewNRGBA(image.Rect(0, 0, c.sw, c.sh))
		for y := range c.sh {
			for x := range c.sw {
				im.SetNRGBA(x, y, color.NRGBA{uint8(40 + x*160/c.sw), uint8(40 + y*160/c.sh), uint8(100 + 50*math.Sin(float64(x+y)/9)), 255})
			}
		}
		want := imaging.Resize(im, c.dw, c.dh, imaging.Lanczos)
		got := resizeBounded(im, c.dw, c.dh)
		if got.Bounds() != want.Bounds() {
			t.Fatalf("%v: размер %v, ждали %v", c, got.Bounds(), want.Bounds())
		}
		worst := 0
		for i := range want.Pix {
			worst = max(worst, int(math.Abs(float64(got.Pix[i])-float64(want.Pix[i]))))
		}
		if c.same && worst != 0 || worst > 3 {
			t.Errorf("%v: наибольшее расхождение %d уровней", c, worst)
		}
	}
	// Нулевые размеры — как у imaging (пустая картинка), без паники.
	if got := resizeBounded(image.NewNRGBA(image.Rect(0, 0, 5, 5)), 0, 0); got.Bounds().Dx() != 0 {
		t.Errorf("0×0: %v", got.Bounds())
	}
}
