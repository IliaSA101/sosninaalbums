package imgfile

import (
	"bytes"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// xmpSegment и xmpAttrs — копии помощников library: пакет XMP для затравки фаззера.
func xmpSegment(packet []byte) []byte {
	return testkit.Segment(0xE1, append([]byte(XMPPrefix), packet...))
}

func xmpAttrs(attrs string) []byte {
	return []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" ` + attrs + `/></rdf:RDF></x:xmpmeta>`)
}

func TestJPEGHeaderExif(t *testing.T) {
	cases := []struct {
		name       string
		big        bool
		orient     int
		colorSp    uint16
		interop    string
		wantOrient int
		wantColor  Color
		wantName   string
	}{
		{"little-endian: поворот 6, Uncalibrated, R03", false, 6, 0xFFFF, "R03", 6, ColorAdobe, ICCAdobeEXIF},
		{"big-endian: поворот 8, Uncalibrated, R03", true, 8, 0xFFFF, "R03", 8, ColorAdobe, ICCAdobeEXIF},
		{"sRGB и R98", false, 3, 1, "R98", 3, ColorNone, ""},
		{"big-endian: sRGB и R98", true, 3, 1, "R98", 3, ColorNone, ""},
		{"только ориентация", true, 6, 0, "", 6, ColorNone, ""},
		{"только Interop", false, 0, 0, "R03", 1, ColorAdobe, ICCAdobeEXIF},
		{"Uncalibrated без R03 — считаем sRGB", false, 1, 0xFFFF, "", 1, ColorNone, ""},
	}
	for _, c := range cases {
		h := Header(bytes.NewReader(testkit.RawJPEG(testkit.ExifColor(c.big, c.orient, c.colorSp, c.interop))))
		if h.Orient != c.wantOrient || h.Color != c.wantColor || h.Profile != c.wantName {
			t.Errorf("%s: получили %+v, ждали {%d %q %q}", c.name, h, c.wantOrient, c.wantColor, c.wantName)
		}
	}
	// не JPEG и пустой поток: ориентация 1, цвета нет
	for _, data := range [][]byte{nil, []byte("\x89PNG\r\n\x1a\n")} {
		if h := Header(bytes.NewReader(data)); h.Orient != 1 || h.Color != ColorNone || h.Profile != "" || h.XMP != nil {
			t.Errorf("не JPEG: %+v", h)
		}
	}
}

func TestJPEGHeaderICCWins(t *testing.T) {
	srgb := testkit.SRGBProfile(testkit.DescV2("sRGB"))
	// ICC и EXIF в любом порядке сегментов: профиль важнее R03
	for _, segs := range [][][]byte{
		{testkit.ExifColor(false, 6, 0xFFFF, "R03"), testkit.ICCSegment(1, 1, srgb)},
		{testkit.ICCSegment(1, 1, srgb), testkit.ExifColor(false, 6, 0xFFFF, "R03")},
	} {
		h := Header(bytes.NewReader(testkit.RawJPEG(segs...)))
		if h.Color != ColorSRGB || h.Profile != "sRGB" || h.Orient != 6 {
			t.Errorf("получили %+v, ждали srgb/sRGB и ориентацию 6", h)
		}
	}
}

// Профиль из двух кусков склеивается; если второго куска нет, теги за пределами первого → other.
func TestJPEGHeaderICCChunk2(t *testing.T) {
	adobe := testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)"))
	first, second := testkit.ICCSegment(1, 2, adobe[:150]), testkit.ICCSegment(2, 2, adobe[150:])
	if h := Header(bytes.NewReader(testkit.RawJPEG(first, second))); h.Color != ColorAdobe {
		t.Errorf("оба куска: %+v", h)
	}
	if h := Header(bytes.NewReader(testkit.RawJPEG(first))); h.Color != ColorOther {
		t.Errorf("только первый кусок: %+v", h)
	}
}

// Заголовок недоверенного JPEG: без паник, пакет XMP не длиннее сегмента.
func FuzzJPEGHead(f *testing.F) {
	srgb := testkit.SRGBProfile(testkit.DescV2("sRGB"))
	f.Add(testkit.RawJPEG(testkit.ExifColor(false, 6, 0xFFFF, "R03"), xmpSegment(xmpAttrs(`xmp:Rating="4"`)),
		testkit.ICCSegment(1, 1, srgb)))
	f.Add(testkit.RawJPEG(testkit.ICCSegment(1, 2, srgb[:100]), testkit.ICCSegment(2, 2, srgb[100:])))
	f.Add(testkit.RawJPEG(testkit.ExifTaken(false, "2024:06:15 14:30:05", "2024:06:15 14:30:06"), testkit.ICCSegment(1, 1, srgb)))
	f.Add(testkit.RawJPEG(testkit.ExifTaken(true, "", "0000:00:00 00:00:00")))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x01})
	f.Fuzz(func(t *testing.T, data []byte) {
		h := Header(bytes.NewReader(data))
		if h.Orient < 1 || h.Orient > 8 || len(h.XMP) > 65533-len(XMPPrefix) {
			t.Fatalf("%d %d", h.Orient, len(h.XMP))
		}
		if h.Taken < 0 || h.Taken > unix(2200, 12, 31, 23, 59, 59) {
			t.Fatalf("время вне пределов: %d", h.Taken)
		}
	})
}
