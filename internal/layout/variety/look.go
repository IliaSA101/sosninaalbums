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
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Family — характер раскладки; подписи для интерфейса считает Go (Choice.family).
type Family string

// Семейства: у Describe — первое подошедшее по порядку bleed, air, hero, dense (ритм 041 в Pick);
// у подборки — по форме (FamilyOf), там же strip и figure (051).
const (
	Dense  Family = "dense"  // «Плотно»
	Hero   Family = "hero"   // «С главным фото»
	Air    Family = "air"    // «Воздух»
	Bleed  Family = "bleed"  // «В край»
	Strip  Family = "strip"  // «Ленты»: лента, ряды, сетка (только FamilyOf)
	Figure Family = "figure" // «Фигуры»: лесенка, мелкое — крупное, рамка (только FamilyOf)
)

// FamilyOf — сегмент фильтра подборки по форме (design §8): воздух и разные страницы — «Воздух»,
// главное, через корешок и пара — «С главным фото», мозаика — «Плотно».
func FamilyOf(f forms.Form) Family {
	switch f {
	case forms.Strip, forms.Rows, forms.Grid:
		return Strip
	case forms.Ladder, forms.Contrast, forms.Frame:
		return Figure
	case forms.Air, forms.Asym:
		return Air
	case forms.Spine, forms.Hero, forms.Pair:
		return Hero
	case forms.Bleed:
		return Bleed
	}
	return Dense
}

// FormName — подпись формы для подсказки плитки; у мозаики пусто: «Плотно» — это и так весь ряд.
func FormName(f forms.Form) string {
	if f == forms.Mosaic {
		return ""
	}
	return forms.Name(f)
}

const (
	// lookQuant — квант вида в долях высоты страницы: меньше глаз не различает.
	lookQuant = 0.03
	// airFill — разворот с заполнением ниже считается воздушным.
	airFill = 0.55
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
	Form forms.Form // форма (forms.Of, 051): соседи чередуют формы
	Q    float64    // оценка внутри формы (forms.Quality): допуск форм в Pick
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
// пару), семейство, оценку без штрафа разброса за главные, форму и её оценку.
func Describe(r layout.Result, p layout.Params) Info {
	f := forms.Of(r.Key, r.Rects, p)
	q, _ := forms.Quality(r, f, p)
	return describe(r, p, f, q)
}

// describe — Describe с уже известными формой и её оценкой (ряд «Авто» их посчитал).
func describe(r layout.Result, p layout.Params, f forms.Form, q float64) Info {
	in := Info{Look: Look(r.Rects, p), Big: -1, Fair: r.Score, Form: f, Q: q}
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
	h := forms.Heroes(r.Rects)
	in.Heroes = h.N
	if h.N == 1 {
		in.Big = h.Idx[0]
	}
	if h.N > 0 {
		// Штраф layout — 0.05·log₂(max/min) по всем; у Fair максимум — у первого не-главного.
		in.Fair += spreadPenalty * math.Log2(h.Top[0]/h.Top[h.N])
	}
	if all := 2 * p.PageW * p.PageH; all > 0 {
		in.Fill = sum / all
	}
	switch {
	case f == forms.Bleed:
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
