package layout

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
)

// Раскладки зависят только от пропорций и геометрии, а на свадьбе почти
// все кадры 3:2 или 2:3 — одни и те же наборы повторяются постоянно.
// Записи разных видов (порядок ленты, перестановки, страницы) различает
// префикс ключа.
//
// Два поколения: переполнилось cur — оно становится old, cur пустеет. Запись из old
// при обращении переезжает в cur, так что нужное переживает смену, а полного сброса
// (после которого весь View считается заново) нет. Худший объём — 2·cacheGen записей.
// inflight — одиночный счёт: параллельные вызовы с тем же ключом ждут первый.
var cache = struct {
	sync.Mutex
	cur, old map[string]any
	inflight map[string]chan struct{}
}{cur: map[string]any{}, inflight: map[string]chan struct{}{}}

// cacheGen — записей в поколении (переменная — чтобы тест не наполнял кэш тысячами).
var cacheGen = 10000

// CacheKey — ключ кэша по геометрии, n и пропорциям. Пропорции — точно (кратчайшая запись
// float64), а не с точностью 0.001: иначе 1334/2000 и 1335/2000 делят запись, и выдача
// зависит от того, кто посчитал первым (окна plan считаются параллельно — подсказка
// и сборка расходились). Кадры одной камеры дают одинаковые float — попадания те же.
// FoldAny — только там, где действует (layflat-разворот): иначе одна выдача в двух записях.
func CacheKey(aspects []float64, p Params, n int) string {
	fold := p.FoldAny && p.Layflat && !p.Sheet
	b := fmt.Appendf(make([]byte, 0, 48+20*len(aspects)), "%g/%g/%g/%g/%t/%t/%t/%d:",
		p.PageW, p.PageH, p.Margin, p.Gap, p.Layflat, fold, p.Sheet, n)
	for _, a := range aspects {
		b = strconv.AppendFloat(b, a, 'g', -1, 64)
		b = append(b, ',')
	}
	return string(b)
}

// cacheGetLocked ищет запись в обоих поколениях; найденную в old переносит в cur. Под cache.Lock.
func cacheGetLocked(key string) (any, bool) {
	if r, ok := cache.cur[key]; ok {
		return r, true
	}
	r, ok := cache.old[key]
	if ok {
		delete(cache.old, key)
		cachePutLocked(key, r)
	}
	return r, ok
}

// cachePutLocked кладёт запись в cur, при переполнении сдвигая поколения. Под cache.Lock.
func cachePutLocked(key string, v any) {
	if len(cache.cur) >= cacheGen {
		cache.old, cache.cur = cache.cur, map[string]any{}
	}
	cache.cur[key] = v
}

func cached[T any](key string, f func() T) T {
	for {
		cache.Lock()
		if r, ok := cacheGetLocked(key); ok {
			cache.Unlock()
			return r.(T)
		}
		ch, busy := cache.inflight[key]
		if !busy {
			ch = make(chan struct{})
			cache.inflight[key] = ch
		}
		cache.Unlock()
		if !busy {
			return compute(key, ch, f)
		}
		// Ждём ведущего и смотрим кэш заново. Если он упал с паникой, записи нет —
		// тогда ведущим становится кто-то из ждущих (паника уйдёт к нему же).
		<-ch
	}
}

// compute считает f и кладёт результат в кэш. Канал закрывается и при панике, чтобы
// ждущие не повисли; запись при этом не создаётся. Закрытие — после записи в кэш.
func compute[T any](key string, ch chan struct{}, f func() T) T {
	defer func() {
		cache.Lock()
		delete(cache.inflight, key)
		cache.Unlock()
		close(ch)
	}()
	v := f()
	cache.Lock()
	cachePutLocked(key, v)
	cache.Unlock()
	return v
}

// resetCache — для бенчмарков на холодном кэше (и матрицы группировки).
func resetCache() {
	cache.Lock()
	cache.cur, cache.old = map[string]any{}, nil
	cache.Unlock()
	runResetHooks()
}

// ResetCache — то же для тестов и бенчмарков других пакетов (album/view: View на холодном кэше).
func ResetCache() { resetCache() }

// Cached — то же, что Layouts, но с запоминанием результата.
func Cached(aspects []float64, p Params, limit int) []Result {
	return cached("L"+CacheKey(aspects, p, limit), func() []Result { return Layouts(aspects, p, limit) })
}

type rearrangedEntry struct {
	res   []Result
	block Block
}

// CachedRearranged — то же, что Rearranged, но с запоминанием результата.
func CachedRearranged(aspects []float64, p Params, limit int) ([]Result, Block) {
	e := cached("R"+CacheKey(aspects, p, limit), func() rearrangedEntry {
		r, b := Rearranged(aspects, p, limit)
		return rearrangedEntry{r, b}
	})
	return e.res, e.block
}

// pagesKey: варианты страницы зависят от её фото и от второй страницы, но не от
// того, какой блок стоит на ней сейчас — листание страницы попадает в кэш.
func pagesKey(kind, key string, aspects []float64, p Params, side, limit int) (string, bool) {
	k, ok := pageForm(key)
	if !ok || side != 0 && side != 1 {
		return "", false
	}
	own := k.pages[side].leaves(nil)
	slices.Sort(own)
	return fmt.Sprintf("%s%d:%s:%v:", kind, side, pageString(k.mods[1-side], k.pages[1-side]), own) + CacheKey(aspects, p, limit), true
}

type pagesEntry struct {
	inOrder, rearranged []Result
	block               Block
	ok                  bool
}

// CachedPages — то же, что PageChoices, но с запоминанием результата.
func CachedPages(key string, aspects []float64, p Params, side, limit int) (inOrder, rearranged []Result, block Block, ok bool) {
	ck, ok := pagesKey("Pk", key, aspects, p, side, limit)
	if !ok {
		return nil, nil, BlockNone, false
	}
	e := cached(ck, func() pagesEntry {
		i, r, b, ok := PageChoices(key, aspects, p, side, limit)
		return pagesEntry{i, r, b, ok}
	})
	return e.inOrder, e.rearranged, e.block, e.ok
}

// CachedPagesInOrder — только ряд страницы «в порядке ленты»: он дешёвый,
// поэтому его можно считать на каждый View.
func CachedPagesInOrder(key string, aspects []float64, p Params, side, limit int) ([]Result, bool) {
	ck, ok := pagesKey("Po", key, aspects, p, side, limit)
	if !ok {
		return nil, false
	}
	e := cached(ck, func() pagesEntry {
		pc, ok := newPageCtx(key, aspects, p, side)
		if !ok {
			return pagesEntry{}
		}
		return pagesEntry{inOrder: results(pc.inOrder(aspects, limit)), ok: true}
	})
	return e.inOrder, e.ok
}
