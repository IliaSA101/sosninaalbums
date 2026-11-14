package plan

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// checkWalls — ни одна группа не пересекает стену: на каждой стене есть граница групп.
func checkWalls(t *testing.T, pl Plan, walls []int, msg string) {
	t.Helper()
	for _, w := range walls {
		for _, g := range pl.Groups {
			if g[0] < w && w < g[1] {
				t.Fatalf("%s: группа %v пересекает стену %d (все: %v)", msg, g, w, pl.Groups)
			}
		}
	}
}

// randWalls — count разных стен в 1…n−1 по возрастанию.
func randWalls(rng *splitmix64, n, count int) []int {
	set := map[int]bool{}
	for len(set) < min(count, n-1) {
		set[1+int(rng.next()%uint64(n-1))] = true
	}
	out := make([]int, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	slices.Sort(out)
	return out
}

// TestPlanWallsNilSame — без стен (nil, пусто, 0 и n, повторы вне диапазона) план тот же,
// что прежний, на обоих путях: «ровно» (набор goldenPlan, эталон many.sha) и диапазон
// (набор plan051Groups, эталон plan051.sha); сами эталоны проверяют TestGoldenPlan*.
func TestPlanWallsNilSame(t *testing.T) {
	if testing.Short() {
		t.Skip("600 фото: полный уровень")
	}
	for g := range 2 {
		p := goldenGeoms[g]
		asp := weddingAspects(uint64(g)+1, 600)
		n := len(asp)
		for _, c := range [][2]int{{150, 64}, {30, 64}, {130, 5}} {
			want := PlanGroups(asp, p, c[0], c[1])
			for _, w := range [][]int{nil, {}, {0, n}, {-3, n + 5, 0}} {
				if got := PlanGroupsWalls(asp, p, c[0], c[1], w); !slices.Equal(got.Groups, want.Groups) {
					t.Fatalf("геометрия %d N=%d m=%d стены %v: план разошёлся с прежним", g, c[0], c[1], w)
				}
			}
			if got := PlanRange(asp, p, PlanOpts{Spreads: c[0], MaxPer: c[1], Walls: []int{0, n}}); !slices.Equal(got.Groups, want.Groups) {
				t.Fatalf("геометрия %d N=%d m=%d: PlanRange без MinPer со стенами {0,n} разошёлся", g, c[0], c[1])
			}
		}
	}
	wed := weddingAspects(51, 100)
	cuts := make([]float64, 101)
	cuts[23], cuts[51], cuts[77] = 1, 0.6, 0.3
	for _, r := range [][2]int{{3, 9}, {2, 18}, {8, 10}} {
		for _, cu := range [][]float64{nil, cuts} {
			o := PlanOpts{Spreads: 11, MinPer: r[0], MaxPer: r[1], Cuts: cu, Phase: 2}
			want := PlanRange(wed, goldenGeoms[0], o)
			for _, w := range [][]int{nil, {}, {0, 100}, {100, 100, 0}} {
				o.Walls = w
				if got := PlanRange(wed, goldenGeoms[0], o); !slices.Equal(got.Groups, want.Groups) {
					t.Fatalf("диапазон %v серии %t стены %v: план разошёлся с прежним", r, cu != nil, w)
				}
			}
		}
	}
	// Вход стен не меняется (нормализация — на копии).
	w := []int{9, 3, 3, 0, 40}
	_ = PlanGroupsWalls(weddingAspects(1, 30), album30, 6, 64, w)
	if !slices.Equal(w, []int{9, 3, 3, 0, 40}) {
		t.Errorf("стены изменены: %v", w)
	}
}

// TestPlanWallsHard — ни одна группа не пересекает стену на всех путях: «ровно» (m = 64 и 6),
// свой диапазон, «Авто» (AutoPer), вариант (Phase), с сериями; число групп — ровно N.
func TestPlanWallsHard(t *testing.T) {
	rng := splitmix64(107)
	iters := 12
	if testkit.Lite() {
		iters = 4
	}
	for it := range iters {
		n := 40 + int(rng.next()%90)
		asp := weddingAspects(uint64(it)+7, n)
		p := invariantParams[it%2]
		walls := randWalls(&rng, n, 1+int(rng.next()%8))
		cuts := make([]float64, n+1)
		for _, w := range randWalls(&rng, n, 5) {
			cuts[w] = rng.unit()
		}
		pick := func(lo, hi int) int { return lo + int(rng.next()%uint64(hi-lo+1)) }

		for _, m := range []int{64, 6} {
			lo, hi := SpreadRangeWalls(n, m, walls)
			N := pick(lo, min(hi, lo+20))
			pl := PlanGroupsWalls(asp, p, N, m, walls)
			checkPlan(t, pl, n, N, m)
			checkWalls(t, pl, walls, fmt.Sprintf("ровно n=%d m=%d N=%d", n, m, N))
		}

		lo, hi := RangeSpreadsWalls(n, 3, 9, walls)
		N := pick(lo, hi)
		o := PlanOpts{Spreads: N, MinPer: 3, MaxPer: 9, Walls: walls}
		for _, cu := range [][]float64{nil, cuts} {
			for _, ph := range []int{0, 2} {
				o.Cuts, o.Phase = cu, ph
				pl := PlanRange(asp, p, o)
				checkPlan(t, pl, n, N, 9)
				checkWalls(t, pl, walls, fmt.Sprintf("свой n=%d N=%d серии %t фаза %d", n, N, cu != nil, ph))
			}
		}

		lo, hi = SpreadRangeWalls(n, layout.MaxPhotos, walls)
		N = pick(lo, min(hi, lo+15))
		from, to := AutoPerWalls(n, N, walls)
		rlo, rhi := RangeSpreadsWalls(n, from, to, walls)
		if rlo > N {
			t.Fatalf("Авто n=%d N=%d: нижний предел по «до» %d выше N (до = %d)", n, N, rlo, to)
		}
		N = min(N, rhi)
		pl := PlanRange(asp, p, PlanOpts{Spreads: N, MinPer: from, MaxPer: to, Cuts: cuts, Walls: walls})
		checkPlan(t, pl, n, N, to)
		checkWalls(t, pl, walls, fmt.Sprintf("Авто n=%d N=%d", n, N))
	}
}

// TestPlanWallsShortSegment — отрезки из 1–2 фото между якорями при «от 3»: план есть,
// короткий отрезок — одна группа, остальные группы не короче «от».
func TestPlanWallsShortSegment(t *testing.T) {
	// Отрезки: 12, 1, 2, 12, 2, 1, 15 → 45 фото.
	segs := []int{12, 1, 2, 12, 2, 1, 15}
	var walls []int
	n := 0
	for _, l := range segs[:len(segs)-1] {
		n += l
		walls = append(walls, n)
	}
	n += segs[len(segs)-1]
	asp := weddingAspects(3, n)
	lo, hi := RangeSpreadsWalls(n, 3, 6, walls)
	if lo != 2+1+1+2+1+1+3 {
		t.Fatalf("нижний предел N = %d", lo)
	}
	for _, N := range []int{lo, (lo + hi) / 2, hi} {
		pl := PlanRange(asp, album30, PlanOpts{Spreads: N, MinPer: 3, MaxPer: 6, Walls: walls})
		checkPlan(t, pl, n, N, 6)
		checkWalls(t, pl, walls, fmt.Sprintf("N=%d", N))
		start := 0
		for _, l := range segs {
			var inSeg [][2]int
			for _, g := range pl.Groups {
				if g[0] >= start && g[1] <= start+l {
					inSeg = append(inSeg, g)
				}
			}
			if l < 3 && len(inSeg) != 1 {
				t.Errorf("N=%d: отрезок из %d фото разбит на %v", N, l, inSeg)
			}
			if l >= 12 {
				for _, g := range inSeg {
					if a, _ := RangePer(n, N, 3, 6); g[1]-g[0] < a { // «от» уступает n/N, как без стен
						t.Errorf("N=%d: в длинном отрезке группа %v короче «от» %d", N, g, a)
					}
				}
			}
			start += l
		}
	}
}

// TestSpreadRangeWalls — пределы N со стенами; без стен — прежние функции.
func TestSpreadRangeWalls(t *testing.T) {
	for _, c := range []struct {
		n, m  int
		walls []int
		lo    int
	}{
		{0, 64, []int{3}, 0},
		{10, 64, nil, 1},
		{10, 64, []int{4}, 2},
		{10, 4, []int{4}, 3},            // 4 + 6 → 1 + 2
		{10, 4, []int{3, 7}, 3},         // 3 + 4 + 3
		{10, 3, []int{3, 7}, 4},         // 1 + 2 + 1
		{10, 64, []int{0, 10, 5, 5}, 2}, // 0 и n отброшены, повтор схлопнут
		{10, 64, []int{10, 0}, 1},
		{600, 7, []int{100, 200, 300}, 15 + 15 + 15 + 43}, // отрезки 100, 100, 100, 300
	} {
		lo, hi := SpreadRangeWalls(c.n, c.m, c.walls)
		if lo != c.lo || hi != c.n {
			t.Errorf("SpreadRangeWalls(%d, %d, %v) = %d, %d, ждали %d, %d", c.n, c.m, c.walls, lo, hi, c.lo, c.n)
		}
		if len(normWalls(c.walls, c.n)) == 0 {
			wl, wh := SpreadRange(c.n, c.m)
			if lo != wl || hi != wh {
				t.Errorf("без стен (%v): %d, %d, SpreadRange %d, %d", c.walls, lo, hi, wl, wh)
			}
		}
	}
	for _, c := range []struct {
		n, a, b int
		walls   []int
		lo, hi  int
	}{
		{0, 3, 6, []int{2}, 0, 0},
		{20, 3, 6, nil, 4, 6},
		{20, 3, 6, []int{1}, 1 + 4, 1 + 6},            // 1 | 19
		{20, 3, 6, []int{1, 3}, 1 + 1 + 3, 1 + 1 + 5}, // 1 | 2 | 17
		{7, 4, 5, []int{3}, 1 + 1, 1 + 1},             // 3 | 4
		{7, 4, 5, []int{0, 7}, 2, 2},                  // стен нет: RangeSpreads
	} {
		lo, hi := RangeSpreadsWalls(c.n, c.a, c.b, c.walls)
		wl, wh := c.lo, c.hi
		if len(normWalls(c.walls, c.n)) == 0 {
			wl, wh = RangeSpreads(c.n, c.a, c.b)
		}
		if lo != wl || hi != wh {
			t.Errorf("RangeSpreadsWalls(%d, %d, %d, %v) = %d, %d, ждали %d, %d", c.n, c.a, c.b, c.walls, lo, hi, wl, wh)
		}
		if lo > hi && c.n > 0 {
			t.Errorf("RangeSpreadsWalls(%d, %d, %d, %v): верх %d ниже низа %d", c.n, c.a, c.b, c.walls, hi, lo)
		}
	}
	// Верх не ниже низа и в узком случае: 7 фото по 4–5 без стен — две группы никак не «от 4».
	if lo, hi := RangeSpreadsWalls(7, 4, 5, []int{3}); lo > hi {
		t.Errorf("отрезок 4 + 3: %d > %d", lo, hi)
	}
}

// TestPlanWallsWidth — длинный отрезок рядом с короткими при N у нижнего предела: окно
// шире planWidth, план есть (иначе длинный отрезок не делится на нужное число групп).
func TestPlanWallsWidth(t *testing.T) {
	// 40 фото, затем десять отрезков по 1–2 фото: нижний предел 1 + 10 при m = 64.
	segs := []int{40, 1, 2, 1, 2, 1, 1, 2, 1, 2, 1}
	var walls []int
	n := 0
	for _, l := range segs[:len(segs)-1] {
		n += l
		walls = append(walls, n)
	}
	n += segs[len(segs)-1]
	asp := weddingAspects(5, n)
	lo, _ := SpreadRangeWalls(n, 64, walls)
	if lo != len(segs) {
		t.Fatalf("нижний предел N = %d, ждали %d", lo, len(segs))
	}
	if K := wallsWidth(n, lo, 64, walls); K < 40 || planWidth(n, lo, 64) >= 40 {
		t.Fatalf("ширина окна %d (planWidth %d): без расширения случай не проверяет", K, planWidth(n, lo, 64))
	}
	pl := PlanGroupsWalls(asp, album30, lo, 64, walls)
	checkPlan(t, pl, n, lo, 64)
	checkWalls(t, pl, walls, "N у нижнего предела")
	if pl.Max != 40 {
		t.Errorf("группы %v: ждали одну группу из 40 фото", pl.Groups)
	}
	// Предел m тоже держится: те же отрезки при m = 12 — длинный на 4 группы.
	lo12, _ := SpreadRangeWalls(n, 12, walls)
	pl = PlanGroupsWalls(asp, album30, lo12, 12, walls)
	checkPlan(t, pl, n, lo12, 12)
	checkWalls(t, pl, walls, "m = 12")
}

// bruteBest — наибольшая сумма по всем разбиениям 0…n на N групп 1…K, не пересекающим стены
// (value оценивает разбиение целиком); −∞, если разбиений нет.
func bruteBest(n, N, K int, caps []int, value func(groups [][2]int) float64) float64 {
	best := math.Inf(-1)
	groups := make([][2]int, 0, N)
	var rec func(pos int)
	rec = func(pos int) {
		if len(groups) == N {
			if pos == n {
				best = max(best, value(groups))
			}
			return
		}
		for k := 1; k <= K && pos+k <= n; k++ {
			if caps[pos+k] < k {
				continue
			}
			groups = append(groups, [2]int{pos, pos + k})
			rec(pos + k)
			groups = groups[:len(groups)-1]
		}
	}
	rec(0)
	return best
}

// randCols — случайные столбцы cols[k][i] = k·оценка для k ≤ K.
func randCols(rng *splitmix64, n, K int) [][]float64 {
	cols := make([][]float64, K+1)
	for k := 1; k <= K; k++ {
		cols[k] = make([]float64, max(0, n-k+1))
		for i := range cols[k] {
			cols[k][i] = float64(k) * (-0.5 + 1.5*rng.unit())
		}
	}
	return cols
}

// TestPlanDPWallsOptimal — planDPCap на малых случайных матрицах даёт оптимум среди всех
// разбиений со стенами (перебором), без стен — прежний planDP.
func TestPlanDPWallsOptimal(t *testing.T) {
	rng := splitmix64(1070)
	for it := range 400 {
		n := 4 + int(rng.next()%9)
		walls := randWalls(&rng, n, int(rng.next()%4))
		m := 2 + int(rng.next()%5)
		lo, hi := SpreadRangeWalls(n, m, walls)
		N := lo + int(rng.next()%uint64(hi-lo+1))
		K := wallsWidth(n, N, m, walls)
		cols := randCols(&rng, n, K)
		caps := wallCaps(walls, n)
		avg := float64(n) / float64(N)
		value := func(groups [][2]int) float64 {
			s := 0.0
			for _, g := range groups {
				k := g[1] - g[0]
				d := float64(k) - avg
				s += cols[k][g[0]] - planLambda*d*d
			}
			return s
		}
		got, ok := planDPCap(cols, n, N, K, 5, caps)
		if !ok {
			t.Fatalf("it=%d n=%d N=%d K=%d стены %v: отказ без столбцов-границ", it, n, N, K, walls)
		}
		pl := Plan{Groups: got}
		checkWalls(t, pl, walls, fmt.Sprintf("it=%d", it))
		want := bruteBest(n, N, K, caps, value)
		if len(got) != N || math.Abs(value(got)-want) > 1e-9 {
			t.Fatalf("it=%d n=%d N=%d K=%d стены %v: группы %v, значение %g, оптимум %g", it, n, N, K, walls, got, value(got), want)
		}
	}
}

// TestRangeDPWallsOptimal — то же для rangeDPWalls: оптимум по цели волны, штрафу «от» и
// наградам серий.
func TestRangeDPWallsOptimal(t *testing.T) {
	rng := splitmix64(1071)
	for it := range 400 {
		n := 5 + int(rng.next()%9)
		walls := randWalls(&rng, n, 1+int(rng.next()%3))
		a := 1 + int(rng.next()%3)
		b := a + int(rng.next()%4)
		lo, hi := RangeSpreadsWalls(n, a, b, walls)
		N := lo + int(rng.next()%uint64(hi-lo+1))
		a, b = RangePer(n, N, a, b)
		cols := randCols(&rng, n, b)
		caps := wallCaps(walls, n)
		t0 := rangeTargetsPhase(n, N, a, b, it%3)
		cuts := make([]float64, n+1)
		if it%2 == 0 {
			for _, w := range randWalls(&rng, n, 3) {
				cuts[w] = rng.unit()
			}
		} else {
			cuts = nil
		}
		lam := rangeLambda * 9 / (float64(n) / float64(N))
		value := func(groups [][2]int) float64 {
			s := 0.0
			for c, g := range groups {
				k := g[1] - g[0]
				d := float64(k) - t0[c]
				s += cols[k][g[0]] - lam*d*d
				if k < a {
					s -= rangeLow * float64((a-k)*(a-k))
				}
				if cuts != nil && c < N-1 {
					s += rangeBeta * cuts[g[1]]
				}
			}
			return s
		}
		got := rangeDPWalls(cols, n, N, a, b, t0, cuts, caps)
		checkWalls(t, Plan{Groups: got}, walls, fmt.Sprintf("it=%d", it))
		want := bruteBest(n, N, b, caps, value)
		if len(got) != N || math.Abs(value(got)-want) > 1e-9 {
			t.Fatalf("it=%d n=%d N=%d a=%d b=%d стены %v: группы %v, значение %g, оптимум %g", it, n, N, a, b, walls, got, value(got), want)
		}
	}
}

// TestWallCaps — caps[j] = j − последняя стена < j.
func TestWallCaps(t *testing.T) {
	got := wallCaps([]int{3, 4, 8}, 10)
	want := []int{0, 1, 2, 3, 1, 1, 2, 3, 4, 1, 2}
	if !slices.Equal(got, want) {
		t.Errorf("wallCaps = %v, ждали %v", got, want)
	}
	if got := wallCaps(nil, 3); !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Errorf("без стен: %v", got)
	}
}
