package testkit

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// exifTags разбирает APP1 из ExifTaken независимо от генератора: тег → строка.
func exifTags(t *testing.T, seg []byte) map[uint16]string {
	t.Helper()
	if len(seg) < 4 || seg[0] != 0xFF || seg[1] != 0xE1 {
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
	if string(tf[:2]) == "MM" {
		bo = binary.BigEndian
	}
	ifd0 := bo.Uint32(tf[4:])
	if bo.Uint16(tf[ifd0:]) != 1 || bo.Uint16(tf[ifd0+2:]) != 0x8769 {
		t.Fatal("IFD0 без указателя на Exif IFD")
	}
	exif := bo.Uint32(tf[ifd0+10:])
	n := int(bo.Uint16(tf[exif:]))
	got := map[uint16]string{}
	for i := range n {
		e := tf[int(exif)+2+12*i:]
		off, cnt := bo.Uint32(e[8:]), bo.Uint32(e[4:])
		s := tf[off : off+cnt]
		if s[len(s)-1] != 0 {
			t.Fatalf("строка тега %#x без NUL", bo.Uint16(e))
		}
		got[bo.Uint16(e)] = string(s[:len(s)-1])
	}
	return got
}

func TestExifTaken(t *testing.T) {
	cases := []struct {
		name       string
		orig, digi string
		want       map[uint16]string
	}{
		{"обе", "2024:06:15 14:30:05", "2024:06:15 14:30:06", map[uint16]string{0x9003: "2024:06:15 14:30:05", 0x9004: "2024:06:15 14:30:06"}},
		{"только оригинал", "2024:06:15 14:30:05", "", map[uint16]string{0x9003: "2024:06:15 14:30:05"}},
		{"только оцифровка", "", "x", map[uint16]string{0x9004: "x"}},
		{"пусто", "", "", map[uint16]string{}},
		{"мусор", "мусор", "0000:00:00 00:00:00", map[uint16]string{0x9003: "мусор", 0x9004: "0000:00:00 00:00:00"}},
	}
	for _, be := range []bool{false, true} {
		for _, c := range cases {
			got := exifTags(t, ExifTaken(be, c.orig, c.digi))
			if len(got) != len(c.want) {
				t.Fatalf("%s be=%v: %v, ждали %v", c.name, be, got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("%s be=%v: тег %#x = %q, ждали %q", c.name, be, k, got[k], v)
				}
			}
		}
	}
}

func TestExifTakenByteOrder(t *testing.T) {
	if s := ExifTaken(false, "a", ""); string(s[10:14]) != "II*\x00" {
		t.Errorf("LE: % x", s[10:14])
	}
	if s := ExifTaken(true, "a", ""); string(s[10:14]) != "MM\x00*" {
		t.Errorf("BE: % x", s[10:14])
	}
}
