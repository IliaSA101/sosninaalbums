package plan

import (
	"crypto/sha256"
	"encoding/binary"
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

// Эталоны группировки: SHA-256 (первые 8 байт) групп каждого случая, как у layout.
// Переехали из layout вместе с plan.go: значения те же, не переснимались.
var updateGolden = flag.Bool("update", false, "перезаписать эталоны testdata/golden/*.sha запущенных тестов")

// dump накапливает побитовое представление групп плана.
type dump struct{ buf []byte }

func (d *dump) u64(v uint64) { d.buf = binary.LittleEndian.AppendUint64(d.buf, v) }
func (d *dump) sum() string  { h := sha256.Sum256(d.buf); return hex.EncodeToString(h[:8]) }

// goldenCase — случай группы: подпись для сообщения и хеш. goldenGroup — группа p0/plan/N30-m64.
type goldenCase struct{ desc, hash string }
type goldenGroup struct {
	name  string
	cases []goldenCase
}

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

const goldenPlan051Path = "testdata/golden/plan051.sha"

// TestGoldenPlan051 — группировка с разбросом (PlanRange, спека 051 §3) побитово: три набора
// (свадьба 100/11, серии 161/16 и 600/50, у каждого своя геометрия) × «Авто», широкий свой,
// узкий свой × без серий и с сериями. -update — только при осознанной смене плана.
func TestGoldenPlan051(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	wed := weddingAspects(51, 100)
	wedCuts := make([]float64, 101)
	wedCuts[23], wedCuts[51], wedCuts[77] = 1, 0.6, 0.3 // папка и паузы разной силы
	s161, c161 := seriesAspects(5, 161)
	s600, c600 := seriesAspects(6, 600)
	sets := []struct {
		name      string
		asp, cuts []float64
		N         int
	}{{"wedding100", wed, wedCuts, 11}, {"series161", s161, c161, 16}, {"series600", s600, c600, 50}}
	var got []goldenGroup
	for g, set := range sets {
		n := len(set.asp)
		a, b := AutoPer(n, set.N)
		avg := n / set.N
		for _, r := range []struct {
			name   string
			lo, hi int
		}{{"auto", a, b}, {"wide", 2, 2 * avg}, {"narrow", avg - 1, avg + 1}} {
			gr := goldenGroup{name: fmt.Sprintf("p%d/%s/%s", g, set.name, r.name)}
			for _, cuts := range [][]float64{nil, set.cuts} {
				var d dump
				for _, x := range PlanRange(set.asp, goldenGeoms[g], PlanOpts{Spreads: set.N, MinPer: r.lo, MaxPer: r.hi, Cuts: cuts}).Groups {
					d.u64(uint64(x[0]))
					d.u64(uint64(x[1]))
				}
				gr.cases = append(gr.cases, goldenCase{fmt.Sprintf("серии %t", cuts != nil), d.sum()})
			}
			got = append(got, gr)
		}
	}
	checkGolden(t, goldenPlan051Path, got)
}

const goldenManyPath = "testdata/golden/many.sha"

// goldenPlan — PlanGroups на 600 фото свадьбы: N = 150 и 30 при m = 64, N = 130 при m = 5.
func goldenPlan(g int) []goldenGroup {
	p := goldenGeoms[g]
	asp := weddingAspects(uint64(g)+1, 600)
	var out []goldenGroup
	for _, c := range [][2]int{{150, 64}, {30, 64}, {130, 5}} {
		var d dump
		pl := PlanGroups(asp, p, c[0], c[1])
		for _, gr := range pl.Groups {
			d.u64(uint64(gr[0]))
			d.u64(uint64(gr[1]))
		}
		name := fmt.Sprintf("p%d/plan/N%d-m%d", g, c[0], c[1])
		out = append(out, goldenGroup{name, []goldenCase{{"600 фото", d.sum()}}})
	}
	return out
}

// TestGoldenPlan600 — группировка на 600 фото свадьбы совпадает с эталоном many.sha побитово
// (строки .../plan/..., прежде в эталоне layout).
func TestGoldenPlan600(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	res := make([][]goldenGroup, 2)
	var wg sync.WaitGroup
	for g := range res {
		wg.Go(func() { res[g] = goldenPlan(g) })
	}
	wg.Wait()
	var all []goldenGroup
	for _, r := range res {
		all = append(all, r...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].name < all[j].name })
	checkGolden(t, goldenManyPath, all)
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
		t.Fatalf("нет эталона (%v): запустите go test ./internal/layout/plan -run %s -update", err, t.Name())
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
