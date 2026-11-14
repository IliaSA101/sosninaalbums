package prefs

import (
	"maps"
	"slices"
)

// Туры обучения (113): состояние в config.json как карта id → pending | done.
const (
	TourFirst   = "first"  // главный тур «Первые шаги»
	TourSpread  = "spread" // разовое продолжение на развороте с фото
	TourPending = "pending"
	TourDone    = "done"
	maxTours    = 8
)

func validTourState(s string) bool { return s == TourPending || s == TourDone }

// cleanTours оставляет годные пары (id по правилу customID, значение pending | done) не больше
// maxTours. Порядок ключей у карты не определён, поэтому при переполнении берём по алфавиту.
func cleanTours(m map[string]string) map[string]string {
	out := map[string]string{}
	for _, id := range slices.Sorted(maps.Keys(m)) {
		if len(out) == maxTours {
			break
		}
		if customID(id) && validTourState(m[id]) {
			out[id] = m[id]
		}
	}
	return out
}

// SetTourState — копия карты с id → state. Негодный id или state — карта как была; новый ключ
// сверх maxTours не добавляется. «done» не откатывается в «pending»: устаревший клиент
// (вторая вкладка с прежним TourDue) не должен вернуть тур, который уже прошли.
// Остальные ключи остаются как были: их пишет другая версия программы.
func SetTourState(m map[string]string, id, state string) map[string]string {
	if !customID(id) || !validTourState(state) {
		return m
	}
	if _, ok := m[id]; !ok && len(m) >= maxTours {
		return m
	}
	if m[id] == TourDone && state == TourPending {
		return m
	}
	out := maps.Clone(m)
	if out == nil {
		out = map[string]string{}
	}
	out[id] = state
	return out
}

// tourFlags — решения «показывать ли» (Prefs.TourDue, Prefs.TourMore): уровня ещё нет —
// первый запуск; first pending — программу закрыли посреди тура. Обновившимся (уровень уже
// есть, тура нет) тур сам не стартует.
func tourFlags(level string, tours map[string]string) (due, more bool) {
	t := cleanTours(tours)
	first := t[TourFirst]
	due = first != TourDone && (level == "" || first == TourPending)
	more = first == TourDone && t[TourSpread] != TourDone
	return due, more
}
