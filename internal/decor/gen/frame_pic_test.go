package gen

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// frameImage — картинка-рамочка w×h: непрозрачная, окно задаёт hole (true — прозрачно).
func frameImage(w, h int, hole func(x, y int) bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if !hole(x, y) {
				img.SetNRGBA(x, y, color.NRGBA{120, 60, 30, 255})
			}
		}
	}
	return img
}

func rectHole(x0, y0, x1, y1 int) func(x, y int) bool {
	return func(x, y int) bool { return x >= x0 && x < x1 && y >= y0 && y < y1 }
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// ringHole — овальное кольцо: прозрачный эллипс с полуосями rx, ry в центре w×h.
func ringHole(w, h int, rx, ry float64) func(x, y int) bool {
	return func(x, y int) bool {
		dx, dy := (float64(x)+0.5-float64(w)/2)/rx, (float64(y)+0.5-float64(h)/2)/ry
		return dx*dx+dy*dy < 1
	}
}

// TestFrameWindow — разбор окна картинки: прямоугольное, овальное кольцо (срез = габарит
// внутреннего эллипса ± 0,2 %), открытое (уголки), сплошное, мелкое, у края (срез зажат), тень,
// кусок вне центра, большая (с уменьшением), пустая.
func TestFrameWindow(t *testing.T) {
	cross := func(x, y int) bool { return (x > 40 && x < 260) || (y > 40 && y < 260) } // уголки: окно достаёт до края
	thirds := ClampCut([4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3})
	shadow := frameImage(400, 400, rectHole(100, 100, 300, 300)) // тень внутрь рамки: альфа 200 — рамка, не окно
	for y := 100; y < 300; y++ {
		for x := 100; x < 112; x++ {
			shadow.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 200})
		}
	}
	for _, c := range []struct {
		name     string
		img      image.Image
		ok, open bool
		cut      [4]float64 // ожидаемый срез ± 0,002; на open — точно
	}{
		{"rect", frameImage(400, 600, rectHole(60, 90, 340, 510)), true, false, [4]float64{0.15, 0.15, 0.15, 0.15}},
		{"oval", frameImage(800, 1000, ringHole(800, 1000, 300, 400)), true, false, [4]float64{0.125, 0.1, 0.125, 0.1}},
		{"open", frameImage(300, 300, cross), true, true, thirds},
		{"solid", frameImage(200, 200, func(x, y int) bool { return false }), false, false, [4]float64{}},
		{"tiny4", frameImage(500, 500, rectHole(200, 200, 300, 300)), false, false, [4]float64{}}, // 4 % площади
		{"tiny6", frameImage(500, 500, rectHole(190, 190, 310, 310)), true, false, [4]float64{0.38, 0.38, 0.38, 0.38}},
		// Окно кончается на 53 % высоты (центр внутри): низ 47 % — табличка, срез 0,467 зажат в 0,45.
		{"lowEdge", frameImage(400, 600, rectHole(60, 60, 340, 320)), true, false, [4]float64{0.15, 0.1, 0.15, MaxCut}},
		{"shadow", shadow, true, false, [4]float64{0.28, 0.25, 0.25, 0.25}},
		{"big", frameImage(3000, 4000, rectHole(420, 560, 2580, 3440)), true, false, [4]float64{0.14, 0.14, 0.14, 0.14}},
		{"empty", image.NewNRGBA(image.Rect(0, 0, 0, 0)), false, false, [4]float64{}},
		{"one", image.NewNRGBA(image.Rect(0, 0, 1, 1)), true, true, thirds}, // один прозрачный пиксель касается края
		// Прозрачный клочок вне связи с центром окном не считается: габарит — только центральная часть.
		{"pieces", frameImage(300, 300, func(x, y int) bool { return rectHole(100, 100, 200, 200)(x, y) || rectHole(10, 10, 40, 40)(x, y) }), true, false, [4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}},
	} {
		cut, open, ok := FrameWindow(c.img)
		if ok != c.ok || open != c.open {
			t.Errorf("%s: ok %v open %v, ждали %v %v", c.name, ok, open, c.ok, c.open)
			continue
		}
		if !ok {
			continue
		}
		if ClampCut(cut) != cut {
			t.Errorf("%s: срез %v вне зажима", c.name, cut)
		}
		for i := range cut {
			if !near(cut[i], c.cut[i], 0.003) {
				t.Errorf("%s: срез %v, ждали %v", c.name, cut, c.cut)
				break
			}
		}
	}
}

// TestFrameWindowKinds — другие типы изображений идут через At и подкартинки.
func TestFrameWindowKinds(t *testing.T) {
	src := frameImage(200, 200, rectHole(50, 50, 150, 150))
	for name, img := range map[string]image.Image{"rgba": toRGBA(src), "shift": shifted(src)} {
		if cut, _, ok := FrameWindow(img); !ok || !near(cut[0], 0.25, 0.01) {
			t.Errorf("%s: cut %v ok %v", name, cut, ok)
		}
	}
	if _, _, ok := FrameWindow(image.NewGray(image.Rect(0, 0, 200, 200))); ok { // gray непрозрачный
		t.Error("gray: окно найдено")
	}
}

func toRGBA(src *image.NRGBA) *image.RGBA {
	out := image.NewRGBA(src.Rect)
	draw.Draw(out, out.Rect, src, image.Point{}, draw.Src)
	return out
}

// shifted — та же картинка с началом не в нуле (подкартинка).
func shifted(src *image.NRGBA) image.Image {
	big := image.NewNRGBA(image.Rect(0, 0, 260, 260))
	draw.Draw(big, image.Rect(30, 30, 230, 230), src, image.Point{}, draw.Src)
	return big.SubImage(image.Rect(30, 30, 230, 230))
}

// TestFrameWindowFast — разбор декодированной 4096² ≤ Limit(200 ms).
func TestFrameWindowFast(t *testing.T) {
	if testing.Short() || testkit.Race {
		t.Skip("замер времени: обычный прогон без -short и -race")
	}
	img := frameImage(4096, 4096, rectHole(600, 600, 3500, 3500))
	if best := testkit.Fastest(3, func() { FrameWindow(img) }); best > testkit.Limit(200*time.Millisecond) {
		t.Errorf("FrameWindow 4096²: %v", best)
	}
}

// FuzzFrameWindow — любой PNG ≤ 64 px: не падает, срез в границах зажима, маска в размере копии.
func FuzzFrameWindow(f *testing.F) {
	for _, img := range []image.Image{
		frameImage(40, 40, rectHole(10, 10, 30, 30)),
		frameImage(33, 17, func(x, y int) bool { return true }),
		frameImage(1, 1, func(x, y int) bool { return false }),
	} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width > 64 || cfg.Height > 64 {
			return
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		cut, _, ok := FrameWindow(img)
		if ok && ClampCut(cut) != cut {
			t.Fatalf("срез вне зажима: %v", cut)
		}
		m := WindowMask(img, cut)
		if m == nil || m.Rect.Dx() != cfg.Width || m.Rect.Dy() != cfg.Height {
			t.Fatalf("маска %v для %dx%d", m, cfg.Width, cfg.Height)
		}
	})
}

// TestWindowMask — маска окна: центр — 255, рамка — 0, край расширен на 1 px, срез обрезает.
func TestWindowMask(t *testing.T) {
	img := frameImage(200, 300, rectHole(40, 60, 160, 240))
	cut, _, ok := FrameWindow(img)
	if !ok {
		t.Fatal("нет окна")
	}
	m := WindowMask(img, cut)
	if m.Rect.Dx() != 200 || m.Rect.Dy() != 300 {
		t.Fatalf("размер маски %v", m.Rect)
	}
	at := func(x, y int) uint8 { return m.AlphaAt(x, y).A }
	if at(100, 150) != 255 || at(10, 10) != 0 || at(20, 150) != 0 {
		t.Error("центр или рамка")
	}
	// Расширение на 1 px: окно 40…159, маска 39…160, дальше — 0.
	if at(39, 150) != 255 || at(160, 150) != 255 || at(38, 150) != 0 || at(161, 150) != 0 || at(100, 59) != 255 || at(100, 58) != 0 {
		t.Error("край не расширен ровно на 1 px")
	}
	// Срез уже окна обрезает область: левые 25 % (50 px) — вне маски.
	m2 := WindowMask(img, [4]float64{0.25, 0, 0, 0})
	if m2.AlphaAt(30, 150).A != 0 || m2.AlphaAt(100, 150).A != 255 {
		t.Error("срез не обрезал маску")
	}
	// Мусорный срез зажимается: сверху и справа 0,45, слева и снизу — 0 (NaN) и 0,45 (+Inf).
	m3 := WindowMask(img, [4]float64{math.NaN(), 9, -1, math.Inf(1)})
	if m3.AlphaAt(100, 150).A != 255 || m3.AlphaAt(100, 130).A != 0 || m3.AlphaAt(100, 170).A != 0 {
		t.Error("мусорный срез не зажат в 0…0,45")
	}
	// Окна нет — пустая маска; картинки нет — nil.
	solid := WindowMask(frameImage(50, 50, func(x, y int) bool { return false }), [4]float64{})
	if solid == nil || solid.Rect.Dx() != 50 {
		t.Fatal("сплошная картинка: нужна пустая маска")
	}
	for _, v := range solid.Pix {
		if v != 0 {
			t.Fatal("сплошная картинка: маска не пуста")
		}
	}
	if WindowMask(image.NewNRGBA(image.Rect(0, 0, 0, 0)), [4]float64{}) != nil {
		t.Error("пустая картинка: ждали nil")
	}
	// Большая: маска на копии ≤ 1024.
	if m := WindowMask(frameImage(3000, 2000, rectHole(400, 300, 2600, 1700)), [4]float64{0.13, 0.15, 0.13, 0.15}); m.Rect.Dx() != 1024 || m.Rect.Dy() != 683 {
		t.Errorf("копия %v", m.Rect)
	}
}

// TestNine — 9-срез: тот же размер — копия; углы 1:1, стороны и середина тянутся; крайние случаи.
func TestNine(t *testing.T) {
	// Источник 60×40: углы 10×8 разных цветов, стороны и середина — свои.
	src := image.NewRGBA(image.Rect(0, 0, 60, 40))
	paintRect := func(r image.Rectangle, c color.RGBA) { draw.Draw(src, r, image.NewUniform(c), image.Point{}, draw.Src) }
	cols := [9]color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}, {0, 0, 0, 0}, {255, 0, 255, 255}, {0, 255, 255, 255}, {128, 128, 128, 255}, {255, 128, 0, 255}}
	xs, ys := [4]int{0, 10, 50, 60}, [4]int{0, 8, 32, 40}
	for j := range 3 {
		for i := range 3 {
			paintRect(image.Rect(xs[i], ys[j], xs[i+1], ys[j+1]), cols[j*3+i])
		}
	}
	sl := [4]int{10, 8, 10, 8}

	same := image.NewRGBA(image.Rect(0, 0, 60, 40))
	Nine(same, same.Rect, src, sl)
	if !bytes.Equal(same.Pix, src.Pix) {
		t.Error("тот же размер — не копия")
	}

	dst := image.NewRGBA(image.Rect(0, 0, 200, 120))
	Nine(dst, dst.Rect, src, sl)
	at := func(x, y int) color.RGBA { return dst.RGBAAt(x, y) }
	for _, c := range []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, cols[0]}, {199, 0, cols[2]}, {0, 119, cols[6]}, {199, 119, cols[8]}, // углы
		{9, 7, cols[0]}, {190, 7, cols[2]}, {9, 112, cols[6]}, {190, 112, cols[8]},
		{100, 3, cols[1]}, {100, 116, cols[7]}, {4, 60, cols[3]}, {195, 60, cols[5]}, // стороны
	} {
		if got := at(c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) = %v, ждали %v", c.x, c.y, got, c.want)
		}
	}
	if at(100, 60).A != 0 {
		t.Errorf("середина прозрачная, а %v", at(100, 60))
	}
	// Углы ровно 10×8 px: граница на 10 и 190.
	if at(10, 3) != cols[1] || at(189, 3) != cols[1] || at(190, 3) != cols[2] || at(9, 3) != cols[0] {
		t.Error("углы не 1:1")
	}

	// dr меньше суммы толщин: углы сжимаются, не падает, края на месте.
	tiny := image.NewRGBA(image.Rect(0, 0, 12, 10))
	Nine(tiny, tiny.Rect, src, sl)
	if tiny.RGBAAt(0, 0) != cols[0] || tiny.RGBAAt(11, 9) != cols[8] {
		t.Error("сжатый 9-срез: углы не на месте")
	}
	// dr со смещением и обрезкой холстом; нулевой срез; nil и пустое.
	off := image.NewRGBA(image.Rect(0, 0, 50, 50))
	Nine(off, image.Rect(-20, -20, 70, 70), src, sl)
	Nine(off, image.Rect(5, 5, 45, 45), src, [4]int{})
	Nine(off, image.Rect(5, 5, 45, 45), src, [4]int{-3, 99, 1e6, 0})
	Nine(nil, dst.Rect, src, sl)
	Nine(off, image.Rectangle{}, src, sl)
	Nine(off, off.Rect, image.NewRGBA(image.Rectangle{}), sl)
	// Источник не от нуля: подкартинка рисуется так же.
	sub := src.SubImage(image.Rect(0, 0, 60, 40))
	dst2 := image.NewRGBA(dst.Rect)
	Nine(dst2, dst2.Rect, sub, sl)
	if !bytes.Equal(dst.Pix, dst2.Pix) {
		t.Error("подкартинка рисуется иначе")
	}
}

// TestNineTo — толщины на месте отдельно (079-40): углы источника 10×8 растянуты до 25×20, граница
// угла и стороны — на 25 и 20, середина прозрачна; тот же размер, но другие толщины — не копия;
// dsl = sl — ровно Nine. Срез больше места сжимается, как у Nine.
func TestNineTo(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 60, 40))
	draw.Draw(src, src.Rect, image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	draw.Draw(src, image.Rect(10, 8, 50, 32), image.Transparent, image.Point{}, draw.Src)
	sl := [4]int{10, 8, 10, 8}
	dst := image.NewRGBA(image.Rect(0, 0, 200, 120))
	NineTo(dst, dst.Rect, src, sl, [4]int{25, 20, 25, 20})
	for _, c := range []struct {
		x, y int
		a    uint8
	}{{24, 60, 255}, {26, 60, 0}, {175, 60, 255}, {173, 60, 0}, {100, 19, 255}, {100, 21, 0}, {100, 100, 255}, {100, 60, 0}} {
		if got := dst.RGBAAt(c.x, c.y).A; got != c.a {
			t.Errorf("(%d,%d): альфа %d, ждали %d", c.x, c.y, got, c.a)
		}
	}
	same := image.NewRGBA(image.Rect(0, 0, 60, 40))
	NineTo(same, same.Rect, src, sl, [4]int{15, 8, 15, 8})
	if same.RGBAAt(12, 20).A != 255 || bytes.Equal(same.Pix, src.Pix) {
		t.Error("тот же размер, другие толщины: угол не растянут")
	}
	a, b := image.NewRGBA(dst.Rect), image.NewRGBA(dst.Rect)
	Nine(a, a.Rect, src, sl)
	NineTo(b, b.Rect, src, sl, sl)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("NineTo(sl, sl) не равен Nine")
	}
	tiny := image.NewRGBA(image.Rect(0, 0, 12, 10))
	NineTo(tiny, tiny.Rect, src, sl, [4]int{100, 100, 100, 100})
	if tiny.RGBAAt(0, 0).A != 255 || tiny.RGBAAt(11, 9).A != 255 {
		t.Error("толщины больше места: углы не на месте")
	}
}

func BenchmarkFrameWindow4096(b *testing.B) {
	img := frameImage(4096, 4096, rectHole(600, 600, 3500, 3500))
	for range b.N {
		FrameWindow(img)
	}
}

func BenchmarkWindowMask4096(b *testing.B) {
	img := frameImage(4096, 4096, rectHole(600, 600, 3500, 3500))
	for range b.N {
		WindowMask(img, [4]float64{0.15, 0.15, 0.15, 0.15})
	}
}

// BenchmarkNine — рамка 100×150 мм при 300 dpi из картинки 3000×4000 в масштабе k.
func BenchmarkNine(b *testing.B) {
	src := toRGBA(frameImage(1181, 1772, rectHole(170, 250, 1010, 1520)))
	dst := image.NewRGBA(image.Rect(0, 0, 1400, 2000)) // другая пропорция — стороны тянутся
	b.ResetTimer()
	for range b.N {
		Nine(dst, dst.Rect, src, [4]int{170, 250, 171, 252})
	}
}
