package layout

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Быстрые раскладки 17…64 фото (спека 027): инварианты, префикс порций, эталон many.sha.

var manyNs = []int{17, 24, 32, 48, 64}

// manyCases — наборы 0–2 (свадьба, кадрирование, случайные) на каждом из manyNs.
func manyCases() [][]float64 {
	var out [][]float64
	for _, n := range manyNs {
		for set := range 3 {
			out = append(out, goldenAspects(uint64(set)+7, n, set))
		}
	}
	return out
}

// TestManyInvariants — инварианты пакета и round-trip через Build на всех рядах, которые
// видит интерфейс (rows), и на полном ряду ManyCap; страницы > 16 фото тоже проверяются.
func TestManyInvariants(t *testing.T) {
	bigPages := 0
	for _, p := range raceParams() {
		for _, asp := range manyCases() {
			all := rows(asp, p)
			all["layouts48"] = Layouts(asp, p, ManyCap)
			if len(all["layouts"]) == 0 {
				t.Fatalf("n=%d: нет раскладок", len(asp))
			}
			if re, b := Rearranged(asp, p, 24); len(re) != 0 || b != BlockMany {
				t.Errorf("n=%d: перестановок %d, причина %q", len(asp), len(re), b)
			}
			for name, rs := range all {
				checkManyRow(t, asp, p, name, rs)
			}
			for _, r := range all["layouts"] {
				if !IsPaged(r.Key) {
					continue
				}
				checkOtherPageKept(t, asp, p, r, 0)
				checkOtherPageKept(t, asp, p, r, 1)
				k, _ := pageForm(r.Key)
				for side := range 2 {
					if len(k.pages[side].leaves(nil)) > 16 {
						bigPages++
					}
				}
			}
		}
	}
	if bigPages == 0 {
		t.Error("ни одной страницы больше 16 фото")
	}
}

// checkManyRow — ключи ряда уникальны, ряд «Авто» в порядке ленты, у каждого варианта
// инварианты и round-trip через Build.
func checkManyRow(t *testing.T, asp []float64, p Params, name string, rs []Result) {
	t.Helper()
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.Key] {
			t.Errorf("n=%d %s: ключ %s повторяется", len(asp), name, r.Key)
		}
		seen[r.Key] = true
		if name == "layouts48" && IsRearranged(r.Key) {
			t.Errorf("n=%d: %s не в порядке ленты", len(asp), r.Key)
		}
		checkResult(t, asp, p, r)
		checkRoundTrip(t, asp, p, name, r)
	}
}

// TestManyPrefix — Layouts(12) — префикс Layouts(24), он — префикс Layouts(48); limit 0 и
// больше ManyCap — весь ряд ManyCap. Ключи уникальны, всё в порядке ленты.
func TestManyPrefix(t *testing.T) {
	for _, p := range raceParams() {
		for _, asp := range manyCases() {
			full := Layouts(asp, p, ManyCap)
			if len(full) != ManyCap {
				t.Fatalf("n=%d layflat=%t: ряд %d вместо %d", len(asp), p.Layflat, len(full), ManyCap)
			}
			for _, limit := range []int{1, ManyBatch, 2 * ManyBatch, 3 * ManyBatch, 0, 100} {
				got := Layouts(asp, p, limit)
				want := full[:min(len(full), max(limit, 0))]
				if limit <= 0 || limit > ManyCap {
					want = full
				}
				if !sameResults(got, want) {
					t.Errorf("n=%d layflat=%t: Layouts(%d) не префикс ряда %d", len(asp), p.Layflat, limit, ManyCap)
				}
			}
			seen := map[string]bool{}
			for _, r := range full {
				if seen[r.Key] || IsRearranged(r.Key) {
					t.Errorf("n=%d: ключ %s повторяется или не в порядке ленты", len(asp), r.Key)
				}
				seen[r.Key] = true
			}
		}
	}
}

// raceParams — геометрии для тяжёлых тестов чистого счёта: под -race одна (остальные
// считает обычный прогон), без него все.
func raceParams() []Params {
	if testkit.Race {
		return invariantParams[:1]
	}
	return invariantParams
}

func sameResults(a, b []Result) bool {
	return slices.EqualFunc(a, b, func(x, y Result) bool {
		return x.Key == y.Key && x.Score == y.Score && slices.Equal(x.Rects, y.Rects)
	})
}

// TestManyPageChoices — страница из многих фото листается рядами и столбцами, текущий
// блок есть в ряду страницы (его поставил manyLayouts), модификатор «поменьше» — один.
func TestManyPageChoices(t *testing.T) {
	for _, p := range invariantParams {
		for _, n := range []int{24, 48, 64} {
			asp := goldenAspects(3, n, 0)
			for _, r := range Layouts(asp, p, ManyBatch) {
				if !IsPaged(r.Key) {
					continue
				}
				for side := range 2 {
					in, re, b, ok := PageChoices(r.Key, asp, p, side, 24)
					if !ok || len(in) < 2 || len(re) != 0 || b == BlockNone {
						t.Fatalf("n=%d %s сторона %d: ok=%t ряд %d, перестановок %d (%q)", n, r.Key, side, ok, len(in), len(re), b)
					}
					if _, has := rowKeys(in)[r.Key]; !has {
						t.Errorf("n=%d %s сторона %d: текущего блока нет в ряду страницы", n, r.Key, side)
					}
				}
				break
			}
		}
	}
}

// TestManyFamiliesShown — каждое семейство есть в ряду: основные — в первой порции
// (крупное первое, при layflat — на весь разворот), остальные — во второй. Иначе пары
// страниц с лучшей оценкой заняли бы все 48 мест.
func TestManyFamiliesShown(t *testing.T) {
	for _, p := range invariantParams {
		for _, asp := range manyCases() {
			n := len(asp)
			row := Layouts(asp, p, ManyCap)
			has := func(from, to int, f func(string) bool) bool {
				return slices.ContainsFunc(row[from:to], func(r Result) bool { return f(r.Key) })
			}
			hero := func(k string) bool { return strings.HasPrefix(k, "P[0|") }
			last := func(k string) bool { return strings.HasSuffix(k, "|"+strconv.Itoa(n-1)+"]") }
			whole := func(k string) bool { return !IsPaged(k) }
			if !has(0, ManyBatch, hero) || !has(ManyBatch, 2*ManyBatch, last) {
				t.Errorf("n=%d layflat=%t: нет крупного первого в 1-й порции или последнего во 2-й", n, p.Layflat)
			}
			if has(0, ManyCap, whole) != p.Layflat || p.Layflat && !has(0, ManyBatch, whole) {
				t.Errorf("n=%d layflat=%t: раскладки на весь разворот не там", n, p.Layflat)
			}
			for i := 1; i < len(row); i++ {
				if i%ManyBatch != 0 && tieQ(row[i].Score) > tieQ(row[i-1].Score) {
					t.Errorf("n=%d: внутри порции не по оценке на %d", n, i)
				}
			}
			// Первый — лучший из основных семейств (§3 design): в порциях 2–4 бывают и выше.
			if first := Layouts(asp, p, 1); !sameResults(first, row[:1]) {
				t.Errorf("n=%d: Layouts(1) не первый вариант ряда", n)
			}
		}
	}
}

// TestManyFill — лучшая раскладка на многих фото заполняет хотя бы 30 % разворота:
// генераторы не вырождаются в мелкий ряд.
func TestManyFill(t *testing.T) {
	for _, asp := range manyCases() {
		best := Layouts(asp, album30, 1)[0]
		g := newGeom(album30)
		var area float64
		for _, r := range best.Rects {
			area += r.W * r.H
		}
		if fill := area / (g.spread.W * g.spread.H); fill < 0.3 {
			t.Errorf("n=%d: лучшая %s заполняет %.2f разворота", len(asp), best.Key, fill)
		}
	}
}

// goldenMany — регрессионный эталон many.sha: Layouts(ManyCap) и PageChoices первых
// постраничных ключей на 17…64 фото, геометрии manyGeoms, наборы manyAspects 0–4;
// группировка PlanGroups на 600 фото (две первые геометрии).
func goldenMany() []goldenGroup {
	var jobs []func() []goldenGroup
	for g := range manyGeoms {
		for _, n := range manyNs {
			jobs = append(jobs, func() []goldenGroup { return goldenManyN(g, n) })
		}
		if g < 2 {
			jobs = append(jobs, func() []goldenGroup { return goldenPlan(g) })
		}
	}
	res := make([][]goldenGroup, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Go(func() { res[i] = job() })
	}
	wg.Wait()
	var all []goldenGroup
	for _, r := range res {
		all = append(all, r...)
	}
	slices.SortStableFunc(all, func(a, b goldenGroup) int {
		switch {
		case a.name < b.name:
			return -1
		case a.name > b.name:
			return 1
		}
		return 0
	})
	return all
}

func goldenManyN(g, n int) []goldenGroup {
	p := manyGeoms[g]
	kinds := []string{"L", "P0", "P1"}
	groups := make([]goldenGroup, len(kinds))
	for i, k := range kinds {
		groups[i].name = fmt.Sprintf("p%d/n%d/%s", g, n, k)
	}
	for set := range 5 {
		asp := manyAspects(uint64(g)+1, n, set)
		desc := manySetNames[set]
		var d dump
		d.results(Layouts(asp, p, ManyCap))
		groups[0].cases = append(groups[0].cases, goldenCase{desc, d.sum()})
		for side := range 2 {
			d = dump{}
			for _, key := range pagedKeys(asp, p) {
				in, re, blk, ok := PageChoices(key, asp, p, side, 24)
				d.str(key)
				d.results(in)
				d.results(re)
				d.str(string(blk))
				if ok {
					d.u64(1)
				} else {
					d.u64(0)
				}
			}
			groups[1+side].cases = append(groups[1+side].cases, goldenCase{desc, d.sum()})
		}
	}
	return groups
}

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

const goldenManyPath = "testdata/golden/many.sha"

// TestGoldenMany — быстрые раскладки и группировка совпадают с эталоном many.sha побитово.
// После выпуска смена генератора меняет вид альбомов с Layout = "": -update — по тем же
// правилам, что у layouts.sha (go test ./internal/layout -run GoldenMany -update).
func TestGoldenMany(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	checkGolden(t, goldenManyPath, goldenMany())
}

// TestPreviewMany печатает лучшие раскладки на 24 и 64 фото — отладочная печать без проверок,
// в наборе не идёт (и под -v): SA_PREVIEW=1 go test ./internal/layout -run PreviewMany -v
func TestPreviewMany(t *testing.T) {
	if os.Getenv("SA_PREVIEW") == "" {
		t.Skip("отладочная печать без проверок: только с SA_PREVIEW=1")
	}
	for _, p := range invariantParams {
		for _, n := range []int{24, 64} {
			asp := weddingAspects(5, n)
			for i, r := range Layouts(asp, p, 3) {
				fmt.Printf("%d фото, layflat=%t, вариант %d  score=%.3f  %s\n%s\n", n, p.Layflat, i+1, r.Score, r.Key, ascii(r, p))
			}
		}
	}
}

// BenchmarkLayouts64 — весь ряд ManyCap на 64 случайных пропорциях (5 наборов по кругу).
func BenchmarkLayouts64(b *testing.B) {
	var sets [5][]float64
	for i := range sets {
		sets[i] = goldenAspects(uint64(i), 64, 2)
	}
	i := 0
	for b.Loop() {
		Layouts(sets[i%len(sets)], album30, ManyCap)
		i++
	}
}

// BenchmarkLayouts64First — первая порция (то, что считает View).
func BenchmarkLayouts64First(b *testing.B) {
	var sets [5][]float64
	for i := range sets {
		sets[i] = goldenAspects(uint64(i), 64, 2)
	}
	i := 0
	for b.Loop() {
		Layouts(sets[i%len(sets)], album30, ManyBatch)
		i++
	}
}
