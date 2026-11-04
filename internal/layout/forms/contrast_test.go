package forms

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// contrastKeys — ключи Gen формы contrast.
func contrastKeys(as []float64, p layout.Params) []string {
	var out []string
	for _, c := range Gen(as, p) {
		if c.Form == Contrast {
			out = append(out, c.Key)
		}
	}
	return out
}

func wantKeys(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	for _, k := range want {
		if !slices.Contains(got, k) {
			t.Errorf("%s: нет ключа %s среди %v", name, k, got)
		}
	}
}

func wantNoKeys(t *testing.T, name string, got []string, bad ...string) {
	t.Helper()
	for _, k := range bad {
		if slices.Contains(got, k) {
			t.Errorf("%s: лишний ключ %s среди %v", name, k, got)
		}
	}
}

// TestContrastMiddleBig: при равных пропорциях крупным пробуется и среднее фото («центр крупнее»),
// прежний вариант (самое широкое — левое) остаётся; правило сгиба (FoldAny) не обойдено.
func TestContrastMiddleBig(t *testing.T) {
	// Разворот 600×305: три вертикальных 2:3.
	p := layout.Params{PageW: 300, PageH: 305, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}
	wantKeys(t, "3 верт, любые фото", contrastKeys(same(3, aspP), p), "H(m0,1,m2)")

	p.FoldAny = false
	wantNoKeys(t, "3 верт, только широкие", contrastKeys(same(3, aspP), p), "H(m0,1,m2)")

	// Горизонтальные проходят сгиб и без FoldAny: прежний и новый варианты рядом.
	wantKeys(t, "3 гориз", contrastKeys(same(3, aspL), geomSq), "H(0,m1,m2)", "H(m0,1,m2)")

	// Чётное число: оба центральных; прежний (левый) тоже.
	p.FoldAny = true
	wantKeys(t, "4 верт", contrastKeys(same(4, aspP), p), "H(0,m1,m2,m3)", "H(m0,1,m2,m3)", "H(m0,m1,2,m3)")

	// Нечётное 5: центр — 2.
	wantKeys(t, "5 гориз", contrastKeys(same(5, aspL), geomSq), "H(0,m1,m2,m3,m4)", "H(m0,m1,2,m3,m4)")
}

// TestContrastMiddleBigPages: на страницах (без layflat) крупное среднее пробуется на каждой.
func TestContrastMiddleBigPages(t *testing.T) {
	got := contrastKeys(same(6, aspL), geomFlat)
	wantKeys(t, "6 гориз, страницы", got,
		"Q[H(0,m1,m2)|H(m3,4,m5)]", "Q[H(m0,1,m2)|H(3,m4,m5)]", "Q[H(m0,1,m2)|H(m3,4,m5)]")
}

// TestContrastDifferentRatios: разные пропорции — крупное по-прежнему самое широкое, среднее
// пробуется только если оно же и самое широкое.
func TestContrastDifferentRatios(t *testing.T) {
	got := contrastKeys([]float64{1.5, 1.4, 1.45}, geomSq)
	wantKeys(t, "разные", got, "Q[0|H(m1,2)]")
	wantNoKeys(t, "разные", got, "H(m0,1,m2)", "H(0,m1,m2)", "H(m0,m1,2)")

	got = contrastKeys([]float64{1.4, 1.5, 1.45}, geomSq)
	wantKeys(t, "широкое в центре", got, "H(m0,1,m2)")
	wantNoKeys(t, "широкое в центре", got, "H(0,m1,m2)", "H(m0,m1,2)")
}

// TestContrastNoiseTie: шум 1e-12 в пропорциях (как FMA на arm64) — по-прежнему ничья в квантах.
func TestContrastNoiseTie(t *testing.T) {
	as := []float64{aspL, aspL + 1e-12, aspL - 1e-12}
	wantKeys(t, "шум", contrastKeys(as, geomSq), "H(m0,1,m2)")
}
