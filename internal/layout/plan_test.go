package layout

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
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
	resetCache()
	warm := map[int][][2]int{}
	for _, N := range []int{30, 12, 5, 30} {
		warm[N] = PlanGroups(asp, album30, N, 64).Groups
	}
	for _, N := range []int{5, 12, 30} {
		resetCache()
		if cold := PlanGroups(asp, album30, N, 64).Groups; !slices.Equal(cold, warm[N]) {
			t.Errorf("N=%d: после досчёта %v, холодный %v", N, warm[N], cold)
		}
	}
}

// TestPlanConcurrent — параллельные запросы одного набора и вытеснение матриц (planKeep).
func TestPlanConcurrent(t *testing.T) {
	resetCache()
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
		resetCache()
		PlanGroups(asp, album30, 30, 64)
	}
}

// BenchmarkPlan600Again — тот же набор, ползунок сдвинули: матрица уже есть, считается DP.
func BenchmarkPlan600Again(b *testing.B) {
	asp := weddingAspects(600, 600)
	resetCache()
	PlanGroups(asp, album30, 30, 64)
	N := 40
	for b.Loop() {
		PlanGroups(asp, album30, N, 64)
		N = 81 - N // 40 ↔ 41
	}
}

// BenchmarkPlanBig — окно автосборки на больших альбомах: холодно при N по умолчанию
// (≈ фото/4, узкая матрица) и при широкой (до 64 фото на разворот), шаг ползунка по
// готовой матрице. go test ./internal/layout -run x -bench PlanBig -benchtime=1x -benchmem
func BenchmarkPlanBig(b *testing.B) {
	for _, n := range []int{600, 1500, 5000} {
		asp := weddingAspects(uint64(n), n)
		wide := max(n/63, (n+63)/64)
		b.Run(fmt.Sprintf("n%d/cold-default", n), func(b *testing.B) {
			for b.Loop() {
				resetCache()
				PlanGroups(asp, album30, n/4, 64)
			}
		})
		b.Run(fmt.Sprintf("n%d/cold-wide", n), func(b *testing.B) {
			for b.Loop() {
				resetCache()
				PlanGroups(asp, album30, wide, 64)
			}
		})
		b.Run(fmt.Sprintf("n%d/default-then-wide", n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				resetCache()
				PlanGroups(asp, album30, n/4, 64)
				b.StartTimer()
				PlanGroups(asp, album30, wide, 64)
			}
		})
		for _, N := range []int{wide, n / 4, n / 2} {
			// Ползунок у N: матрица для N и N+1 уже есть, считается только DP.
			b.Run(fmt.Sprintf("n%d/step-N%d", n, N), func(b *testing.B) {
				resetCache()
				PlanGroups(asp, album30, N, 64)
				PlanGroups(asp, album30, N+1, 64)
				for i := 0; b.Loop(); i++ {
					PlanGroups(asp, album30, N+i%2, 64)
				}
			})
		}
	}
}
