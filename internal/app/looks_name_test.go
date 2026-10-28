package app

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
)

// decor.ValidItemPath повторяет грамматику pack.ValidName (decor её не импортирует): расхождение — либо
// запись манифеста не найдётся LookByItem, либо stageLook пропустит то, что pack отверг бы.
func TestValidItemPathMatchesPack(t *testing.T) {
	long := strings.Repeat("a", 64)
	names := []string{
		"", "a", "looks/main.json", "a/b/c.json", "a/b/c/d.json", "../x", "a/../b", "a..b", "a/.b", ".a",
		"-a", "_a", "A.json", "a/", "/a", "a//b", "a b", "a/b.JSON", long, long + "a", long + "/" + long + "/" + long,
		long + "/" + long + "/" + long + "a", "я.json", "a\\b", "a\x00",
	}
	r := rand.New(rand.NewPCG(1, 131))
	const abc = "ab09._-/A.\\ "
	for range 20000 {
		b := make([]byte, r.IntN(80))
		for i := range b {
			b[i] = abc[r.IntN(len(abc))]
		}
		names = append(names, string(b))
	}
	for _, n := range names {
		if got, want := decor.ValidItemPath(n), pack.ValidName(n); got != want {
			t.Errorf("ValidItemPath(%q) = %v, pack.ValidName — %v", n, got, want)
		}
	}
}
