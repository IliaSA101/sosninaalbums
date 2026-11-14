package colorspace

import (
	"bytes"
	"encoding/binary"
	"image"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// xfOf разбирает профиль, ошибка — провал теста.
func xfOf(t testing.TB, p []byte) *Xform {
	t.Helper()
	x, err := ParseICC(p)
	if err != nil {
		t.Fatalf("ParseICC: %v", err)
	}
	return x
}

// refProfile — профиль из design §6 и опорные значения LittleCMS 2.19 (Pillow ImageCms,
// относительная колориметрия, вывод во встроенный sRGB): {вход R,G,B, выход R,G,B}.
type refProfile struct {
	name    string
	profile []byte
	cases   [][6]uint8
}

var (
	refP3 = [][6]uint8{
		{0, 0, 0, 0, 0, 0}, {255, 255, 255, 255, 255, 255}, {128, 128, 128, 128, 128, 128}, {64, 64, 64, 64, 64, 64},
		{10, 10, 10, 10, 10, 10}, {255, 0, 0, 255, 0, 0}, {0, 255, 0, 0, 255, 0}, {0, 0, 255, 0, 0, 255},
		{200, 100, 50, 215, 93, 31}, {50, 160, 90, 0, 163, 82}, {230, 190, 170, 238, 188, 167}, {30, 60, 120, 17, 61, 124},
		{1, 2, 3, 1, 2, 3}, {250, 240, 10, 252, 240, 0}, {255, 128, 0, 255, 119, 0}, {120, 0, 200, 132, 0, 208},
	}
	refProPhoto = [][6]uint8{
		{0, 0, 0, 0, 0, 0}, {255, 255, 255, 255, 255, 255}, {128, 128, 128, 146, 146, 146}, {64, 64, 64, 81, 81, 81},
		{10, 10, 10, 10, 10, 10}, {255, 0, 0, 255, 0, 0}, {0, 255, 0, 0, 255, 0}, {0, 0, 255, 0, 0, 255},
		{200, 100, 50, 255, 80, 47}, {50, 160, 90, 0, 191, 94}, {230, 190, 170, 255, 193, 181}, {30, 60, 120, 0, 83, 146},
		{1, 2, 3, 0, 1, 1}, {250, 240, 10, 255, 241, 0}, {255, 128, 0, 255, 100, 0}, {120, 0, 200, 155, 0, 224},
	}
	refLinear = [][6]uint8{
		{128, 128, 128, 188, 188, 188}, {64, 64, 64, 137, 137, 137}, {10, 10, 10, 56, 56, 56}, {200, 100, 50, 229, 168, 122},
		{50, 160, 90, 122, 208, 160}, {30, 60, 120, 96, 133, 182}, {1, 2, 3, 13, 22, 28}, {255, 128, 0, 255, 188, 0},
	}
	refRec709 = [][6]uint8{
		{128, 128, 128, 140, 140, 140}, {64, 64, 64, 79, 79, 79}, {10, 10, 10, 23, 23, 23}, {200, 100, 50, 206, 114, 66},
		{50, 160, 90, 66, 169, 104}, {30, 60, 120, 46, 75, 132}, {1, 2, 3, 3, 6, 9}, {255, 128, 0, 255, 140, 0},
	}
)

func refProfiles() []refProfile {
	desc := testkit.DescV4
	p3 := testkit.XYZP3Exact
	// тип 4: те же числа с e = f = 0 — кривая та же
	para4 := append(append([]float64{}, testkit.SRGBPara3...), 0, 0)
	return []refProfile{
		{"P3 v4 para 3", testkit.ParaProfile(p3, 3, testkit.SRGBPara3, desc("Display P3")), refP3},
		{"P3 para 4", testkit.ParaProfile(p3, 4, para4, desc("P3 p4")), refP3},
		{"P3 v2 таблица 1024", testkit.TableProfile(p3, 1024, testkit.SRGBEOTF, testkit.DescV2("Display P3 (table)")), refP3},
		{"Xiaomi v2", testkit.ProfileXiaomiP3(false), refP3},
		{"Xiaomi v4", testkit.ProfileXiaomiP3(true), refP3},
		{"ProPhoto para 0", testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{117965.0 / 65536}, desc("ProPhoto RGB")), refProPhoto},
		{"линейный sRGB curv 0", testkit.LinearProfile(testkit.XYZLinearSRGB, desc("Linear sRGB")), refLinear},
		{"Rec. 709 таблица 4096", testkit.TableProfile(testkit.XYZRec709Exact, 4096, testkit.Rec709EOTF, desc("Rec. 709")), refRec709},
	}
}

func level(a, b uint8) int { return abs(int(a) - int(b)) }

// Эталон LittleCMS: не больше уровня на канал.
func TestParseICCReference(t *testing.T) {
	for _, rp := range refProfiles() {
		x := xfOf(t, rp.profile)
		for _, c := range rp.cases {
			r, g, b := x.px(c[0], c[1], c[2])
			if level(r, c[3]) > 1 || level(g, c[4]) > 1 || level(b, c[5]) > 1 {
				t.Errorf("%s %v: получили %d,%d,%d, LittleCMS %v", rp.name, c[:3], r, g, b, c[3:])
			}
		}
	}
}

// Таблицы против float-эталона: не больше уровня на 100 000 случайных цветов и на всех серых.
func TestXformMatchesFloat(t *testing.T) {
	n := testkit.Steps(100_000, 10_000)
	for _, rp := range refProfiles() {
		x := xfOf(t, rp.profile)
		check := func(r, g, b uint8) {
			r1, g1, b1 := x.px(r, g, b)
			r2, g2, b2 := x.refPx(r, g, b)
			if level(r1, r2) > 1 || level(g1, g2) > 1 || level(b1, b2) > 1 {
				t.Fatalf("%s (%d,%d,%d): таблицы %d,%d,%d, float %d,%d,%d", rp.name, r, g, b, r1, g1, b1, r2, g2, b2)
			}
		}
		for v := range 256 {
			check(uint8(v), uint8(v), uint8(v))
		}
		rnd := rand.New(rand.NewSource(1))
		for range n {
			check(uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)))
		}
	}
}

// Пресет Adobe и прежний AdobeToSRGB — один и тот же результат, байт в байт, на кубе 17³ и попиксельно.
func TestAdobeXformSame(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 17*17, 17))
	i := 0
	for r := range 17 {
		for g := range 17 {
			for b := range 17 {
				copy(m.Pix[i:], []uint8{uint8(min(r*16, 255)), uint8(min(g*16, 255)), uint8(min(b*16, 255)), 255})
				i += 4
			}
		}
	}
	a, b := image.NewNRGBA(m.Rect), image.NewNRGBA(m.Rect)
	copy(a.Pix, m.Pix)
	copy(b.Pix, m.Pix)
	if !bytes.Equal(Adobe.ToSRGB(a).Pix, AdobeToSRGB(b).Pix) {
		t.Fatal("Adobe.ToSRGB и AdobeToSRGB разошлись")
	}
	for v := range 256 {
		r1, g1, b1 := Adobe.px(uint8(v), uint8(255-v), uint8(v/2))
		r2, g2, b2 := px(uint8(v), uint8(255-v), uint8(v/2))
		if r1 != r2 || g1 != g2 || b1 != b2 {
			t.Fatalf("px(%d): пресет и обёртка расходятся", v)
		}
	}
}

// Профиль самого Adobe RGB, разобранный ParseICC, и пресет расходятся не больше чем на уровень-два
// (колоранты testkit.XYZAdobe округлены до 4 знаков) — поэтому пресет и нужен: байты прежние.
func TestAdobeProfileNearPreset(t *testing.T) {
	p := testkit.ParaProfile(testkit.XYZAdobe, 0, []float64{563.0 / 256}, testkit.DescV2("Adobe RGB (1998)"))
	x := xfOf(t, p)
	rnd := rand.New(rand.NewSource(3))
	for range 20_000 {
		r, g, b := uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256))
		r1, g1, b1 := x.px(r, g, b)
		r2, g2, b2 := Adobe.px(r, g, b)
		if level(r1, r2) > 2 || level(g1, g2) > 2 || level(b1, b2) > 2 {
			t.Fatalf("(%d,%d,%d): разбор %d,%d,%d, пресет %d,%d,%d", r, g, b, r1, g1, b1, r2, g2, b2)
		}
	}
}

// SRGBCurves смотрит только кривые (119-14): матрица любая, дрейф колорантов не мешает.
func TestSRGBCurves(t *testing.T) {
	srgbXYZ := [9]float64{0.43607, 0.22249, 0.01392, 0.38515, 0.71687, 0.09708, 0.14307, 0.06061, 0.71410}
	drift := srgbXYZ
	for i := range drift {
		drift[i] += 0.002 * float64(i%2*2-1)
	}
	d := testkit.DescV2("x")
	curves := map[string]struct {
		p    []byte
		want bool
	}{
		"v4 para 3":       {testkit.ParaProfile(srgbXYZ, 3, testkit.SRGBPara3, d), true},
		"v2 таблица 1024": {testkit.TableProfile(srgbXYZ, 1024, testkit.SRGBEOTF, d), true},
		"v2 таблица 182":  {testkit.TableProfile(srgbXYZ, 182, testkit.SRGBEOTF, d), true},
		"таблица 16":      {testkit.TableProfile(srgbXYZ, 16, testkit.SRGBEOTF, d), true},
		"дрейф ±0,002":    {testkit.ParaProfile(drift, 3, testkit.SRGBPara3, d), true},
		"P3 (кривая sRGB, колоранты решает вызывающий)": {testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, d), true},
		"линейный":  {testkit.LinearProfile(srgbXYZ, d), false},
		"гамма 2.2": {testkit.ParaProfile(srgbXYZ, 0, []float64{2.2}, d), false},
		"Rec. 709":  {testkit.TableProfile(srgbXYZ, 4096, testkit.Rec709EOTF, d), false},
		"ProPhoto":  {testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, d), false},
	}
	for n, c := range curves {
		if got := xfOf(t, c.p).SRGBCurves(); got != c.want {
			t.Errorf("%s: SRGBCurves = %v, ждали %v", n, got, c.want)
		}
	}
}

// Каждая обрезка и порча каждого байта заголовка и таблицы тегов — без паники; принятый Xform считает.
func TestParseICCTruncations(t *testing.T) {
	for _, rp := range refProfiles() {
		p := rp.profile
		for n := range len(p) {
			if x, err := ParseICC(p[:n]); err == nil {
				x.px(255, 128, 0)
			}
		}
		lim := min(len(p), iccHeaderLen+iccEntryLen*12)
		for i := range lim {
			q := bytes.Clone(p)
			for _, v := range []byte{0, 0xff, q[i] ^ 0x80} {
				q[i] = v
				if x, err := ParseICC(q); err == nil {
					x.px(1, 2, 3)
				}
			}
		}
	}
}

func setU32(p []byte, off int, v uint32) []byte {
	q := bytes.Clone(p)
	binary.BigEndian.PutUint32(q[off:], v)
	return q
}

// tagOff — смещение данных тега sig в профиле, собранном BuildICC.
func tagOff(t *testing.T, p []byte, sig string) int {
	t.Helper()
	for i := range int(binary.BigEndian.Uint32(p[128:])) {
		e := iccHeaderLen + iccEntryLen*i
		if string(p[e:e+4]) == sig {
			return int(binary.BigEndian.Uint32(p[e+4:]))
		}
	}
	t.Fatalf("нет тега %s", sig)
	return 0
}

// build — профиль с колорантами c и тремя кривыми.
func build(c [9]float64, r, g, b []byte) []byte {
	return testkit.BuildICC("RGB ",
		testkit.ICCTag{Sig: "rXYZ", Data: testkit.XYZTag(c[0:3]...)}, testkit.ICCTag{Sig: "gXYZ", Data: testkit.XYZTag(c[3:6]...)},
		testkit.ICCTag{Sig: "bXYZ", Data: testkit.XYZTag(c[6:9]...)},
		testkit.ICCTag{Sig: "rTRC", Data: r}, testkit.ICCTag{Sig: "gTRC", Data: g}, testkit.ICCTag{Sig: "bTRC", Data: b})
}

// Пределы design §4: каждое нарушение — ошибка, а не паника и не странный Xform.
func TestParseICCRejects(t *testing.T) {
	good := testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, testkit.DescV2("P3"))
	if _, err := ParseICC(good); err != nil {
		t.Fatalf("исходный профиль: %v", err)
	}
	withSpace := func(off int, s string) []byte {
		q := bytes.Clone(good)
		copy(q[off:], s)
		return q
	}
	g22 := testkit.CurvTag(2.2)
	curv := func(c []byte) []byte { return build(testkit.XYZP3Exact, c, c, c) }
	badCurv := func(mod func([]byte)) []byte { // curv на 16 точек, подпорченный
		c := testkit.TableTag(16, func(v float64) float64 { return v })
		mod(c)
		return curv(c)
	}
	d65 := [9]float64{0.4124, 0.2126, 0.0193, 0.3576, 0.7152, 0.1192, 0.1805, 0.0722, 0.9505} // D65-колоранты v2: Z расходится
	huge := testkit.XYZP3Exact
	huge[0] = 9
	flat := [9]float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}

	cases := map[string][]byte{
		"CMYK":                    testkit.BuildICC("CMYK"),
		"PCS Lab":                 withSpace(20, "Lab "),
		"класс link":              withSpace(12, "link"),
		"класс abst":              withSpace(12, "abst"),
		"класс nmcl":              withSpace(12, "nmcl"),
		"A2B0":                    testkit.LUTProfile(testkit.XYZP3Exact, testkit.DescV2("P3")),
		"колорант не XYZ":         setU32(good, tagOff(t, good, "rXYZ"), 0x6e6f7065),
		"колоранты D65":           build(d65, g22, g22, g22),
		"вырожденная":             build(flat, g22, g22, g22),
		"матрица вне пределов":    build(huge, g22, g22, g22),
		"нет кривой":              build(testkit.XYZP3Exact, nil, g22, g22),
		"curv слишком велика":     badCurv(func(c []byte) { binary.BigEndian.PutUint32(c[8:], iccMaxCurve+1) }),
		"curv длиннее данных":     badCurv(func(c []byte) { binary.BigEndian.PutUint32(c[8:], 17) }),
		"curv гамма 0":            curv(append([]byte("curv\x00\x00\x00\x00\x00\x00\x00\x01"), 0, 0)),
		"curv гамма 11":           curv(testkit.CurvTag(11)),
		"para тип 5":              curv(testkit.ParaTag(5, 2.2)),
		"para короче типа":        curv(testkit.ParaTag(3, 2.4, 1)),
		"para гамма 0":            curv(testkit.ParaTag(0, 0)),
		"para гамма 11":           curv(testkit.ParaTag(0, 11)),
		"para тип 1 с a = 0":      curv(testkit.ParaTag(1, 2.4, 0, 0.1)),
		"para тип 2 с a = 0":      curv(testkit.ParaTag(2, 2.4, 0, 0.1, 0)),
		"неизвестный тип кривой":  curv([]byte("mft2\x00\x00\x00\x00\x00\x00\x00\x00")),
		"кривая короче заголовка": curv([]byte("curv")),
	}
	for name, p := range cases {
		if x, err := ParseICC(p); err == nil {
			t.Errorf("%s: ждали ошибку, получили Xform (%v)", name, x != nil)
		}
	}
	// на границах пределов профиль годится
	ok := map[string][]byte{
		"curv n=65536": curv(testkit.TableTag(1<<16, func(v float64) float64 { return v })),
		"curv n=2":     curv(testkit.TableTag(2, func(v float64) float64 { return v })),
		"гамма 10":     curv(testkit.CurvTag(10)),
		"para тип 1":   curv(testkit.ParaTag(1, 2.4, 1, 0)),
		"para тип 2":   curv(testkit.ParaTag(2, 2.4, 1, 0, 0)),
	}
	for name, p := range ok {
		if _, err := ParseICC(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Края кривых: вне области определения — 0 без NaN, таблица из двух точек — линейная.
func TestParseICCCurveEdges(t *testing.T) {
	// para тип 1: (aX+b)^g при X ≥ -b/a, иначе 0. a=2, b=-0.5: ниже 0,25 — 0; X=1 → 1,5^2 → зажим в 1
	x := xfOf(t, testkit.ParaProfile(testkit.XYZP3Exact, 1, []float64{2, 2, -0.5}, testkit.DescV2("p")))
	if x.dec[0][0] != 0 || x.dec[0][32] != 0 || x.dec[0][255] != 65535 {
		t.Errorf("para тип 1: dec 0/32/255 = %d/%d/%d", x.dec[0][0], x.dec[0][32], x.dec[0][255])
	}
	// отрицательное основание (a=-1, b=0, X>0) — 0, а не NaN
	y := xfOf(t, testkit.ParaProfile(testkit.XYZP3Exact, 3, []float64{2, -1, 0, 0.5, 0.9}, testkit.DescV2("p")))
	if y.dec[0][255] != 0 {
		t.Errorf("отрицательное основание: %d", y.dec[0][255])
	}
	t2 := xfOf(t, testkit.TableProfile(testkit.XYZP3Exact, 2, func(v float64) float64 { return v }, testkit.DescV2("t")))
	if t2.dec[0][0] != 0 || t2.dec[0][255] != 65535 || t2.dec[0][128] != int32(math.Round(128.0/255*65535)) {
		t.Errorf("таблица из двух точек: %d %d %d", t2.dec[0][0], t2.dec[0][128], t2.dec[0][255])
	}
}

// Предельный профиль (ProPhoto: сумма модулей строки матрицы ≈ 3) не переполняет и даёт белый на белом.
func TestXformProPhotoExtremes(t *testing.T) {
	x := xfOf(t, testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, testkit.DescV2("ProPhoto")))
	for _, c := range [][6]uint8{{255, 255, 255, 255, 255, 255}, {0, 0, 0, 0, 0, 0}, {255, 0, 255, 255, 0, 255}} {
		r, g, b := x.px(c[0], c[1], c[2])
		if r != c[3] || g != c[4] || b != c[5] {
			t.Errorf("%v: %d,%d,%d", c[:3], r, g, b)
		}
	}
}

// Несколько профилей на одной картинке: NRGBA на месте, альфа не трогается, YCbCr и NRGBA дают одно.
func TestXformToSRGBPaths(t *testing.T) {
	x := xfOf(t, testkit.ProfileXiaomiP3(true))
	m := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	copy(m.Pix, []uint8{200, 100, 50, 77, 0, 0, 0, 255, 255, 255, 255, 0})
	got := x.ToSRGB(m)
	if got != m {
		t.Error("NRGBA должно пересчитываться на месте")
	}
	if m.Pix[0] != 215 || m.Pix[1] != 93 || m.Pix[2] != 31 || m.Pix[3] != 77 || m.Pix[11] != 0 {
		t.Errorf("пиксели %v", m.Pix)
	}
	yc := image.NewYCbCr(image.Rect(0, 0, 9, 7), image.YCbCrSubsampleRatio420)
	rnd := rand.New(rand.NewSource(5))
	for i := range yc.Y {
		yc.Y[i] = uint8(rnd.Intn(256))
	}
	for i := range yc.Cb {
		yc.Cb[i], yc.Cr[i] = uint8(rnd.Intn(256)), uint8(rnd.Intn(256))
	}
	a := x.ToSRGB(yc)
	ref := image.NewNRGBA(yc.Bounds())
	for yy := range 7 {
		for xx := range 9 {
			r, g, b, _ := yc.At(xx, yy).RGBA()
			copy(ref.Pix[yy*ref.Stride+xx*4:], []uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255})
		}
	}
	if !bytes.Equal(a.Pix, x.ToSRGB(ref).Pix) {
		t.Error("YCbCr и NRGBA дают разные байты")
	}
}

func ycbcr50() *image.YCbCr {
	m := image.NewYCbCr(image.Rect(0, 0, 8660, 5774), image.YCbCrSubsampleRatio420)
	for i := range m.Y {
		m.Y[i] = uint8(i * 7)
	}
	for i := range m.Cb {
		m.Cb[i], m.Cr[i] = uint8(i*3), uint8(i*5)
	}
	return m
}

// 50 Мп YCbCr→NRGBA с пересчётом P3 — не дольше Adobe ×1,3 (замер прототипа: 73 мс против 78 мс);
// только под SA_BENCH.
func BenchmarkXform50Mp(b *testing.B) {
	if os.Getenv("SA_BENCH") == "" {
		b.Skip("задайте SA_BENCH=1")
	}
	x := xfOf(b, testkit.ProfileXiaomiP3(true))
	src := ycbcr50()
	ta := testkit.Fastest(5, func() { AdobeToSRGB(src) })
	tx := testkit.Fastest(5, func() { x.ToSRGB(src) })
	b.Logf("50 Мп: Adobe %v, общий %v", ta, tx)
	if tx > testkit.Limit(ta*13/10) {
		b.Fatalf("общий путь %v дольше Adobe %v ×1,3", tx, ta)
	}
	b.ResetTimer()
	for b.Loop() {
		x.ToSRGB(src)
	}
}
