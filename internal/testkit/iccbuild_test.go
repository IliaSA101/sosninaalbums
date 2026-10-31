package testkit

import (
	"encoding/binary"
	"math"
	"testing"
	"unicode/utf16"
)

// iccTagMap разбирает профиль независимо от сборщика: подпись тега → его данные.
// Заголовок обязан быть «настоящим» (acsp, класс, PCS), иначе LittleCMS профиль не откроет.
func iccTagMap(t *testing.T, p []byte, space string) map[string][]byte {
	t.Helper()
	if len(p) < iccHeaderLen || string(p[36:40]) != "acsp" || string(p[12:16]) != "mntr" ||
		string(p[16:20]) != space || string(p[20:24]) != "XYZ " {
		t.Fatalf("заголовок профиля: % x", p[:min(len(p), 40)])
	}
	n := int(binary.BigEndian.Uint32(p[128:]))
	out := map[string][]byte{}
	for i := range n {
		e := p[iccHeaderLen+12*i:]
		off, size := binary.BigEndian.Uint32(e[4:]), binary.BigEndian.Uint32(e[8:])
		if int(off+size) > len(p) {
			t.Fatalf("тег %q выходит за профиль", e[:4])
		}
		out[string(e[:4])] = p[off : off+size]
	}
	if len(out) != n {
		t.Fatalf("тегов в таблице %d, разных подписей %d", n, len(out))
	}
	return out
}

// s15 читает s15Fixed16 по смещению.
func s15(b []byte, off int) float64 {
	return float64(int32(binary.BigEndian.Uint32(b[off:]))) / 65536
}

// ParaTag пишет тип para: подпись, тип функции, параметры s15Fixed16 в порядке аргументов.
func TestParaTag(t *testing.T) {
	b := ParaTag(3, SRGBPara3...)
	if string(b[:4]) != "para" || binary.BigEndian.Uint16(b[8:]) != 3 || len(b) != 12+4*5 {
		t.Fatalf("para: % x", b)
	}
	for i, want := range SRGBPara3 {
		if got := s15(b, 12+4*i); math.Abs(got-want) > 1e-5 {
			t.Errorf("параметр %d: %v, ждали %v", i, got, want)
		}
	}
	// без параметров и с отрицательным — края s15Fixed16
	if b0 := ParaTag(0); len(b0) != 12 {
		t.Errorf("пустая para: %d байт", len(b0))
	}
	if got := s15(ParaTag(1, -1.5), 12); got != -1.5 {
		t.Errorf("отрицательный параметр: %v", got)
	}
}

// TableTag: n точек f(i/(n-1)) в 16 бит; значения вне 0…1 зажимаются, n=2 — две точки края.
func TestTableTag(t *testing.T) {
	b := TableTag(5, func(x float64) float64 { return x * 2 }) // выше 1 — зажим
	if string(b[:4]) != "curv" || binary.BigEndian.Uint32(b[8:]) != 5 || len(b) != 12+2*5 {
		t.Fatalf("curv: % x", b)
	}
	want := []uint16{0, 32768, 65535, 65535, 65535}
	for i, w := range want {
		if got := binary.BigEndian.Uint16(b[12+2*i:]); got != w {
			t.Errorf("точка %d: %d, ждали %d", i, got, w)
		}
	}
	low := TableTag(2, func(float64) float64 { return -3 })
	if binary.BigEndian.Uint16(low[12:]) != 0 || binary.BigEndian.Uint16(low[14:]) != 0 {
		t.Errorf("отрицательное не зажато: % x", low)
	}
}

func TestLinearTag(t *testing.T) {
	b := LinearTag()
	if string(b[:4]) != "curv" || binary.BigEndian.Uint32(b[8:]) != 0 || len(b) != 12 {
		t.Errorf("тождество: % x", b)
	}
}

// Все сборщики профилей: тегов ровно столько, у каждого канала своя кривая нужного вида.
func TestICCProfiles(t *testing.T) {
	desc := DescV2("тест")
	cases := []struct {
		name    string
		p       []byte
		trc     string // подпись типа кривой
		tags    int
		wantA2B bool
	}{
		{"para", ParaProfile(XYZP3Exact, 3, SRGBPara3, desc), "para", 7, false},
		{"table", TableProfile(XYZProPhoto, 256, SRGBEOTF, desc), "curv", 7, false},
		{"linear", LinearProfile(XYZLinearSRGB, desc), "curv", 7, false},
		{"lut", LUTProfile(XYZRec709Exact, desc), "curv", 8, true},
		{"srgb", SRGBProfile(desc), "para", 7, false},
		{"xiaomi v2", ProfileXiaomiP3(false), "para", 7, false},
		{"xiaomi v4", ProfileXiaomiP3(true), "para", 7, false},
	}
	for _, c := range cases {
		tags := iccTagMap(t, c.p, "RGB ")
		if len(tags) != c.tags {
			t.Errorf("%s: тегов %d, ждали %d", c.name, len(tags), c.tags)
		}
		for _, ch := range []string{"rTRC", "gTRC", "bTRC"} {
			if string(tags[ch][:4]) != c.trc {
				t.Errorf("%s: %s = %q, ждали %q", c.name, ch, tags[ch][:4], c.trc)
			}
		}
		for _, ch := range []string{"rXYZ", "gXYZ", "bXYZ"} {
			if string(tags[ch][:4]) != "XYZ " || len(tags[ch]) != 20 {
				t.Errorf("%s: %s: % x", c.name, ch, tags[ch])
			}
		}
		if _, has := tags["A2B0"]; has != c.wantA2B {
			t.Errorf("%s: A2B0 есть=%v, ждали %v", c.name, has, c.wantA2B)
		}
	}
}

// Колоранты ложатся в профиль теми же числами: красный X из rXYZ совпадает с константой.
func TestICCProfileColorants(t *testing.T) {
	for name, xyz := range map[string][9]float64{"p3": XYZP3Exact, "prophoto": XYZProPhoto, "linear": XYZLinearSRGB, "rec709": XYZRec709Exact} {
		tags := iccTagMap(t, LinearProfile(xyz, DescV2(name)), "RGB ")
		for i, sig := range []string{"rXYZ", "gXYZ", "bXYZ"} {
			for k := range 3 {
				if got := s15(tags[sig], 8+4*k); math.Abs(got-xyz[3*i+k]) > 1e-4 {
					t.Errorf("%s %s[%d]: %v, ждали %v", name, sig, k, got, xyz[3*i+k])
				}
			}
		}
	}
	// строки колорантов из целых s15Fixed16: сумма Y трёх колорантов sRGB D50 даёт белую 1.0
	if y := XYZLinearSRGB[1] + XYZLinearSRGB[4] + XYZLinearSRGB[7]; math.Abs(y-1) > 1e-3 {
		t.Errorf("сумма Y колорантов %v", y)
	}
}

// Профиль Xiaomi: имя в desc читается обратно и в v2 (ASCII), и в v4 (UTF-16BE).
func TestProfileXiaomiName(t *testing.T) {
	v2 := iccTagMap(t, ProfileXiaomiP3(false), "RGB ")["desc"]
	if n := binary.BigEndian.Uint32(v2[8:]); string(v2[12:12+n-1]) != XiaomiP3Name {
		t.Errorf("desc v2: %q", v2[12:])
	}
	v4 := iccTagMap(t, ProfileXiaomiP3(true), "RGB ")["desc"]
	if string(v4[:4]) != "mluc" {
		t.Fatalf("desc v4: %q", v4[:4])
	}
	n, off := binary.BigEndian.Uint32(v4[20:]), binary.BigEndian.Uint32(v4[24:])
	u := make([]uint16, n/2)
	for i := range u {
		u[i] = binary.BigEndian.Uint16(v4[int(off)+2*i:])
	}
	if got := string(utf16.Decode(u)); got != XiaomiP3Name {
		t.Errorf("desc v4: %q", got)
	}
}

// Кривые: sRGB и Rec. 709 в опорных точках, монотонность, края 0 и 1.
func TestEOTF(t *testing.T) {
	for name, f := range map[string]func(float64) float64{"srgb": SRGBEOTF, "rec709": Rec709EOTF} {
		if f(0) != 0 || math.Abs(f(1)-1) > 1e-9 {
			t.Errorf("%s: края %v и %v", name, f(0), f(1))
		}
		prev := -1.0
		for i := 0; i <= 100; i++ {
			v := f(float64(i) / 100)
			if v <= prev && i > 0 || v < 0 || v > 1+1e-9 {
				t.Fatalf("%s: %d/100 → %v после %v", name, i, v, prev)
			}
			prev = v
		}
	}
	// участок прямой (ниже излома) и участок степени; sRGB 0.5 → 0.2140
	if got := SRGBEOTF(0.04); math.Abs(got-0.04/12.92) > 1e-12 {
		t.Errorf("sRGB линейный участок: %v", got)
	}
	if got := SRGBEOTF(0.5); math.Abs(got-0.2140) > 1e-3 {
		t.Errorf("sRGB(0.5)=%v", got)
	}
	if got := Rec709EOTF(0.045); math.Abs(got-0.01) > 1e-12 {
		t.Errorf("Rec709 линейный участок: %v", got)
	}
	if got := Rec709EOTF(0.5); math.Abs(got-0.2596) > 1e-3 {
		t.Errorf("Rec709(0.5)=%v", got)
	}
	// таблица из sRGB-кривой даёт ту же 0.5 в середине (±1/65535 округления)
	tb := TableTag(3, SRGBEOTF)
	if got := float64(binary.BigEndian.Uint16(tb[14:])) / 65535; math.Abs(got-SRGBEOTF(0.5)) > 1e-4 {
		t.Errorf("таблица: %v", got)
	}
}
