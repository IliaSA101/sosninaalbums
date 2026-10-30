package spreads

// Семейства раскладок в автосборке и «Разнообразить» (065, design §2): порядок отбора —
// генерация пула (правило сгиба внутри) → семейства (variety.ByFamily) → Pick.

import "github.com/IliaSA101/sosninaalbums/internal/layout/variety"

// allowOf — разрешённые семейства из окна: неизвестные имена отбрасываются, повторы
// схлопываются; пусто или все шесть — nil («Все», поведение 0.14 побитово).
func allowOf(fs []string) map[variety.Family]bool {
	known := variety.Families()
	var allow map[variety.Family]bool
	for _, f := range fs {
		for _, k := range known {
			if variety.Family(f) == k {
				if allow == nil {
					allow = make(map[variety.Family]bool, len(known))
				}
				allow[k] = true
			}
		}
	}
	if len(allow) == len(known) {
		return nil
	}
	return allow
}

// poolLv — уровень, с которым считается пул: с фильтром — не ниже «Обычного» (ряд и «Другие
// варианты» — то, что видит подборка), иначе «Меньше» с фильтром выбирать было бы не из чего.
func poolLv(lv variety.Level, allow map[variety.Family]bool) variety.Level {
	if allow != nil {
		return max(lv, variety.Normal)
	}
	return lv
}
