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
