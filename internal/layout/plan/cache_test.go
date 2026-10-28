package plan

import (
	"fmt"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Проверки, которые были в layout (cache_test, layout_test) и опираются на группировку.

// nearSets — наборы, которые ключ с точностью 0.001 путал: 1334/2000 и 1335/2000 давали
// один ключ «0.667». Копия из layout; 1, 3, 5 фото (перебор, перестановки, страницы) и 18 (many).
func nearSets() [][2][]float64 {
	a, b := 1334.0/2000, 1335.0/2000
	with := func(x float64, n int) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = [...]float64{x, 1.5, x, 0.75, 1.5, x}[i%6]
		}
		return out
	}
	var out [][2][]float64
	for _, n := range []int{1, 3, 5, 18} {
		out = append(out, [2][]float64{with(a, n), with(b, n)})
	}
	return out
}

// matrixSnap — столбцы матрицы группировки набора (то, что раньше входило в cachedSnap layout).
func matrixSnap(asp []float64) string {
	return fmt.Sprintf("G%v", planMatrix(asp, album30, 1, min(len(asp), layout.MaxFullTree)))
}

// TestMatrixOrderFree — матрица зависит только от набора: близкие пропорции, посчитанные
// в любом порядке, дают то же, что каждая на холодном кэше.
func TestMatrixOrderFree(t *testing.T) {
	t.Cleanup(layout.ResetCache)
	for _, s := range nearSets() {
		var cold [2]string
		for i := range 2 {
			layout.ResetCache()
			cold[i] = matrixSnap(s[i])
		}
		if cold[0] == cold[1] {
			t.Fatalf("%d фото: наборы неразличимы, тест ничего не проверяет", len(s[0]))
		}
		for first := range 2 {
			layout.ResetCache()
			var got [2]string
			got[first] = matrixSnap(s[first])
			got[1-first] = matrixSnap(s[1-first])
			for i := range 2 {
				if got[i] != cold[i] {
					t.Errorf("%d фото, первым %d: набор %d разошёлся с холодным счётом", len(s[0]), first, i)
				}
			}
		}
	}
}

// TestMatrixOrderFreeParallel — то же при параллельном счёте: гонять с -race.
func TestMatrixOrderFreeParallel(t *testing.T) {
	t.Cleanup(layout.ResetCache)
	sets := nearSets()[:3]
	cold := make([][2]string, len(sets))
	for k, s := range sets {
		for i := range 2 {
			layout.ResetCache()
			cold[k][i] = matrixSnap(s[i])
		}
	}
	layout.ResetCache()
	var wg sync.WaitGroup
	for g := range 12 {
		wg.Go(func() {
			k, i := g%len(sets), g/len(sets)%2
			if got := matrixSnap(sets[k][i]); got != cold[k][i] {
				t.Errorf("горутина %d: набор %d/%d разошёлся с холодным счётом", g, k, i)
			}
		})
	}
	wg.Wait()
}

// TestPlanSheet — автосборка листа (031, Sheet: корешка нет, постраничных окон нет):
// окна 17+ считаются и покрывают все фото.
func TestPlanSheet(t *testing.T) {
	sheet := layout.Params{PageW: 102, PageH: 304, Margin: 8, Gap: 3, Layflat: true, Sheet: true}
	asp := make([]float64, 40)
	for i := range asp {
		asp[i] = []float64{P, L, S}[i%3]
	}
	if pl := PlanGroups(asp, sheet, 2, 32); len(pl.Groups) != 2 || pl.Groups[1][1] != 40 {
		t.Errorf("автосборка листа: %+v", pl)
	}
}

// TestResetCacheClearsMatrices — layout.ResetCache сбрасывает и матрицы группировки
// (подписка OnReset): холодные бенчмарки и тесты других пакетов на это опираются.
func TestResetCacheClearsMatrices(t *testing.T) {
	asp := weddingAspects(4, 30)
	PlanGroups(asp, album30, 5, 64)
	plans.Lock()
	warm := len(plans.m)
	plans.Unlock()
	if warm == 0 {
		t.Fatal("матрица не легла в кэш — тест ничего не проверяет")
	}
	layout.ResetCache()
	plans.Lock()
	cold := len(plans.m)
	plans.Unlock()
	if cold != 0 {
		t.Errorf("после layout.ResetCache матриц %d, ждали 0", cold)
	}
}
