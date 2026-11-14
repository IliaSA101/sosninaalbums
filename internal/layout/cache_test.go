package layout

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// withCacheGen ставит малый размер поколения и чистый кэш на время теста.
func withCacheGen(t *testing.T, n int) {
	t.Helper()
	old := cacheGen
	cacheGen = n
	resetCache()
	t.Cleanup(func() { cacheGen = old; resetCache() })
}

func inCache(key string) bool {
	cache.Lock()
	defer cache.Unlock()
	_, ok := cache.cur[key]
	if !ok {
		_, ok = cache.old[key]
	}
	return ok
}

func put(keys ...string) {
	for _, k := range keys {
		cached(k, func() int { return 1 })
	}
}

// Запись переживает одну смену поколения и не переживает вторую.
func TestCacheGenerations(t *testing.T) {
	withCacheGen(t, 3)
	put("a", "b", "c")
	put("d") // cur переполнен: a, b, c ушли в old
	if !inCache("a") {
		t.Fatal("запись пропала после первой смены поколения")
	}
	put("e", "f", "g") // d, e, f заполнили cur — вторая смена
	if inCache("a") || inCache("b") || inCache("c") {
		t.Fatal("запись пережила вторую смену поколения")
	}
	if !inCache("g") || !inCache("d") {
		t.Fatal("свежие записи потеряны")
	}
}

// Обращение к записи из old переносит её в cur, и она переживает ещё одну смену.
func TestCacheKeepsRecent(t *testing.T) {
	withCacheGen(t, 3)
	put("a", "b", "c", "d")
	calls := 0
	cached("a", func() int { calls++; return 1 }) // из old → cur
	put("e", "f", "g")                            // вторая смена: b, c ушли, a — нет
	if calls != 0 {
		t.Fatal("запись из old посчитана заново")
	}
	if !inCache("a") || inCache("b") {
		t.Fatalf("a=%v b=%v: перенос из old не сработал", inCache("a"), inCache("b"))
	}
}

// Переполнение не сбрасывает кэш целиком: недавняя запись остаётся.
func TestCacheNoFullReset(t *testing.T) {
	withCacheGen(t, 100)
	put("recent")
	for i := range 150 {
		put(string(rune('A'+i%26)) + string(rune('a'+i/26)))
	}
	if !inCache("recent") {
		t.Fatal("после переполнения кэш сброшен целиком")
	}
}

// Дать остальным горутинам дойти до ожидания. Счёт один и без этого, но тест
// тогда проверял бы слабее.
func settle(started *atomic.Int32, n int32) {
	for started.Load() < n {
		runtime.Gosched()
	}
	for range 100 {
		runtime.Gosched()
	}
}

// N горутин на один ключ дают ровно один счёт.
func TestCachedSingleFlight(t *testing.T) {
	withCacheGen(t, 100)
	const n = 8
	var calls, started atomic.Int32
	release := make(chan struct{})
	res := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			started.Add(1)
			res[i] = cached("k", func() int {
				calls.Add(1)
				<-release // держим счёт, пока остальные не встанут в ожидание
				return 42
			})
		})
	}
	settle(&started, n)
	close(release)
	wg.Wait()
	if c := calls.Load(); c != 1 {
		t.Fatalf("счётов %d, нужен 1", c)
	}
	for _, r := range res {
		if r != 42 {
			t.Fatalf("результат %d", r)
		}
	}
}

// Паника в счёте: ждущий не виснет, мусора в кэше нет, ждущий считает сам.
func TestCachedPanic(t *testing.T) {
	withCacheGen(t, 100)
	entered, release := make(chan struct{}), make(chan struct{})
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		cached("k", func() int {
			close(entered)
			<-release
			panic("сбой счёта")
		})
	}()
	<-entered
	var started atomic.Int32
	got := make(chan int, 1)
	go func() {
		started.Add(1)
		got <- cached("k", func() int { return 7 })
	}()
	settle(&started, 1)
	close(release)
	if p := <-panicked; p != "сбой счёта" {
		t.Fatalf("паника ведущего: %v", p)
	}
	if v := <-got; v != 7 {
		t.Fatalf("ждущий получил %d, нужно 7 (посчитал сам)", v)
	}
	cache.Lock()
	n := len(cache.inflight)
	cache.Unlock()
	if n != 0 {
		t.Fatalf("в inflight осталось %d записей", n)
	}
	if v := cached("k", func() int { return -1 }); v != 7 {
		t.Fatalf("в кэше %d, нужно 7", v)
	}
}

// Паника без ждущих: записи нет, inflight чист, следующий вызов считает заново.
func TestCachedPanicNoEntry(t *testing.T) {
	withCacheGen(t, 100)
	func() {
		defer func() { _ = recover() }()
		cached("k", func() int { panic("x") })
	}()
	if inCache("k") {
		t.Fatal("после паники в кэше запись")
	}
	if v := cached("k", func() int { return 5 }); v != 5 {
		t.Fatalf("получили %d", v)
	}
}

// nearSets — наборы, которые ключ с точностью 0.001 путал: 1334/2000 и 1335/2000 давали
// один ключ «0.667», а в кэше оставалось то, что посчитано первым. Пары — один набор
// с той и с другой пропорцией; 1, 3, 5 фото (перебор, перестановки, страницы) и 18 (many).
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

// cachedSnap — всё, что пакет отдаёт из кэша по набору: ряды разворота и перестановок,
// ряды страниц (первый постраничный ключ ряда, обе стороны). Матрица группировки — в layout/plan.
func cachedSnap(asp []float64) string {
	var b strings.Builder
	row := Cached(asp, album30, 24)
	rr, blk := CachedRearranged(asp, album30, 24)
	fmt.Fprintf(&b, "L%v\nR%v %v\n", row, rr, blk)
	for _, r := range row {
		if !IsPaged(r.Key) {
			continue
		}
		for side := range 2 {
			in, ok := CachedPagesInOrder(r.Key, asp, album30, side, 24)
			pin, prr, pb, pok := CachedPages(r.Key, asp, album30, side, 24)
			fmt.Fprintf(&b, "P%d %v %v|%v %v %v %v\n", side, in, ok, pin, prr, pb, pok)
		}
		break
	}
	return b.String()
}

// TestCacheOrderFree — выдача кэша зависит только от ключа: близкие пропорции, посчитанные
// в любом порядке, дают то же, что каждая на холодном кэше (иначе подсказка AutoPlan
// и сборка AutoBuild могли разойтись, а эталоны — зависеть от порядка тестов).
func TestCacheOrderFree(t *testing.T) {
	t.Cleanup(resetCache)
	for _, s := range nearSets() {
		var cold [2]string
		for i := range 2 {
			resetCache()
			cold[i] = cachedSnap(s[i])
		}
		if cold[0] == cold[1] {
			t.Fatalf("%d фото: наборы неразличимы, тест ничего не проверяет", len(s[0]))
		}
		for first := range 2 {
			resetCache()
			var got [2]string
			got[first] = cachedSnap(s[first])
			got[1-first] = cachedSnap(s[1-first])
			for i := range 2 {
				if got[i] != cold[i] {
					t.Errorf("%d фото, первым %d: набор %d разошёлся с холодным счётом", len(s[0]), first, i)
				}
			}
		}
	}
}

// TestCacheOrderFreeParallel — то же при параллельном счёте (как окна layout/plan): гонять с -race.
func TestCacheOrderFreeParallel(t *testing.T) {
	t.Cleanup(resetCache)
	sets := nearSets()[:3] // 18 фото — в TestCacheOrderFree: под -race он долгий
	cold := make([][2]string, len(sets))
	for k, s := range sets {
		for i := range 2 {
			resetCache()
			cold[k][i] = cachedSnap(s[i])
		}
	}
	resetCache()
	var wg sync.WaitGroup
	for g := range 12 {
		wg.Go(func() {
			k, i := g%len(sets), g/len(sets)%2
			if got := cachedSnap(sets[k][i]); got != cold[k][i] {
				t.Errorf("горутина %d: набор %d/%d разошёлся с холодным счётом", g, k, i)
			}
		})
	}
	wg.Wait()
}
