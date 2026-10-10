package layout

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Эталон раскладки: страховка перед ускорением (спека 020). Хранит не полный дамп, а
// SHA-256 каждого случая (первые 8 байт): ключ, оценка и Rects сверяются побитово.
var updateGolden = flag.Bool("update", false, "перезаписать testdata/golden/layouts.sha")

const goldenPath = "testdata/golden/layouts.sha"

func formatGolden(gs []goldenGroup) string {
	var b strings.Builder
	for _, g := range gs {
		hs := make([]string, len(g.cases))
		for i, c := range g.cases {
			hs[i] = c.hash
		}
		fmt.Fprintf(&b, "%s %s\n", g.name, strings.Join(hs, ","))
	}
	return b.String()
}

// TestGoldenLayouts: Layouts, Rearranged, PageChoices (обе страницы) и Group на
// детерминированном корпусе совпадают с эталоном побитово. Эталон снят с кода до
// ускорения; перезапись — go test ./internal/layout -run Golden -update (или SA_GOLDEN_UPDATE=1).
func TestGoldenLayouts(t *testing.T) {
	// На arm64 компилятор вправе сливать x*y+z в FMA — биты оценок другие.
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	got := goldenCorpus()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(formatGolden(got)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d групп)", goldenPath, len(got))
		return
	}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): запустите go test ./internal/layout -run Golden -update", err)
	}
	want := map[string][]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		name, hs, _ := strings.Cut(line, " ")
		want[name] = strings.Split(hs, ",")
	}
	var bad []string
	for _, g := range got {
		w, ok := want[g.name]
		delete(want, g.name)
		switch {
		case !ok:
			bad = append(bad, g.name+": группы нет в эталоне")
		case len(w) != len(g.cases):
			bad = append(bad, fmt.Sprintf("%s: случаев %d, в эталоне %d", g.name, len(g.cases), len(w)))
		default:
			for i, c := range g.cases {
				if w[i] != c.hash {
					bad = append(bad, fmt.Sprintf("%s: первый разошедшийся случай %d (%s)", g.name, i, c.desc))
					break
				}
			}
		}
	}
	for name := range want {
		bad = append(bad, name+": группы нет в результате")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		if len(bad) > 8 {
			bad = append(bad[:8], fmt.Sprintf("… и ещё %d", len(bad)-8))
		}
		t.Fatalf("раскладка разошлась с эталоном %s (если изменение задумано — перезаписать: -update):\n%s",
			goldenPath, strings.Join(bad, "\n"))
	}
}
