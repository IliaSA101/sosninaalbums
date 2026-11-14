package imgfile

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var webpICCSamples = []string{"lossy16.webp", "lossless24x16.webp", "mem_alpha.webp"} // lossy, lossless, lossy с альфой

// webpSample — образец из testdata как есть.
func webpSample(t testing.TB, name string) []byte {
	t.Helper()
	return readFile(t, filepath.Join("testdata", name))
}

// Header у WebP берёт ICCP только при флаге ICC в VP8X (как Chromium): классы и имена те же, что у JPEG и PNG.
func TestHeaderWebPICC(t *testing.T) {
	for _, f := range webpICCSamples {
		src := webpSample(t, f)
		for _, c := range profileCases() {
			h := Header(bytes.NewReader(testkit.WebPWithICC(src, c.profile, true)))
			if h.Color != c.color || h.Profile != c.desc || h.Orient != 1 || h.XMP != nil {
				t.Errorf("%s, %s: %+v, ждали %q %q", f, c.name, h, c.color, c.desc)
			}
			if (c.color == ColorP3 || c.color == ColorICC || c.color == ColorAdobe) != (h.xf != nil) {
				t.Errorf("%s, %s: таблицы пересчёта %v", f, c.name, h.xf)
			}
		}
		checkWebPICCEdges(t, f, src)
	}
}

// checkWebPICCEdges — края профиля WebP: флаг, нечётная длина, предел, мусор, чтение без Seek.
func checkWebPICCEdges(t *testing.T, f string, src []byte) {
	t.Helper()
	p3 := testkit.ProfileXiaomiP3(true)
	class := func(data []byte) Head { return Header(bytes.NewReader(data)) }
	// Нет флага — чанк не действует; нет чанка при флаге — тоже.
	if h := class(testkit.WebPWithICC(src, p3, false)); h.Color != ColorNone || h.xf != nil {
		t.Errorf("%s: ICCP без флага: %+v", f, h)
	}
	if h := class(src); h.Color != ColorNone || h.Orient != 1 {
		t.Errorf("%s: без профиля: %+v", f, h)
	}
	// Нечётная длина: байт выравнивания не мешает ни профилю, ни следующему чанку (кадру).
	odd := testkit.WebPWithICC(src, append(append([]byte{}, p3...), 0), true)
	want, err := DimsOf(bytes.NewReader(src), 0)
	if err != nil {
		t.Fatal(err)
	}
	if h, d := class(odd), mustDims(t, odd); h.Color != ColorP3 || d != want {
		t.Errorf("%s: нечётная длина: %+v, размеры %+v (ждали %+v)", f, h, d, want)
	}
	// Профиль больше 1 МиБ — other без чтения; ровно в пределе читается.
	big := append(append([]byte{}, p3...), make([]byte, iccMaxBytes+1-len(p3))...)
	if h := class(testkit.WebPWithICC(src, big, true)); h.Color != ColorOther || h.Profile != iccUnknownName {
		t.Errorf("%s: ICCP больше предела: %+v", f, h)
	}
	if h := class(testkit.WebPWithICC(src, big[:iccMaxBytes], true)); h.Color != ColorP3 {
		t.Errorf("%s: ICCP ровно в пределе: %+v", f, h)
	}
	// Мусор, пустой чанк — other.
	for _, junk := range [][]byte{nil, []byte("не профиль"), p3[:100]} {
		if h := class(testkit.WebPWithICC(src, junk, true)); h.Color != ColorOther {
			t.Errorf("%s: профиль %d байт: %+v", f, len(junk), h)
		}
	}
	// Читатель без Seek: тот же ответ.
	if h := Header(iotest.OneByteReader(bytes.NewReader(testkit.WebPWithICC(src, p3, true)))); h.Color != ColorP3 {
		t.Errorf("%s: по байту: %+v", f, h)
	}
}

func mustDims(t *testing.T, data []byte) Dims {
	t.Helper()
	d, err := DimsOf(bytes.NewReader(data), 0)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Кривые WebP с ICCP: профиль не берётся, паники нет. Анимация, ICCP до VP8X, обрыв данных, второй ICCP.
func TestHeaderWebPICCBroken(t *testing.T) {
	p3 := testkit.ProfileXiaomiP3(true)
	srgb := testkit.SRGBProfile(testkit.DescV2("sRGB"))
	const flagICC = vp8xICC
	frame := vp8Chunk(16, 16)
	cases := map[string]struct {
		data []byte
		want Color
	}{
		"флаг ICC, чанка нет":      {riffWebP(vp8xChunk(flagICC, 16, 16), frame), ColorNone},
		"ICCP после кадра":         {riffWebP(vp8xChunk(flagICC, 16, 16), frame, chunk("ICCP", p3)), ColorNone},
		"ICCP без VP8X":            {riffWebP(chunk("ICCP", p3), frame), ColorNone},
		"анимация":                 {riffWebP(vp8xChunk(flagICC|vp8xAnimation, 16, 16), chunk("ICCP", p3), frame), ColorNone},
		"второй ICCP не действует": {riffWebP(vp8xChunk(flagICC, 16, 16), chunk("ICCP", srgb), chunk("ICCP", p3), frame), ColorSRGB},
		"чужой чанк до ICCP":       {riffWebP(vp8xChunk(flagICC, 16, 16), chunk("EXIF", []byte("xx")), chunk("ICCP", p3), frame), ColorP3},
		"ICCP оборван":             {riffWebP(vp8xChunk(flagICC, 16, 16), chunk("ICCP", p3))[:60], ColorNone},
		"обрыв в заголовке ICCP":   {riffWebP(vp8xChunk(flagICC, 16, 16), chunk("ICCP", p3))[:34], ColorNone},
		"только RIFF":              {[]byte("RIFF\x04\x00\x00\x00WEBP"), ColorNone},
	}
	for name, c := range cases {
		if h := Header(bytes.NewReader(c.data)); h.Color != c.want {
			t.Errorf("%s: %+v, ждали %q", name, h, c.want)
		}
	}
	// Длина ICCP 4 ГиБ−1 без данных: other без чтения.
	head := append(riffWebP(vp8xChunk(flagICC, 16, 16))[:30], "ICCP\xff\xff\xff\xff"...)
	if h := Header(io.MultiReader(bytes.NewReader(head), noRead{t})); h.Color != ColorOther {
		t.Errorf("ICCP 4 ГиБ: %+v", h)
	}
}

// HeaderShot у PNG и WebP — тот же Head, что у Header, и пустая съёмка.
func TestHeaderShotPNGWebP(t *testing.T) {
	p3 := testkit.ProfileXiaomiP3(true)
	for name, data := range map[string][]byte{
		"png":  testkit.PNGWithChunks(rgbPNG(t), iccp(p3)),
		"webp": testkit.WebPWithICC(webpSample(t, "lossy16.webp"), p3, true),
	} {
		h, s := HeaderShot(bytes.NewReader(data))
		want := Header(bytes.NewReader(data))
		if h.Color != ColorP3 || h.Color != want.Color || h.Profile != want.Profile || h.Orient != want.Orient || h.xf == nil {
			t.Errorf("%s: HeaderShot %+v, Header %+v", name, h, want)
		}
		if !s.Empty() {
			t.Errorf("%s: съёмка %+v", name, s)
		}
	}
	// ReadColor идёт по подписи: WebP под .jpg и PNG под .jpg — тот же класс.
	dir := testkit.TempDir(t)
	for name, data := range map[string][]byte{
		"a.jpg": testkit.WebPWithICC(webpSample(t, "lossless24x16.webp"), p3, true),
		"b.jpg": testkit.PNGWithChunks(rgbPNG(t), iccp(p3)),
		"c.png": testkit.WebPWithICC(webpSample(t, "lossy16.webp"), p3, true),
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if c, n := ReadColor(p); c != ColorP3 || n != testkit.XiaomiP3Name {
			t.Errorf("%s: ReadColor = %q %q", name, c, n)
		}
	}
}
