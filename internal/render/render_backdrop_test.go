package render

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var (
	bdGray  = color.RGBA{200, 200, 200, 255}
	bdCream = color.RGBA{255, 240, 200, 255}
)

func renderBd(t *testing.T, s Spread, dpi int) *image.RGBA {
	t.Helper()
	img, err := Render(s, Options{PageW: 100, PageH: 100, DPI: dpi})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func quadsBd(t *testing.T) string {
	c := flipCols
	return quadrants(t, testkit.TempDir(t), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
}

func wantAt(t *testing.T, what string, img *image.RGBA, dpi int, x, y float64, want color.RGBA) {
	t.Helper()
	if got := img.RGBAAt(px(x, dpi), px(y, dpi)); !near(got, want, 14) {
		t.Errorf("%s: (%v, %v) мм — %v, ждали %v", what, x, y, got, want)
	}
}

// Фон лежит над цветом страниц и под местами; вне места виден сам.
func TestRenderBackdropUnderPhotos(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	area := layout.Rect{W: 200, H: 100}
	bd := Backdrop{Area: area, Box: area, Img: layout.Rect{Y: -50, W: 200, H: 200}, Path: q}
	cell := layout.Rect{X: 80, Y: 30, W: 40, H: 40}
	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{bd}, Photos: []Photo{coverPhoto(cell, 1, solid)}}
	img := renderBd(t, s, 127)
	wantAt(t, "красный фон", img, 127, 50, 25, c.red)
	wantAt(t, "зелёный фон", img, 127, 150, 25, c.grn)
	wantAt(t, "жёлтый фон", img, 127, 150, 75, c.yel)
	wantAt(t, "место поверх фона", img, 127, 100, 50, bdCream)
	// Без фона виден цвет страницы.
	s.Backdrops = nil
	wantAt(t, "без фона", renderBd(t, s, 127), 127, 50, 25, bdGray)
}

// Поворот 90°: четверти на своих местах, вне рамки — цвет страницы, вне области — не закрашено.
// dpi 40 — исходник больше нужного более чем вдвое (уменьшение Lanczos перед поворотом).
func TestRenderBackdropRot(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	left := layout.Rect{W: 100, H: 100}
	frame := framedPhoto(layout.Rect{W: 60, H: 40}, 1, 1, 0, 0, q).Img
	for _, dpi := range []int{127, 40} {
		bd := Backdrop{Area: left, Box: layout.Rect{X: 80, Y: 10, W: 60, H: 40}, Rot: 90, Img: frame, Path: q}
		out := Backdrop{Area: left, Box: layout.Rect{X: 130, Y: 10, W: 60, H: 40}, Rot: 90, Img: frame, Path: q}
		s := Spread{Bg: [2]color.RGBA{bdGray, bdCream}, Backdrops: []Backdrop{bd}}
		img := renderBd(t, s, dpi)
		wantAt(t, "красная", img, dpi, 70, 25, c.red)
		wantAt(t, "зелёная", img, dpi, 70, 55, c.grn)
		wantAt(t, "синяя", img, dpi, 50, 25, c.blu)
		wantAt(t, "жёлтая", img, dpi, 50, 55, c.yel)
		wantAt(t, "вне рамки справа", img, dpi, 90, 40, bdGray)
		wantAt(t, "вне рамки слева", img, dpi, 30, 30, bdGray)
		wantAt(t, "вне рамки сверху", img, dpi, 60, 5, bdGray)

		// Рамка торчит за область: внутри области рисуется, за ней — цвет правой страницы.
		s.Backdrops = []Backdrop{out}
		img = renderBd(t, s, dpi)
		wantAt(t, "в области", img, dpi, 95, 30, c.blu)
		wantAt(t, "за областью", img, dpi, 105, 30, bdCream)

		// Зеркало ↔ по горизонтали рамки: красная и зелёная меняются с синей и жёлтой по u.
		bd.FlipH = true
		s.Backdrops = []Backdrop{bd}
		img = renderBd(t, s, dpi)
		wantAt(t, "↔ красная", img, dpi, 70, 55, c.red)
		wantAt(t, "↔ зелёная", img, dpi, 70, 25, c.grn)
		wantAt(t, "↔ синяя", img, dpi, 50, 55, c.blu)
		wantAt(t, "↔ жёлтая", img, dpi, 50, 25, c.yel)
	}
}

// Без поворота зеркало идёт прежним путём prepare и не берётся ни откуда, кроме самого фона.
func TestRenderBackdropFlip(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	area := layout.Rect{W: 100, H: 100}
	bd := Backdrop{Area: area, Box: area, Img: area, Path: q}
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{bd}}
	img := renderBd(t, s, 127)
	wantAt(t, "красный слева", img, 127, 25, 25, c.red)
	bd.FlipH = true
	s.Backdrops = []Backdrop{bd}
	img = renderBd(t, s, 127)
	wantAt(t, "↔ красный справа", img, 127, 75, 25, c.red)
	wantAt(t, "↔ зелёный слева", img, 127, 25, 25, c.grn)
	bd.FlipV = true
	s.Backdrops = []Backdrop{bd}
	img = renderBd(t, s, 127)
	wantAt(t, "↔↕ красный справа внизу", img, 127, 75, 75, c.red)
}

// Рамка меньше области и сдвинута: фон обрезается рамкой, без поворота и с ним одинаково.
func TestRenderBackdropBoxClips(t *testing.T) {
	q := quadsBd(t)
	area := layout.Rect{W: 200, H: 100}
	box := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	img := framedPhoto(layout.Rect{W: 40, H: 40}, 1, 1, 0, 0, q).Img
	for _, rot := range []float64{0, 0.1} {
		s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{{Area: area, Box: box, Rot: rot, Img: img, Path: q}}}
		out := renderBd(t, s, 127)
		wantAt(t, "внутри рамки", out, 127, 30, 30, flipCols.red)
		wantAt(t, "справа от рамки", out, 127, 70, 40, bdGray)
		wantAt(t, "над рамкой", out, 127, 40, 10, bdGray)
	}
}

// BenchmarkRenderBackdropRot — разворот 600×300 мм при 300 dpi, фон 6000×4000 под углом 5°
// (design-backdrop §8: не больше 3 с на разворот).
func BenchmarkRenderBackdropRot(b *testing.B) {
	path := testkit.JPEG(b, testkit.TempDir(b), "big.jpg", 6000, 4000)
	area := layout.Rect{W: 600, H: 300}
	bd := Backdrop{Area: area, Box: area, Rot: 5, Img: framedPhoto(area, 1.5, 1, 0, 0, path).Img, Path: path}
	s := Spread{Backdrops: []Backdrop{bd}}
	opt := Options{PageW: 300, PageH: 300, DPI: 300}
	b.ResetTimer()
	for range b.N {
		if _, err := Render(s, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// Плашка (корешок обложки, 032) лежит над фоном и цветом страниц, под местами; за край холста
// не выходит, цвет страниц вне плашки виден.
func TestRenderPlate(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	area := layout.Rect{W: 200, H: 100}
	bd := Backdrop{Area: area, Box: area, Img: layout.Rect{Y: -50, W: 200, H: 200}, Path: q}
	solid := quadrants(t, testkit.TempDir(t), [4]color.RGBA{bdCream, bdCream, bdCream, bdCream})
	cell := layout.Rect{X: 90, Y: 30, W: 20, H: 20}
	plate := Plate{Rect: layout.Rect{X: 80, W: 40, H: 100}, Color: color.RGBA{10, 20, 30, 255}}
	edge := Plate{Rect: layout.Rect{X: 180, Y: 90, W: 60, H: 30}, Color: color.RGBA{200, 0, 200, 255}} // выходит за холст
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Backdrops: []Backdrop{bd}, Plates: []Plate{plate, edge},
		Photos: []Photo{coverPhoto(cell, 1, solid)}}
	img := renderBd(t, s, 127)
	wantAt(t, "фон левее плашки", img, 127, 50, 25, c.red)
	wantAt(t, "фон правее плашки", img, 127, 150, 25, c.grn)
	wantAt(t, "плашка поверх фона", img, 127, 85, 25, plate.Color)
	wantAt(t, "плашка внизу", img, 127, 115, 75, plate.Color)
	wantAt(t, "место поверх плашки", img, 127, 100, 40, bdCream)
	wantAt(t, "плашка у края", img, 127, 195, 95, edge.Color)
	// Без фона вокруг плашки — цвет страницы.
	s.Backdrops = nil
	wantAt(t, "цвет страницы", renderBd(t, s, 127), 127, 50, 25, bdGray)
}

// Повёрнутый фон с кадром из .salbum, не совпадающим с файлом (узкая полоса 4000×10 при
// рамке 100×2500 мм): окно оригинала нельзя растягивать до размера печати (~140 МБ), иначе
// подделанный файл просит гигабайты. Заодно ничего не пропадает: полоса остаётся на холсте.
func TestBackdropRotNeverUpscales(t *testing.T) {
	dir := testkit.TempDir(t)
	src := image.NewRGBA(image.Rect(0, 0, 4000, 10))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	path := filepath.Join(dir, "wide.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, src, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	opt := Options{PageW: 100, PageH: 300, DPI: 300}
	bd := Backdrop{Area: layout.Rect{W: 200, H: 300}, Box: layout.Rect{W: 100, H: 2500}, Rot: 1,
		Img: layout.Rect{W: 100, H: 2500}, Path: path}

	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	img, err := Render(Spread{Backdrops: []Backdrop{bd}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&b)
	// Холст (~33 МБ) + декод полосы; растянутое окно добавило бы ещё ~140 МБ.
	if got := (b.TotalAlloc - a.TotalAlloc) >> 20; got > 100 {
		t.Fatalf("выделено %d МБ: окно фона растянуто сверх исходника", got)
	}
	if img.Bounds().Empty() {
		t.Fatal("пустой холст")
	}
}
