package variety

import (
	"fmt"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// layout.ResetCache сбрасывает и кэши variety (подписка OnReset, 075-15): ряд «Авто» и
// «Другие варианты» — обоих наборов сразу; повторный сброс на пустом — холостой.
func TestResetCacheResetsVariety(t *testing.T) {
	t.Cleanup(ResetCache)
	sets := [][]float64{mixedAspects(9, 3), mixedAspects(7, 5)}
	for _, as := range sets {
		Row(as, geomSq, 24)
		Extra(as, geomSq, 24)
		if !RowCached(as, geomSq, 24) || !Cached(as, geomSq, 24) {
			t.Fatalf("заготовка: ряд %v, другие %v", RowCached(as, geomSq, 24), Cached(as, geomSq, 24))
		}
	}
	layout.ResetCache()
	for i, as := range sets {
		if RowCached(as, geomSq, 24) || Cached(as, geomSq, 24) {
			t.Errorf("набор %d: после layout.ResetCache кэш variety тёплый", i)
		}
	}
	layout.ResetCache() // на пустом — без паники и без следов
	if c, o := rowCache.size(); c != 0 || o != 0 {
		t.Errorf("после повторного сброса %d + %d", c, o)
	}
}

// Края genCache: пустой кэш ничего не отдаёт, nil — тоже значение, смена поколений не теряет
// свежее, reset очищает оба поколения.
func TestGenCacheEdges(t *testing.T) {
	was := varietyGen
	varietyGen = 1
	t.Cleanup(func() { varietyGen = was })
	var c genCache[[]int]
	if v, ok := c.get("x"); ok || v != nil || c.has("x") {
		t.Fatal("пустой кэш что-то отдал")
	}
	if cur, old := c.size(); cur != 0 || old != 0 {
		t.Fatalf("пустой кэш: %d + %d", cur, old)
	}
	c.put("a", nil) // nil как значение: запись есть
	if v, ok := c.get("a"); !ok || v != nil {
		t.Fatalf("nil-значение: %v, %v", v, ok)
	}
	c.put("b", []int{1}) // поколение 1: a уходит в old
	if !c.has("a") || !c.has("b") {
		t.Fatalf("после смены поколения: a=%v b=%v", c.has("a"), c.has("b"))
	}
	if cur, old := c.size(); cur != 1 || old != 1 {
		t.Fatalf("размер %d + %d, ждали 1 + 1", cur, old)
	}
	c.reset()
	if c.has("a") || c.has("b") {
		t.Error("reset оставил записи")
	}
	if cur, old := c.size(); cur != 0 || old != 0 {
		t.Errorf("после reset %d + %d", cur, old)
	}
}

// Параллельные get/put/has при частой смене поколений (под -race): ни гонок, ни выхода за предел.
func TestGenCacheConcurrent(t *testing.T) {
	was := varietyGen
	varietyGen = 4
	t.Cleanup(func() { varietyGen = was })
	var c genCache[int]
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 200 {
				k := fmt.Sprint((g*7 + i) % 13)
				if _, ok := c.get(k); !ok {
					c.put(k, i)
				}
				c.has(k)
			}
		})
	}
	wg.Wait()
	if cur, old := c.size(); cur > varietyGen || cur+old > 2*varietyGen {
		t.Errorf("записей %d + %d при поколении %d", cur, old, varietyGen)
	}
}

// Результат после вытеснения из обоих поколений тот же, что был: пересчёт даёт тот же ряд.
func TestGenerationsKeepResult(t *testing.T) {
	ResetCache()
	was := varietyGen
	varietyGen = 2
	t.Cleanup(func() { varietyGen = was; ResetCache() })
	as := func(i int) []float64 { return sameAspects(i+2, aspL) }
	first := fmt.Sprint(Row(as(0), geomSq, 24), Extra(as(0), geomSq, 24))
	for i := 1; i < 8; i++ { // вытесняем из обоих поколений
		Row(as(i), geomSq, 24)
		Extra(as(i), geomSq, 24)
	}
	if Cached(as(0), geomSq, 24) || RowCached(as(0), geomSq, 24) {
		t.Fatal("заготовка: запись не вытеснена")
	}
	if again := fmt.Sprint(Row(as(0), geomSq, 24), Extra(as(0), geomSq, 24)); again != first {
		t.Error("пересчёт после вытеснения дал другой ряд")
	}
}
