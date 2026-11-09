package imgfile

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func unix(y int, mo time.Month, d, h, mi, s int) int64 {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC).Unix()
}

// tiff — TIFF-часть APP1 (без маркера, длины и «Exif\0\0»).
func tiff(seg []byte) []byte { return seg[4+6:] }

func TestHeaderTaken(t *testing.T) {
	want := unix(2024, 6, 15, 14, 30, 5)
	const ok, other = "2024:06:15 14:30:05", "2020:01:02 03:04:05"
	cases := []struct {
		name       string
		orig, digi string
		want       int64
	}{
		{"оригинал", ok, "", want},
		{"только оцифровка", "", ok, want},
		{"оригинал важнее оцифровки", ok, other, want},
		{"мусор в оригинале — берётся оцифровка", "abcdefghijklmnopqrs", ok, want},
		{"нет дат", "", "", 0},
		{"нули камеры", "0000:00:00 00:00:00", "", 0},
		{"пустые поля камеры", "    :  :     :  :  ", "", 0},
		{"30 февраля", "2024:02:30 10:00:00", "", 0},
		{"29 февраля високосного", "2024:02:29 10:00:00", "", unix(2024, 2, 29, 10, 0, 0)},
		{"29 февраля невисокосного", "2023:02:29 10:00:00", "", 0},
		{"месяц 13", "2024:13:01 10:00:00", "", 0},
		{"час 24", "2024:06:15 24:00:00", "", 0},
		{"секунда 60", "2024:06:15 10:00:60", "", 0},
		{"год 1899", "1899:12:31 23:59:59", "", 0},
		{"год 2201", "2201:01:01 00:00:00", "", 0},
		{"год 2200", "2200:01:01 00:00:00", "", unix(2200, 1, 1, 0, 0, 0)},
		{"до 1970 — не время съёмки", "1960:01:01 00:00:00", "", 0},
		{"другие разделители", "2024-06-15 14:30:05", "", 0},
		{"короче 19", "2024:06:15 14:30", "", 0},
		{"длиннее 19", "2024:06:15 14:30:05.5", "", 0},
		{"минус в числе", "2024:06:-5 14:30:05", "", 0},
	}
	for _, c := range cases {
		for _, be := range []bool{false, true} {
			seg := testkit.ExifTaken(be, c.orig, c.digi)
			h := Header(bytes.NewReader(testkit.RawJPEG(seg)))
			if h.Taken != c.want {
				t.Errorf("%s (BE=%v): %d, ждали %d", c.name, be, h.Taken, c.want)
			}
		}
	}
}

// Нет EXIF вообще, пустой EXIF, EXIF без Exif IFD: время 0, остальное как прежде.
func TestHeaderTakenAbsent(t *testing.T) {
	for name, raw := range map[string][]byte{
		"нет сегментов":               testkit.RawJPEG(),
		"EXIF с ориентацией, без дат": testkit.RawJPEG(testkit.ExifColor(false, 6, 0xFFFF, "R03")),
		"не JPEG": []byte("не картинка"),
		"пусто":   nil,
	} {
		if h := Header(bytes.NewReader(raw)); h.Taken != 0 {
			t.Errorf("%s: %d", name, h.Taken)
		}
	}
	h := Header(bytes.NewReader(testkit.RawJPEG(testkit.ExifColor(false, 6, 0xFFFF, "R03"))))
	if h.Orient != 6 || h.Color != ColorAdobe {
		t.Errorf("прежние поля: %+v", h)
	}
}

// Две камеры с разными часами: у каждого файла своё время, порядок разбора на него не влияет.
func TestHeaderTakenTwoCameras(t *testing.T) {
	a := Header(bytes.NewReader(testkit.RawJPEG(testkit.ExifTaken(false, "2024:06:15 14:30:05", ""))))
	b := Header(bytes.NewReader(testkit.RawJPEG(testkit.ExifTaken(true, "2024:06:15 14:28:00", ""))))
	if a.Taken-b.Taken != 125 {
		t.Errorf("сдвиг камер: %d и %d", a.Taken, b.Taken)
	}
}

// Враждебный EXIF: смещения вне буфера, усечение, огромное число записей — без паники и без чужих байт.
func TestHeaderTakenHostile(t *testing.T) {
	seg := testkit.ExifTaken(false, "2024:06:15 14:30:05", "")
	good := tiff(seg)
	if exifTaken(good) == 0 {
		t.Fatal("исходный EXIF не разобран")
	}
	for n := range len(good) {
		exifTaken(good[:n]) // усечение не паникует
	}
	// строка времени за пределами буфера: смещение 0xFFFFFFFF, смещение у самого края
	for _, off := range []uint32{0xFFFFFFFF, uint32(len(good)) - 19, uint32(len(good))} {
		bad := bytes.Clone(good)
		binary.LittleEndian.PutUint32(bad[8+2+12+4+2+8:], off) // поле значения первой записи Exif IFD
		if got := exifTaken(bad); got != 0 {
			t.Errorf("смещение %d: %d", off, got)
		}
	}
	// указатель на Exif IFD вне буфера, внутрь заголовка, на самого себя (цикл)
	for _, off := range []uint32{0xFFFFFFFF, 0, 4, 8, uint32(len(good))} {
		bad := bytes.Clone(good)
		binary.LittleEndian.PutUint32(bad[8+2+8:], off)
		exifTaken(bad)
	}
	// IFD0 заявляет 65535 записей: обход ограничен пределом и буфером
	bad := bytes.Clone(good)
	binary.LittleEndian.PutUint16(bad[8:], 0xFFFF)
	exifTaken(bad)
	var n int
	exifWalk(bad, func(int, exifEntry) { n++ })
	if n > exifMaxEntries*3 {
		t.Errorf("записей обойдено: %d", n)
	}
}

func TestExifWalkEntryCap(t *testing.T) {
	// IFD0 в 64 КБ нулевых записей с заявленными 0xFFFF
	buf := make([]byte, 8+2+0xFFFF*12)
	copy(buf, "II*\x00")
	binary.LittleEndian.PutUint32(buf[4:], 8)
	binary.LittleEndian.PutUint16(buf[8:], 0xFFFF)
	var n int
	exifWalk(buf, func(level int, _ exifEntry) {
		if level != exifLevelIFD0 {
			t.Errorf("уровень %d", level)
		}
		n++
	})
	if n != exifMaxEntries {
		t.Errorf("обошли %d записей, предел %d", n, exifMaxEntries)
	}
}

func TestParseExifTimeNoNUL(t *testing.T) {
	if parseExifTime([]byte("2024:06:15 14:30:05 ")) != 0 {
		t.Error("без NUL в конце — мусор")
	}
	if parseExifTime(nil) != 0 || parseExifTime(make([]byte, 20)) != 0 {
		t.Error("пусто и нули — мусор")
	}
}

// interopTIFF — TIFF с одной записью InteropIndex: тип ASCII, заданные Count и 4 байта поля.
func interopTIFF(count uint32, field string) []byte {
	d := tiff(testkit.ExifColor(false, 0, 0xFFFF, "R03"))
	const e = 8 + (2 + 12 + 4) + (2 + 2*12 + 4) + 2 // первая запись Interop IFD: после IFD0 и Exif IFD
	binary.LittleEndian.PutUint32(d[e+4:], count)
	copy(d[e+8:e+12], field+"\x00\x00\x00\x00")
	return d
}

// InteropIndex разбирается строго по Count: 0 — не Adobe (в 0.12 читалось поле целиком), 1–3 —
// первые Count байт, 4 — «R03» + NUL. Испорченные файлы с «R03» и неверным Count — не Adobe.
func TestExifInteropCount(t *testing.T) {
	if got := exifInterop(interopTIFF(4, "R03\x00")); got != "R03" {
		t.Fatalf("исходный: %q", got)
	}
	for _, c := range []struct {
		count uint32
		field string
		want  string
	}{
		{0, "R03\x00", ""},
		{1, "R03\x00", "R"},
		{2, "R03\x00", "R0"},
		{3, "R03\x00", "R03"},
		{4, "R03\x00", "R03"},
		{4, "R03X", "R03X"},
		{5, "R03\x00", ""}, // больше 4 байт — по смещению, не разбирается
	} {
		if got := exifInterop(interopTIFF(c.count, c.field)); got != c.want {
			t.Errorf("Count=%d %q: %q, ждали %q", c.count, c.field, got, c.want)
		}
	}
}

// Ориентация идёт через exifWalk: первая запись 0x0112 в IFD0, значение вне 1–8 — 0.
func TestExifOrientationValues(t *testing.T) {
	for _, be := range []bool{false, true} {
		for v, want := range map[int]int{1: 1, 6: 6, 8: 8, 9: 0, 255: 0} {
			if got := exifOrientation(testkit.ExifColor(be, v, 0, "")[4:]); got != want {
				t.Errorf("BE=%v %d: %d, ждали %d", be, v, got, want)
			}
		}
		if got := exifOrientation(testkit.ExifTaken(be, "2024:06:15 14:30:05", "")[4:]); got != 0 {
			t.Errorf("BE=%v без тега: %d", be, got)
		}
	}
	if exifOrientation([]byte("Exif\x00\x00II*")) != 0 || exifOrientation(nil) != 0 {
		t.Error("обрубок и пусто — 0")
	}
}

// Указатель Exif/Interop IFD на IFD0: дата из IFD0 не читается как DateTimeOriginal.
func TestExifWalkSelfPointer(t *testing.T) {
	// IFD0 (смещение 8): указатель 0x8769 на себя и DateTimeOriginal с корректной строкой.
	d := []byte("II*\x00")
	d = binary.LittleEndian.AppendUint32(d, 8)
	d = binary.LittleEndian.AppendUint16(d, 2)
	ent := func(tag, typ uint16, count, val uint32) {
		d = binary.LittleEndian.AppendUint16(d, tag)
		d = binary.LittleEndian.AppendUint16(d, typ)
		d = binary.LittleEndian.AppendUint32(d, count)
		d = binary.LittleEndian.AppendUint32(d, val)
	}
	ent(0x8769, 4, 1, 8)
	ent(0x9003, 2, exifTakenLen, 8+2+2*12+4)
	d = binary.LittleEndian.AppendUint32(d, 0)
	d = append(d, "2024:06:15 14:30:05"...)
	d = append(d, 0)
	if got := exifTaken(d); got != 0 {
		t.Errorf("Exif IFD = IFD0: %d", got)
	}
	// Interop IFD на IFD0 и на Exif IFD: InteropIndex оттуда не берётся
	for _, off := range []uint32{8, 26} {
		c := tiff(testkit.ExifColor(false, 0, 0xFFFF, "R03"))
		binary.LittleEndian.PutUint32(c[8+(2+12+4)+2+12+8:], off) // поле значения 0xA005 — вторая запись Exif IFD
		if got := exifInterop(c); got != "" {
			t.Errorf("Interop IFD на %d: %q", off, got)
		}
	}
}

// Время съёмки из TIFF-части напрямую: затравки EXIF, без паники, результат в календарных пределах.
func FuzzExifTaken(f *testing.F) {
	for _, be := range []bool{false, true} {
		f.Add(tiff(testkit.ExifTaken(be, "2024:06:15 14:30:05", "2020:01:02 03:04:05")))
		f.Add(tiff(testkit.ExifTaken(be, "", "0000:00:00 00:00:00")))
	}
	f.Add(tiff(testkit.ExifColor(false, 6, 0xFFFF, "R03")))
	f.Add(interopTIFF(4, "R03\x00"))
	f.Add([]byte("II*\x00\x08\x00\x00\x00\xff\xff"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := exifTaken(data)
		if got < 0 || got > unix(2200, 12, 31, 23, 59, 59) {
			t.Fatalf("вне пределов: %d", got)
		}
		exifInterop(data)
		if bytes.Equal(data, interopTIFF(4, "R03\x00")) && exifInterop(data) != "R03" {
			t.Fatal("затравка Count=4 «R03» не разобрана")
		}
	})
}
