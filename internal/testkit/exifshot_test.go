package testkit

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// shotEntryOut — запись IFD, разобранная независимо от построителя.
type shotEntryOut struct {
	typ   uint16
	count uint32
	val   []byte // данные записи (из поля значения или по смещению)
}

// parseShot разбирает APP1 из ExifShot: IFD0 и Exif IFD (тег → запись).
func parseShot(t *testing.T, seg []byte) (ifd0, exif map[uint16]shotEntryOut) {
	t.Helper()
	if len(seg) < 12 || seg[0] != 0xFF || seg[1] != 0xE1 {
		t.Fatalf("не APP1: % x", seg[:min(4, len(seg))])
	}
	if int(binary.BigEndian.Uint16(seg[2:]))+2 != len(seg) {
		t.Fatalf("длина сегмента %d не сходится с %d", binary.BigEndian.Uint16(seg[2:]), len(seg))
	}
	if !bytes.HasPrefix(seg[4:], []byte("Exif\x00\x00")) {
		t.Fatal("нет заголовка Exif")
	}
	tf := seg[10:]
	var bo binary.ByteOrder = binary.LittleEndian
	switch string(tf[:4]) {
	case "II*\x00":
	case "MM\x00*":
		bo = binary.BigEndian
	default:
		t.Fatalf("заголовок TIFF: % x", tf[:4])
	}
	if bo.Uint32(tf[4:]) != 8 {
		t.Fatalf("смещение IFD0 = %d, нужно 8", bo.Uint32(tf[4:]))
	}
	sizes := map[uint16]int{2: 1, 3: 2, 4: 4, 5: 8}
	table := func(off uint32) (map[uint16]shotEntryOut, uint32) {
		n := uint32(bo.Uint16(tf[off:]))
		out := map[uint16]shotEntryOut{}
		prev := -1
		for i := range n {
			e := tf[off+2+12*i:]
			tag := bo.Uint16(e)
			if int(tag) <= prev {
				t.Fatalf("теги не по возрастанию: %#x после %#x", tag, prev)
			}
			prev = int(tag)
			r := shotEntryOut{typ: bo.Uint16(e[2:]), count: bo.Uint32(e[4:])}
			size := sizes[r.typ] * int(r.count)
			if r.typ == 5 {
				size = 8 // RatCount — намеренно «врущий» Count: данных всё равно одно число
			}
			if size <= 4 {
				r.val = e[8 : 8+size]
			} else {
				p := bo.Uint32(e[8:])
				r.val = tf[p : int(p)+size]
			}
			out[tag] = r
		}
		if next := bo.Uint32(tf[off+2+12*n:]); next != 0 {
			t.Fatalf("следующий IFD %d, нужно 0", next)
		}
		return out, off + 2 + 12*n + 4
	}
	ifd0, end := table(8)
	ptr, ok := ifd0[0x8769]
	if !ok || ptr.typ != 4 || ptr.count != 1 {
		t.Fatalf("нет указателя на Exif IFD: %+v", ptr)
	}
	if off := bo.Uint32(ptr.val); off != end {
		t.Fatalf("Exif IFD по смещению %d, ожидали сразу за IFD0 (%d)", off, end)
	}
	exif, _ = table(end)
	return ifd0, exif
}

func TestExifShotFull(t *testing.T) {
	for _, big := range []bool{false, true} {
		var bo binary.ByteOrder = binary.LittleEndian
		if big {
			bo = binary.BigEndian
		}
		seg := ExifShot(big, ShotFields{
			Make: "Nikon", Model: "Z f", Lens: "50mm\x01\x00x",
			Exposure: [2]uint32{1, 250}, FNumber: [2]uint32{14, 10}, Focal: [2]uint32{500, 10},
			ISO: 400, Orig: "2026:10:20 12:00:00", Orient: 6,
		})
		ifd0, exif := parseShot(t, seg)
		if len(ifd0) != 4 || len(exif) != 6 {
			t.Fatalf("big=%v: записей IFD0 %d, Exif %d", big, len(ifd0), len(exif))
		}
		if got := string(ifd0[0x010F].val); got != "Nikon\x00" {
			t.Errorf("Make = %q", got)
		}
		if got := string(ifd0[0x0110].val); got != "Z f\x00" {
			t.Errorf("Model = %q", got)
		}
		if o := ifd0[0x0112]; o.typ != 3 || o.count != 1 || bo.Uint16(o.val) != 6 {
			t.Errorf("Orientation = %+v", o)
		}
		if got := string(exif[0xA434].val); got != "50mm\x01\x00x\x00" {
			t.Errorf("Lens = %q", got)
		}
		if got := string(exif[0x9003].val); got != "2026:10:20 12:00:00\x00" {
			t.Errorf("Orig = %q", got)
		}
		for tag, want := range map[uint16][2]uint32{0x829A: {1, 250}, 0x829D: {14, 10}, 0x920A: {500, 10}} {
			r := exif[tag]
			if r.typ != 5 || r.count != 1 || bo.Uint32(r.val) != want[0] || bo.Uint32(r.val[4:]) != want[1] {
				t.Errorf("big=%v тег %#x = %+v, ждали %v", big, tag, r, want)
			}
		}
		if iso := exif[0x8827]; iso.typ != 3 || bo.Uint16(iso.val) != 400 {
			t.Errorf("ISO = %+v", iso)
		}
	}
}

func TestExifShotEmpty(t *testing.T) {
	for _, big := range []bool{false, true} {
		ifd0, exif := parseShot(t, ExifShot(big, ShotFields{}))
		if len(ifd0) != 1 || len(exif) != 0 {
			t.Fatalf("big=%v: пустые поля дали IFD0 %d, Exif %d записей", big, len(ifd0), len(exif))
		}
	}
}

func TestExifShotISOLongAndRatCount(t *testing.T) {
	seg := ExifShot(false, ShotFields{
		ISO: 100000, ISOLong: true, RatCount: 3,
		Exposure: [2]uint32{1, 0}, // знаменатель 0 пишется как есть
	})
	_, exif := parseShot(t, seg)
	if iso := exif[0x8827]; iso.typ != 4 || binary.LittleEndian.Uint32(iso.val) != 100000 {
		t.Errorf("ISO LONG = %+v", iso)
	}
	r := exif[0x829A]
	if r.typ != 5 || r.count != 3 || len(r.val) != 8 || binary.LittleEndian.Uint32(r.val) != 1 ||
		binary.LittleEndian.Uint32(r.val[4:]) != 0 {
		t.Errorf("рациональное с Count=3 и знаменателем 0: %+v", r)
	}
}

func TestExifShotOnlyOne(t *testing.T) {
	// Каждое поле по отдельности: ровно одна запись в своей таблице.
	cases := []struct {
		name       string
		f          ShotFields
		in0, inExi int
	}{
		{"make", ShotFields{Make: "A"}, 1, 0},
		{"model", ShotFields{Model: "B"}, 1, 0},
		{"orient", ShotFields{Orient: 3}, 1, 0},
		{"exposure", ShotFields{Exposure: [2]uint32{0, 1}}, 0, 1},
		{"fnumber", ShotFields{FNumber: [2]uint32{1, 0}}, 0, 1},
		{"iso", ShotFields{ISO: 1}, 0, 1},
		{"orig", ShotFields{Orig: "x"}, 0, 1},
		{"focal", ShotFields{Focal: [2]uint32{35, 1}}, 0, 1},
		{"lens", ShotFields{Lens: "L"}, 0, 1},
	}
	for _, c := range cases {
		ifd0, exif := parseShot(t, ExifShot(false, c.f))
		if len(ifd0) != c.in0+1 || len(exif) != c.inExi {
			t.Errorf("%s: IFD0 %d (без указателя), Exif %d", c.name, len(ifd0)-1, len(exif))
		}
	}
}
