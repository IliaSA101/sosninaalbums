package plan

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// weddingAspects — свадьба: 3:2 — 65 %, 2:3 — 30 %, 1:1 и 16:9 — по 2,5 %.
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

// checkPlan — группы подряд покрывают все фото, их ровно N, каждая ≤ m; Min/Max/Avg — по группам.
func checkPlan(t *testing.T, pl Plan, n, N, m int) {
	t.Helper()
	if len(pl.Groups) != N || pl.Spreads != N {
		t.Fatalf("групп %d (Spreads %d), ждали %d", len(pl.Groups), pl.Spreads, N)
	}
	pos, lo, hi := 0, n, 0
	for _, g := range pl.Groups {
		k := g[1] - g[0]
		if g[0] != pos || k < 1 || k > m {
			t.Fatalf("плохая группа %v после %d (m = %d)", g, pos, m)
		}
		pos = g[1]
		lo, hi = min(lo, k), max(hi, k)
	}
	if pos != n {
		t.Fatalf("покрыто %d из %d", pos, n)
	}
	if pl.Min != lo || pl.Max != hi || pl.Avg != float64(n)/float64(N) {
		t.Errorf("Min/Max/Avg = %d/%d/%g, по группам %d/%d/%g", pl.Min, pl.Max, pl.Avg, lo, hi, float64(n)/float64(N))
	}
}

// TestPlanExact — ровно N разворотов; на свадьбе 600 фото размеры групп в
// [⌊avg/2⌋, ⌈1,5·avg⌉ + 1]; ограничение m соблюдается.
func TestPlanExact(t *testing.T) {
	var asp []float64
	for g, p := range invariantParams {
		// Наборы те же, что в эталоне many.sha (goldenPlan, та же геометрия): матрица уже в кэше.
		asp = weddingAspects(uint64(g)+1, 600)
		for _, N := range []int{30, 150} {
			pl := PlanGroups(asp, p, N, 64)
			checkPlan(t, pl, 600, N, 64)
			avg := 600.0 / float64(N)
			if lo, hi := int(avg/2), int(math.Ceil(1.5*avg))+1; pl.Min < lo || pl.Max > hi {
				t.Errorf("layflat=%t N=%d: группы от %d до %d, ждали в [%d, %d]", p.Layflat, N, pl.Min, pl.Max, lo, hi)
			}
			t.Logf("layflat=%t N=%d: от %d до %d", p.Layflat, N, pl.Min, pl.Max)
		}
		for _, m := range []int{1, 5, 9} {
			lo, _ := SpreadRange(600, m)
			checkPlan(t, PlanGroups(asp, p, lo, m), 600, lo, m)
		}
	}
	// Мало фото и крайние N.
	small := asp[:10]
	checkPlan(t, PlanGroups(small, album30, 1, 64), 10, 1, 64)
	checkPlan(t, PlanGroups(small, album30, 10, 64), 10, 10, 64)
	checkPlan(t, PlanGroups(small, album30, 0, 64), 10, 1, 64)   // зажим снизу
	checkPlan(t, PlanGroups(small, album30, 99, 64), 10, 10, 64) // сверху
	checkPlan(t, PlanGroups(small, album30, 1, 3), 10, 4, 3)     // ⌈10/3⌉
	if pl := PlanGroups(nil, album30, 5, 64); pl.Spreads != 0 || pl.Groups != nil {
		t.Errorf("пустой набор: %+v", pl)
	}
}

func TestSpreadRange(t *testing.T) {
	for _, c := range []struct{ n, m, lo, hi int }{
		{0, 64, 0, 0}, {1, 64, 1, 1}, {600, 64, 10, 600}, {600, 7, 86, 600},
		{600, 0, 600, 600}, {600, 1000, 10, 600}, {64, 64, 1, 64}, {65, 64, 2, 65},
	} {
		if lo, hi := SpreadRange(c.n, c.m); lo != c.lo || hi != c.hi {
			t.Errorf("SpreadRange(%d, %d) = %d, %d, ждали %d, %d", c.n, c.m, lo, hi, c.lo, c.hi)
		}
	}
}

// TestPlanCacheSame — план не зависит от того, что уже посчитано в матрице: досчёт
// столбцов (N от большего к меньшему) даёт то же, что холодный счёт.
func TestPlanCacheSame(t *testing.T) {
	asp := weddingAspects(2, 90)
	layout.ResetCache()
	warm := map[int][][2]int{}
	for _, N := range []int{30, 12, 5, 30} {
		warm[N] = PlanGroups(asp, album30, N, 64).Groups
	}
	for _, N := range []int{5, 12, 30} {
		layout.ResetCache()
		if cold := PlanGroups(asp, album30, N, 64).Groups; !slices.Equal(cold, warm[N]) {
			t.Errorf("N=%d: после досчёта %v, холодный %v", N, warm[N], cold)
		}
	}
}

// TestPlanConcurrent — параллельные запросы одного набора и вытеснение матриц (planKeep).
func TestPlanConcurrent(t *testing.T) {
	layout.ResetCache()
	sets := make([][]float64, planKeep+2)
	for i := range sets {
		sets[i] = weddingAspects(uint64(i)+10, 60)
	}
	want := make([][][2]int, len(sets))
	for i, a := range sets {
		want[i] = PlanGroups(a, album30, 12, 64).Groups
	}
	var wg sync.WaitGroup
	for r := range 16 {
		wg.Go(func() {
			i := r % len(sets)
			N := 12 - r%3 // разные N: досчёт столбцов при параллельном чтении
			got := PlanGroups(sets[i], album30, N, 64)
			if N == 12 && !slices.Equal(got.Groups, want[i]) {
				t.Errorf("набор %d: план разошёлся при параллельном счёте", i)
			}
		})
	}
	wg.Wait()
	plans.Lock()
	kept := len(plans.m)
	plans.Unlock()
	if kept > planKeep {
		t.Errorf("матриц в кэше %d, больше %d", kept, planKeep)
	}
}

// BenchmarkPlan600 — окно автосборки на свадьбе 600 фото, холодный кэш, N = 30.
func BenchmarkPlan600(b *testing.B) {
	asp := weddingAspects(600, 600)
	for b.Loop() {
		layout.ResetCache()
		PlanGroups(asp, album30, 30, 64)
	}
}

// BenchmarkPlan600Again — тот же набор, ползунок сдвинули: матрица уже есть, считается DP.
func BenchmarkPlan600Again(b *testing.B) {
	asp := weddingAspects(600, 600)
	layout.ResetCache()
	PlanGroups(asp, album30, 30, 64)
	N := 40
	for b.Loop() {
		PlanGroups(asp, album30, N, 64)
		N = 81 - N // 40 ↔ 41
	}
}

// BenchmarkPlanBig — окно автосборки на больших альбомах: холодно при N по умолчанию
// (≈ фото/4, узкая матрица) и при широкой (до 64 фото на разворот), шаг ползунка по
// готовой матрице. go test ./internal/layout/plan -run x -bench PlanBig -benchtime=1x -benchmem
func BenchmarkPlanBig(b *testing.B) {
	for _, n := range []int{600, 1500, 5000} {
		asp := weddingAspects(uint64(n), n)
		wide := max(n/63, (n+63)/64)
		b.Run(fmt.Sprintf("n%d/cold-default", n), func(b *testing.B) {
			for b.Loop() {
				layout.ResetCache()
				PlanGroups(asp, album30, n/4, 64)
			}
		})
		b.Run(fmt.Sprintf("n%d/cold-wide", n), func(b *testing.B) {
			for b.Loop() {
				layout.ResetCache()
				PlanGroups(asp, album30, wide, 64)
			}
		})
		b.Run(fmt.Sprintf("n%d/default-then-wide", n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				layout.ResetCache()
				PlanGroups(asp, album30, n/4, 64)
				b.StartTimer()
				PlanGroups(asp, album30, wide, 64)
			}
		})
		for _, N := range []int{wide, n / 4, n / 2} {
			// Ползунок у N: матрица для N и N+1 уже есть, считается только DP.
			b.Run(fmt.Sprintf("n%d/step-N%d", n, N), func(b *testing.B) {
				layout.ResetCache()
				PlanGroups(asp, album30, N, 64)
				PlanGroups(asp, album30, N+1, 64)
				for i := 0; b.Loop(); i++ {
					PlanGroups(asp, album30, N+i%2, 64)
				}
			})
		}
	}
}

// stockAspects — пропорции 100 фото stock (testkit.StockAspects: общие с variety).
func stockAspects(t testing.TB) []float64 { return testkit.StockAspects(t) }

// seriesAspects — синтетика «серии» (research §6): серии по 2–30 фото, в серии 80 % одной
// ориентации; cuts — 1 перед первым фото каждой серии, кроме первой (как папки).
func seriesAspects(seed uint64, n int) (asp, cuts []float64) {
	rng := splitmix64(seed*104729 + uint64(n))
	asp, cuts = make([]float64, n), make([]float64, n+1)
	for i := 0; i < n; {
		if i > 0 {
			cuts[i] = 1
		}
		size, main, other := 2+int(rng.unit()*29), L, P
		if rng.unit() < 0.35 {
			main, other = P, L
		}
		for end := min(n, i+size); i < end; i++ {
			asp[i] = main
			if rng.unit() >= 0.8 {
				asp[i] = other
			}
		}
	}
	return asp, cuts
}

// rangeStats — метрики разброса плана: скачков (соседи различаются ≥ 1,5 раза), ровных
// (соседи ±1), попаданий разреза в границу серии и самих границ.
type rangeStats struct{ jumps, even, hits, bounds int }

func statsOf(pl Plan, cuts []float64) rangeStats {
	var s rangeStats
	for i := 1; i < len(pl.Groups); i++ {
		x, y := pl.Groups[i-1][1]-pl.Groups[i-1][0], pl.Groups[i][1]-pl.Groups[i][0]
		if float64(max(x, y)) >= 1.5*float64(min(x, y)) {
			s.jumps++
		}
		if x-y <= 1 && y-x <= 1 {
			s.even++
		}
		if cuts != nil && cuts[pl.Groups[i][0]] >= 0.5 {
			s.hits++
		}
	}
	for _, c := range cuts {
		if c >= 0.5 {
			s.bounds++
		}
	}
	return s
}

func sizes(pl Plan) []int {
	out := make([]int, len(pl.Groups))
	for i, g := range pl.Groups {
		out[i] = g[1] - g[0]
	}
	return out
}

// distinctAspects — n разных пропорций: ни одно окно
// не берётся из кэша раскладок, худший случай холодной матрицы.
func distinctAspects(n int) []float64 {
	asp := make([]float64, n)
	for i := range asp {
		asp[i] = 0.55 + 1.3*float64((i*389)%n)/float64(n)
	}
	return asp
}

// BenchmarkPlanRange600 — окно автосборки 051: матрица в кэше, «Авто» и серии, ползунок
// N 50 ↔ 51 — считается только DP (§12: ≤ 20 мс).
func BenchmarkPlanRange600(b *testing.B) {
	asp, cuts := seriesAspects(1, 600)
	opts := func(N int) PlanOpts {
		lo, hi := AutoPer(600, N)
		return PlanOpts{Spreads: N, MinPer: lo, MaxPer: hi, Cuts: cuts}
	}
	layout.ResetCache()
	PlanRange(asp, album30, opts(50))
	PlanRange(asp, album30, opts(51))
	for i := 0; b.Loop(); i++ {
		PlanRange(asp, album30, opts(50+i%2))
	}
}

// BenchmarkPlanRange600Distinct — холодно, 600 разных пропорций, «Авто» (§12: ≤ 1,5 с,
// иначе запасной ход). old — прежний путь на том же N для сравнения: окна до 7 фото
// (полный перебор) у обоих.
func BenchmarkPlanRange600Distinct(b *testing.B) {
	asp := distinctAspects(600)
	lo, hi := AutoPer(600, 50)
	b.Run("new", func(b *testing.B) {
		for b.Loop() {
			layout.ResetCache()
			PlanRange(asp, album30, PlanOpts{Spreads: 50, MinPer: lo, MaxPer: hi})
		}
	})
	b.Run("old", func(b *testing.B) {
		for b.Loop() {
			layout.ResetCache()
			PlanGroups(asp, album30, 50, 64)
		}
	})
}

func TestAutoPer(t *testing.T) {
	for _, c := range []struct{ n, N, a, b int }{
		{100, 11, 3, 16}, {124, 10, 4, 22}, {200, 10, 7, 35}, {39, 10, 1, 7}, // спека: 9,1 / 12,4 / 20 / 3,9
		{0, 5, 0, 0}, {-1, 5, 0, 0}, {1, 1, 1, 3}, {10, 99, 1, 3}, // N зажат сверху n
		{10, 0, 3, 18}, {600, 5, 20, 64}, // снизу ⌈n/64⌉; «до» не больше layout.MaxPhotos
	} {
		if a, b := AutoPer(c.n, c.N); a != c.a || b != c.b {
			t.Errorf("AutoPer(%d, %d) = %d…%d, ждали %d…%d", c.n, c.N, a, b, c.a, c.b)
		}
	}
}

func TestRangeSpreads(t *testing.T) {
	for _, c := range []struct{ n, a, b, lo, hi int }{
		{0, 3, 16, 0, 0}, {100, 3, 16, 7, 33}, {100, 8, 10, 10, 12}, // Q3: свой 8–10 — N в [10, 12]
		{100, 0, 16, 7, 100}, {100, 1, 200, 2, 100}, {100, 20, 10, 10, 10}, // «от» > «до» — «от» = «до»
		{5, 8, 10, 1, 1}, // фото меньше, чем «от», — один разворот
		{10, 6, 7, 2, 2}, // 10 не делится на группы по 6–7: верх = низу, «от» уступает
	} {
		if lo, hi := RangeSpreads(c.n, c.a, c.b); lo != c.lo || hi != c.hi {
			t.Errorf("RangeSpreads(%d, %d, %d) = %d, %d, ждали %d, %d", c.n, c.a, c.b, lo, hi, c.lo, c.hi)
		}
	}
	// RangePer — действующие «от» и «до» подсказки (AutoPlan.From/To); spreads 0 не делит на ноль.
	for _, c := range []struct{ n, N, a, b, from, to int }{
		{100, 11, 3, 16, 3, 16}, {10, 2, 6, 7, 5, 7}, {5, 1, 8, 10, 5, 10}, {100, 10, 20, 10, 10, 10}, {7, 0, 3, 0, 1, 1},
	} {
		if a, b := RangePer(c.n, c.N, c.a, c.b); a != c.from || b != c.to {
			t.Errorf("RangePer(%d, %d, %d, %d) = %d, %d, ждали %d, %d", c.n, c.N, c.a, c.b, a, b, c.from, c.to)
		}
	}
}

// TestPlanRangeZeroIsPlanGroups — MinPer ≤ 0 — прежний PlanGroups побитово, серии не действуют.
func TestPlanRangeZeroIsPlanGroups(t *testing.T) {
	stock := stockAspects(t)
	ser, cuts := seriesAspects(3, 90)
	for _, p := range invariantParams {
		for _, set := range []struct {
			asp, cuts []float64
		}{{stock, nil}, {ser, cuts}, {nil, nil}} {
			for _, c := range [][2]int{{11, 64}, {1, 64}, {40, 5}, {0, 64}, {999, 3}} {
				for _, minPer := range []int{0, -3} {
					want := PlanGroups(set.asp, p, c[0], c[1])
					got := PlanRange(set.asp, p, PlanOpts{Spreads: c[0], MinPer: minPer, MaxPer: c[1], Cuts: set.cuts})
					if !reflect.DeepEqual(got, want) {
						t.Errorf("n=%d N=%d m=%d от %d: %v, PlanGroups %v", len(set.asp), c[0], c[1], minPer, got.Groups, want.Groups)
					}
				}
			}
		}
	}
}

// TestPlanRangeBounds — ровно N (зажатое в RangeSpreads) групп подряд, каждая в [от, до];
// края: мало фото, «от» > «до», неделимый диапазон, N вне пределов, пустой набор.
func TestPlanRangeBounds(t *testing.T) {
	if pl := PlanRange(nil, album30, PlanOpts{Spreads: 3, MinPer: 2, MaxPer: 5}); !reflect.DeepEqual(pl, Plan{}) {
		t.Errorf("пустой набор: %+v", pl)
	}
	for _, n := range []int{1, 2, 5, 10, 37, 100} {
		asp := weddingAspects(uint64(n), n)
		for _, r := range [][2]int{{1, 1}, {1, 64}, {3, 16}, {6, 7}, {8, 10}, {20, 30}, {5, 3}, {2, 99}} {
			lo, hi := RangeSpreads(n, r[0], r[1])
			for _, N := range []int{-1, 0, 1, lo, (lo + hi) / 2, hi, 999} {
				pl := PlanRange(asp, album30, PlanOpts{Spreads: N, MinPer: r[0], MaxPer: r[1]})
				want := max(lo, min(N, hi))
				a, b := perRange(n, r[0], r[1])
				checkPlan(t, pl, n, want, b)
				if pl.Min < min(a, n/want) {
					t.Errorf("n=%d %d…%d N=%d: группа %d меньше «от»", n, r[0], r[1], N, pl.Min)
				}
				if want*a <= n && pl.Min < a {
					t.Errorf("n=%d %d…%d N=%d: «от» %d делимо, а группа %d", n, r[0], r[1], N, a, pl.Min)
				}
			}
		}
	}
}

// TestPlanRangeMetrics — разброс «Авто» (спека 051, критерии): stock 100 на 11 разворотов —
// от ≤ 4 до ≥ 15, ≥ 4 соседних пар с разницей ≥ 1,5 раза, первый разворот редкий; синтетика
// 600/50 с папками — разрез на ≥ 50 % границ серий и заметно чаще, чем без серий.
// Подбор чисел — go test ./internal/layout/plan -run PlanRangeMetrics -v.
func TestPlanRangeMetrics(t *testing.T) {
	stock := stockAspects(t)
	for _, p := range invariantParams {
		a, b := AutoPer(100, 11)
		pl := PlanRange(stock, p, PlanOpts{Spreads: 11, MinPer: a, MaxPer: b})
		old := PlanGroups(stock, p, 11, 64)
		st := statsOf(pl, nil)
		t.Logf("stock layflat=%t %d…%d: %v %+v | 0.12: %v %+v", p.Layflat, a, b, sizes(pl), st, sizes(old), statsOf(old, nil))
		if pl.Min > 4 || pl.Max < 15 || st.jumps < 4 || st.even > 2 {
			t.Errorf("stock layflat=%t: от %d до %d, скачков %d, ровных %d — мало разброса", p.Layflat, pl.Min, pl.Max, st.jumps, st.even)
		}
		if first := sizes(pl)[0]; float64(first) > pl.Avg*0.6 {
			t.Errorf("stock layflat=%t: первый разворот %d фото — не редкий", p.Layflat, first)
		}
	}
	for seed := uint64(1); seed <= 3; seed++ {
		asp, cuts := seriesAspects(seed, 600)
		a, b := AutoPer(600, 50)
		with := statsOf(PlanRange(asp, album30, PlanOpts{Spreads: 50, MinPer: a, MaxPer: b, Cuts: cuts}), cuts)
		without := statsOf(PlanRange(asp, album30, PlanOpts{Spreads: 50, MinPer: a, MaxPer: b}), cuts)
		t.Logf("серии 600/50 сид %d: %+v | без серий %+v", seed, with, without)
		if 2*with.hits < with.bounds || with.hits < 2*without.hits {
			t.Errorf("сид %d: на границах серий %d из %d (без серий %d)", seed, with.hits, with.bounds, without.hits)
		}
	}
	// Прочие объёмы research §6 — серии не должны сжимать разброс: низ у «от», верх у «до».
	for _, c := range [][2]int{{100, 11}, {161, 16}, {600, 30}, {900, 30}} {
		asp, cuts := seriesAspects(1, c[0])
		a, b := AutoPer(c[0], c[1])
		pl := PlanRange(asp, album30, PlanOpts{Spreads: c[1], MinPer: a, MaxPer: b, Cuts: cuts})
		st := statsOf(pl, cuts)
		t.Logf("серии %d/%d %d…%d: от %d до %d %+v", c[0], c[1], a, b, pl.Min, pl.Max, st)
	}
}

// TestPlanRangeNarrow — узкий диапазон почти ровный, как 0.12 (спека, сценарий 3):
// размеры в его пределах, перепад не больше, чем у прежнего плана.
func TestPlanRangeNarrow(t *testing.T) {
	stock := stockAspects(t)
	for _, p := range invariantParams {
		old := PlanGroups(stock, p, 11, 64)
		pl := PlanRange(stock, p, PlanOpts{Spreads: 11, MinPer: 8, MaxPer: 10})
		checkPlan(t, pl, 100, 11, 10)
		if pl.Min < 8 || pl.Max-pl.Min > old.Max-old.Min {
			t.Errorf("layflat=%t 8…10: %v, 0.12 %v", p.Layflat, sizes(pl), sizes(old))
		}
		if pl := PlanRange(stock, p, PlanOpts{Spreads: 10, MinPer: 10, MaxPer: 10}); pl.Min != 10 || pl.Max != 10 {
			t.Errorf("layflat=%t 10…10: %v", p.Layflat, sizes(pl))
		}
	}
}

// TestPlanRangeCuts — силы границ: длина не n+1 — серий нет, NaN и минус — 0, больше 1 — 1;
// сильная граница рядом с разрезом без серий притягивает его.
func TestPlanRangeCuts(t *testing.T) {
	asp, cuts := seriesAspects(2, 120)
	o := PlanOpts{Spreads: 12, MinPer: 3, MaxPer: 18}
	plain := PlanRange(asp, album30, o)
	for _, bad := range [][]float64{cuts[:120], append(slices.Clone(cuts), 1), make([]float64, 121)} {
		o.Cuts = bad
		if got := PlanRange(asp, album30, o); !reflect.DeepEqual(got, plain) {
			t.Errorf("Cuts длины %d (нули или чужая длина): %v, без серий %v", len(bad), got.Groups, plain.Groups)
		}
	}
	odd, norm := slices.Clone(cuts), slices.Clone(cuts)
	for j := range odd {
		switch j % 3 {
		case 0:
			odd[j], norm[j] = math.NaN(), 0
		case 1:
			if odd[j] > 0 {
				odd[j] = 7
			} else {
				odd[j] = -2
			}
		}
	}
	o.Cuts = norm
	want := PlanRange(asp, album30, o)
	o.Cuts = odd
	if got := PlanRange(asp, album30, o); !reflect.DeepEqual(got, want) {
		t.Errorf("NaN/−2/7 вместо 0/0/1: %v, ждали %v", got.Groups, want.Groups)
	}
	// Граница в 2 фото от каждого разреза плана без серий: разрез к ней переезжает.
	for _, g := range plain.Groups[1:] {
		at := g[0] + 2
		one := make([]float64, 121)
		one[at] = 1
		o.Cuts = one
		got := PlanRange(asp, album30, o)
		if !slices.ContainsFunc(got.Groups, func(h [2]int) bool { return h[0] == at }) {
			t.Errorf("граница серии перед %d не стала разрезом: %v", at, got.Groups)
		}
	}
}

// TestPlanRangeDeterministic — план не зависит от прогретого кэша, порядка вызовов и
// параллельного счёта: подсказка окна и сборка совпадают.
func TestPlanRangeDeterministic(t *testing.T) {
	asp, cuts := seriesAspects(4, 200)
	o := PlanOpts{Spreads: 20, MinPer: 3, MaxPer: 18, Cuts: cuts}
	layout.ResetCache()
	want := PlanRange(asp, album30, o)
	layout.ResetCache()
	PlanGroups(asp, album30, 40, 64) // матрица частично: столбцы прежнего пути
	PlanRange(asp, album30, PlanOpts{Spreads: 30, MinPer: 2, MaxPer: 12})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := PlanRange(asp, album30, o); !reflect.DeepEqual(got, want) {
				t.Errorf("после прогрева или параллельно: %v, холодный %v", got.Groups, want.Groups)
			}
		})
	}
	wg.Wait()
}

// TestRangeTargets — волна: сумма целей n, первый разворот — самый редкий, при «от» = «до»
// цели ровные.
func TestRangeTargets(t *testing.T) {
	tg := rangeTargets(100, 11, 3, 16)
	sum := 0.0
	for _, v := range tg {
		sum += v
	}
	if math.Abs(sum-100) > 1e-9 || slices.Min(tg) != tg[0] || slices.Max(tg) < 15 {
		t.Errorf("цели %v (сумма %g)", tg, sum)
	}
	for _, v := range rangeTargets(100, 10, 10, 10) {
		if math.Abs(v-10) > 1e-9 {
			t.Fatalf("«от» = «до»: цель %g", v)
		}
	}
}
