package colorspace

import (
	"bytes"
	"image"
	"image/draw"
	"math/rand"
	"os"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func abs(d int) int {
	if d < 0 {
		return -d
	}
	return d
}

// Опорные значения LittleCMS (Pillow ImageCms, относительная колориметрия,
// Adobe v2 → sRGB-v2-magic): вход, ожидаемый выход.
func TestAdobeToSRGBReference(t *testing.T) {
	cases := [][6]uint8{
		{0, 0, 0, 0, 0, 0}, {255, 255, 255, 255, 255, 255},
		{128, 128, 128, 129, 129, 129}, {64, 64, 64, 62, 62, 62}, {10, 10, 10, 3, 3, 3},
		{255, 0, 0, 255, 0, 0}, {0, 255, 0, 0, 255, 0}, {0, 0, 255, 0, 0, 255},
		{200, 100, 50, 227, 100, 42}, {50, 160, 90, 0, 161, 85},
		{230, 190, 170, 244, 191, 171}, {30, 60, 120, 0, 57, 123},
		{1, 2, 3, 0, 0, 0}, {250, 240, 10, 254, 240, 0},
	}
	for _, c := range cases {
		m := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		m.Pix[0], m.Pix[1], m.Pix[2], m.Pix[3] = c[0], c[1], c[2], 255
		got := AdobeToSRGB(m)
		if got.Pix[0] != c[3] || got.Pix[1] != c[4] || got.Pix[2] != c[5] || got.Pix[3] != 255 {
			t.Errorf("%v: получили %v, ждали %v", c[:3], got.Pix[:4], c[3:])
		}
	}
}

func TestAdobeToSRGBMatchesFloat(t *testing.T) {
	check := func(r, g, b uint8) {
		r1, g1, b1 := px(r, g, b)
		r2, g2, b2 := refPx(r, g, b)
		if abs(int(r1)-int(r2)) > 1 || abs(int(g1)-int(g2)) > 1 || abs(int(b1)-int(b2)) > 1 {
			t.Fatalf("(%d,%d,%d): LUT %d,%d,%d, float %d,%d,%d", r, g, b, r1, g1, b1, r2, g2, b2)
		}
	}
	for v := range 256 {
		check(uint8(v), uint8(v), uint8(v))
	}
	rnd := rand.New(rand.NewSource(1))
	for range 100_000 {
		check(uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)))
	}
}

// Быстрый путь YCbCr обязан давать то же, что обычное преобразование в NRGBA
// и пересчёт, в том числе для Rect не от нуля и SubImage.
func TestAdobeToSRGBYCbCr(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	for _, ratio := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422,
		image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio440,
	} {
		full := image.NewYCbCr(image.Rect(0, 0, 37, 29), ratio)
		for i := range full.Y {
			full.Y[i] = uint8(rnd.Intn(256))
		}
		for i := range full.Cb {
			full.Cb[i], full.Cr[i] = uint8(rnd.Intn(256)), uint8(rnd.Intn(256))
		}
		for _, src := range []*image.YCbCr{full, full.SubImage(image.Rect(5, 3, 30, 22)).(*image.YCbCr)} {
			b := src.Bounds()
			ref := image.NewNRGBA(b)
			draw.Draw(ref, b, src, b.Min, draw.Src)
			want := AdobeToSRGB(ref)
			got := AdobeToSRGB(src)
			if got.Rect != want.Rect {
				t.Fatalf("ratio %v: Rect %v, ждали %v", ratio, got.Rect, want.Rect)
			}
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if got.NRGBAAt(x, y) != want.NRGBAAt(x, y) {
						t.Fatalf("ratio %v (%d,%d): %v, ждали %v", ratio, x, y, got.NRGBAAt(x, y), want.NRGBAAt(x, y))
					}
				}
			}
		}
	}
}

// Чистые каналы, белый и чёрный: без переполнения, NRGBA пересчитывается на месте.
func TestAdobeToSRGBExtremes(t *testing.T) {
	in := [][3]uint8{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}}
	m := image.NewNRGBA(image.Rect(0, 0, len(in), 1))
	for i, c := range in {
		copy(m.Pix[i*4:], []uint8{c[0], c[1], c[2], 255})
	}
	got := AdobeToSRGB(m)
	if got != m {
		t.Error("NRGBA должно пересчитываться на месте")
	}
	// Все пять крайних цветов Adobe лежат на границе охвата sRGB или вне его.
	for i, c := range in {
		if p := got.Pix[i*4 : i*4+3]; p[0] != c[0] || p[1] != c[1] || p[2] != c[2] {
			t.Errorf("%v: получили %v", c, p)
		}
	}
}

// Не NRGBA и не YCbCr идут через draw, Rect сохраняется.
func TestAdobeToSRGBOtherType(t *testing.T) {
	src := image.NewGray(image.Rect(2, 2, 4, 4))
	for i := range src.Pix {
		src.Pix[i] = 128
	}
	got := AdobeToSRGB(src)
	if got.Rect != src.Rect || got.Pix[0] != 129 || got.Pix[3] != 255 {
		t.Errorf("Rect %v, пиксель %v", got.Rect, got.Pix[:4])
	}
}

func TestAdobeToSRGBEmpty(t *testing.T) {
	AdobeToSRGB(image.NewNRGBA(image.Rect(0, 0, 0, 0)))
	AdobeToSRGB(image.NewYCbCr(image.Rect(0, 0, 0, 0), image.YCbCrSubsampleRatio420))
}

// Замер 24 Мп (спека: ≈ 60 мс на 12 потоках); только под SA_BENCH.
func BenchmarkAdobeToSRGB24Mp(b *testing.B) {
	if os.Getenv("SA_BENCH") == "" {
		b.Skip("задайте SA_BENCH=1")
	}
	m := image.NewNRGBA(image.Rect(0, 0, 6000, 4000))
	for i := range m.Pix {
		m.Pix[i] = uint8(i * 7)
	}
	b.ResetTimer()
	for b.Loop() {
		AdobeToSRGB(m)
	}
}

// noiseRGBA — RGBA с шумом; alpha 255 везде, кроме явно заданных точек.
func noiseRGBA(r image.Rectangle, seed int64) *image.RGBA {
	m := image.NewRGBA(r)
	rnd := rand.New(rand.NewSource(seed))
	for i := range m.Pix {
		m.Pix[i] = uint8(rnd.Intn(256))
		if i%4 == 3 {
			m.Pix[i] = 255
		}
	}
	return m
}

// viaDraw — прежний путь: draw в новый NRGBA и пересчёт на месте; вход не меняется.
func viaDraw(x *Xform, src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	x.convertNRGBA(dst)
	return dst
}

// Непрозрачный RGBA пересчитывается на месте, в том же буфере, байт в байт как через draw;
// RGBA с альфой меньше 255 идёт через draw: новый буфер, вход цел.
func TestToSRGBRGBA(t *testing.T) {
	xs := map[string]*Xform{"adobe": Adobe, "p3": xfOf(t, testkit.ProfileXiaomiP3(true))}
	big := noiseRGBA(image.Rect(0, 0, 40, 33), 3)
	cases := map[string]*image.RGBA{
		"от нуля":        noiseRGBA(image.Rect(0, 0, 13, 9), 1),
		"смещённый Rect": noiseRGBA(image.Rect(7, 3, 20, 14), 2),
		"SubImage":       big.SubImage(image.Rect(5, 4, 29, 25)).(*image.RGBA),
		"SubImage узкий": big.SubImage(image.Rect(39, 32, 40, 33)).(*image.RGBA),
		"одна строка":    noiseRGBA(image.Rect(0, 0, 50, 1), 4),
		"один столбец":   noiseRGBA(image.Rect(0, 0, 1, 50), 5),
		"1×1":            noiseRGBA(image.Rect(0, 0, 1, 1), 6),
		"пустой":         image.NewRGBA(image.Rect(0, 0, 0, 0)),
		"пустой Rect":    image.NewRGBA(image.Rect(5, 5, 5, 9)),
	}
	if big.Stride <= 4*24 {
		t.Fatal("для SubImage нужен Stride больше 4·W")
	}
	for xn, x := range xs {
		for name, in := range cases {
			want := viaDraw(x, in)
			pix0, orig := in.Pix, bytes.Clone(in.Pix)
			out := x.ToSRGB(in)
			if out.Rect != in.Rect || out.Stride != in.Stride || len(out.Pix) != len(pix0) {
				t.Errorf("%s/%s: геометрия %v/%d, ждали %v/%d", xn, name, out.Rect, out.Stride, in.Rect, in.Stride)
				continue
			}
			if len(out.Pix) == 0 {
				continue
			}
			if &out.Pix[0] != &pix0[0] {
				t.Errorf("%s/%s: буфер новый, ждали тот же", xn, name)
			}
			sameRows(t, xn+"/"+name, out, want, in, orig)
		}
	}
	toSRGBRGBAAlpha(t)
}

// toSRGBRGBAAlpha — RGBA с альфой меньше 255 идёт через draw: новый буфер, вход цел.
func toSRGBRGBAAlpha(t *testing.T) {
	t.Helper()
	// Точки: первая и последняя строки, чтобы ловить любую долю rows; a = 0 и 254.
	for _, pt := range [][2]int{{0, 0}, {12, 8}, {6, 4}} {
		for _, a := range []uint8{0, 254} {
			in := noiseRGBA(image.Rect(0, 0, 13, 9), 7)
			in.Pix[pt[1]*in.Stride+pt[0]*4+3] = a
			orig := bytes.Clone(in.Pix)
			want := viaDraw(Adobe, in)
			out := Adobe.ToSRGB(in)
			if &out.Pix[0] == &in.Pix[0] {
				t.Errorf("%v a=%d: буфер общий, ждали draw-путь", pt, a)
			}
			if !bytes.Equal(in.Pix, orig) {
				t.Errorf("%v a=%d: вход изменён", pt, a)
			}
			if !bytes.Equal(out.Pix, want.Pix) {
				t.Errorf("%v a=%d: байты расходятся с draw", pt, a)
			}
		}
	}
}

// sameRows сверяет результат с draw-путём по строкам: у SubImage Stride шире строки,
// а хвост строки (вне Rect) должен остаться нетронутым.
func sameRows(t *testing.T, name string, out, want *image.NRGBA, in *image.RGBA, orig []byte) {
	t.Helper()
	w := in.Rect.Dx() * 4
	for y := range in.Rect.Dy() {
		got, ref := out.Pix[y*out.Stride:], want.Pix[y*want.Stride:]
		if !bytes.Equal(got[:w], ref[:w]) {
			t.Errorf("%s: строка %d расходится с draw-путём", name, y)
			return
		}
		end := min((y+1)*out.Stride, len(orig))
		if !bytes.Equal(out.Pix[y*out.Stride+w:end], orig[y*out.Stride+w:end]) {
			t.Errorf("%s: строка %d: изменены байты вне Rect", name, y)
			return
		}
	}
}

// Замер 48 Мп быстрого пути RGBA; только под SA_BENCH, порога нет (разброс 2,5 раза).
func BenchmarkToSRGBRGBA(b *testing.B) {
	if os.Getenv("SA_BENCH") == "" {
		b.Skip("задайте SA_BENCH=1")
	}
	m := noiseRGBA(image.Rect(0, 0, 8000, 6000), 9)
	b.ResetTimer()
	for b.Loop() {
		Adobe.ToSRGB(m)
	}
}
