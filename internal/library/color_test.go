package library

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // как в cacheKey
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// solidJPEG пишет однотонный JPEG с сегментами segs сразу после SOI.
func solidJPEG(t *testing.T, dir, name string, w, h int, c color.RGBA, segs ...[]byte) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	out := append([]byte{}, data[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, append(out, data[2:]...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func within(a, b uint8, tol int) bool { return int(a)-int(b) <= tol && int(b)-int(a) <= tol }

// Adobe-фото пересчитывается в sRGB прямо при декоде: (200,100,50) → (227,100,42) по LittleCMS.
// Фото без профиля и с sRGB не трогается. Бюджет занят 2n и возвращается целиком.
func TestOpenOrientedConvertsAdobe(t *testing.T) {
	dir := testkit.TempDir(t)
	src := color.RGBA{200, 100, 50, 255}
	adobe := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)")))
	srgb := testkit.ICCSegment(1, 1, testkit.SRGBProfile(testkit.DescV2("sRGB")))

	cases := []struct {
		name string
		segs [][]byte
		want color.RGBA
		cost int64
	}{
		{"adobe icc", [][]byte{adobe}, color.RGBA{227, 100, 42, 255}, 2 * 40 * 30},
		{"adobe по EXIF R03", [][]byte{testkit.ExifColor(false, 0, 0xFFFF, "R03")}, color.RGBA{227, 100, 42, 255}, 2 * 40 * 30},
		// 119: опорные значения LittleCMS (design §6); P3 и прочие матричные профили — тоже вдвое в бюджете
		{"p3 как у Xiaomi", [][]byte{testkit.ICCSegment(1, 1, testkit.ProfileXiaomiP3(true))}, color.RGBA{215, 93, 31, 255}, 2 * 40 * 30},
		{"prophoto", [][]byte{testkit.ICCSegment(1, 1, testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{117965.0 / 65536}, testkit.DescV2("ProPhoto RGB")))}, color.RGBA{255, 80, 47, 255}, 2 * 40 * 30},
		{"p3 с таблицей A2B0 не трогается", [][]byte{testkit.ICCSegment(1, 1, testkit.LUTProfile(testkit.XYZP3Exact, testkit.DescV2("P3 LUT")))}, src, 40 * 30},
		{"srgb", [][]byte{srgb}, src, 40 * 30},
		{"без профиля", nil, src, 40 * 30},
	}
	for _, c := range cases {
		p := solidJPEG(t, dir, "a.jpg", 40, 30, src, c.segs...)
		before := imgfile.DecodeBudget.Used()
		img, release, err := imgfile.OpenOriented(p)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := imgfile.DecodeBudget.Used() - before; got != c.cost {
			t.Errorf("%s: бюджет %d, ждали %d", c.name, got, c.cost)
		}
		r, g, b, _ := img.At(20, 15).RGBA()
		got := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
		if !within(got.R, c.want.R, 3) || !within(got.G, c.want.G, 3) || !within(got.B, c.want.B, 3) {
			t.Errorf("%s: пиксель %v, ждали %v ± 3", c.name, got, c.want)
		}
		release()
		if imgfile.DecodeBudget.Used() != before {
			t.Errorf("%s: бюджет не возвращён: %d", c.name, imgfile.DecodeBudget.Used()-before)
		}
	}
}

// Пересчёт работает и вместе с EXIF-поворотом.
func TestOpenOrientedAdobeRotated(t *testing.T) {
	dir := testkit.TempDir(t)
	adobe := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)")))
	p := solidJPEG(t, dir, "r.jpg", 40, 30, color.RGBA{200, 100, 50, 255}, testkit.ExifColor(false, 6, 0, ""), adobe)
	img, release, err := imgfile.OpenOriented(p)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if b := img.Bounds(); b.Dx() != 30 || b.Dy() != 40 {
		t.Fatalf("размер %v, ждали 30x40", b)
	}
	r, _, _, _ := img.At(15, 20).RGBA()
	if !within(uint8(r>>8), 227, 3) {
		t.Errorf("красный %d, ждали 227 ± 3", r>>8)
	}
}

// У Adobe-фото ключ кэша другой (старые блёклые копии не подхватятся),
// у остальных — прежний: кэш уже созданных миниатюр сохраняется.
func TestCacheKeyColor(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	old := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d", p, st.Size(), st.ModTime().UnixNano())) //nolint:gosec // как в cacheKey
	want := hex.EncodeToString(old[:10])

	for _, c := range []imgfile.Color{imgfile.ColorNone, imgfile.ColorSRGB, imgfile.ColorOther} {
		if got, _ := cacheKey(p, c); got != want {
			t.Errorf("цвет %q: ключ %s, ждали прежний %s", c, got, want)
		}
	}
	adobe, err := cacheKey(p, imgfile.ColorAdobe)
	if err != nil || adobe == want {
		t.Errorf("у Adobe ключ должен отличаться: %s (%v)", adobe, err)
	}
	if again, _ := cacheKey(p, imgfile.ColorAdobe); again != adobe {
		t.Error("ключ нестабилен")
	}
	// 119: p3 и icc — соль «icc1», у adobe остаётся «srgb1» (эталоны и кэш не меняются)
	salted := func(salt string) string {
		sum := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d%s", p, st.Size(), st.ModTime().UnixNano(), salt)) //nolint:gosec // как в cacheKey
		return hex.EncodeToString(sum[:10])
	}
	if adobe != salted("|srgb1") {
		t.Errorf("ключ Adobe изменился: %s, ждали %s", adobe, salted("|srgb1"))
	}
	for _, c := range []imgfile.Color{imgfile.ColorP3, imgfile.ColorICC} {
		got, err := cacheKey(p, c)
		if err != nil || got != salted("|icc1") || got == want || got == adobe {
			t.Errorf("цвет %q: ключ %s (%v), ждали %s", c, got, err, salted("|icc1"))
		}
	}
}

// Профиль PNG и WebP (125): лента, ReadInfo и сканер видят класс по содержимому, а не по расширению;
// ключ кэша PNG с матричным профилем меняется (соль по классу), у PNG без профиля и с «other» остаётся прежним.
func TestReadHeadColorPNGWebP(t *testing.T) {
	dir := testkit.TempDir(t)
	desc := testkit.DescV2
	img := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	p3 := testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, desc("Display P3"))
	pro := testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, desc("ProPhoto RGB"))
	adobe := testkit.MatrixProfile(testkit.XYZAdobe, 2.2, desc("Adobe RGB (1998)"))
	webp, err := os.ReadFile(filepath.Join("..", "imgfile", "testdata", "lossy16.webp"))
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		path    string
		color   imgfile.Color
		profile string
	}{
		{testkit.PNGWithICC(t, dir, "p3.png", img, p3), imgfile.ColorP3, "Display P3"},
		{testkit.PNGWithICC(t, dir, "xiaomi.png", img, testkit.ProfileXiaomiP3(true)), imgfile.ColorP3, testkit.XiaomiP3Name},
		{testkit.PNGWithICC(t, dir, "pro.png", img, pro), imgfile.ColorICC, "ProPhoto RGB"},
		{testkit.PNGWithICC(t, dir, "adobe.png", img, adobe), imgfile.ColorAdobe, "Adobe RGB (1998)"},
		{testkit.PNGWithICC(t, dir, "srgb.png", img, testkit.SRGBProfile(desc("sRGB"))), imgfile.ColorSRGB, "sRGB"},
		{testkit.PNGWithICC(t, dir, "lut.png", img, testkit.LUTProfile(testkit.XYZP3, desc("P3 LUT"))), imgfile.ColorOther, "P3 LUT"},
		{testkit.PNGWithICC(t, dir, "bad.png", img, []byte("мусор")), imgfile.ColorOther, "неизвестный профиль"},
		{testkit.PNG(t, dir, "plain.png", 16, 12, color.NRGBA{200, 10, 30, 255}), imgfile.ColorNone, ""},
		{write("webp-under.jpg", testkit.WebPWithICC(webp, p3, true)), imgfile.ColorP3, "Display P3"},
		{write("webp-under.png", testkit.WebPWithICC(webp, pro, true)), imgfile.ColorICC, "ProPhoto RGB"},
		{write("webp-noflag.jpg", testkit.WebPWithICC(webp, p3, false)), imgfile.ColorNone, ""},
	}
	for _, c := range cases {
		ph, err := ReadInfo(c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if ph.Color != c.color || ph.Profile != c.profile {
			t.Errorf("%s: %q %q, ждали %q %q", filepath.Base(c.path), ph.Color, ph.Profile, c.color, c.profile)
		}
	}
	// Сканер той же папки отдаёт тот же класс (PNG и WebP под .png; .webp по расширению не берётся).
	write("skipped.webp", testkit.WebPWithICC(webp, p3, true))
	photos, _ := Scan([]string{dir})
	seen := map[string]imgfile.Color{}
	for _, ph := range photos {
		seen[ph.Name] = ph.Color
	}
	if len(seen) != len(cases) || seen["p3.png"] != imgfile.ColorP3 || seen["webp-under.jpg"] != imgfile.ColorP3 || seen["plain.png"] != imgfile.ColorNone {
		t.Errorf("сканер: %v", seen)
	}
	if _, ok := seen["skipped.webp"]; ok {
		t.Error(".webp по расширению попал в ленту")
	}

	// Ключ кэша PNG: соль класса поверх «png1»; без профиля и other — прежний.
	p := cases[0].path
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	key := func(salt string) string {
		sum := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d%s|png1", p, st.Size(), st.ModTime().UnixNano(), salt)) //nolint:gosec // как в cacheKey
		return hex.EncodeToString(sum[:10])
	}
	for col, want := range map[imgfile.Color]string{
		imgfile.ColorNone: key(""), imgfile.ColorOther: key(""), imgfile.ColorSRGB: key(""),
		imgfile.ColorP3: key("|icc1"), imgfile.ColorICC: key("|icc1"), imgfile.ColorAdobe: key("|srgb1"),
	} {
		if got, err := cacheKey(p, col); err != nil || got != want {
			t.Errorf("PNG, класс %q: ключ %s (%v), ждали %s", col, got, err, want)
		}
	}
}
