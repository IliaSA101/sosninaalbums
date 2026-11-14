package gen

import (
	"math"
	"testing"
)

// TestClampCut — контракт с моделью (079-25): NaN и минус — 0, предел MaxCut, шаг 1e-4,
// идемпотентна; значение в границах не меняется.
func TestClampCut(t *testing.T) {
	cases := []struct{ in, want [4]float64 }{
		{[4]float64{}, [4]float64{}},
		{[4]float64{0.14, 0.14, 0.14, 0.14}, [4]float64{0.14, 0.14, 0.14, 0.14}},
		{[4]float64{math.NaN(), -1, math.Inf(1), math.Inf(-1)}, [4]float64{0, 0, MaxCut, 0}},
		{[4]float64{0.6, 1e308, -1e-9, 0.45}, [4]float64{MaxCut, MaxCut, 0, MaxCut}},
		{[4]float64{0.123456, 0.00004, 0.00005, 0.44996}, [4]float64{0.1235, 0, 0.0001, 0.45}},
	}
	for _, c := range cases {
		got := ClampCut(c.in)
		if got != c.want {
			t.Errorf("ClampCut(%v) = %v, ждали %v", c.in, got, c.want)
		}
		if ClampCut(got) != got {
			t.Errorf("не идемпотентна: %v → %v", got, ClampCut(got))
		}
	}
	// Шаг без хвоста двоичной дроби: 0,1235 пишется в файл как 0.1235.
	for i := range 4501 {
		v := float64(i) / cutPrec
		if c := ClampCut([4]float64{v}); c[0] != v || c[0] > MaxCut {
			t.Fatalf("%v → %v", v, c[0])
		}
	}
}
