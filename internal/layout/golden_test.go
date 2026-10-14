package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Эталон раскладки: страховка перед ускорением (спека 020). Хранит не полный дамп, а
// SHA-256 каждого случая (первые 8 байт): ключ, оценка и Rects сверяются побитово.
var updateGolden = flag.Bool("update", false, "перезаписать эталоны testdata/golden/*.sha запущенных тестов")

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

// TestGoldenLayouts: Layouts, Rearranged и PageChoices (обе страницы) на
// детерминированном корпусе совпадают с эталоном побитово. Эталон снят с кода до
// ускорения; перезапись — go test ./internal/layout -run Golden -update (или SA_GOLDEN_UPDATE=1).
func TestGoldenLayouts(t *testing.T) {
	// На arm64 компилятор вправе сливать x*y+z в FMA — биты оценок другие.
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	checkGolden(t, goldenPath, goldenCorpus())
}

const goldenLegacyPath = "testdata/golden/legacy.sha"

// TestGoldenLegacy: LegacyLayouts побитово совпадает с Layouts до решения 95 — строки L
// прежнего layouts.sha (миграция альбомов до 0.10 берёт по нему прежнее «Авто»).
// Эталон не перезаписывать: он и есть прежнее поведение.
func TestGoldenLegacy(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	var got []goldenGroup
	for g, p := range goldenGeoms {
		for n := 1; n <= goldenMaxN; n++ {
			if g > 0 && n >= 13 && n < goldenMaxN {
				continue // как goldenCorpus
			}
			gr := goldenGroup{name: fmt.Sprintf("p%d/n%d/L", g, n)}
			for _, set := range goldenSets(n) {
				var d dump
				d.results(LegacyLayouts(goldenAspects(uint64(g)+1, n, set), p, 24))
				gr.cases = append(gr.cases, goldenCase{fmt.Sprintf("набор %d", set), d.sum()})
			}
			got = append(got, gr)
		}
	}
	sort.SliceStable(got, func(i, j int) bool { return got[i].name < got[j].name })
	checkGolden(t, goldenLegacyPath, got)
}

const goldenFirst16Path = "testdata/golden/first16.sha"

// TestGoldenFirst16 — ключ первой раскладки (Layouts(…, 24)[0], «Авто») на 1…16 фото:
// goldenGeoms × три набора goldenAspects. Как first.sha для 17–64 — без оценок и Rects, поэтому
// сверяется и на arm64: ничьи решают tieQ и ключ (решение 95), а не шум FMA. -update — только
// при осознанной смене раскладки.
func TestGoldenFirst16(t *testing.T) {
	var got []goldenGroup
	for g, p := range goldenGeoms {
		for set := range 3 {
			gr := goldenGroup{name: fmt.Sprintf("p%d/набор%d", g, set), cases: make([]goldenCase, goldenMaxN)}
			var wg sync.WaitGroup
			for n := 1; n <= goldenMaxN; n++ {
				wg.Go(func() {
					key := Layouts(goldenAspects(uint64(g)+1, n, set), p, 24)[0].Key
					h := sha256.Sum256([]byte(key))
					gr.cases[n-1] = goldenCase{fmt.Sprintf("n=%d", n), hex.EncodeToString(h[:4])}
				})
			}
			wg.Wait()
			got = append(got, gr)
		}
	}
	checkGolden(t, goldenFirst16Path, got)
}

// checkGolden сверяет группы с файлом эталона path или перезаписывает его (-update).
func checkGolden(t *testing.T, path string, got []goldenGroup) {
	t.Helper()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(formatGolden(got)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d групп)", path, len(got))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("нет эталона (%v): запустите go test ./internal/layout -run %s -update", err, t.Name())
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
			path, strings.Join(bad, "\n"))
	}
}
