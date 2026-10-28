package layout

import (
	"crypto/sha256"
	"encoding/hex"
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
		checkRow(t, asp, p, r)
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
	if testkit.Lite() {
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
			// При «только широкие» (064) разворотных может не быть: узкое фото на сгибе.
			anyFold := p.Layflat && p.FoldAny
			if has(0, ManyCap, whole) && !p.Layflat || anyFold && !has(0, ManyBatch, whole) {
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
// постраничных ключей на 17…64 фото, геометрии manyGeoms, наборы manyAspects 0–4.
// Группировка на 600 фото переехала в layout/plan (testdata/golden/many.sha там).
// goldenMany — группы many.sha; wide — как geomAt (many-wide.sha).
func goldenMany(wide bool) []goldenGroup {
	var jobs []func() []goldenGroup
	for g := range manyGeoms {
		p, ok := geomAt(manyGeoms, g, wide)
		if !ok {
			continue
		}
		for _, n := range manyNs {
			jobs = append(jobs, func() []goldenGroup { return goldenManyN(g, p, n) })
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

func goldenManyN(g int, p Params, n int) []goldenGroup {
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

const (
	goldenManyPath     = "testdata/golden/many.sha"
	goldenManyWidePath = "testdata/golden/many-wide.sha"
)

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
	checkGolden(t, goldenManyPath, goldenMany(false))
	checkGolden(t, goldenManyWidePath, goldenMany(true)) // «только широкие» (064)
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

// Быстрые раскладки 17…64: ничьи, текущий блок в ряду страницы, эталон ключей первых.

// manyGeoms — геометрии эталонов 17…64: goldenGeoms (в т. ч. книжная страница с
// промежутком 0), квадрат 20×20 без layflat и широкий 40×30.
var manyGeoms = append(slices.Clone(goldenGeoms),
	Params{PageW: 200, PageH: 200, Margin: 12, Gap: 3, Layflat: false},
	Params{PageW: 400, PageH: 300, Margin: 12, Gap: 3, Layflat: true, FoldAny: true},
)

// manySetNames — наборы manyAspects (0–2 как у goldenAspects, 3–4 — все фото одной
// ориентации); имя без пробелов — оно в имени группы эталона.
var manySetNames = []string{"свадьба", "кадрирование", "случайные", "горизонтальные",
	"вертикальные", "чередование", "панорама"}

// weddingAspects — свадьба: 3:2 — 65 %, 2:3 — 30 %, 1:1 и 16:9 — по 2,5 %. Копия
// layout/plan (тест-хелперы двух пакетов не делятся); последовательность — часть эталона.
func weddingAspects(seed uint64, n int) []float64 {
	rng := splitmix64(seed*7919 + uint64(n))
	asp := make([]float64, n)
	for i := range asp {
		switch x := rng.unit(); {
		case x < 0.65:
			asp[i] = L
		case x < 0.95:
			asp[i] = P
		case x < 0.975:
			asp[i] = S
		default:
			asp[i] = 16.0 / 9
		}
	}
	return asp
}

// manyAspects — набор set из n пропорций (имена — manySetNames).
func manyAspects(seed uint64, n, set int) []float64 {
	if set <= 2 {
		return goldenAspects(seed, n, set)
	}
	out := make([]float64, n)
	for i := range out {
		switch set {
		case 3:
			out[i] = L
		case 4:
			out[i] = P
		case 5:
			out[i] = []float64{P, L}[i%2]
		}
	}
	if set == 6 {
		out = weddingAspects(seed, n)
		out[n/3] = 3.5
	}
	return out
}

// TestManyTiesNoise — шум 1e-12 в пропорциях (как FMA на arm64 в оценках) не меняет ряд:
// ничьи решает порядок построения, а не последний бит оценки.
func TestManyTiesNoise(t *testing.T) {
	geoms, ns := manyGeoms, []int{17, 24, 33, 48, 64}
	if testkit.Lite() {
		geoms, ns = manyGeoms[:2], []int{17, 48} // чистый счёт: остальное — в обычном прогоне
	}
	for _, p := range geoms {
		for _, n := range ns {
			for set := range manySetNames {
				asp := manyAspects(uint64(n), n, set)
				noisy := slices.Clone(asp)
				rng := splitmix64(uint64(n)*100 + uint64(set))
				for i := range noisy {
					noisy[i] *= 1 + (rng.unit()*2-1)*1e-12
				}
				a, b := Layouts(asp, p, ManyBatch), Layouts(noisy, p, ManyBatch)
				if !slices.EqualFunc(a, b, func(x, y Result) bool { return x.Key == y.Key }) {
					t.Errorf("%+v n=%d %s: шум 1e-12 поменял ряд", p, n, manySetNames[set])
				}
			}
		}
	}
}

// TestManyPageRowHasBlock — блок, который поставил manyLayouts, есть в ряду своей страницы
// и на маленькой странице (до maxFullTree фото), где ряд — из перебора деревьев.
func TestManyPageRowHasBlock(t *testing.T) {
	small, step := 0, 1
	if testkit.Lite() {
		step = 3
	}
	for _, p := range []Params{album30, manyGeoms[3]} {
		for n := ManyFrom; n <= 26; n += step {
			for _, set := range []int{0, 2, 4} {
				asp := manyAspects(uint64(n), n, set)
				for _, r := range Layouts(asp, p, ManyBatch) {
					small += checkBlockInPageRow(t, asp, p, r.Key)
				}
			}
		}
	}
	if small == 0 {
		t.Error("ни одной страницы из 2…7 фото")
	}
}

// checkBlockInPageRow — у постраничного key каждая страница находит его в своём ряду;
// возвращает, сколько страниц из 2…maxFullTree фото проверено.
func checkBlockInPageRow(t *testing.T, asp []float64, p Params, key string) int {
	t.Helper()
	k, ok := pageForm(key)
	if !ok {
		return 0
	}
	small := 0
	for side := range 2 {
		if c := len(k.pages[side].leaves(nil)); c > 1 && c <= maxFullTree {
			small++
		}
		in, _, _, ok := PageChoices(key, asp, p, side, 24)
		if _, has := rowKeys(in)[key]; !ok || !has {
			t.Errorf("n=%d %s сторона %d: текущего блока нет в ряду страницы", len(asp), key, side)
		}
	}
	return small
}

const goldenFirstPath = "testdata/golden/first.sha"

// goldenFirst — ключ первой раскладки (Layouts(…, 1)) на 17…64 фото: геометрии
// manyGeoms × наборы manyAspects. Без оценок и Rects — не зависит от FMA, сверяется на любой
// архитектуре. Хеш ключа — 4 байта: эталон ловит смену, а не ищет её место.
func goldenFirst() []goldenGroup {
	var out []goldenGroup
	for g, p := range manyGeoms {
		for set, name := range manySetNames {
			out = append(out, goldenGroup{name: fmt.Sprintf("p%d/%s", g, name)})
			gr := &out[len(out)-1]
			gr.cases = make([]goldenCase, MaxPhotos-ManyFrom+1)
			var wg sync.WaitGroup
			for n := ManyFrom; n <= MaxPhotos; n++ {
				wg.Go(func() {
					key := Layouts(manyAspects(uint64(g)+1, n, set), p, 1)[0].Key
					h := sha256.Sum256([]byte(key))
					gr.cases[n-ManyFrom] = goldenCase{fmt.Sprintf("n=%d", n), hex.EncodeToString(h[:4])}
				})
			}
			wg.Wait()
		}
	}
	return out
}

// TestGoldenFirst — первая раскладка 17…64 совпадает с эталоном first.sha на любой
// архитектуре: так проверяется, что ничьи не решает шум округления. -update — по правилам many.sha.
func TestGoldenFirst(t *testing.T) {
	checkGolden(t, goldenFirstPath, goldenFirst())
}

// TestBreakTableSame — общая таблица разреза левых страниц даёт те же границы, что и
// отдельный счёт на каждую страницу: ускорение не меняет раскладку.
func TestBreakTableSame(t *testing.T) {
	g := newGeom(album30)
	for _, set := range []int{0, 2, 6} {
		asp := manyAspects(5, MaxPhotos, set)
		e := newEnum(asp, album30.Gap)
		e.breaks = map[breakKey]*breakTable{}
		for _, box := range []Rect{g.pages[0], g.spread} {
			for _, cols := range []bool{false, true} {
				for hi := 1; hi <= len(asp); hi++ {
					for r := 0; r <= 12; r++ {
						got, want := e.lineBreaks(asp, 0, hi, box, r, cols), lineBreaks(asp, 0, hi, box, album30.Gap, r, cols)
						if !slices.Equal(got, want) {
							t.Fatalf("набор %d hi=%d r=%d cols=%t: %v, ждали %v", set, hi, r, cols, got, want)
						}
					}
				}
			}
		}
	}
}
