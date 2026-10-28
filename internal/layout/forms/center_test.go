package forms

import "testing"

// TestCenterBig: «центр крупнее» — обычное одно и в середине каждого ряда H от трёх фото.
func TestCenterBig(t *testing.T) {
	for key, want := range map[string]bool{
		"H(m0,1,m2)":                   true,
		"H(0,m1,m2)":                   false, // левое крупное
		"H(m0,m1,2)":                   false,
		"H(m0,1,2)":                    false, // два обычных
		"H(m0,m1,m2)":                  false, // нет крупного
		"H(m0,1,m2,m3)":                true,  // чётное: левое из двух центральных
		"H(m0,m1,2,m3)":                true,  // и правое
		"H(0,m1,m2,m3)":                false,
		"H(m0,m1,2,m3,m4)":             true,
		"Q[H(m0,1,m2)|H(m3,4,m5)]":     true,
		"Q[H(m0,1,m2)|H(3,m4,m5)]":     false, // справа не центр
		"Q[0|H(1,m2)]":                 false, // ряд из двух центра не имеет
		"Q[0|H(m1,2,m3)]":              true,  // одиночное фото слева не мешает
		"H(0,1,2)":                     false,
		"":                             false,
		"H(m0,1)":                      false,
		"L(0,1,2)":                     false,
		"P[-|H(m0,1,m2)]":              true,
		"H(m0,V(1,2),m3)":              false, // вложенный узел: плоского ряда нет
		"H(m0,1,m2,m3,m4,m5)":          false,
		"H(m0,m1,m2,3,m4,m5)":          true,
		"H(m0,m1,m2,3,m4,m5,m6)":       true,
		"H(m0,m1,2,m3,m4,m5,m6)":       false,
		"H(m0,m1,m2,m3,4,m5,m6,m7,m8)": true,
	} {
		if got := CenterBig(key); got != want {
			t.Errorf("CenterBig(%q) = %v, ждали %v", key, got, want)
		}
	}
}
