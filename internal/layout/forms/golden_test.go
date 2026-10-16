package forms

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Эталон генератора (design §11): ключи и виды Gen на 12 наборах × 4 геометрии (30×30, 30×20, без
// layflat, лист) — sha256 отсортированных «ключ вид». Ловит любую смену генераторов, Of и полов.
// Оценки Q не в хэше: они плавающие, их сверяют TestFormsInvariants и TestQualityMatchesScoreTerms.
// amd64: ничьи — в квантах, но эталон снят на amd64 (на arm64 — наблюдение).
var updateGolden = flag.Bool("update", false, "перезаписать testdata/golden/forms.sha")

const goldenPath = "testdata/golden/forms.sha"

// goldenSets — 12 наборов: разные размеры и составы.
func goldenSets() map[string][]float64 {
	return map[string][]float64{
		"гориз3": same(3, aspL), "гориз6": same(6, aspL), "гориз10": same(10, aspL), "гориз16": same(16, aspL),
		"гориз25": same(25, aspL), "верт4": same(4, aspP), "верт8": same(8, aspP), "верт12": same(12, aspP),
		"смесь5": mixed(5, 5), "смесь9": mixed(9, 9), "смесь16": mixed(16, 16), "серия30": series(30, 30),
	}
}

func goldenLines() []string {
	sets := goldenSets()
	var out []string
	for _, g := range geoms {
		for _, name := range sortedNames(sets) {
			var lines []string
			for _, c := range Gen(sets[name], g.p) {
				lines = append(lines, c.Key+" "+string(c.Form))
			}
			slices.Sort(lines)
			h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
			out = append(out, fmt.Sprintf("%s/%s %d %s", g.name, name, len(lines), hex.EncodeToString(h[:8])))
		}
	}
	return out
}

func TestGoldenForms(t *testing.T) {
	if runtime.GOARCH != "amd64" && os.Getenv("SA_GOLDEN_ANYARCH") != "1" {
		t.Skip("эталон снят на amd64 (SA_GOLDEN_ANYARCH=1 — гонять и здесь)")
	}
	got := goldenLines()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", goldenPath, len(got))
		return
	}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/layout/forms -run GoldenForms -update", err)
	}
	want := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(want) != len(got) {
		t.Fatalf("строк в эталоне %d, получили %d", len(want), len(got))
	}
	bad := 0
	for i := range got {
		if got[i] != want[i] {
			if bad++; bad <= 5 {
				t.Errorf("расхождение:\n  есть   %s\n  эталон %s", got[i], want[i])
			}
		}
	}
	if bad > 0 {
		t.Errorf("разошлось %d из %d строк", bad, len(got))
	}
}
