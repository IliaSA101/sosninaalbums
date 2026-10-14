package variety

import (
	"regexp"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Level — уровень разнообразия (зеркало model.Variety: пакет не знает model).
type Level int

// Уровни. Вне 0…2 — зажим (clamp).
const (
	Less   Level = 0 // как 0.10: на каждом развороте лучшая раскладка
	Normal Level = 1
	More   Level = 2
)

func (l Level) clamp() Level { return max(Less, min(l, More)) }

const (
	// extraPaged — у скольких первых постраничных ключей ряда ищем воздушные варианты.
	extraPaged = 3
	// pageRow — длина ряда страницы, из которого берутся варианты с модификатором: как
	// подборка страницы в программе.
	pageRow = 24
)

// pageMod — вариант страницы «поменьше» (S) или прижатый к краю/корешку (O, I).
var pageMod = regexp.MustCompile(`[SOI]\(`)

// Cand — кандидат разворота: раскладка и её измеренные свойства.
type Cand struct {
	layout.Result
	Info
	InRow bool // из ряда «Авто» (Cached)
	Kind  Kind // семейство генератора featured; NoKind — ряд «Авто» или воздух
}

// Pool — пул кандидатов разворота: сначала ряд Cached(aspects, p, rowLen) как есть (префикс
// = ряд, порядок тот же), затем (lv ≥ Normal) «Другие варианты» (Extra) в их порядке: воздух и
// раскладки с главным фото. Выбор сборки ставит только видимое в подборке и счётчике (решение
// 0.11 №38). Все ключи строит layout, так что Build их разбирает. Только «в порядке ленты».
//
// Воздух — это варианты страниц из подборки страницы: у первых трёх постраничных ключей ряда
// страницы с модификатором S/O/I, а у P[S(…)|…] ещё и правая страница поменьше (обе поменьше).
// Раскладки с главным фото — генератор featured (8–16 фото).
func Pool(aspects []float64, p layout.Params, rowLen int, lv Level) []Cand {
	row := layout.Cached(aspects, p, rowLen)
	if lv.clamp() < Normal {
		out := make([]Cand, len(row))
		for i, r := range row {
			out[i] = Cand{Result: r, Info: Describe(r, p), InRow: true}
		}
		return out
	}
	e := extraOf(aspects, p, rowLen)
	out := make([]Cand, 0, len(row)+len(e.row))
	for _, r := range row {
		out = append(out, Cand{Result: r, Info: Describe(r, p), InRow: true})
	}
	for _, r := range e.row {
		out = append(out, Cand{Result: r, Info: Describe(r, p), Kind: e.kind[r.Key]})
	}
	return out
}

// extras — воздушные варианты вне ряда row без повторов ключей (seen — уже взятые; пополняется):
// у первых трёх постраничных ключей ряда страницы с модификатором S/O/I, у P[S(…)|…] ещё и
// правая страница поменьше. Без Describe: подборке и счётчику нужны только ключи.
func extras(row []layout.Result, aspects []float64, p layout.Params, seen map[string]bool) []layout.Result {
	var out []layout.Result
	add := func(r layout.Result) {
		if !seen[r.Key] {
			seen[r.Key] = true
			out = append(out, r)
		}
	}
	paged := 0
	for _, r := range row {
		if !layout.IsPaged(r.Key) {
			continue
		}
		if paged++; paged > extraPaged {
			break
		}
		for side := range 2 {
			rs, ok := layout.CachedPagesInOrder(r.Key, aspects, p, side, pageRow)
			if !ok {
				continue
			}
			for _, x := range rs {
				if !pageMod.MatchString(x.Key) {
					continue
				}
				add(x)
				if side == 0 && strings.HasPrefix(x.Key, "P[S(") {
					addSmallRight(x.Key, aspects, p, add)
				}
			}
		}
	}
	return out
}

// addSmallRight — к ключу с левой страницей поменьше добавляет варианты, где и правая поменьше.
func addSmallRight(key string, aspects []float64, p layout.Params, add func(layout.Result)) {
	rs, ok := layout.CachedPagesInOrder(key, aspects, p, 1, pageRow)
	if !ok {
		return
	}
	for _, y := range rs {
		if strings.Contains(y.Key, "|S(") {
			add(y)
		}
	}
}
