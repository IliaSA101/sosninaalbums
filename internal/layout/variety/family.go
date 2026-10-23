package variety

import (
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Families — все семейства фильтра подборки в порядке сегментов окна (065): по ним App и
// spreads узнают имена; «все шесть» — то же, что «Все».
func Families() []Family { return []Family{Dense, Hero, Air, Bleed, Strip, Figure} }

// ByFamily — выборка пула разворота по семействам формы (FamilyOf) в прежнем порядке (065):
// allow == nil — «Все», пул как есть (без копии). Разрешённых нет — только лучшая раскладка
// (pool[:1]) и fallback: весь пул дал бы ленты и воздух вместо «Плотно» (замер design §2).
func ByFamily(pool []Cand, allow map[Family]bool) (out []Cand, fallback bool) {
	if allow == nil {
		return pool, false
	}
	for _, c := range pool {
		if allow[FamilyOf(c.Form)] {
			out = append(out, c)
		}
	}
	if len(out) == 0 && len(pool) > 0 {
		return pool[:1], true
	}
	return out, false
}

// DenseFill — заполнение области в полях, с которого «Две крупных» считается «Плотно»
// (0.23-q7): семья плитки — не только вид, но и «лист закрыт».
const DenseFill = 0.8

// TileFamily — семейство плитки галереи и фильтра 065: FamilyOf, но «Две крупных», закрывающие
// не меньше DenseFill области в полях, — «Плотно» (шесть вертикальных на широком листе иначе
// не бывают плотными: мозаика уходит за границу ряда). Ряд «Авто», Pick и ритм считают по
// FamilyOf, их эталоны не меняются.
func TileFamily(f forms.Form, rects []layout.Rect, p layout.Params) Family {
	if f == forms.Pair && areaFill(rects, p) >= DenseFill {
		return Dense
	}
	return FamilyOf(f)
}

// areaFill — сумма площадей фото / область разворота в полях; 0 при пустой области.
func areaFill(rects []layout.Rect, p layout.Params) float64 {
	area := (2*p.PageW - 2*p.Margin) * (p.PageH - 2*p.Margin)
	if area <= 0 {
		return 0
	}
	var sum float64
	for _, r := range rects {
		sum += r.W * r.H
	}
	return sum / area
}
