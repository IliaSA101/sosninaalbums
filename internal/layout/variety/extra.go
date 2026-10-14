package variety

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Пределы «Других вариантов»: воздуха — не длиннее порции ряда «Авто», раскладок с главным фото
// (featured) — столько же: по одной на позицию главного, чтобы звезда в любом месте ленты могла
// стать главной (решения 0.11 №33, №57). Вместе — не длиннее ряда «Авто» (24).
const (
	airLimit   = 12
	featLimit  = 12
	ExtraLimit = airLimit + featLimit
)

// featSlack — featured хуже потолка лучшей (min(best, heroCap)) по Fair больше этого не выберет
// ни один уровень (= deltaStar «Больше»): в подборку их не берём.
const featSlack = 0.15

// extraGen — записей кэша, после которых он начинается заново: запись — до 24 раскладок (≈ 14 КБ
// на 16 фото), 5000 — ≈ 70 МБ, как было при 12 раскладках и 10 000; это впятеро больше предела
// разворотов альбома (model.MaxSpreads), а память не растёт без предела.
const extraGen = 5000

// extraEntry — «Другие варианты» разворота и семейства featured в них (Pool берёт их оттуда).
type extraEntry struct {
	row  []layout.Result
	kind map[string]Kind
}

// extraCache — ряды «Других вариантов» по пропорциям и геометрии: View считает счётчик
// «i / N» по ряду «Авто» + «Другие варианты» (решение 0.11 №36) у каждого разворота на каждый
// запрос, а без кэша это десяток поисков в кэше layout, генератор featured и сортировка.
var extraCache = struct {
	sync.Mutex
	m map[string]extraEntry
}{m: map[string]extraEntry{}}

// Extra — «Другие варианты» разворота вне ряда Cached(aspects, p, rowLen): воздух (≤ airLimit
// лучших по оценке) и раскладки с главным фото (≤ featLimit, bestFeatured), вместе по убыванию
// оценки (ничья — по ключу). Порядок — и подборки, и листания (Пробел и ‹ › после ряда «Авто»),
// и счётчика. Результат общий: не менять.
func Extra(aspects []float64, p layout.Params, rowLen int) []layout.Result {
	return extraOf(aspects, p, rowLen).row
}

func extraOf(aspects []float64, p layout.Params, rowLen int) extraEntry {
	key := extraKey(aspects, p, rowLen)
	extraCache.Lock()
	e, ok := extraCache.m[key]
	extraCache.Unlock()
	if ok {
		return e
	}
	row := layout.Cached(aspects, p, rowLen)
	seen := make(map[string]bool, len(row))
	for _, x := range row {
		seen[x.Key] = true
	}
	air := extras(row, aspects, p, seen)
	byScore(air)
	feats := bestFeatured(featured(aspects, p), row, seen)
	e = extraEntry{row: air[:min(len(air), airLimit)], kind: make(map[string]Kind, len(feats))}
	for _, f := range feats {
		e.row = append(e.row, f.Result)
		e.kind[f.Key] = f.Kind
	}
	byScore(e.row)
	e.row = slices.Clip(e.row)
	extraCache.Lock()
	if len(extraCache.m) >= extraGen {
		extraCache.m = map[string]extraEntry{}
	}
	extraCache.m[key] = e
	extraCache.Unlock()
	return e
}

// byScore — по убыванию оценки в квантах, ничья — по ключу.
func byScore(rs []layout.Result) {
	slices.SortStableFunc(rs, func(a, b layout.Result) int {
		return cmp.Or(cmp.Compare(quanta(b.Score), quanta(a.Score)), cmp.Compare(a.Key, b.Key))
	})
}

// bestFeatured — раскладки генератора для подборки: вне ряда «Авто» и уже взятых (seen;
// пополняется), не хуже потолка лучшей на featSlack по Fair; сначала лучшая на каждое главное
// фото (пара — на семейство), затем остальные по Fair, всего ≤ featLimit.
func bestFeatured(fs []Feat, row []layout.Result, seen map[string]bool) []Feat {
	if len(fs) == 0 || len(row) == 0 {
		return nil
	}
	floor := quanta(min(row[0].Score, heroCap) - featSlack)
	fs = slices.DeleteFunc(fs, func(f Feat) bool { return seen[f.Key] || quanta(f.Fair) < floor })
	slices.SortStableFunc(fs, func(a, b Feat) int {
		return cmp.Or(cmp.Compare(quanta(b.Fair), quanta(a.Fair)), cmp.Compare(a.Key, b.Key))
	})
	var out []Feat
	taken := make([]bool, len(fs))
	group := map[int]bool{}
	for i, f := range fs {
		g := f.Big
		if g < 0 {
			g = -1 - int(f.Kind)
		}
		if len(out) < featLimit && !group[g] {
			group[g], taken[i] = true, true
			out = append(out, f)
		}
	}
	for i, f := range fs {
		if len(out) < featLimit && !taken[i] {
			out = append(out, f)
		}
	}
	for _, f := range out {
		seen[f.Key] = true
	}
	return out
}

// Cached — ряд «Других вариантов» уже посчитан (проверка прогрева: Extra на холодном кэше — десятки мс).
func Cached(aspects []float64, p layout.Params, rowLen int) bool {
	extraCache.Lock()
	defer extraCache.Unlock()
	_, ok := extraCache.m[extraKey(aspects, p, rowLen)]
	return ok
}

// ResetCache — холодный кэш «Других вариантов» для бенчмарков (вместе с layout.ResetCache).
func ResetCache() {
	extraCache.Lock()
	extraCache.m = map[string]extraEntry{}
	extraCache.Unlock()
}

// extraKey — как ключ кэша layout: пропорции с точностью 0.001 (он тоже их так различает).
func extraKey(aspects []float64, p layout.Params, rowLen int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%g/%g/%g/%g/%t/%t/%d:", p.PageW, p.PageH, p.Margin, p.Gap, p.Layflat, p.Sheet, rowLen)
	for _, a := range aspects {
		fmt.Fprintf(&b, "%.3f,", a)
	}
	return b.String()
}
