package model

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Общий порядок слоёв (спека 010, design §4): места и тексты разворота-сетки —
// один стек, текст может лечь под фото. Spread.Order хранит его, только если он
// отличается от порядка по умолчанию (места по Grid, затем тексты): файл без
// «текста под фото» поля не несёт, холостая правка остаётся холостой.

// Виды элементов стека; новый вид (календарь, виньетка) — ветка в layerItems.
const (
	LayerPlace = "place"
	LayerText  = "text"
)

// LayerItem — элемент стека слоёв. Box — рамка в форме «угол до поворота + rot»
// (у места через placeBox): одна геометрия для слоёв и выравнивания.
type LayerItem struct {
	ID   string
	Kind string
	Box  geom.TextBox
}

// LayerItems — стек разворота снизу вверх (stack) с рамками.
func (s *Spread) LayerItems() []LayerItem {
	byID := make(map[string]LayerItem, len(s.Grid)+len(s.Texts))
	for _, pl := range s.Grid {
		b := geom.TextBox{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H} // ровное — без шума сложений placeBox
		if pl.Rot != 0 {
			b = PlaceBoxOf(pl)
		}
		byID[pl.ID] = LayerItem{ID: pl.ID, Kind: LayerPlace, Box: b}
	}
	for _, t := range s.Texts {
		byID[t.ID] = LayerItem{ID: t.ID, Kind: LayerText, Box: BoxOf(t)}
	}
	st := s.Stack()
	out := make([]LayerItem, len(st))
	for i, id := range st {
		out[i] = byID[id]
	}
	return out
}

// Stack — ID мест и текстов снизу вверх, Order с починкой (шаги 1–3 SyncOrder):
// у «Авто» — только тексты; чужие ID и повторы выбрасываются; недостающее место
// встаёт сразу над предшествующим ему в Grid (первое — под первым местом стека,
// мест нет — вниз), недостающий текст — наверх. Так applyGrid, AddPlace, деление
// и AddText ведут себя как в 0.4.0.
func (s *Spread) Stack() []string {
	out := make([]string, 0, len(s.Grid)+len(s.Texts))
	if len(s.Grid) == 0 || s.Order == nil {
		for _, pl := range s.Grid {
			out = append(out, pl.ID)
		}
		for _, t := range s.Texts {
			out = append(out, t.ID)
		}
		return out
	}
	kind := s.Kinds()
	place := func(id string) bool { return kind[id] == LayerPlace }
	in := make(map[string]bool, cap(out))
	for _, id := range s.Order {
		if kind[id] != "" && !in[id] {
			in[id] = true
			out = append(out, id)
		}
	}
	for i, pl := range s.Grid {
		if in[pl.ID] {
			continue
		}
		at := 0
		if i > 0 {
			at = slices.Index(out, s.Grid[i-1].ID) + 1
		} else if j := slices.IndexFunc(out, place); j >= 0 {
			at = j
		}
		out = slices.Insert(out, at, pl.ID)
		in[pl.ID] = true
	}
	for _, t := range s.Texts {
		if !in[t.ID] {
			in[t.ID] = true
			out = append(out, t.ID)
		}
	}
	return out
}

// SetStack ставит порядок st (перестановка stack): Grid и Texts — в его
// относительном порядке (код, читающий порядок мест из Grid, не меняется),
// Photos сетки — по местам, Order — st или nil, если st — порядок по умолчанию.
func (s *Spread) SetStack(st []string) {
	pos := make(map[string]int, len(st))
	for i, id := range st {
		pos[id] = i
	}
	// Новые срезы, а не сортировка на месте: срез мог достаться от копии разворота.
	if grid := slices.SortedStableFunc(slices.Values(s.Grid), func(a, b *Place) int { return pos[a.ID] - pos[b.ID] }); !slices.Equal(grid, s.Grid) {
		s.Grid = grid
		s.Photos = GridPhotos(s)
	}
	if texts := slices.SortedStableFunc(slices.Values(s.Texts), func(a, b *TextBlock) int { return pos[a.ID] - pos[b.ID] }); !slices.Equal(texts, s.Texts) {
		s.Texts = texts
	}
	// По умолчанию — все места ниже всех текстов: st — перестановка мест и текстов.
	s.Order = nil
	kind := s.Kinds()
	if len(s.Grid) > 0 && slices.ContainsFunc(st[:min(len(s.Grid), len(st))], func(id string) bool { return kind[id] != LayerPlace }) {
		s.Order = slices.Clone(st)
	}
}

// Kinds — вид элемента стека по ID.
func (s *Spread) Kinds() map[string]string {
	kind := make(map[string]string, len(s.Grid)+len(s.Texts))
	for _, pl := range s.Grid {
		kind[pl.ID] = LayerPlace
	}
	for _, t := range s.Texts {
		kind[t.ID] = LayerText
	}
	return kind
}

// SyncOrder — шаг конвейера change после FitTexts и последний шаг Load: Order
// чинится (stack) и приводится к nil, если совпал с порядком по умолчанию.
// Идемпотентна.
func (p *Project) SyncOrder() {
	for _, s := range p.Spreads {
		if len(s.Grid) == 0 {
			s.Order = nil
			continue
		}
		s.SetStack(s.Stack())
	}
}

// LoadOrder — Order из файла: огромный список обрезается до починки, затем
// SyncOrder. Вызывается последним в Load: после SplitBig (копирует Order в первую
// часть) и syncGrid(true) (добавляет места), иначе первая правка не холостая.
func (p *Project) LoadOrder() {
	for _, s := range p.Spreads {
		if n := MaxPlaces + MaxTexts; len(s.Order) > n {
			s.Order = s.Order[:n]
		}
	}
	p.SyncOrder()
}

// overlapMM2 — площадь пересечения, меньше которой рамки не считаются перекрытыми.
const overlapMM2 = 0.01

// overlaps — пересечение прямоугольников больше overlapMM2.
func overlaps(a, b layout.Rect) bool {
	w := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
	h := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	return w > 0 && h > 0 && w*h > overlapMM2
}

// RestackLayers — op над общим стеком s для выделенных ID; неизвестный op — ничего.
// Рамка элемента — описанный прямоугольник: «Выше» может перешагнуть повёрнутое
// место, которого рамка не касается, — допустимо.
func (s *Spread) RestackLayers(op string, sel []string) {
	if op != "front" && op != "up" && op != "down" && op != "back" {
		return
	}
	items := s.LayerItems()
	ids := make([]string, len(items))
	rects := make(map[string]layout.Rect, len(items))
	for i, it := range items {
		ids[i], rects[it.ID] = it.ID, it.Box.AABB()
	}
	s.SetStack(restack(ids, sel, op, func(id string) layout.Rect { return rects[id] }))
}

// restack переставляет выделенные sel в стеке (последний — сверху) и отдаёт новый
// срез; stack не меняется. rect — рамка элемента.
func restack[T comparable](stack, sel []T, op string, rect func(T) layout.Rect) []T {
	if len(sel) == 0 {
		return stack
	}
	out := slices.Clone(stack)
	picked := func(v T) bool { return slices.Contains(sel, v) }
	switch op {
	case "front", "back":
		rest := slices.DeleteFunc(slices.Clone(out), picked)
		moved := slices.DeleteFunc(slices.Clone(out), func(v T) bool { return !picked(v) })
		if op == "front" {
			return append(rest, moved...)
		}
		return append(moved, rest...)
	case "up":
		// Сверху вниз: каждый встаёт сразу над ближайшим выше невыделенным перекрывающимся.
		for i := len(out) - 1; i >= 0; i-- {
			if !picked(out[i]) {
				continue
			}
			for j := i + 1; j < len(out); j++ {
				if !picked(out[j]) && overlaps(rect(out[i]), rect(out[j])) {
					v := out[i]
					out = slices.Insert(slices.Delete(out, i, i+1), j, v)
					break
				}
			}
		}
	case "down":
		// Снизу вверх, зеркально: встаёт сразу под ближайшим ниже перекрывающимся.
		for i := range out {
			if !picked(out[i]) {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				if !picked(out[j]) && overlaps(rect(out[i]), rect(out[j])) {
					v := out[i]
					out = slices.Insert(slices.Delete(out, i, i+1), j, v)
					break
				}
			}
		}
	}
	return out
}
