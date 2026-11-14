package text

// noWrap — ширина «без переноса», мм: больше любой страницы.
const noWrap = 1e5

// NaturalWidth — ширина текста без переноса по словам, мм: самая длинная строка
// по абзацам. Через Cached, поэтому повторный вызов дешёвый; пустой текст — 0.
func NaturalWidth(s string, st Style) float64 {
	w := 0.0
	for _, l := range Cached(s, st, noWrap).Lines {
		w = max(w, l.W)
	}
	return w
}
