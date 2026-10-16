package plan

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Прежний путь матрицы и DP (до ускорения 027) — эталон для сверки: ускорение не должно
// менять ни одного числа матрицы и ни одной группы.

// refCols — прежняя матрица cols[k][i] = k · оценка окна [i, i+k), k ≤ K (параллельно по k).
func refCols(asp []float64, p layout.Params, K int) [][]float64 {
	n := len(asp)
	cols := make([][]float64, K+1)
	var wg sync.WaitGroup
	for k := 1; k <= K; k++ {
		cols[k] = make([]float64, max(0, n-k+1))
		wg.Go(func() {
			for i := 0; i+k <= n; i++ {
				w := asp[i : i+k]
				s := smallScore
				if k > layout.MaxFullTree {
					s = layout.RefQuickScore
				}
				cols[k][i] = float64(k) * s(w, p)
			}
		})
	}
	wg.Wait()
	return cols
}

// refPlanDP — прежний planDP без изменений.
func refPlanDP(cols [][]float64, n, N, K int) [][2]int {
	avg := float64(n) / float64(N)
	inf := math.Inf(-1)
	prev, cur := make([]float64, n+1), make([]float64, n+1)
	for j := range prev {
		prev[j] = inf
	}
	prev[0] = 0
	from := make([]uint8, N*(n+1))
	for c := 1; c <= N; c++ {
		for j := range cur {
			cur[j] = inf
		}
		for j := max(c, n-(N-c)*K); j <= min(n, c*K); j++ {
			at := (c-1)*(n+1) + j
			for k := 1; k <= K && k <= j; k++ {
				pv := prev[j-k]
				if math.IsInf(pv, -1) {
					continue
				}
				d := float64(k) - avg
				if v := pv + cols[k][j-k] - planLambda*d*d; v > cur[j] {
					cur[j], from[at] = v, uint8(k)
				}
			}
		}
		prev, cur = cur, prev
	}
	groups := make([][2]int, N)
	for c, j := N, n; c > 0; c-- {
		k := int(from[(c-1)*(n+1)+j])
		groups[c-1] = [2]int{j - k, j}
		j -= k
	}
	return groups
}

// planGeoms — геометрии эталона и поля 0 (фото страниц у самого корешка).
var planGeoms = append(slices.Clone(goldenGeoms), layout.Params{PageW: 300, PageH: 300, Margin: 0, Gap: 3, Layflat: true})

// planSets — наборы пропорций: свадьба, кадрирование, случайные и крайние (панорамы, полоски).
func planSets(seed uint64, n int) [][]float64 {
	sets := [][]float64{weddingAspects(seed, n), goldenAspects(seed, n, 1), goldenAspects(seed, n, 2)}
	rng := splitmix64(seed*31 + uint64(n))
	wild := make([]float64, n)
	for i := range wild {
		wild[i] = math.Exp(3.2 * (rng.unit() - 0.5)) // от 0,2 до 5
	}
	return append(sets, wild)
}

func sameFloat(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || math.IsNaN(a) && math.IsNaN(b)
}

// TestRowsSame — деревья рядов segRows и их раскладка fit совпадают с lineNodes + fitTree
// до бита: тот же порядок деревьев, высота, глубина, прямоугольники, отказ.
func TestRowsSame(t *testing.T) {
	w := newPlanWork()
	var seg segRows
	geoms := planGeoms
	if testkit.Race {
		geoms = geoms[:1] // чистый счёт: остальные геометрии — в обычном прогоне
	}
	for gi, p := range geoms {
		g := layout.NewGeom(p)
		for si, asp := range planSets(uint64(gi)+3, 80) {
			ref := layout.RefRows(asp, p.Gap)
			for _, box := range []layout.Rect{g.Pages[0], g.Pages[1], g.Spread} {
				for _, s := range []int{0, 7, 16} {
					w.rows(asp, s, 1, 64, box, p.Gap, &seg)
					for m := 1; m <= 64; m++ {
						where := fmt.Sprintf("geom %d set %d box %v s=%d m=%d", gi, si, box, s, m)
						checkRows(t, where, w, &seg, ref(s, m, box), asp[s:s+m], box, p.Gap)
					}
				}
			}
		}
	}
}

// checkRows сверяет деревья отрезка [s, s+m) с прежними want.
func checkRows(t *testing.T, where string, w *planWork, seg *segRows, want []layout.RefFit, asp []float64, box layout.Rect, gap float64) {
	t.Helper()
	m := len(asp)
	var got [][]int
	for d := -1; d <= 1; d++ {
		if c := seg.tree(m, d); c != nil {
			got = append(got, c)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s: деревьев %d, ждали %d", where, len(got), len(want))
	}
	for x, ref := range want {
		fast := make([]layout.Rect, m)
		hf, depth, okf := w.fit(asp, got[x], box, gap, fast)
		if ref.Ok != okf || ref.Ok && (!sameFloat(ref.H, hf) || depth != ref.Depth) {
			t.Fatalf("%s дерево %d: ok %t/%t h %g/%g depth %d/%d", where, x, okf, ref.Ok, hf, ref.H, depth, ref.Depth)
		}
		if !ref.Ok {
			continue
		}
		for i, r := range fast {
			if q := ref.Rects[i]; !sameFloat(r.X, q.X) || !sameFloat(r.Y, q.Y) || !sameFloat(r.W, q.W) || !sameFloat(r.H, q.H) {
				t.Fatalf("%s дерево %d фото %d: %v, ждали %v", where, x, i, r, q)
			}
		}
	}
}

func sameCols(t *testing.T, where string, got, want [][]float64) {
	t.Helper()
	for k := range got {
		for i := range got[k] {
			if !sameFloat(got[k][i], want[k][i]) {
				t.Fatalf("%s: окно [%d, +%d) = %v, прежнее %v", where, i, k, got[k][i], want[k][i])
			}
		}
	}
}

// TestWindowColsSame — матрица побитово как прежняя: все k до 64, и досчёт столбцов
// с середины (from > 1, часть окон до maxFullTree). Прежний счёт дорогой, поэтому на
// геометрию — два набора из четырёх, по очереди.
func TestWindowColsSame(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	for gi, p := range planGeoms {
		sets := planSets(uint64(gi)+1, 66)
		for _, si := range []int{gi % 2, 2 + gi%2} {
			asp := sets[si]
			want := refCols(asp, p, layout.MaxPhotos)
			where := fmt.Sprintf("geom %d set %d", gi, si)
			sameCols(t, where, windowCols(asp, p, 1, layout.MaxPhotos), want[1:])
			sameCols(t, where+" с 5", windowCols(asp, p, 5, 20), want[5:21])
			sameCols(t, where+" с 21", windowCols(asp, p, 21, 33), want[21:34])
		}
	}
}

// TestPlanSameAsBefore — группы PlanGroups те же, что у прежнего пути (прежняя матрица
// и прежний DP), при разных N и maxPer.
func TestPlanSameAsBefore(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	// Прежняя матрица на сотнях фото — секунды, поэтому большие n — по одному набору.
	for _, c := range []struct {
		n, g int
		sets []int
	}{{9, 0, []int{0, 1, 2, 3}}, {40, 1, []int{0, 1, 2, 3}}, {137, 2, []int{2}}, {180, 0, []int{0}}, {120, 3, []int{3}}} {
		p := planGeoms[c.g]
		for _, si := range c.sets {
			asp := planSets(uint64(c.n), c.n)[si]
			ref := refCols(asp, p, layout.MaxPhotos)
			resetPlans() // окна до maxFullTree — из того же Cached, что у прежнего пути
			for _, m := range []int{64, 9, 5, 2} {
				lo, hi := SpreadRange(c.n, m)
				for _, N := range []int{lo, lo + 1, (lo + hi) / 3, c.n / 4, c.n / 2, hi - 1, hi} {
					N = max(lo, min(N, hi))
					K := planWidth(c.n, N, m)
					want := refPlanDP(ref, c.n, N, K)
					if got := PlanGroups(asp, p, N, m).Groups; !slices.Equal(got, want) {
						t.Fatalf("n=%d geom %d set %d m=%d N=%d: %v, прежний %v", c.n, c.g, si, m, N, got, want)
					}
				}
			}
		}
	}
}

// TestPlanDPBound — DP на случайных матрицах (с равенствами: оценки по сетке): со всеми
// столбцами — как прежний; без столбцов меньше lo (граница ub) — либо тот же план, либо
// отказ (false). Нужны оба исхода, иначе граница не проверена.
func TestPlanDPBound(t *testing.T) {
	rng := splitmix64(27)
	const ub = 1.2
	same, refused := 0, 0
	iters := 3000
	if testkit.Race {
		iters = 600
	}
	for it := range iters {
		n := 5 + int(rng.next()%150)
		lo0, hi0 := SpreadRange(n, 64)
		N := lo0 + int(rng.next()%uint64(hi0-lo0+1))
		K := planWidth(n, N, 64)
		grid := it%2 == 0
		cols := make([][]float64, K+1)
		for k := 1; k <= K; k++ {
			cols[k] = make([]float64, max(0, n-k+1))
			for i := range cols[k] {
				s := -1 + (ub+1)*rng.unit()
				if grid {
					s = float64(rng.next()%5) * ub / 4
				}
				cols[k][i] = float64(k) * s
			}
		}
		want := refPlanDP(cols, n, N, K)
		if got, ok := planDP(cols, n, N, K, ub); !ok || !slices.Equal(got, want) {
			t.Fatalf("n=%d N=%d K=%d: %v (ok %t), прежний %v", n, N, K, got, ok, want)
		}
		lo := 2 + int(rng.next()%uint64(max(1, K/3)))
		pruned := slices.Clone(cols)
		for k := 1; k < min(lo, K+1); k++ {
			pruned[k] = nil
		}
		got, ok := planDP(pruned, n, N, K, ub)
		if !ok {
			refused++
			continue
		}
		same++
		if !slices.Equal(got, want) {
			t.Fatalf("n=%d N=%d K=%d lo=%d: с границей %v, прежний %v", n, N, K, lo, got, want)
		}
	}
	if same == 0 || refused == 0 {
		t.Errorf("с границей: совпало %d, отказов %d — нужны оба исхода", same, refused)
	}
	t.Logf("с границей: совпало %d, отказов %d", same, refused)
}

// Паника воркера planEach не роняет процесс: остальные доходят до Wait, паника повторяется
// в вызывающей горутине (её ловят try и guard), значение — от первой.
func TestPlanEachPanic(t *testing.T) {
	var calls sync.Map
	defer func() {
		r := recover()
		if s, ok := r.(string); !ok || !strings.Contains(s, "сбой 37") {
			t.Fatalf("паника воркера не дошла до вызывающего: %v", r)
		}
		if _, ok := calls.Load(37); !ok {
			t.Fatal("f(37) не звали")
		}
	}()
	planEach(500, func(_ *planWork, i int) {
		calls.Store(i, true)
		if i == 37 {
			panic(fmt.Sprintf("сбой %d", i))
		}
	})
	t.Fatal("planEach вернулся без паники")
}
