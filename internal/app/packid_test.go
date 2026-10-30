package app

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
)

// TestValidPackIDMatchesPack — model.ValidPackID (зажим ссылки на макет, 131 design §3) и
// pack.ValidID (реестр наборов) — одна грамматика: model не импортирует pack, разойдутся —
// Load снимет ссылку на набор, который реестр принимает, или наоборот.
func TestValidPackIDMatchesPack(t *testing.T) {
	cases := []string{
		"", "a", "ab", "abc", "a-b", "-ab", "ab-", "a--b", "0ab", "123", "my-pack-2",
		strings.Repeat("a", 39), strings.Repeat("a", 40), strings.Repeat("a", 41),
		"Abc", "abC", "a_b", "a.b", "a b", "a/b", `a\b`, "a:b", "ab\x00", "абв", "ab¹",
		"con", "prn", "aux", "nul", "CON", "conin$", "conout$", "cons", "nuls",
		"com", "com0", "com1", "com9", "coma", "com10", "lpt", "lpt0", "lpt1", "lpt9", "lpta", "lpt10", "com¹",
	}
	for _, id := range cases {
		if got, want := model.ValidPackID(id), pack.ValidID(id); got != want {
			t.Errorf("ValidPackID(%q) = %v, pack.ValidID = %v", id, got, want)
		}
	}
	// Перебор коротких id: все строки до 4 знаков из алфавита с краевыми знаками.
	const abc = "a0-zc9mopnrltux_A"
	var walk func(s string)
	walk = func(s string) {
		if model.ValidPackID(s) != pack.ValidID(s) {
			t.Errorf("расхождение на %q", s)
		}
		if len(s) == 4 {
			return
		}
		for i := range len(abc) {
			walk(s + abc[i:i+1])
		}
	}
	walk("")
}
