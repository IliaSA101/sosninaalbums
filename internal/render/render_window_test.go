package render

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"math/rand"
	"strconv"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// noise заполняет срез случайными байтами (сид задаёт вызывающий).
func noise(rng *rand.Rand, b []uint8) {
	for i := range b {
		b[i] = uint8(rng.Intn(256))
	}
}

// windowImages — картинки всех типов, которые отдаёт декодер или проект, 37×29 (нечётные стороны).
func windowImages() map[string]image.Image {
	rng := rand.New(rand.NewSource(91))
	r := image.Rect(0, 0, 37, 29)
	out := map[string]image.Image{}
	for name, sr := range map[string]image.YCbCrSubsampleRatio{
		"ycbcr420": image.YCbCrSubsampleRatio420, "ycbcr422": image.YCbCrSubsampleRatio422,
		"ycbcr444": image.YCbCrSubsampleRatio444, "ycbcr440": image.YCbCrSubsampleRatio440,
	} {
		im := image.NewYCbCr(r, sr)
		noise(rng, im.Y)
		noise(rng, im.Cb)
		noise(rng, im.Cr)
		out[name] = im
	}
	nrgba := image.NewNRGBA(r)
	noise(rng, nrgba.Pix)
	out["nrgba"] = nrgba
	rgba := image.NewRGBA(r)
	noise(rng, rgba.Pix)
	for i := 0; i < len(rgba.Pix); i += 4 { // предумноженный цвет: не больше альфы
		a := rgba.Pix[i+3]
		rgba.Pix[i], rgba.Pix[i+1], rgba.Pix[i+2] = min(rgba.Pix[i], a), min(rgba.Pix[i+1], a), min(rgba.Pix[i+2], a)
	}
	out["rgba"] = rgba
	gray := image.NewGray(r)
	noise(rng, gray.Pix)
	out["gray"] = gray
	n64 := image.NewNRGBA64(r)
	noise(rng, n64.Pix)
	out["nrgba64"] = n64
	cmyk := image.NewCMYK(r)
	noise(rng, cmyk.Pix)
	out["cmyk"] = cmyk
	pal := image.NewPaletted(r, color.Palette{color.NRGBA{10, 20, 30, 255}, color.NRGBA{200, 100, 50, 255}, color.NRGBA{0, 255, 0, 128}, color.NRGBA{255, 255, 255, 255}})
	for i := range pal.Pix {
		pal.Pix[i] = uint8(rng.Intn(4))
	}
	out["paletted"] = pal
	// Начало не в нуле: так устроены SubImage и некоторые декодеры.
	shifted := image.NewNRGBA(image.Rect(3, 5, 40, 34))
	noise(rng, shifted.Pix)
	out["nrgba-min35"] = shifted
	return out
}

func sameNRGBA(t *testing.T, name string, got, want *image.NRGBA) {
	t.Helper()
	if got.Bounds() != want.Bounds() || got.Stride != want.Stride || !bytes.Equal(got.Pix, want.Pix) {
		t.Errorf("%s: окно без копии даёт другие пиксели (%v против %v)", name, got.Bounds(), want.Bounds())
	}
}

// window == imaging.Crop для ресайза: все типы, окна с чётным и нечётным краем (у YCbCr 4:2:0 —
// общий отсчёт цветности), все порядки resizeBounded и «тот же размер» (Clone).
func TestWindowSameAsCrop(t *testing.T) {
	wins := []image.Rectangle{
		image.Rect(0, 0, 37, 29), image.Rect(1, 1, 20, 15), image.Rect(2, 2, 31, 25),
		image.Rect(3, 5, 36, 29), image.Rect(5, 3, 6, 4), image.Rect(4, 6, 30, 20), image.Rect(1, 0, 37, 28),
	}
	for name, src := range windowImages() {
		b := src.Bounds()
		for _, w := range wins {
			win := w.Add(b.Min)
			for _, sz := range []image.Point{
				{w.Dx() / 2, w.Dy() / 2}, {w.Dx() * 2, max(w.Dy()/2, 1)}, {max(w.Dx()/2, 1), w.Dy() * 2},
				{w.Dx(), w.Dy()}, {w.Dx() * 2, w.Dy() * 2}, {1, 1}, {w.Dx() + 1, w.Dy()},
			} {
				dw, dh := max(sz.X, 1), max(sz.Y, 1)
				got := resizeBounded(window(src, win), dw, dh)
				want := resizeBounded(imaging.Crop(src, win), dw, dh)
				sameNRGBA(t, name+" "+win.String()+"→"+strconv.Itoa(dw)+"x"+strconv.Itoa(dh), got, want)
			}
		}
	}
}

// Результат ресайза не делит память с оригиналом (инвариант window): правка ячейки на месте
// оригинал не трогает — и в коротком пути «размер не меняется».
func TestWindowResultDoesNotAliasSource(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	noise(rand.New(rand.NewSource(1)), src.Pix)
	orig := bytes.Clone(src.Pix)
	win := image.Rect(2, 2, 12, 12)
	for _, sz := range []int{10, 5, 20} {
		out := resizeBounded(window(src, win), sz, sz)
		for i := range out.Pix {
			out.Pix[i] ^= 0xFF
		}
	}
	if !bytes.Equal(src.Pix, orig) {
		t.Fatal("правка результата ресайза испортила оригинал")
	}
}

// resizeClipped: окно без копии даёт те же пиксели, что прежняя версия, — основная ветка
// (вырез с полем) и запасная (окно слишком велико).
func TestResizeClippedSameAsOld(t *testing.T) {
	img := layout.Rect{X: 0, Y: 0, W: 100, H: 75}
	type tc struct {
		w      layout.Rect
		dw, dh int
		clip   image.Rectangle
	}
	cases := []tc{
		{layout.Rect{X: 20, Y: 10, W: 30, H: 20}, 40, 30, image.Rect(0, 0, 40, 30)},
		{layout.Rect{X: 21.3, Y: 11.7, W: 31.1, H: 19.9}, 33, 21, image.Rect(0, 0, 33, 21)},
		{layout.Rect{X: 0, Y: 0, W: 100, H: 75}, 20, 15, image.Rect(0, 0, 20, 15)},
		{layout.Rect{X: 40, Y: 30, W: 10, H: 10}, 80, 80, image.Rect(0, 0, 80, 80)},
		{layout.Rect{X: 5, Y: 5, W: 90, H: 60}, 50, 30, image.Rect(0, 0, 3, 3)},   // запасная ветка
		{layout.Rect{X: -10, Y: 0, W: 5, H: 5}, 10, 10, image.Rect(0, 0, 10, 10)}, // вне кадра
	}
	for name, src := range windowImages() {
		for i, c := range cases {
			got := resizeClipped(src, c.w, img, c.dw, c.dh, c.clip)
			want := resizeClippedOld(src, c.w, img, c.dw, c.dh, c.clip)
			sameNRGBA(t, name+" случай "+strconv.Itoa(i), got, want)
		}
	}
}

// cropSource: ветка с уменьшением (окно крупнее печати вдвое) и без неё — как прежняя версия;
// padEdge и тон правят уже копию.
func TestCropSourceSameAsOld(t *testing.T) {
	dir := testkit.TempDir(t)
	plain := testkit.JPEG(t, dir, "big.jpg", 400, 300)
	orient := testkit.JPEGOriented(t, dir, "o6.jpg", 400, 300, 6)
	for _, path := range []string{plain, orient} {
		ph := Photo{Path: path, Img: layout.Rect{X: 0, Y: 0, W: 100, H: 75}}
		if path == orient {
			ph.Img = layout.Rect{X: 0, Y: 0, W: 75, H: 100}
		}
		// k — px/мм: 0,5 даёт уменьшение, 6 — окно меньше печати (ветка без Resize).
		for _, k := range []float64{0.5, 1.2, 6} {
			for _, covers := range []bool{true, false} {
				got, err := cropSource(context.Background(), ph, k, covers, 5, 7, 60, 50)
				if err != nil {
					t.Fatal(err)
				}
				want, err := cropSourceOld(context.Background(), ph, k, covers, 5, 7, 60, 50)
				if err != nil {
					t.Fatal(err)
				}
				if got == nil || want == nil {
					t.Fatalf("окно пусто: %v %v", got, want)
				}
				sameNRGBA(t, path+" k="+strconv.FormatFloat(k, 'f', -1, 64), got.im, want.im)
				if got.win != want.win || got.pad != want.pad || got.sw != want.sw || got.sh != want.sh {
					t.Errorf("%s k=%v: rotSrc %+v против %+v", path, k, got, want)
				}
			}
		}
	}
}

// Пиксели листа не зависят от числа слотов: 8 фото разных видов (JPEG, EXIF-ориентация, PNG
// с альфой, наклон, зеркало, форма места, тон), слоты 1, 3 и 6. CMYK-файл тестовой фабрике
// не сделать (заголовок без данных) — его тип покрыт в TestWindowSameAsCrop.
func TestRenderSlotsSamePixels(t *testing.T) {
	dir := testkit.TempDir(t)
	files := []string{
		testkit.JPEG(t, dir, "a.jpg", 640, 480),
		testkit.JPEGOriented(t, dir, "b.jpg", 640, 480, 6),
		testkit.JPEGOriented(t, dir, "c.jpg", 480, 640, 3),
		testkit.PNG(t, dir, "d.png", 300, 200, color.NRGBA{R: 200, G: 30, B: 30, A: 255}),
	}
	var photos []Photo
	for i := range 8 {
		cell := layout.Rect{X: float64(i%4) * 25, Y: float64(i/4) * 38, W: 24, H: 36}
		ph := coverPhoto(cell, 1.33, files[i%4])
		if i%4 == 1 { // после EXIF-поворота кадр стоит
			ph = coverPhoto(cell, 0.75, files[i%4])
		}
		ph.Z = i
		switch i {
		case 4:
			ph.ImgRot = 4.5
		case 5:
			ph.FlipH = true
		case 6:
			ph.Rot = 7
			ph.Outline = geom.Outline{Oval: true}
		case 7:
			ph.Tone = Tone{Gray: true}
		}
		photos = append(photos, ph)
	}
	sp := Spread{Photos: photos}
	opt := Options{PageW: 50, PageH: 76, DPI: 120}
	old := photoSlots
	t.Cleanup(func() { photoSlots = old })
	var ref []byte
	for _, n := range []int{1, 3, 6} {
		photoSlots = func() int { return n }
		img, err := Render(sp, opt)
		if err != nil {
			t.Fatal(err)
		}
		if ref == nil {
			ref = img.Pix
			continue
		}
		if !bytes.Equal(ref, img.Pix) {
			t.Fatalf("слотов %d: пиксели листа другие", n)
		}
	}
}

func TestPhotoSlotsRange(t *testing.T) {
	if n := photoSlots(); n < 3 || n > 6 {
		t.Fatalf("photoSlots() = %d, ждали 3..6", n)
	}
}

// BenchmarkRenderSpreadSlots — разворот 600×300 мм, 300 dpi, 4 фото по 24 Мп: слоты 3 (было) и
// 6. Вручную: -bench RenderSpreadSlots -benchtime 3x.
func BenchmarkRenderSpreadSlots(b *testing.B) {
	dir := testkit.TempDir(b)
	p := testkit.JPEG(b, dir, "big.jpg", 6000, 4000)
	var photos []Photo
	for i := range 4 {
		cell := layout.Rect{X: float64(i) * 150, Y: 0, W: 148, H: 296}
		photos = append(photos, coverPhoto(cell, 1.5, p))
	}
	sp := Spread{Photos: photos}
	opt := Options{PageW: 300, PageH: 300, DPI: 300}
	old := photoSlots
	b.Cleanup(func() { photoSlots = old })
	for _, n := range []int{3, 6} {
		b.Run("slots"+strconv.Itoa(n), func(b *testing.B) {
			photoSlots = func() int { return n }
			for range b.N {
				if _, err := Render(sp, opt); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWindowVsCrop — копия окна против SubImage на оригинале 24 Мп (YCbCr 4:2:0, как JPEG).
func BenchmarkWindowVsCrop(b *testing.B) {
	src := image.NewYCbCr(image.Rect(0, 0, 6000, 4000), image.YCbCrSubsampleRatio420)
	win := image.Rect(0, 0, 4000, 4000)
	b.Run("crop", func(b *testing.B) {
		for range b.N {
			resizeBounded(imaging.Crop(src, win), 1750, 1750)
		}
	})
	b.Run("window", func(b *testing.B) {
		for range b.N {
			resizeBounded(window(src, win), 1750, 1750)
		}
	})
}
