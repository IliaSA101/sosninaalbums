package imgfile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// pngOf кодирует img в PNG; тип цвета выбирает кодировщик (RGBA без альфы — RGB, NRGBA — RGBA, Gray — ч/б).
func pngOf(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func rgbPNG(t testing.TB) []byte  { return pngOf(t, image.NewRGBA(image.Rect(0, 0, 8, 6))) }
func grayPNG(t testing.TB) []byte { return pngOf(t, image.NewGray(image.Rect(0, 0, 8, 6))) }

// iccp — чанк iCCP с профилем p; srgbChunk — чанк sRGB.
func iccp(p []byte) []byte { return testkit.PNGChunk("iCCP", testkit.PNGICCData(p)) }

var srgbChunk = testkit.PNGChunk("sRGB", []byte{0})

// iccpData — iCCP с заданными данными (ключевое слово, метод и поток собраны руками).
func iccpData(keyword string, method byte, stream []byte) []byte {
	return testkit.PNGChunk("iCCP", append(append([]byte(keyword+"\x00"), method), stream...))
}

// zlibOf сжимает p в поток zlib.
func zlibOf(p []byte) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(p)
	_ = w.Close()
	return z.Bytes()
}

// afterIDAT вставляет чанки перед IEND (то есть после IDAT).
func afterIDAT(png []byte, chunks ...[]byte) []byte {
	out := append([]byte{}, png[:len(png)-12]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, png[len(png)-12:]...)
}

// noRead — читатель, которого трогать нельзя: проверка «без чтения».
type noRead struct{ t *testing.T }

func (n noRead) Read([]byte) (int, error) {
	n.t.Error("прочитано больше, чем нужно: данные чанка не должны читаться")
	return 0, io.EOF
}

// profileCases — профили для таблиц: имя, байты, класс и имя из desc.
func profileCases() []struct {
	name    string
	profile []byte
	color   Color
	desc    string
} {
	d := testkit.DescV2
	return []struct {
		name    string
		profile []byte
		color   Color
		desc    string
	}{
		{"p3 v4 para (Xiaomi)", testkit.ProfileXiaomiP3(true), ColorP3, testkit.XiaomiP3Name},
		{"p3 v2", testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, d("Display P3")), ColorP3, "Display P3"},
		{"p3 без desc", testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, nil), ColorP3, ""},
		{"prophoto", testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, d("ProPhoto")), ColorICC, "ProPhoto"},
		{"adobe", testkit.MatrixProfile(testkit.XYZAdobe, 2.2, d("Adobe RGB (1998)")), ColorAdobe, "Adobe RGB (1998)"},
		{"srgb", testkit.SRGBProfile(d("sRGB")), ColorSRGB, "sRGB"},
		{"gray", testkit.BuildICC("GRAY", testkit.ICCTag{Sig: "desc", Data: d("Gray")}), ColorSRGB, "Gray"},
		{"lut", testkit.LUTProfile(testkit.XYZP3, d("P3 LUT")), ColorOther, "P3 LUT"},
	}
}

// Header у PNG берёт профиль iCCP: те же классы и имена, что у JPEG; ориентации, времени и XMP нет.
func TestHeaderPNGICC(t *testing.T) {
	for _, c := range profileCases() {
		for _, withSRGB := range []bool{false, true} {
			chunks := [][]byte{iccp(c.profile)}
			if withSRGB { // iCCP важнее sRGB (порядок Skia), в любом порядке
				chunks = [][]byte{srgbChunk, iccp(c.profile), srgbChunk}
			}
			h := Header(bytes.NewReader(testkit.PNGWithChunks(rgbPNG(t), chunks...)))
			if h.Color != c.color || h.Profile != c.desc || h.Orient != 1 || h.XMP != nil || h.Taken != 0 || h.Ours {
				t.Errorf("%s (sRGB рядом: %v): %+v, ждали %q %q", c.name, withSRGB, h, c.color, c.desc)
			}
			if (c.color == ColorP3 || c.color == ColorICC || c.color == ColorAdobe) != (h.xf != nil) {
				t.Errorf("%s: таблицы пересчёта %v", c.name, h.xf)
			}
		}
	}

	p3 := testkit.ProfileXiaomiP3(true)
	rgba := pngOf(t, image.NewNRGBA(image.Rect(0, 0, 8, 6))) // тип 6
	pal := pngOf(t, image.NewPaletted(image.Rect(0, 0, 8, 6), color.Palette{color.Black, color.White}))
	for name, base := range map[string][]byte{"RGBA": rgba, "палитра": pal} {
		if h := Header(bytes.NewReader(testkit.PNGWithChunks(base, iccp(p3)))); h.Color != ColorP3 {
			t.Errorf("%s: %+v", name, h)
		}
	}

	// Не по правилам — профиль не берётся.
	none := map[string][]byte{
		"ч/б с RGB-профилем":      testkit.PNGWithChunks(grayPNG(t), iccp(p3)),
		"iCCP после IDAT":         afterIDAT(rgbPNG(t), iccp(p3)),
		"iCCP после PLTE":         testkit.PNGWithChunks(pal, testkit.PNGChunk("PLTE", make([]byte, 6)), iccp(p3)),
		"только sRGB":             testkit.PNGWithChunks(rgbPNG(t), srgbChunk),
		"gAMA и cHRM":             testkit.PNGWithChunks(rgbPNG(t), testkit.PNGChunk("gAMA", make([]byte, 4)), testkit.PNGChunk("cHRM", make([]byte, 32))),
		"cICP":                    testkit.PNGWithChunks(rgbPNG(t), testkit.PNGChunk("cICP", []byte{12, 16, 0, 1}), srgbChunk),
		"без профиля":             rgbPNG(t),
		"первый чанк не IHDR":     append(append([]byte{}, rgbPNG(t)[:8]...), testkit.PNGChunk("tEXt", nil)...),
		"IHDR не 13 байт":         append(append([]byte{}, rgbPNG(t)[:8]...), testkit.PNGChunk("IHDR", make([]byte, 12))...),
		"подпись без чанков":      rgbPNG(t)[:12],
		"обрыв посреди IHDR":      rgbPNG(t)[:20],
		"обрыв перед iCCP":        testkit.PNGWithChunks(rgbPNG(t))[:33],
		"обрыв в заголовке чанка": append(testkit.PNGWithChunks(rgbPNG(t))[:33], 0, 0, 0),
	}
	for name, data := range none {
		if h := Header(bytes.NewReader(data)); h.Color != ColorNone || h.Profile != "" || h.xf != nil || h.Orient != 1 {
			t.Errorf("%s: %+v, ждали без профиля", name, h)
		}
	}

	// Второй iCCP не действует: остаётся первый (и это не P3 второго).
	two := testkit.PNGWithChunks(rgbPNG(t), iccp(testkit.SRGBProfile(testkit.DescV2("sRGB"))), iccp(p3))
	if h := Header(bytes.NewReader(two)); h.Color != ColorSRGB {
		t.Errorf("второй iCCP: %+v", h)
	}
	// Битый второй iCCP тоже не портит первый.
	two = testkit.PNGWithChunks(rgbPNG(t), iccp(p3), iccpData("x", 1, nil))
	if h := Header(bytes.NewReader(two)); h.Color != ColorP3 {
		t.Errorf("битый второй iCCP: %+v", h)
	}

	// Читатель без Seek и по байту (канал, сеть): те же ответы, что у bytes.Reader.
	data := testkit.PNGWithChunks(rgbPNG(t), testkit.PNGChunk("tEXt", []byte("Software\x00test")), iccp(p3))
	if h := Header(iotest.OneByteReader(bytes.NewReader(data))); h.Color != ColorP3 {
		t.Errorf("по байту: %+v", h)
	}
}

// Битый профиль — «!» («неизвестный профиль»), а не молчаливый sRGB; каждый случай — ColorOther.
func TestHeaderPNGICCBroken(t *testing.T) {
	p3 := testkit.ProfileXiaomiP3(true)
	good := zlibOf(p3)
	badCRC := iccp(p3)
	badCRC[len(badCRC)-1] ^= 0xFF
	longKey := strings.Repeat("k", 80)
	cases := map[string][]byte{
		"неверный CRC":           badCRC,
		"метод сжатия 1":         iccpData("ICC Profile", 1, good),
		"пустое ключевое слово":  iccpData("", 0, good),
		"ключевое слово 80 байт": iccpData(longKey, 0, good),
		"нет NUL":                    testkit.PNGChunk("iCCP", []byte("ICC Profile")),
		"пустые данные":              testkit.PNGChunk("iCCP", nil),
		"нет метода после NUL":       testkit.PNGChunk("iCCP", []byte("ICC Profile\x00")),
		"пустой поток":               iccpData("ICC Profile", 0, nil),
		"мусор вместо zlib":          iccpData("ICC Profile", 0, []byte("not a zlib stream at all")),
		"zlib со словарём (FDICT)":   iccpData("ICC Profile", 0, []byte{0x78, 0xBB, 0, 0, 0, 1, 0x03, 0, 0, 0, 0, 1}),
		"поток оборван":              iccpData("ICC Profile", 0, good[:len(good)/2]),
		"Adler-32 испорчен":          iccpData("ICC Profile", 0, append(append([]byte{}, good[:len(good)-1]...), good[len(good)-1]^0xFF)),
		"мусор внутри профиля":       iccpData("ICC Profile", 0, zlibOf([]byte("это не профиль ICC, а просто текст"))),
		"профиль короче заголовка":   iccpData("ICC Profile", 0, zlibOf(p3[:100])),
		"пустой профиль":             iccpData("ICC Profile", 0, zlibOf(nil)),
		"сжатый больше 1 МиБ":        testkit.PNGChunk("iCCP", append([]byte("ICC Profile\x00\x00"), make([]byte, iccMaxBytes)...)),
		"распакованный больше 1 МиБ": iccpData("ICC Profile", 0, zlibOf(make([]byte, iccMaxBytes+1))),
	}
	for name, chunk := range cases {
		h := Header(bytes.NewReader(testkit.PNGWithChunks(rgbPNG(t), chunk)))
		if h.Color != ColorOther || h.Profile != iccUnknownName || h.xf != nil {
			t.Errorf("%s: %+v, ждали other %q", name, h, iccUnknownName)
		}
	}

	// Ровно iccMaxBytes распакованного — ещё в пределе (профиль-мусор, но не «слишком большой»): читается до classify.
	if h := Header(bytes.NewReader(testkit.PNGWithChunks(rgbPNG(t), iccpData("ICC Profile", 0, zlibOf(append(append([]byte{}, p3...), make([]byte, iccMaxBytes-len(p3))...)))))); h.Color != ColorP3 {
		t.Errorf("профиль ровно в пределе: %+v", h)
	}

	// Заявленная длина iCCP больше предела (в том числе 2³¹ и 2³²−1) — other без чтения данных.
	for _, n := range []uint32{iccMaxBytes + 1, 1 << 30, 1 << 31, 1<<32 - 1} {
		head := binary.BigEndian.AppendUint32(rgbPNG(t)[:33], n)
		head = append(head, "iCCP"...)
		h := Header(io.MultiReader(bytes.NewReader(head), noRead{t}))
		if h.Color != ColorOther || h.Profile != iccUnknownName {
			t.Errorf("длина %d: %+v", n, h)
		}
	}

	// Заявлено 1000 байт, файл кончился.
	trunc := append(binary.BigEndian.AppendUint32(rgbPNG(t)[:33], 1000), "iCCP\x00\x00\x00"...)
	if h := Header(bytes.NewReader(trunc)); h.Color != ColorOther {
		t.Errorf("обрыв данных: %+v", h)
	}
	// Данные целы, CRC отрезан.
	noCRC := iccp(p3)
	if h := Header(bytes.NewReader(append(rgbPNG(t)[:33], noCRC[:len(noCRC)-2]...))); h.Color != ColorOther {
		t.Errorf("обрыв CRC: %+v", h)
	}

	// Бомба 1 ГиБ нулей в 1 МиБ сжатого: не растёт и не тратит время. Под -short — 48 МиБ: сжатие 1 ГиБ
	// съедало почти всё время теста, а регресс без LimitReader ловится и так (48 МиБ ≫ 4 МиБ TotalAlloc).
	bombSize := 1 << 30
	if testkit.Lite() {
		bombSize = 48 << 20
	}
	bomb := zipBomb(t, bombSize)
	if len(bomb) > iccMaxBytes {
		t.Fatalf("бомба не влезает в предел сжатого: %d байт", len(bomb))
	}
	data := testkit.PNGWithChunks(rgbPNG(t), iccpData("ICC Profile", 0, bomb))
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	start := time.Now()
	h := Header(bytes.NewReader(data))
	took := time.Since(start)
	runtime.ReadMemStats(&b)
	if h.Color != ColorOther {
		t.Errorf("бомба: %+v", h)
	}
	if limit := testkit.Limit(50 * time.Millisecond); took > limit {
		t.Errorf("бомба разбиралась %v (предел %v)", took, limit)
	}
	// Без -race бомба стоит ≈3,3 МиБ (рост ReadAll до 1 МиБ+1); под -race детектор добавляет свои
	// выделения (≈4,6 МиБ). Регресс без LimitReader — десятки МиБ, поэтому под -race предел 6 МиБ.
	limitAlloc := uint64(4 << 20)
	if testkit.Race {
		limitAlloc = 6 << 20
	}
	if alloc := b.TotalAlloc - a.TotalAlloc; alloc > limitAlloc {
		t.Errorf("бомба выделила %d Б (предел %d Б)", alloc, limitAlloc)
	}
}

// zipBomb — поток zlib из n нулей; сжимается примерно в тысячу раз.
func zipBomb(t testing.TB, n int) []byte {
	t.Helper()
	var z bytes.Buffer
	w, err := zlib.NewWriterLevel(&z, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	zero := make([]byte, 1<<20)
	for left := n; left > 0; left -= len(zero) {
		if _, err := w.Write(zero[:min(left, len(zero))]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return z.Bytes()
}

// Пределы обхода до iCCP: чужой огромный чанк, 64 чанка, 16 МиБ пропуска — «без профиля»,
// не ColorOther (профиль не заявлен). Длина чужого чанка не читается.
func TestHeaderPNGLimits(t *testing.T) {
	p3 := testkit.ProfileXiaomiP3(true)
	base := rgbPNG(t)[:33] // подпись и IHDR
	pad := func(n int) [][]byte {
		var cs [][]byte
		for range n {
			cs = append(cs, testkit.PNGChunk("tEXt", nil))
		}
		return cs
	}
	build := func(chunks ...[]byte) []byte {
		out := append([]byte{}, base...)
		for _, c := range chunks {
			out = append(out, c...)
		}
		return out
	}

	// Прочий чанк длиннее 2³¹−1 (и ровно 2³¹) — конец обхода без чтения.
	for _, n := range []uint32{1 << 31, 1<<32 - 1} {
		head := append(binary.BigEndian.AppendUint32(append([]byte{}, base...), n), "tEXt"...)
		if h := Header(io.MultiReader(bytes.NewReader(head), noRead{t})); h.Color != ColorNone {
			t.Errorf("tEXt длиной %d: %+v", n, h)
		}
	}

	// Число чанков до iCCP: iCCP — 64-й ещё читается, 65-й нет.
	for n, want := range map[int]Color{0: ColorP3, 62: ColorP3, 63: ColorP3, 64: ColorNone, 65: ColorNone, 500: ColorNone} {
		data := build(append(pad(n), iccp(p3))...)
		if h := Header(bytes.NewReader(data)); h.Color != want {
			t.Errorf("%d чанков до iCCP: %q, ждали %q", n, h.Color, want)
		}
	}

	// Пропуск до iCCP: ровно 16 МиБ (данные и CRC) проходит, на байт больше — нет. Seek и чтение дают одно и то же.
	for over, want := range map[int]Color{0: ColorP3, 1: ColorNone} {
		big := testkit.PNGChunk("tEXt", make([]byte, pngMaxSkip-4+over))
		data := build(big, iccp(p3))
		for name, r := range map[string]io.Reader{
			"Seek":     bytes.NewReader(data),
			"без Seek": iotest.DataErrReader(bytes.NewBuffer(data)),
		} {
			if h := Header(r); h.Color != want {
				t.Errorf("пропуск 16 МиБ%+d, %s: %q, ждали %q", over, name, h.Color, want)
			}
		}
	}
	// Много чанков вместе дают тот же итог пропуска.
	var many [][]byte
	for range 20 {
		many = append(many, testkit.PNGChunk("tEXt", make([]byte, 1<<20)))
	}
	if h := Header(bytes.NewReader(build(append(many, iccp(p3))...))); h.Color != ColorNone {
		t.Errorf("20 МиБ мелкими чанками: %q", h.Color)
	}
}

// FuzzPNGProfile: данные iCCP идут прямо в pngICCP с пересчитанным CRC (в FuzzHeader любая правка
// ломала бы CRC, и до zlib и classify дело не доходило). Без паник; класс из набора; имя не длиннее предела;
// профиль, выданный pngProfile, не больше iccMaxBytes. Сиды — обычный, FDICT, пустой поток, обрыв, бомба.
func FuzzPNGProfile(f *testing.F) {
	good := testkit.PNGICCData(testkit.ProfileXiaomiP3(true))
	f.Add(good, byte(2))
	f.Add(good[:len(good)/2], byte(2))
	f.Add(good, byte(0))                                                                          // ч/б PNG с RGB-профилем
	f.Add([]byte("ICC Profile\x00\x00\x78\xBB\x00\x00\x00\x01\x03\x00\x00\x00\x00\x01"), byte(2)) // FDICT
	f.Add([]byte("ICC Profile\x00\x00"), byte(2))                                                 // пустой поток
	f.Add([]byte("ICC Profile\x00"), byte(6))                                                     // нет метода
	f.Add(append([]byte("ICC Profile\x00\x00"), zlibOf(make([]byte, 4<<20))...), byte(2))         // распакуется за предел
	f.Fuzz(func(t *testing.T, data []byte, colorType byte) {
		if p, ok := pngProfile(data); ok && len(p) > iccMaxBytes {
			t.Fatalf("профиль %d Б больше предела", len(p))
		}
		buf := binary.BigEndian.AppendUint32(nil, crc32.Update(crc32.ChecksumIEEE([]byte("iCCP")), crc32.IEEETable, data))
		h := pngICCP(bytes.NewReader(append(append([]byte{}, data...), buf...)), int64(len(data)), colorType)
		switch h.Color {
		case ColorNone, ColorSRGB, ColorAdobe, ColorP3, ColorICC, ColorOther:
		default:
			t.Fatalf("класс %q", h.Color)
		}
		if h.Orient != 1 || len([]rune(h.Profile)) > max(iccNameRunes, len([]rune(iccUnknownName))) {
			t.Fatalf("%d %q", h.Orient, h.Profile)
		}
		if h.Color.converts() != (h.xf != nil) {
			t.Fatalf("класс %q и таблицы %v расходятся", h.Color, h.xf)
		}
	})
}
