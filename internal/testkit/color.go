package testkit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
	"unicode/utf16"
)

// Колорант-матрицы в XYZ D50 (rX rY rZ gX gY gZ bX bY bZ) для крафтовых профилей.
var (
	XYZSRGB  = [9]float64{0.4361, 0.2225, 0.0139, 0.3851, 0.7169, 0.0971, 0.1431, 0.0606, 0.7141}
	XYZAdobe = [9]float64{0.6097, 0.3111, 0.0195, 0.2053, 0.6257, 0.0609, 0.1492, 0.0632, 0.7446}
	XYZP3    = [9]float64{0.5151, 0.2412, -0.0011, 0.2920, 0.6922, 0.0419, 0.1571, 0.0666, 0.7841}
)

const (
	iccHeaderLen   = 132 // заголовок профиля + число тегов
	iccTagEntryLen = 12
)

// Segment — сегмент JPEG: 0xFF, маркер, длина (с учётом самих двух байт), данные.
func Segment(marker byte, payload []byte) []byte {
	b := []byte{0xFF, marker}
	b = binary.BigEndian.AppendUint16(b, uint16(len(payload)+2)) //nolint:gosec // тестовые сегменты короче 64 КБ
	return append(b, payload...)
}

// ICCSegment — APP2 «ICC_PROFILE» с куском seq из count.
func ICCSegment(seq, count byte, data []byte) []byte {
	return Segment(0xE2, append([]byte("ICC_PROFILE\x00"+string([]byte{seq, count})), data...))
}

// RawJPEG собирает «JPEG»: SOI, переданные сегменты и SOS-заглушка. Для проверки разбора
// заголовка; декодер такой файл не откроет — настоящие файлы делает JPEGSegments.
func RawJPEG(segs ...[]byte) []byte {
	b := []byte{0xFF, 0xD8}
	for _, s := range segs {
		b = append(b, s...)
	}
	return append(b, 0xFF, 0xDA, 0x00, 0x02, 0x11, 0x22)
}

// JPEGSegments пишет настоящий JPEG w×h (градиент), вставив segs сразу после SOI.
func JPEGSegments(t testing.TB, dir, name string, w, h int, segs ...[]byte) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / max(w, 1)), uint8(y * 255 / max(h, 1)), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	out := append([]byte{}, data[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return write(t, dir, name, append(out, data[2:]...))
}

// order — порядок байт и для чтения, и для дописывания в срез.
type order interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// ExifColor — APP1 с EXIF: Orientation (orient > 0), ColorSpace (colorSpace != 0) и
// InteropIndex (interop != "", до 3 символов). Порядок байт — bigEndian или little.
func ExifColor(bigEndian bool, orient int, colorSpace uint16, interop string) []byte {
	var bo order = binary.LittleEndian
	t := []byte("II*\x00")
	if bigEndian {
		bo, t = binary.BigEndian, []byte("MM\x00*")
	}
	t = bo.AppendUint32(t, 8)
	// SHORT лежит в записи по левому краю, поэтому пишется как Uint16 в начало поля.
	short := func(tag, v uint16) []byte {
		e := make([]byte, 12)
		bo.PutUint16(e, tag)
		bo.PutUint16(e[2:], 3)
		bo.PutUint32(e[4:], 1)
		bo.PutUint16(e[8:], v)
		return e
	}
	long := func(tag uint16, v uint32) []byte {
		e := make([]byte, 12)
		bo.PutUint16(e, tag)
		bo.PutUint16(e[2:], 4)
		bo.PutUint32(e[4:], 1)
		bo.PutUint32(e[8:], v)
		return e
	}
	var ifd0, exif, inter [][]byte
	if orient > 0 {
		ifd0 = append(ifd0, short(0x0112, uint16(orient))) //nolint:gosec // 1..8
	}
	hasExif := colorSpace != 0 || interop != ""
	size := func(n int) uint32 { return uint32(2 + 12*n + 4) } //nolint:gosec // записей единицы
	exifOff := 8 + size(len(ifd0)+btoi(hasExif))
	if hasExif {
		ifd0 = append(ifd0, long(0x8769, exifOff))
	}
	if colorSpace != 0 {
		exif = append(exif, short(0xA001, colorSpace))
	}
	if interop != "" {
		exif = append(exif, long(0xA005, exifOff+size(len(exif)+1)))
		e := make([]byte, 12)
		bo.PutUint16(e, 0x0001)
		bo.PutUint16(e[2:], 2)
		bo.PutUint32(e[4:], uint32(len(interop)+1)) //nolint:gosec // единицы
		copy(e[8:], interop)
		inter = append(inter, e)
	}
	for _, ifd := range [][][]byte{ifd0, exif, inter} {
		if len(ifd) == 0 {
			continue
		}
		t = bo.AppendUint16(t, uint16(len(ifd))) //nolint:gosec // единицы
		for _, e := range ifd {
			t = append(t, e...)
		}
		t = bo.AppendUint32(t, 0)
	}
	return Segment(0xE1, append([]byte("Exif\x00\x00"), t...))
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ICCTag — тег профиля: подпись и данные.
type ICCTag struct {
	Sig  string
	Data []byte
}

func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// XYZTag — тип XYZ с тремя числами s15Fixed16.
func XYZTag(v ...float64) []byte {
	b := []byte("XYZ \x00\x00\x00\x00")
	for _, x := range v {
		b = append(b, be32(uint32(int32(math.Round(x*65536))))...) //nolint:gosec // s15Fixed16
	}
	return b
}

// CurvTag — кривая curv из одного числа (гамма u8Fixed8).
func CurvTag(g float64) []byte {
	b := append([]byte("curv\x00\x00\x00\x00"), be32(1)...)
	return binary.BigEndian.AppendUint16(b, uint16(math.Round(g*256))) //nolint:gosec // u8Fixed8
}

// DescV2 — описание профиля ICC v2 (тип desc, ASCII).
func DescV2(s string) []byte {
	b := append([]byte("desc\x00\x00\x00\x00"), be32(uint32(len(s)+1))...) //nolint:gosec // короткие строки
	return append(append(b, s...), 0)
}

// DescV4 — описание профиля ICC v4 (тип mluc, одна запись UTF-16BE).
func DescV4(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := append([]byte("mluc\x00\x00\x00\x00"), be32(1)...)
	b = append(b, be32(12)...)
	b = append(b, "enUS"...)
	b = append(b, be32(uint32(len(u)*2))...) //nolint:gosec // короткие строки
	b = append(b, be32(28)...)
	for _, c := range u {
		b = binary.BigEndian.AppendUint16(b, c)
	}
	return b
}

// BuildICC собирает профиль в памяти: заголовок, таблица тегов, данные.
func BuildICC(space string, tags ...ICCTag) []byte {
	hdr := make([]byte, iccHeaderLen)
	copy(hdr[16:], space)
	copy(hdr[36:], "acsp")
	binary.BigEndian.PutUint32(hdr[128:], uint32(len(tags))) //nolint:gosec // единицы
	off := iccHeaderLen + iccTagEntryLen*len(tags)
	var table, body []byte
	for _, t := range tags {
		table = append(table, t.Sig...)
		table = append(table, be32(uint32(off+len(body)))...) //nolint:gosec // короткие профили
		table = append(table, be32(uint32(len(t.Data)))...)   //nolint:gosec // то же
		body = append(body, t.Data...)
	}
	return append(append(hdr, table...), body...)
}

// MatrixProfile — RGB-профиль «матрица/TRC»: колоранты xyz, одна гамма на три канала,
// описание desc (DescV2 или DescV4).
func MatrixProfile(xyz [9]float64, gamma float64, desc []byte) []byte {
	c := CurvTag(gamma)
	return BuildICC("RGB ",
		ICCTag{"desc", desc},
		ICCTag{"rXYZ", XYZTag(xyz[0:3]...)}, ICCTag{"gXYZ", XYZTag(xyz[3:6]...)}, ICCTag{"bXYZ", XYZTag(xyz[6:9]...)},
		ICCTag{"rTRC", c}, ICCTag{"gTRC", c}, ICCTag{"bTRC", c})
}
