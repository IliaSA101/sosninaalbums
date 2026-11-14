package render

import (
	"image"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Форма места в экспорте (спека 044, design §5): белое фото на пурпурной странице — доля
// фото в пикселе равна G/255 (у пурпура G = 0), поэтому площадь и сглаживание видны точно.

const shapeDPI = 100

// photoArea — площадь фото на холсте, мм² (сумма долей G/255 по пикселям).
func photoArea(img *image.RGBA) float64 {
	var sum float64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum += float64(img.RGBAAt(x, y).G) / 255
		}
	}
	mm := 25.4 / shapeDPI
	return sum * mm * mm
}

// gAt — доля фото в пикселе точки (x, y) мм.
func gAt(img *image.RGBA, x, y float64) uint8 {
	return img.RGBAAt(px(x, shapeDPI), px(y, shapeDPI)).G
}

// edgeSoft — на краю формы есть промежуточные значения (сглаживание), а не только 0 и 255.
func edgeSoft(img *image.RGBA) bool {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if g := img.RGBAAt(x, y).G; g > 20 && g < 235 {
				n++
			}
		}
	}
	return n > 50
}

func nearArea(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01*want {
		t.Fatalf("%s: площадь фото %.1f мм², ждали %.1f (±1 %%)", what, got, want)
	}
}

// Круг: в углах описанного квадрата — цвет страницы, в центре — фото, край сглажен, доля
// фото в квадрате — π/4 ± 1 %.
func TestRenderOval(t *testing.T) {
	path := whiteJPEG(t)
	cell := layout.Rect{X: 10, Y: 10, W: 80, H: 80}
	o := geom.Outline{Oval: true}
	img := renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1, geom.Crop{S: 1}, o), Path: path, Outline: o}, magenta, shapeDPI)
	for _, c := range [][2]float64{{11, 11}, {89, 11}, {11, 89}, {89, 89}} {
		if g := gAt(img, c[0], c[1]); g != 0 {
			t.Fatalf("угол (%v, %v): G %d, ждали цвет страницы", c[0], c[1], g)
		}
	}
	if gAt(img, 50, 50) != 255 || gAt(img, 50, 11) != 255 || gAt(img, 11, 50) != 255 {
		t.Fatal("внутри круга не фото")
	}
	if !edgeSoft(img) {
		t.Fatal("край круга без сглаживания")
	}
	nearArea(t, "круг", photoArea(img), math.Pi/4*80*80)
	// Без формы — прежний прямоугольник целиком.
	rect := renderOn(t, Photo{Cell: cell, Img: cell, Path: path}, magenta, shapeDPI)
	if gAt(rect, 11, 11) != 255 {
		t.Fatal("прямоугольник: угол не фото")
	}
}

// Круг в повёрнутом месте с наклоном фото (путь 3) и овал в месте под 90° с наклоном
// (fold, путь 3 с прямой рамкой): форма поворачивается с рамкой, фото её закрывает.
func TestRenderOvalRotTilt(t *testing.T) {
	path := whiteJPEG(t)
	o := geom.Outline{Oval: true}
	cell := layout.Rect{X: 15, Y: 15, W: 70, H: 70}
	f := geom.Crop{S: 1, R: 12}
	img := renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, f, o), Path: path, Rot: 30, ImgRot: 12, Outline: o}, magenta, shapeDPI)
	nearArea(t, "круг под 30° с наклоном", photoArea(img), math.Pi*35*35)
	// Угол описанного квадрата, повёрнутого на 30°, — страница.
	cx, cy := turn(15+2, 15+2, 50, 50, 30)
	if gAt(img, cx, cy) != 0 || gAt(img, 50, 50) != 255 || !edgeSoft(img) {
		t.Fatalf("круг под 30°: угол G %d, центр G %d", gAt(img, cx, cy), gAt(img, 50, 50))
	}
	// Овал 60×40 в месте под 90°: на экране длинная ось вертикальна.
	cell = layout.Rect{X: 20, Y: 30, W: 60, H: 40}
	f = geom.Crop{S: 1, R: 7}
	img = renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, f, o), Path: path, Rot: 90, ImgRot: 7, Outline: o}, magenta, shapeDPI)
	nearArea(t, "овал под 90° с наклоном", photoArea(img), math.Pi*30*20)
	if gAt(img, 50, 50+27) != 255 || gAt(img, 50+27, 50) != 0 || gAt(img, 50+17, 50) != 255 {
		t.Fatalf("овал под 90°: по вертикали G %d, по горизонтали G %d / %d", gAt(img, 50, 77), gAt(img, 77, 50), gAt(img, 67, 50))
	}
	// Наклон при прямой рамке (без формы — путь 2) с формой идёт путём 3 и режется формой.
	cell = layout.Rect{X: 10, Y: 10, W: 80, H: 80}
	img = renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, geom.Crop{S: 1, R: 10}, o), Path: path, ImgRot: 10, Outline: o}, magenta, shapeDPI)
	nearArea(t, "круг с наклоном фото", photoArea(img), math.Pi*40*40)
	if gAt(img, 11, 11) != 0 {
		t.Fatal("наклон в круге: угол не страница")
	}
}

// «Целиком» в круге: фото целиком внутри круга, по центру; угол рамки — страница.
func TestRenderOvalContain(t *testing.T) {
	path := whiteJPEG(t)
	o := geom.Outline{Oval: true}
	cell := layout.Rect{X: 10, Y: 10, W: 80, H: 80}
	in := geom.ContainImageIn(cell, 1.5, 0, o)
	img := renderOn(t, Photo{Cell: cell, Img: in, Path: path, Outline: o}, magenta, shapeDPI)
	nearArea(t, "«Целиком» в круге", photoArea(img), in.W*in.H)
	if gAt(img, 11, 50) != 0 || gAt(img, 50, 50) != 255 {
		t.Fatal("«Целиком»: край рамки или центр")
	}
	// С наклоном (путь 3, край фото внутри маски) — то же.
	in = geom.ContainImageIn(cell, 1.5, 20, o)
	img = renderOn(t, Photo{Cell: cell, Img: in, Path: path, ImgRot: 20, Outline: o}, magenta, shapeDPI)
	nearArea(t, "«Целиком» в круге с наклоном", photoArea(img), in.W*in.H)
}

// Скругление 25 %: площадь — рамка без четырёх уголков (4 − π)·ρ²; угол — страница, середина
// края — фото. И путём 1, и в повёрнутом месте.
func TestRenderRoundCorners(t *testing.T) {
	path := whiteJPEG(t)
	cell := layout.Rect{X: 10, Y: 20, W: 80, H: 60}
	r := 0.25 * 60
	o := geom.Outline{Radius: r}
	want := 80*60 - (4-math.Pi)*r*r
	img := renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, geom.Crop{S: 1}, o), Path: path, Outline: o}, magenta, shapeDPI)
	nearArea(t, "скругление", photoArea(img), want)
	if gAt(img, 11, 21) != 0 || gAt(img, 50, 21) != 255 || gAt(img, 11, 50) != 255 || !edgeSoft(img) {
		t.Fatal("скругление: угол или середины краёв")
	}
	img = renderOn(t, Photo{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, geom.Crop{S: 1, R: 5}, o), Path: path, Rot: 20, ImgRot: 5, Outline: o}, magenta, shapeDPI)
	nearArea(t, "скругление под 20°", photoArea(img), want)
}

// Маска: эллипс и скругление по формуле площади (до 0,2 %), повёрнутая форма — та же площадь.
func TestShapeMaskArea(t *testing.T) {
	cell := layout.Rect{X: 3, Y: 4, W: 120, H: 70}
	k := 300 / 25.4
	r := image.Rect(0, 0, int(140*k), int(90*k))
	for _, c := range []struct {
		o    geom.Outline
		rot  float64
		want float64
	}{
		{geom.Outline{Oval: true}, 0, math.Pi * 60 * 35},
		{geom.Outline{Oval: true}, 17, math.Pi * 60 * 35},
		{geom.Outline{Radius: 20}, 0, 120*70 - (4-math.Pi)*400},
		{geom.Outline{Radius: 99}, 0, 120*70 - (4-math.Pi)*35*35}, // радиус зажат половиной стороны
	} {
		m := shapeMask(r, layout.Rect{X: cell.X + 10, Y: cell.Y + 10, W: cell.W, H: cell.H}, c.rot, c.o, k)
		var sum float64
		for _, a := range m.Pix {
			sum += float64(a) / 255
		}
		if got := sum / k / k; math.Abs(got-c.want) > 0.002*c.want {
			t.Errorf("%+v под %v°: %.1f мм², ждали %.1f", c.o, c.rot, got, c.want)
		}
	}
}

// Бюджет маски (design §5): круг 300 мм при 300 dpi — не дольше прямоугольника × 1,15.
func benchShape(b *testing.B, o geom.Outline) {
	path := testkit.JPEG(b, testkit.TempDir(b), "big.jpg", 6000, 4000)
	cell := layout.Rect{W: 300, H: 300}
	s := Spread{Photos: []Photo{{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, geom.Crop{S: 1}, o), Path: path, Outline: o}}}
	opt := Options{PageW: 300, PageH: 300, DPI: 300}
	b.ResetTimer()
	for range b.N {
		if _, err := Render(s, opt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderOval(b *testing.B) { benchShape(b, geom.Outline{Oval: true}) }
func BenchmarkRenderRect(b *testing.B) { benchShape(b, geom.Outline{}) }

// Только маска круга 300 мм при 300 dpi (3543² px).
func BenchmarkShapeMask300(b *testing.B) {
	k := 300 / 25.4
	cell := layout.Rect{W: 300, H: 300}
	r := image.Rect(0, 0, px(300, 300), px(300, 300))
	for range b.N {
		shapeMask(r, cell, 0, geom.Outline{Oval: true}, k)
	}
}
