package geom

import (
	"math"
	"testing"
)

// Round2 не оставляет «-0»: шум float у нуля иначе давал в JSON «-0» вместо «0» и лишний шаг отмены.
func TestRound2NoNegativeZero(t *testing.T) {
	for _, v := range []float64{-1e-14, -0.004, math.Copysign(0, -1), 0, 0.004} {
		if r := Round2(v); r != 0 || math.Signbit(r) {
			t.Errorf("Round2(%v) = %v, ждали +0", v, r)
		}
	}
	if Round2(-1.234) != -1.23 || Round2(5.678) != 5.68 {
		t.Error("обычное округление нарушено")
	}
}
