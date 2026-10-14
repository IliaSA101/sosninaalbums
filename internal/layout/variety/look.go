// Package variety — разнообразие автосборки (спека 041): вид и семейство раскладки,
// пул кандидатов разворота и выбор с учётом соседей. Только публичный API internal/layout:
// ключи строит layout (Cached, Build), здесь они лишь выбираются.
package variety

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Family — характер раскладки; подписи для интерфейса считает Go (Choice.family).
type Family string

// Семейства: первое подошедшее по порядку bleed, air, hero, dense.
const (
	Dense Family = "dense" // «Плотно»
	Hero  Family = "hero"  // «С главным фото»
	Air   Family = "air"   // «Воздух»
	Bleed Family = "bleed" // «В край»
)

const (
	// lookQuant — квант вида в долях высоты страницы: меньше глаз не различает.
	lookQuant = 0.03
	// airFill — разворот с заполнением ниже считается воздушным.
	airFill = 0.55
	// heroRatio — во сколько раз фото больше второго по площади, чтобы стать главным.
	heroRatio = 1.6
	// heroMinPhotos — главное фото бывает от трёх фото (у пары «главное» — просто неровная пара).
	heroMinPhotos = 3
	// pairMinPhotos — две крупных (оба ≥ 1.6× третьего) — от четырёх фото: «две крупных + лента».
	pairMinPhotos = 4
	// spreadPenalty — штраф оценки layout за разброс площадей: 0.05·log₂(max/min). Копия (layout
	// не трогаем, решение №22); расхождение с layout ловит TestGridFitIsLayoutScore.
	spreadPenalty = 0.05
)

// Info — измеренные свойства готовой раскладки.
type Info struct {
	Family  Family
	Look    string  // вид: картинка, а не форма ключа
	LookID  uint64  // хэш вида (lookID): Pick помнит виды цепочки в map без хэша длинных строк
	Fill    float64 // сумма площадей фото / площадь разворота
	MinSide float64 // мельчайшая сторона фото, мм
	Big     int     // индекс главного фото; −1 — нет
	Heroes  int     // главных по геометрии: 0, 1 (Big) или 2 (пара крупных, Big = −1)
	// Fair — оценка без штрафа разброса площадей за главные (решение 0.11 №30): разброс считается
	// только у остальных фото. Без главных — Score.
	Fair float64
}

// Look — вид раскладки: для каждого фото (центр, размер) в квантах 3 % высоты страницы,
// строки отсортированы. Одна картинка с точностью до кванта — один вид, какие бы ни были
// деревья ключа и номера фото; зеркало — другой вид (крупное слева ≠ справа).
func Look(rects []layout.Rect, p layout.Params) string {
	q := lookQuant * p.PageH
	if q <= 0 {
		return ""
	}
	type cell [4]int
	cells := make([]cell, len(rects))
	for i, r := range rects {
		cells[i] = cell{
			int(math.Round((r.X + r.W/2) / q)), int(math.Round((r.Y + r.H/2) / q)),
			int(math.Round(r.W / q)), int(math.Round(r.H / q)),
		}
	}
	slices.SortFunc(cells, func(a, b cell) int { return slices.Compare(a[:], b[:]) })
	var sb strings.Builder
	for _, c := range cells {
		for j, v := range c {
			if j > 0 {
				sb.WriteByte(':')
			}
			sb.WriteString(strconv.Itoa(v))
		}
		sb.WriteByte(';')
	}
	return sb.String()
}

// Describe измеряет раскладку: вид, заполнение, мельчайшую сторону, главное фото (одно или
// пару), семейство и оценку без штрафа разброса за главные.
func Describe(r layout.Result, p layout.Params) Info {
	in := Info{Look: Look(r.Rects, p), Big: -1, Fair: r.Score}
	in.LookID = lookID(in.Look)
	var sum float64
	in.MinSide = math.Inf(1)
	for _, x := range r.Rects {
		sum += x.W * x.H
		in.MinSide = math.Min(in.MinSide, math.Min(x.W, x.H))
	}
	if len(r.Rects) == 0 {
		in.MinSide = 0
	}
	h := heroesOf(r.Rects)
	in.Heroes = h.n
	if h.n == 1 {
		in.Big = h.idx[0]
	}
	if h.n > 0 {
		// Штраф layout — 0.05·log₂(max/min) по всем; у Fair максимум — у первого не-главного.
		in.Fair += spreadPenalty * math.Log2(h.top[0]/h.top[h.n])
	}
	if all := 2 * p.PageW * p.PageH; all > 0 {
		in.Fill = sum / all
	}
	switch {
	case strings.Contains(r.Key, "B[") || strings.Contains(r.Key, "E("):
		in.Family = Bleed
	case in.Fill < airFill:
		in.Family = Air
	case in.Heroes > 0:
		in.Family = Hero
	default:
		in.Family = Dense
	}
	return in
}

// heroes — главные фото раскладки по геометрии.
type heroes struct {
	n   int        // 0, 1 или 2 (пара крупных)
	idx [2]int     // индексы двух наибольших по площади (ничья — раньше)
	top [3]float64 // три наибольшие площади по убыванию
}

// heroesOf — главное: от трёх фото, ≥ 1.6× второго; иначе пара — от четырёх фото, обе ≥ 1.6×
// третьего («две крупных + лента»).
func heroesOf(rs []layout.Rect) heroes {
	h := heroes{idx: [2]int{-1, -1}}
	for i, x := range rs {
		switch s := x.W * x.H; {
		case s > h.top[0]:
			h.top, h.idx = [3]float64{s, h.top[0], h.top[1]}, [2]int{i, h.idx[0]}
		case s > h.top[1]:
			h.top[1], h.top[2], h.idx[1] = s, h.top[1], i
		case s > h.top[2]:
			h.top[2] = s
		}
	}
	switch k := len(rs); {
	case k >= heroMinPhotos && h.top[0] >= heroRatio*h.top[1]:
		h.n = 1
	case k >= pairMinPhotos && h.top[1] >= heroRatio*h.top[2]:
		h.n = 2
	}
	return h
}

// lookID — FNV-1a вида: совпадение строк ⇒ совпадение хэша; обратное — с вероятностью 2⁻⁶⁴,
// а цена ошибки — лишние 0.25 штрафа «уже был».
func lookID(look string) uint64 {
	h := uint64(14695981039346656037)
	for i := range len(look) {
		h = (h ^ uint64(look[i])) * 1099511628211
	}
	return h
}

// Twins — для каждого вида индекс самого раннего с тем же видом, −1 — такого нет
// (подборка сворачивает «близнецов» в плитку «Похожие»).
func Twins(looks []string) []int {
	out := make([]int, len(looks))
	first := make(map[string]int, len(looks))
	for i, l := range looks {
		if j, ok := first[l]; ok {
			out[i] = j
			continue
		}
		first[l], out[i] = i, -1
	}
	return out
}
