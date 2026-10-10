package imgfile

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func rgbProfile(xyz [9]float64, gamma float64, desc []byte) []byte {
	return testkit.MatrixProfile(xyz, gamma, desc)
}

func TestICCClassify(t *testing.T) {
	descV2, descV4 := testkit.DescV2, testkit.DescV4
	shifted := adobeXYZ
	shifted[0] += 0.002 // в допуске
	far := adobeXYZ
	far[0] += 0.01 // вне допуска
	cases := []struct {
		name  string
		p     []byte
		color Color
		pname string
	}{
		{"srgb v2", rgbProfile(srgbXYZ, 2.2, descV2("sRGB IEC61966-2.1")), ColorSRGB, "sRGB IEC61966-2.1"},
		{"srgb v4 mluc, кривая не важна", rgbProfile(srgbXYZ, 1.0, descV4("sRGB")), ColorSRGB, "sRGB"},
		{"adobe", rgbProfile(adobeXYZ, 2.2, descV2("Adobe RGB (1998)")), ColorAdobe, "Adobe RGB (1998)"},
		{"adobe в допуске", rgbProfile(shifted, 2.2, descV2("A")), ColorAdobe, "A"},
		{"adobe вне допуска", rgbProfile(far, 2.2, descV2("A")), ColorOther, "A"},
		{"adobe, но гамма 1.8", rgbProfile(adobeXYZ, 1.8, descV2("A")), ColorOther, "A"},
		{"display p3", rgbProfile(testkit.XYZP3, 2.2, descV4("Display P3")), ColorOther, "Display P3"},
		{"gray", testkit.BuildICC("GRAY", testkit.ICCTag{Sig: "desc", Data: descV2("Gray")}), ColorSRGB, "Gray"},
		{"cmyk", testkit.BuildICC("CMYK", testkit.ICCTag{Sig: "desc", Data: descV2("Fogra")}), ColorOther, "Fogra"},
		{"rgb без колорантов", testkit.BuildICC("RGB ", testkit.ICCTag{Sig: "desc", Data: descV2("X")}), ColorOther, "X"},
		{"rgb без имени", testkit.BuildICC("RGB "), ColorOther, iccUnknownName},
		{"нет acsp", make([]byte, 200), ColorOther, iccUnknownName},
		{"короткий", []byte("abc"), ColorOther, iccUnknownName},
		{"непечатные символы режутся", rgbProfile(srgbXYZ, 2.2, descV2("a\x01b\xffc")), ColorSRGB, "abÿc"},
	}
	for _, c := range cases {
		col, name := classifyICC(c.p)
		if col != c.color || name != c.pname {
			t.Errorf("%s: получили (%q, %q), ждали (%q, %q)", c.name, col, name, c.color, c.pname)
		}
	}
}

func TestICCClassifyHostile(t *testing.T) {
	p := rgbProfile(srgbXYZ, 2.2, testkit.DescV2("sRGB"))
	// число тегов огромное, смещения и размеры за границей, переполнение off+size
	bad := bytes.Clone(p)
	binary.BigEndian.PutUint32(bad[128:], 0xFFFFFFFF)
	for i := range 3 {
		e := iccHeaderLen + iccTagEntryLen*i
		binary.BigEndian.PutUint32(bad[e+4:], 0xFFFFFFF0)
		binary.BigEndian.PutUint32(bad[e+8:], 0x20)
	}
	if c, _ := classifyICC(bad); c != ColorOther {
		t.Errorf("битая таблица: %q, ждали other", c)
	}
	// усечение на каждом байте не должно паниковать
	for n := range len(p) {
		classifyICC(p[:n])
	}
	// имя: длина вне тега и mluc с огромным смещением
	if n := iccName(append([]byte("desc\x00\x00\x00\x00"), 0xFF, 0xFF, 0xFF, 0xFF)); n != "" {
		t.Errorf("desc с огромной длиной: %q", n)
	}
	m := testkit.DescV4("x")
	binary.BigEndian.PutUint32(m[24:], 0xFFFFFFFF)
	if n := iccName(m); n != "" {
		t.Errorf("mluc с огромным смещением: %q", n)
	}
	// длинное имя обрезается
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if n := iccName(testkit.DescV2(string(long))); len([]rune(n)) != iccNameRunes {
		t.Errorf("длина имени %d, ждали %d", len([]rune(n)), iccNameRunes)
	}
}

// FuzzClassifyICC: любой мусор не роняет разбор, имя не длиннее лимита.
func FuzzClassifyICC(f *testing.F) {
	f.Add(rgbProfile(srgbXYZ, 2.2, testkit.DescV2("sRGB")))
	f.Add(rgbProfile(adobeXYZ, 2.2, testkit.DescV4("Adobe")))
	f.Add(testkit.BuildICC("CMYK"))
	f.Add([]byte("acsp"))
	f.Fuzz(func(t *testing.T, p []byte) {
		c, name := classifyICC(p)
		if len([]rune(name)) > iccNameRunes && name != iccUnknownName {
			t.Fatalf("имя длиннее лимита: %d рун", len([]rune(name)))
		}
		switch c {
		case ColorSRGB, ColorAdobe, ColorOther:
		default:
			t.Fatalf("неожиданный класс %q", c)
		}
	})
}

func TestICCReadColor(t *testing.T) {
	adobe := rgbProfile(adobeXYZ, 2.2, testkit.DescV2("Adobe RGB (1998)"))
	srgb := rgbProfile(srgbXYZ, 2.2, testkit.DescV2("sRGB"))
	app0 := testkit.Segment(0xE0, []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"))
	icc, exif, raw := testkit.ICCSegment, testkit.ExifColor, testkit.RawJPEG

	cases := []struct {
		name  string
		data  []byte
		color Color
		pname string
	}{
		{"icc adobe", raw(app0, icc(1, 1, adobe)), ColorAdobe, "Adobe RGB (1998)"},
		{"icc важнее exif R03", raw(exif(false, 6, 0xFFFF, "R03"), icc(1, 1, srgb)), ColorSRGB, "sRGB"},
		{"exif R03 без icc", raw(exif(false, 6, 0xFFFF, "R03")), ColorAdobe, ICCAdobeEXIF},
		{"exif R98 без icc", raw(exif(false, 6, 0xFFFF, "R98")), ColorNone, ""},
		{"ничего", raw(app0), ColorNone, ""},
		{"не JPEG", []byte("\x89PNG\r\n\x1a\n"), ColorNone, ""},
		{"пусто", nil, ColorNone, ""},
		{"два куска", raw(icc(1, 2, adobe[:300]), icc(2, 2, adobe[300:])), ColorAdobe, "Adobe RGB (1998)"},
		{"два куска в обратном порядке", raw(icc(2, 2, adobe[300:]), icc(1, 2, adobe[:300])), ColorAdobe, "Adobe RGB (1998)"},
		// второго куска нет: теги за пределами первого → other
		{"не хватает куска", raw(icc(1, 2, adobe[:300])), ColorOther, ""},
		{"усечённый сегмент", raw(icc(1, 1, adobe))[:100], ColorNone, ""},
	}
	for _, c := range cases {
		col, name := readColor(bytes.NewReader(c.data))
		if col != c.color || (c.pname != "" && name != c.pname) || (c.pname == "" && c.color != ColorOther && name != "") {
			t.Errorf("%s: получили (%q, %q), ждали (%q, %q)", c.name, col, name, c.color, c.pname)
		}
	}
}

func TestICCReadColorLimits(t *testing.T) {
	adobe := rgbProfile(adobeXYZ, 2.2, testkit.DescV2("Adobe RGB (1998)"))
	// профиль после 64-го сегмента не читается
	segs := make([][]byte, 0, 70)
	for range iccMaxSegments {
		segs = append(segs, testkit.Segment(0xE0, []byte("x")))
	}
	segs = append(segs, testkit.ICCSegment(1, 1, adobe))
	if c, _ := readColor(bytes.NewReader(testkit.RawJPEG(segs...))); c != ColorNone {
		t.Errorf("после лимита сегментов: %q", c)
	}
	// кусков больше лимита по байтам: склейка не идёт, остаётся первый
	big := make([]byte, 65000)
	var many [][]byte
	for i := 1; i <= 20; i++ {
		many = append(many, testkit.ICCSegment(byte(i), 20, big))
	}
	if c, _ := readColor(bytes.NewReader(testkit.RawJPEG(many...))); c != ColorOther {
		t.Errorf("слишком большой профиль: %q", c)
	}
	// «сегмент» обрывается раньше длины
	short := testkit.RawJPEG(testkit.Segment(0xE2, []byte("ICC_PROFILE\x00")))[:20]
	if c, _ := readColor(bytes.NewReader(short)); c != ColorNone {
		t.Errorf("короткий ICC-сегмент: %q", c)
	}
}

func TestICCExifHostile(t *testing.T) {
	good := testkit.ExifColor(false, 6, 0xFFFF, "R03")[4+6:] // без маркера, длины и «Exif\0\0»
	if got := exifInterop(good); got != "R03" {
		t.Fatalf("исходный EXIF: %q", got)
	}
	for n := range len(good) {
		exifInterop(good[:n]) // усечение не паникует
	}
	bad := bytes.Clone(good)
	binary.LittleEndian.PutUint32(bad[4:], 0xFFFFFFFF)
	if got := exifInterop(bad); got != "" {
		t.Errorf("смещение вне границ: %q", got)
	}
}
