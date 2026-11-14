package imgfile

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func readShot(f testkit.ShotFields, be bool) Shot {
	return ReadShot(bytes.NewReader(testkit.RawJPEG(testkit.ExifShot(be, f))))
}

var fullShot = testkit.ShotFields{
	Make: "Canon", Model: "Canon EOS R6", Lens: "RF24-70mm F2.8 L IS USM",
	Exposure: [2]uint32{1, 200}, FNumber: [2]uint32{28, 10}, Focal: [2]uint32{50, 1},
	ISO: 400, Orig: "2026:06:12 14:32:00",
}

// Все поля из IFD0 и Exif IFD, оба порядка байт; ISO типа LONG; частичный набор; несколько
// файлов сразу дают независимые результаты.
func TestShotFields(t *testing.T) {
	want := Shot{
		Make: "Canon", Model: "Canon EOS R6", Lens: "RF24-70mm F2.8 L IS USM",
		Exposure: [2]uint32{1, 200}, FNumber: [2]uint32{28, 10}, Focal: [2]uint32{50, 1},
		ISO: 400, Taken: unix(2026, 6, 12, 14, 32, 0),
	}
	for _, be := range []bool{false, true} {
		if got := readShot(fullShot, be); got != want {
			t.Errorf("BE=%v: %+v, ждали %+v", be, got, want)
		}
		long := fullShot
		long.ISOLong = true
		if got := readShot(long, be); got != want {
			t.Errorf("BE=%v, ISO LONG: %+v, ждали %+v", be, got, want)
		}
		// только часть тегов: остальное нулевое
		part := testkit.ShotFields{Model: "X100V", ISO: 200}
		if got := readShot(part, be); got != (Shot{Model: "X100V", ISO: 200}) {
			t.Errorf("BE=%v, часть: %+v", be, got)
		}
		if got := readShot(testkit.ShotFields{}, be); !got.Empty() {
			t.Errorf("BE=%v, пустой EXIF: %+v", be, got)
		}
	}
	// два файла разными камерами в одном процессе не смешиваются
	a := readShot(testkit.ShotFields{Make: "A", Model: "A1"}, false)
	b := readShot(testkit.ShotFields{Make: "B", Model: "B1"}, true)
	if a.Model != "A1" || b.Model != "B1" {
		t.Errorf("a=%+v b=%+v", a, b)
	}
}

// Рациональные: знаменатель 0, смещение за буфером (и переполнение uint32), Count ≠ 1 — поля нет;
// остальные поля при этом читаются.
func TestShotRational(t *testing.T) {
	for _, be := range []bool{false, true} {
		zero := fullShot
		zero.Exposure = [2]uint32{1, 0}
		zero.FNumber = [2]uint32{0, 0} // не пишется вовсе
		got := readShot(zero, be)
		if got.Exposure != ([2]uint32{}) || got.Focal != ([2]uint32{50, 1}) || got.ISO != 400 {
			t.Errorf("BE=%v знаменатель 0: %+v", be, got)
		}
		for _, n := range []uint32{2, 1 << 31} { // Count 0 в построителе значит «один»
			c := fullShot
			c.RatCount = n
			got = readShot(c, be)
			if got.Exposure != ([2]uint32{}) || got.FNumber != ([2]uint32{}) || got.Focal != ([2]uint32{}) {
				t.Errorf("BE=%v Count=%d: %+v", be, n, got)
			}
			if got.ISO != 400 || got.Model == "" {
				t.Errorf("BE=%v Count=%d: прочие поля потеряны: %+v", be, n, got)
			}
		}
		// смещение значения: за буфером, у самого края (off+8 > len), 0xFFFFFFFF, внутрь заголовка
		seg := testkit.ExifShot(be, testkit.ShotFields{Exposure: [2]uint32{1, 200}})
		tf := seg[10:]
		var bo binary.ByteOrder = binary.LittleEndian
		if be {
			bo = binary.BigEndian
		}
		ifd0 := bo.Uint32(tf[4:])
		exif := bo.Uint32(tf[ifd0+2+8:]) // указатель IFD0 — единственная запись
		valAt := int(exif) + 2 + 8       // поле значения первой записи Exif IFD
		for _, off := range []uint32{uint32(len(tf)), uint32(len(tf)) - 7, 0xFFFFFFFF, 0xFFFFFFF9} {
			bad := bytes.Clone(tf)
			bo.PutUint32(bad[valAt:], off)
			if got := parseShot(bad); got.Exposure != ([2]uint32{}) {
				t.Errorf("BE=%v смещение %#x: %+v", be, off, got)
			}
		}
		// последний допустимый: ровно впритык к концу буфера
		bad := bytes.Clone(tf)
		bo.PutUint32(bad[valAt:], uint32(len(tf))-exifRatLen)
		if got := parseShot(bad); got.Exposure[1] == 0 {
			t.Errorf("BE=%v впритык к краю: поле потеряно: %+v", be, got)
		}
	}
}

// Строки: NUL в конце и внутри, управляющие знаки, не UTF-8, длинные, пробелы, смена направления.
func TestShotStrings(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"обычная", "Canon", "Canon"},
		{"NUL в конце", "Canon\x00\x00", "Canon"},
		{"NUL внутри — срез", "Canon\x00EOS", "Canon"},
		{"управляющие", "Ca\tno\nn\x7f R6\x1b", "Ca no n R6"},
		{"пробелы", "  Canon   EOS   R6  ", "Canon EOS R6"},
		{"не UTF-8", "Can\xff\xfeon", "Canon"},
		{"смена направления", "Ca\u202enon", "Ca non"},
		{"кириллица", "Объектив 24–70", "Объектив 24–70"},
		{"только мусор", "\x01\x02\x03", ""},
		{"длинная", strings.Repeat("a", 200), strings.Repeat("a", shotMaxText)},
		{"длинная кириллица — по рунам", strings.Repeat("ж", 200), strings.Repeat("ж", shotMaxText)},
		{"обрезка по пробелу", strings.Repeat("a", shotMaxText-1) + " bbb", strings.Repeat("a", shotMaxText-1)},
	}
	for _, c := range cases {
		for _, be := range []bool{false, true} {
			got := readShot(testkit.ShotFields{Make: c.in, Lens: c.in}, be)
			if got.Make != c.want || got.Lens != c.want {
				t.Errorf("%s (BE=%v): %q / %q, ждали %q", c.name, be, got.Make, got.Lens, c.want)
			}
			if !utf8.ValidString(got.Make) {
				t.Errorf("%s: не UTF-8: %q", c.name, got.Make)
			}
		}
	}
	// короткие строки лежат прямо в записи (до 4 байт с NUL)
	if got := readShot(testkit.ShotFields{Make: "Abc"}, false); got.Make != "Abc" {
		t.Errorf("в записи: %q", got.Make)
	}
	// первый тег на уровне побеждает: два Make подряд
	tf := testkit.ExifShot(false, testkit.ShotFields{Make: "One", Model: "Two"})[10:]
	binary.LittleEndian.PutUint16(tf[8+2+12:], 0x010F) // тег второй записи IFD0 тоже Make
	if got := parseShot(tf); got.Make != "One" || got.Model != "" {
		t.Errorf("первый тег: %q / %q", got.Make, got.Model)
	}
}

// Не JPEG и обрывки: пусто, без паники.
func TestShotNotJPEG(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	for name, data := range map[string][]byte{
		"PNG": png, "пусто": nil, "один байт": {0xFF}, "SOI без сегментов": {0xFF, 0xD8},
		"JPEG без EXIF":     testkit.RawJPEG(),
		"мусор вместо Exif": testkit.RawJPEG(testkit.Segment(0xE1, []byte("Exif\x00\x00garbage-not-tiff"))),
	} {
		if got := ReadShot(bytes.NewReader(data)); !got.Empty() {
			t.Errorf("%s: %+v", name, got)
		}
		if _, got := HeaderShot(bytes.NewReader(data)); !got.Empty() {
			t.Errorf("%s (HeaderShot): %+v", name, got)
		}
	}
	// HeaderShot — то же, что Header и ReadShot по отдельности
	data := testkit.RawJPEG(testkit.ExifShot(false, fullShot))
	h, s := HeaderShot(bytes.NewReader(data))
	if h.Taken != s.Taken || s != readShot(fullShot, false) || h.Orient != 1 {
		t.Errorf("HeaderShot: %+v %+v", h, s)
	}
}

// Недоверенный EXIF: без паники и выхода за буфер, строки ≤ 64 рун и валидный UTF-8,
// знаменатели ≠ 0, ISO в пределах.
func FuzzShot(f *testing.F) {
	for _, be := range []bool{false, true} {
		f.Add(testkit.ExifShot(be, fullShot)[10:])
		f.Add(testkit.ExifShot(be, testkit.ShotFields{Make: "a\x00b\xff", Lens: strings.Repeat("ж", 100), ISO: 100, ISOLong: true})[10:])
		f.Add(testkit.ExifShot(be, testkit.ShotFields{Exposure: [2]uint32{1, 0}, RatCount: 5})[10:])
	}
	f.Add(testkit.ExifTaken(false, "2024:06:15 14:30:05", "")[10:])
	f.Add([]byte("II*\x00\x08\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, tf []byte) {
		s := parseShot(tf)
		for _, v := range []string{s.Make, s.Model, s.Lens} {
			if utf8.RuneCountInString(v) > shotMaxText || !utf8.ValidString(v) || v != strings.TrimSpace(v) {
				t.Fatalf("строка: %q", v)
			}
		}
		for _, r := range [][2]uint32{s.Exposure, s.FNumber, s.Focal} {
			if r != ([2]uint32{}) && r[1] == 0 {
				t.Fatalf("знаменатель 0: %v", r)
			}
		}
		if s.ISO < 0 || s.ISO > shotMaxISO {
			t.Fatalf("ISO: %d", s.ISO)
		}
		// тот же разбор из сегмента целиком
		ReadShot(bytes.NewReader(testkit.RawJPEG(testkit.Segment(0xE1, append([]byte("Exif\x00\x00"), tf...)))))
	})
}
