package variety

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
