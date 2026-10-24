package imgfile

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// nrgbaOf копирует картинку в NRGBA для попиксельного сравнения.
func nrgbaOf(img image.Image) *image.NRGBA {
	b := img.Bounds()
	m := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			m.SetNRGBA(x, y, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
		}
	}
	return m
}

func openNRGBA(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	img, release, err := OpenOriented(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	return nrgbaOf(img)
}

// OpenOriented пересчитывает JPEG с матричным профилем: результат = пиксели того же файла без профиля,
// пропущенные через Xform профиля. Профили sRGB и LUT картинку не меняют. Поворот EXIF не мешает.
func TestOpenOrientedICC(t *testing.T) {
	dir := testkit.TempDir(t)
	const w, h = 40, 30
	desc := testkit.DescV2
	cases := []struct {
		name    string
		profile []byte
		convert bool
	}{
		{"p3 (xiaomi)", testkit.ProfileXiaomiP3(true), true},
		{"p3 v2 таблица", testkit.TableProfile(testkit.XYZP3Exact, 1024, testkit.SRGBEOTF, desc("Display P3")), true},
		{"prophoto", testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, desc("ProPhoto")), true},
		{"линейный sRGB", testkit.LinearProfile(testkit.XYZLinearSRGB, desc("Linear")), true},
		{"adobe", testkit.MatrixProfile(testkit.XYZAdobe, 2.2, desc("Adobe RGB (1998)")), true},
		{"srgb", testkit.SRGBProfile(desc("sRGB")), false},
		{"lut", testkit.LUTProfile(testkit.XYZP3, desc("P3 LUT")), false},
	}
	for _, orient := range []int{0, 6} {
		var exif [][]byte
		if orient > 0 {
			exif = append(exif, testkit.ExifColor(true, orient, 0, ""))
		}
		plainPath := testkit.JPEGSegments(t, dir, "plain.jpg", w, h, exif...)
		plain := openNRGBA(t, plainPath)
		for _, c := range cases {
			segs := append(append([][]byte{}, exif...), testkit.ICCSegment(1, 1, c.profile))
			got := openNRGBA(t, testkit.JPEGSegments(t, dir, "icc.jpg", w, h, segs...))
			want := plain
			if c.convert {
				xf, err := colorspace.ParseICC(c.profile)
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
				if c.name == "adobe" {
					xf = colorspace.Adobe // Adobe — пресет, не разбор: байты прежние
				}
				want = xf.ToSRGB(nrgbaOf(plain))
			}
			if got.Rect != want.Rect || string(got.Pix) != string(want.Pix) {
				t.Errorf("%s (поворот %d): картинка не совпала с ожидаемой", c.name, orient)
			}
			if c.convert && string(got.Pix) == string(plain.Pix) {
				t.Errorf("%s (поворот %d): пересчёт не изменил ни одного пикселя", c.name, orient)
			}
		}
	}
}

// iccOpenCases — профили для сверки OpenOriented: пересчитываемые и нет.
func iccOpenCases() []struct {
	name    string
	profile []byte
	convert bool
} {
	desc := testkit.DescV2
	return []struct {
		name    string
		profile []byte
		convert bool
	}{
		{"p3 (xiaomi)", testkit.ProfileXiaomiP3(true), true},
		{"prophoto", testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, desc("ProPhoto")), true},
		{"adobe", testkit.MatrixProfile(testkit.XYZAdobe, 2.2, desc("Adobe RGB (1998)")), true},
		{"srgb", testkit.SRGBProfile(desc("sRGB")), false},
		{"lut", testkit.LUTProfile(testkit.XYZP3, desc("P3 LUT")), false},
		{"битый профиль", []byte("не профиль"), false},
	}
}

// wantConverted — ожидаемая картинка: plain, пропущенная через таблицы профиля (или без изменений).
func wantConverted(t *testing.T, name string, profile []byte, convert bool, plain *image.NRGBA) *image.NRGBA {
	t.Helper()
	if !convert {
		return plain
	}
	xf, err := colorspace.ParseICC(profile)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if name == "adobe" {
		xf = colorspace.Adobe // Adobe — пресет, не разбор: байты прежние
	}
	return xf.ToSRGB(nrgbaOf(plain))
}

// sameImage сравнивает границы и байты картинок.
func sameImage(t *testing.T, label string, got, want *image.NRGBA) {
	t.Helper()
	if got.Rect != want.Rect || string(got.Pix) != string(want.Pix) {
		t.Errorf("%s: картинка не совпала с ожидаемой", label)
	}
}

// OpenOriented пересчитывает PNG с iCCP так же, как JPEG: пиксели = тот же файл без профиля через Xform;
// sRGB, LUT и битый профиль картинку не меняют. Альфа та же. Типы PNG: RGB (*image.RGBA), с альфой
// (*image.NRGBA), палитра, ч/б (профиль RGB на нём игнорируется).
func TestOpenOrientedPNGICC(t *testing.T) {
	dir := testkit.TempDir(t)
	const w, h = 64, 48
	grad := func(alpha bool) *image.NRGBA {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				a := uint8(255)
				if alpha {
					a = uint8(40 + x*3) // не 0 и не 255: проверка, что альфа не трогается
				}
				m.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 5), uint8(255 - x*2 - y), a})
			}
		}
		return m
	}
	rgb := nrgbaToRGBA(grad(false)) // без альфы кодировщик пишет RGB, декодер отдаёт *image.RGBA
	if dec, err := png.Decode(bytes.NewReader(pngOf(t, rgb))); err != nil {
		t.Fatal(err)
	} else if _, ok := dec.(*image.RGBA); !ok {
		t.Fatalf("RGB PNG декодируется в %T, не в *image.RGBA", dec)
	}
	pal := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.NRGBA{200, 30, 20, 255}, color.NRGBA{10, 220, 90, 255}, color.NRGBA{30, 40, 230, 255}})
	for i := range pal.Pix {
		pal.Pix[i] = uint8(i % 3)
	}
	srcs := []struct {
		name string
		img  image.Image
	}{{"RGB", rgb}, {"RGBA с альфой", grad(true)}, {"палитра", pal}}
	for _, s := range srcs {
		plainPath := filepath.Join(dir, "plain.png")
		if err := os.WriteFile(plainPath, pngOf(t, s.img), 0o644); err != nil {
			t.Fatal(err)
		}
		plain := openNRGBA(t, plainPath)
		for _, c := range iccOpenCases() {
			got := openNRGBA(t, testkit.PNGWithICC(t, dir, "icc.png", s.img, c.profile))
			want := wantConverted(t, c.name, c.profile, c.convert, plain)
			sameImage(t, s.name+", "+c.name, got, want)
			if c.convert && string(got.Pix) == string(plain.Pix) {
				t.Errorf("%s, %s: пересчёт не изменил ни одного пикселя", s.name, c.name)
			}
			for i := 3; i < len(got.Pix); i += 4 {
				if got.Pix[i] != plain.Pix[i] {
					t.Fatalf("%s, %s: альфа изменилась в байте %d: %d вместо %d", s.name, c.name, i, got.Pix[i], plain.Pix[i])
				}
			}
		}
	}

	// Ч/б PNG с RGB-профилем: профиль не берётся, картинка как без него.
	gray := image.NewGray(image.Rect(0, 0, w, h))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i)
	}
	plainPath := filepath.Join(dir, "gray-plain.png")
	if err := os.WriteFile(plainPath, pngOf(t, gray), 0o644); err != nil {
		t.Fatal(err)
	}
	got := openNRGBA(t, testkit.PNGWithICC(t, dir, "gray.png", gray, testkit.ProfileXiaomiP3(true)))
	sameImage(t, "ч/б с RGB-профилем", got, openNRGBA(t, plainPath))
}

// nrgbaToRGBA — те же байты как *image.RGBA (для непрозрачной картинки совпадают).
func nrgbaToRGBA(m *image.NRGBA) *image.RGBA {
	return &image.RGBA{Pix: m.Pix, Stride: m.Stride, Rect: m.Rect}
}

// То же для WebP под любым именем: lossy, lossless и lossy с альфой (1024×1024, NYCbCrA → NRGBA через draw).
// Профиль берётся только при флаге в VP8X.
func TestOpenOrientedWebPICC(t *testing.T) {
	dir := testkit.TempDir(t)
	files := webpICCSamples
	if testkit.Lite() {
		files = files[:2] // 1024×1024 с альфой — в полном прогоне
	}
	for _, f := range files {
		src := webpSample(t, f)
		plainPath := filepath.Join(dir, "plain.webp")
		if err := os.WriteFile(plainPath, src, 0o644); err != nil {
			t.Fatal(err)
		}
		plain := openNRGBA(t, plainPath)
		for _, c := range iccOpenCases() {
			p := filepath.Join(dir, "icc.webp")
			if err := os.WriteFile(p, testkit.WebPWithICC(src, c.profile, true), 0o644); err != nil {
				t.Fatal(err)
			}
			got := openNRGBA(t, p)
			want := wantConverted(t, c.name, c.profile, c.convert, plain)
			sameImage(t, f+", "+c.name, got, want)
			for i := 3; i < len(got.Pix); i += 4 {
				if got.Pix[i] != plain.Pix[i] {
					t.Fatalf("%s, %s: альфа изменилась в байте %d", f, c.name, i)
				}
			}
		}
		// Флаг не стоит — чанк не действует.
		if err := os.WriteFile(filepath.Join(dir, "noflag.webp"), testkit.WebPWithICC(src, testkit.ProfileXiaomiP3(true), false), 0o644); err != nil {
			t.Fatal(err)
		}
		sameImage(t, f+", без флага", openNRGBA(t, filepath.Join(dir, "noflag.webp")), plain)
	}
}

// Header отдаёт таблицы пересчёта вместе с классом — их OpenOrientedCtx берёт с того же дескриптора, что и декод
func TestHeaderICCXform(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEGSegments(t, dir, "x.jpg", 20, 20, testkit.ICCSegment(1, 1, testkit.ProfileXiaomiP3(false)))
	f, err := OpenRegular(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // тест
	_, hd, err := checkOpened(f)
	if err != nil || hd.Color != ColorP3 || hd.xf == nil {
		t.Fatalf("заголовок: %+v, %v", hd, err)
	}
}

// Вес декода: p3 и icc вдвое (оригинал и NRGBA-копия), как adobe; srgb, other и без профиля — W×H.
func TestDecodeWeightICC(t *testing.T) {
	dir := testkit.TempDir(t)
	const w, h = 10, 6
	icc := func(p []byte) [][]byte { return [][]byte{testkit.ICCSegment(1, 1, p)} }
	cases := []struct {
		name string
		segs [][]byte
		want int64
	}{
		{"p3", icc(testkit.ProfileXiaomiP3(true)), 2 * w * h},
		{"icc", icc(testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, testkit.DescV2("PP"))), 2 * w * h},
		{"adobe", icc(testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("A"))), 2 * w * h},
		{"p3 с поворотом 6: одна лишняя копия, не две", [][]byte{testkit.ExifColor(true, 6, 0, ""), testkit.ICCSegment(1, 1, testkit.ProfileXiaomiP3(true))}, 2 * w * h},
		{"srgb", icc(testkit.SRGBProfile(testkit.DescV2("s"))), w * h},
		{"lut", icc(testkit.LUTProfile(testkit.XYZP3, testkit.DescV2("L"))), w * h},
		{"без профиля", nil, w * h},
	}
	for _, c := range cases {
		p := testkit.JPEGSegments(t, dir, "w.jpg", w, h, c.segs...)
		n, _, err := checkFile(p)
		if err != nil || n != c.want {
			t.Errorf("%s: вес %d, ждали %d (%v)", c.name, n, c.want, err)
			continue
		}
		before := DecodeBudget.Used()
		_, release, err := OpenOriented(p)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := DecodeBudget.Used() - before; got != c.want {
			t.Errorf("%s: в бюджете %d, ждали %d", c.name, got, c.want)
		}
		release()
	}
	decodeWeightPNGWebP(t, dir)
	for _, c := range []Color{ColorAdobe, ColorP3, ColorICC} {
		if !c.converts() {
			t.Errorf("%q должен пересчитываться", c)
		}
	}
	for _, c := range []Color{ColorNone, ColorSRGB, ColorOther} {
		if c.converts() {
			t.Errorf("%q не должен пересчитываться", c)
		}
	}
}

// decodeWeightPNGWebP — вес PNG и WebP с матричным профилем вдвое больше, чем без него; sRGB, LUT и битый
// профиль не добавляют. Бюджет при открытии равен весу.
func decodeWeightPNGWebP(t *testing.T, dir string) {
	t.Helper()
	const w, h = 10, 6
	p3, pro := testkit.ProfileXiaomiP3(true), testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, testkit.DescV2("PP"))
	lut, srgb := testkit.LUTProfile(testkit.XYZP3, testkit.DescV2("L")), testkit.SRGBProfile(testkit.DescV2("s"))
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	type tc struct {
		name string
		path string
		want int64
	}
	cases := []tc{
		{"png p3", testkit.PNGWithICC(t, dir, "w1.png", img, p3), 2 * w * h},
		{"png icc", testkit.PNGWithICC(t, dir, "w2.png", img, pro), 2 * w * h},
		{"png srgb", testkit.PNGWithICC(t, dir, "w3.png", img, srgb), w * h},
		{"png lut", testkit.PNGWithICC(t, dir, "w4.png", img, lut), w * h},
		{"png пустой профиль (other)", testkit.PNGWithICC(t, dir, "w5.png", img, nil), w * h},
	}
	const px = 1024 * 1024
	for _, f := range []struct {
		file string
		base int64
	}{{"mem_lossy.webp", px}, {"mem_truecolor.webp", 2 * px}, {"mem_alpha.webp", px * 9 / 4}} {
		src := webpSample(t, f.file)
		for name, d := range map[string]struct {
			prof []byte
			mult int64
		}{"p3": {p3, 2}, "prophoto": {pro, 2}, "srgb": {srgb, 1}, "lut": {lut, 1}} {
			p := filepath.Join(dir, name+"-"+f.file)
			if err := os.WriteFile(p, testkit.WebPWithICC(src, d.prof, true), 0o644); err != nil {
				t.Fatal(err)
			}
			cases = append(cases, tc{"webp " + f.file + " " + name, p, f.base * d.mult})
		}
	}
	for _, c := range cases {
		n, _, err := checkFile(c.path)
		if err != nil || n != c.want {
			t.Errorf("%s: вес %d, ждали %d (%v)", c.name, n, c.want, err)
			continue
		}
		before := DecodeBudget.Used()
		_, release, err := OpenOriented(c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := DecodeBudget.Used() - before; got != c.want {
			t.Errorf("%s: в бюджете %d, ждали %d", c.name, got, c.want)
		}
		release()
	}
}
