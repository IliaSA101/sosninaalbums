package paths

import (
	"runtime"
	"strings"
	"testing"
)

// Оба режима свёртки на любой ОС: без регистра — ToLower побайтно (ключи Windows 0.22 не
// меняются) и EqualFold в сравнении; с регистром — строка как есть. Края: пусто, ASCII,
// кириллица, буквы, у которых нижний регистр другой длины в байтах.
func TestFoldWith(t *testing.T) {
	for _, s := range []string{"", `C:\Фото\Весна\IMG_1.JPG`, "/x/ВЕСНА/dsc.jpg", "/x/İstanbul/Ⱥ.jpg", "/уже/нижний"} {
		if got := foldWith(true, s); got != strings.ToLower(s) {
			t.Errorf("без регистра %q → %q, ждали ToLower", s, got)
		}
		if got := foldWith(false, s); got != s {
			t.Errorf("с регистром %q → %q, ждали как есть", s, got)
		}
	}
	for _, c := range []struct {
		a, b   string
		ci, cs bool
	}{
		{"/x/Весна", "/x/ВЕСНА", true, false},
		{"/x/Весна", "/x/Весна", true, true},
		{"", "", true, true},
		{"/x/a", "/x/a/", false, false},
		{"/x/σ", "/x/ς", true, false}, // EqualFold, как SamePath Windows до 0.23, а не ToLower
	} {
		if got := foldEqualWith(true, c.a, c.b); got != c.ci {
			t.Errorf("без регистра %q = %q: %v, ждали %v", c.a, c.b, got, c.ci)
		}
		if got := foldEqualWith(false, c.a, c.b); got != c.cs {
			t.Errorf("с регистром %q = %q: %v, ждали %v", c.a, c.b, got, c.cs)
		}
	}
}

// Режим по ОС: Windows и macOS — без регистра, остальные — с регистром; Fold, FoldEqual, DirKey и
// SamePath держат один режим.
func TestFoldPerOS(t *testing.T) {
	ci := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	if foldCase != ci {
		t.Fatalf("foldCase = %v на %s", foldCase, runtime.GOOS)
	}
	a, b := "/x/Весна", "/x/ВЕСНА"
	if got := Fold(b) == Fold(a); got != ci {
		t.Errorf("Fold: %q и %q совпали = %v", a, b, got)
	}
	if got := FoldEqual(a, b); got != ci {
		t.Errorf("FoldEqual = %v", got)
	}
	if got := DirKey(a) == DirKey(b); got != ci {
		t.Errorf("DirKey совпали = %v", got)
	}
	if got := SamePath(a+"/", b); got != ci {
		t.Errorf("SamePath = %v", got)
	}
}
