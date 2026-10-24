package imgfile

import (
	"image"
	"image/color"
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
