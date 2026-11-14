package colorspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/jpeg"
	"math"
	"testing"
)

// Тег XYZ в формате s15Fixed16: «XYZ », 4 нуля, три числа.
func iccXYZ(t *testing.T, p []byte, sig string) [3]float64 {
	t.Helper()
	d := iccTag(p, sig)
	if len(d) < 20 || string(d[:4]) != "XYZ " {
		t.Fatalf("тег %s не найден или не XYZ", sig)
	}
	var v [3]float64
	for i := range v {
		v[i] = float64(int32(binary.BigEndian.Uint32(d[8+4*i:]))) / 65536
	}
	return v
}

func TestSRGBProfileEmbedded(t *testing.T) {
	p := SRGBProfile()
	sum := sha256.Sum256(p)
	if got := hex.EncodeToString(sum[:]); got != "af4efe28f6d311799865f325ba39184a2e978b113ca74124d60af7de22b105f4" {
		t.Fatalf("sha256 профиля: %s", got)
	}
	if len(p) != 736 || int(binary.BigEndian.Uint32(p)) != len(p) {
		t.Fatalf("размер профиля %d, в заголовке %d", len(p), binary.BigEndian.Uint32(p))
	}
	if string(p[36:40]) != "acsp" || p[8] != 2 || string(p[12:16]) != "mntr" || string(p[16:20]) != "RGB " {
		t.Fatal("заголовок не ICC v2 монитора RGB")
	}
	// Колориметрия sRGB D50.
	want := map[string][3]float64{
		"rXYZ": {0.4361, 0.2225, 0.0139},
		"gXYZ": {0.3851, 0.7169, 0.0971},
		"bXYZ": {0.1431, 0.0606, 0.7141},
	}
	for sig, w := range want {
		got := iccXYZ(t, p, sig)
		for i := range w {
			if math.Abs(got[i]-w[i]) > 0.003 {
				t.Errorf("%s[%d] = %.4f, ожидалось %.4f", sig, i, got[i], w[i])
			}
		}
	}
	if iccTag(p, "desc") == nil {
		t.Error("нет тега desc")
	}
	// Копия: правка результата не портит встроенный профиль.
	p[0] ^= 0xFF
	if SRGBProfile()[0] == p[0] {
		t.Error("SRGBProfile вернул не копию")
	}
}

func TestSRGBSegment(t *testing.T) {
	s := SRGBSegment()
	if s[0] != 0xFF || s[1] != 0xE2 {
		t.Fatalf("маркер %x%x", s[0], s[1])
	}
	if int(binary.BigEndian.Uint16(s[2:]))+2 != len(s) {
		t.Fatalf("длина в сегменте %d, фактически %d", binary.BigEndian.Uint16(s[2:])+2, len(s))
	}
	if string(s[4:16]) != "ICC_PROFILE\x00" || s[16] != 1 || s[17] != 1 {
		t.Fatal("метка ICC_PROFILE или номер куска")
	}
	if !bytes.Equal(s[18:], srgbICC) {
		t.Fatal("в сегменте не тот профиль")
	}

	// Порядок как в saveJPEG: SOI, APP0 JFIF, COM, APP2 ICC, остальное.
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 32, 16)), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	d := buf.Bytes()
	app0 := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 1, 1, 0x2C, 1, 0x2C, 0, 0}
	com := append([]byte{0xFF, 0xFE, 0, 16}, "Sosnina Albums"...)
	out := append([]byte{}, d[:2]...)
	out = append(out, app0...)
	out = append(out, com...)
	out = append(out, s...)
	out = append(out, d[2:]...)
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("image/jpeg не читает файл с сегментом: %v", err)
	}
}
