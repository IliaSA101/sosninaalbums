package colorspace

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// FuzzParseICC: любой мусор не роняет разбор, а принятый профиль считает без паники.
func FuzzParseICC(f *testing.F) {
	for _, rp := range refProfiles() {
		f.Add(rp.profile)
	}
	f.Add(testkit.LUTProfile(testkit.XYZP3Exact, testkit.DescV2("lut")))
	f.Add(testkit.BuildICC("RGB "))
	f.Add([]byte("acsp"))
	f.Fuzz(func(t *testing.T, p []byte) {
		x, err := ParseICC(p)
		if err != nil {
			return
		}
		// свойства принятого профиля: таблицы в пределах, матрица ограничена, таблицы против float-эталона
		for c := range 3 {
			for i, v := range x.dec[c] {
				if v < 0 || v > 65535 {
					t.Fatalf("dec[%d][%d] = %d вне 0…65535", c, i, v)
				}
			}
		}
		for i, v := range x.mf {
			if math.IsNaN(v) || math.Abs(v) > iccMaxCoef {
				t.Fatalf("mf[%d] = %v вне пределов", i, v)
			}
		}
		for _, c := range [][3]uint8{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {7, 200, 99}, {128, 128, 128}} {
			r1, g1, b1 := x.px(c[0], c[1], c[2])
			r2, g2, b2 := x.refPx(c[0], c[1], c[2])
			if level(r1, r2) > 1 || level(g1, g2) > 1 || level(b1, b2) > 1 {
				t.Fatalf("%v: таблицы %d,%d,%d, float %d,%d,%d", c, r1, g1, b1, r2, g2, b2)
			}
		}
		x.SRGBCurves()
	})
}
