package variety

import (
	"cmp"
	"maps"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Пределы «Других вариантов»: воздуха — не длиннее порции ряда «Авто», раскладок с главным фото
// (featured) — столько же: по одной на позицию главного, чтобы звезда в любом месте ленты могла
// стать главной (решения 0.11 №33, №57). Вместе — не длиннее ряда «Авто» (24).
const (
	airLimit  = 12
	featLimit = 12
	midLimit  = 1 // «центр крупнее» (forms.CenterBig): одна на разворот, 0.23-q6
	// scatterLimit — плитки «Разброса» (forms.Free): по одной на профиль, 105.
	scatterLimit = 7
	ExtraLimit   = airLimit + featLimit + midLimit + scatterLimit
)

// featSlack — featured хуже потолка лучшей (min(best, heroCap)) по Fair больше этого не выберет
// ни один уровень (= deltaStar «Больше»): в подборку их не берём.
const featSlack = 0.15

// extraGen — записей в поколении кэшей ряда и «Других вариантов» (varietyGen, gencache.go): запись —
// до 24 раскладок (≈ 14 КБ на 16 фото), 5000 — ≈ 70 МБ на поколение; память не растёт без предела.
const extraGen = 5000

// extraEntry — «Другие варианты» разворота и семейства featured в них (Pool берёт их оттуда).
type extraEntry struct {
	row    []layout.Result
	kind   map[string]Kind
	manual map[string]bool // центр крупнее и «Разброс» в row — только ручной выбор (Cand.Manual)
	hidden int             // раскладок «Разброса» на разворот скрыто сгибом при «Только широкие» (105 В1)
}

// extraCache — ряды «Других вариантов» по пропорциям и геометрии: View считает счётчик
// «i / N» по ряду «Авто» + «Другие варианты» (решение 0.11 №36) у каждого разворота на каждый
// запрос, а без кэша это десяток поисков в кэше layout, генератор featured и сортировка.
var extraCache genCache[extraEntry]

// Extra — «Другие варианты» разворота вне ряда «Авто» Row(aspects, p, rowLen): сначала свободные
// композиции (105) — центр крупнее и плитки «Разброса» (forms.Free) в порядке профилей, затем воздух
// (≤ airLimit лучших по оценке) и раскладки с главным фото (≤ featLimit, bestFeatured; с 051 — от
// featLo фото, а не 8), вместе по убыванию оценки (ничья — по ключу). Порядок — и подборки, и листания (Пробел и
// ‹ › после ряда «Авто»), и счётчика. Результат общий: не менять.
func Extra(aspects []float64, p layout.Params, rowLen int) []layout.Result {
	return extraOf(aspects, p, rowLen).row
}

func extraOf(aspects []float64, p layout.Params, rowLen int) extraEntry {
	key := extraKey(aspects, p, rowLen)
	e, ok := extraCache.get(key)
	if ok {
		return e
	}
	re := rowOf(aspects, p, rowLen)
	seen := make(map[string]bool, len(re.row))
	for _, x := range re.row {
		seen[x.Key] = true
	}
	// Воздух — от страниц ряда layout (страницы с модификатором), featured — посчитаны рядом.
	air := extras(layout.Cached(aspects, p, rowLen), aspects, p, seen)
	byScore(air)
	e = extraEntry{row: air[:min(len(air), airLimit)], kind: make(map[string]Kind, len(re.feats))}
	for _, f := range re.feats {
		if seen[f.Key] {
			continue // ключ воздуха: featured его не строит, но ряд без повторов — обязательство
		}
		e.row = append(e.row, f.Result)
		e.kind[f.Key] = f.Kind
	}
	byScore(e.row)
	head := freeHead(aspects, p, re, seen, &e)
	e.row = slices.Clip(slices.Concat(head, e.row))
	extraCache.put(key, e)
	return e
}

// freeHead — начало «Других вариантов» (105): центр крупнее (n из forms.Free, иначе m из 0.23) и
// плитки «Разброса». Без ключа и вида (Look) ряда «Авто» и друг друга. Заполняет e.manual и
// e.hidden. Сборка этих раскладок не берёт (Cand.Manual), порядок задаёт таблица профилей, а не Q.
func freeHead(aspects []float64, p layout.Params, re rowEntry, seen map[string]bool, e *extraEntry) []layout.Result {
	fr, hidden := forms.FreeFold(aspects, p)
	e.hidden = hidden
	looks := maps.Clone(re.looks)
	if looks == nil {
		looks = map[string]bool{}
	}
	var head []layout.Result
	take := func(r layout.Result) bool {
		l := Look(r.Rects, p)
		if seen[r.Key] || looks[l] {
			return false
		}
		looks[l] = true
		head = append(head, r)
		return true
	}
	centered := false
	for _, c := range fr {
		if c.Form == forms.Contrast && !centered {
			centered = take(c.Result)
		}
	}
	// Центр из 0.23 (m): прежнее условие — только ключ, вид ряда не проверяется.
	if m := re.mid; !centered && m != nil && !seen[m.Key] && !hasKey(e.row, m.Key) {
		looks[Look(m.Rects, p)] = true
		head = append(head, m.Result)
	}
	for _, c := range fr {
		if c.Form == forms.Scatter {
			take(c.Result)
		}
	}
	if len(head) > 0 {
		e.manual = make(map[string]bool, len(head))
		for _, r := range head {
			e.manual[r.Key] = true
		}
	}
	return head
}

// ExtraFoldHidden — сколько раскладок «Разброса» (центр и Z на разворот) разворота скрыто только
// правилом сгиба при «Только широкие» (105 В1): с «Любые фото» они встали бы в «Другие варианты».
func ExtraFoldHidden(aspects []float64, p layout.Params, rowLen int) int {
	return extraOf(aspects, p, rowLen).hidden
}

// hasKey — ключ уже в ряду.
func hasKey(rs []layout.Result, key string) bool {
	return slices.ContainsFunc(rs, func(r layout.Result) bool { return r.Key == key })
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
	return extraCache.has(extraKey(aspects, p, rowLen))
}

// ResetCache — холодный кэш ряда «Авто» и «Других вариантов» для бенчмарков. Зовётся и из
// layout.ResetCache (подписка OnReset, образец — plan): иначе холодный замер оставлял variety тёплым.
func ResetCache() {
	extraCache.reset()
	rowCache.reset()
}

func init() { layout.OnReset(ResetCache) }

// extraKey — ключ кэша layout: пропорции точно, иначе ряд зависел бы от того, какой из близких
// наборов посчитан первым (extras и featured считаются по точным пропорциям).
func extraKey(aspects []float64, p layout.Params, rowLen int) string {
	return layout.CacheKey(aspects, p, rowLen)
}
