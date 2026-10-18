package spreads

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// SetLayout — операция над проектом.
func SetLayout(p *model.Project, spreadID, key string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	asp, prm := p.Aspects(s.Photos), p.Params()
	if r, ok := layout.Build(key, asp, prm); ok && foldAllowed(r, s, asp, prm) {
		p.Unfreeze(s)
		p.SetLayout(s, key)
	}
}

// foldAllowed — ключ ставится, если раскладка проходит правило сгиба (layout.FoldOK: ряды
// подборки подчинены ему же, искать ключ в них незачем) или уже стоит — прежний ключ старого
// альбома переставляется холостой правкой (decisions 0.15 №7, пересмотр 0.13.1 №23, 0.14 №25).
func foldAllowed(r layout.Result, s *model.Spread, asp []float64, prm layout.Params) bool {
	return layout.FoldOK(r.Rects, asp, prm) || r.Key == s.Layout
}

// StepLayout — операция над проектом.
func StepLayout(p *model.Project, spreadID, scope string, delta int) {
	_, s := p.Find(spreadID)
	if s == nil || delta == 0 || len(s.Grid) > 0 { // в сетке листать нечего
		return
	}
	cur, _, ok := p.Layout(s)
	if !ok {
		return
	}
	asp, prm := p.Aspects(s.Photos), p.Params()
	var inOrder, rearranged []layout.Result
	switch scope {
	case "spread":
		row, i, more := model.AutoRow(asp, prm, cur.Key)
		if i >= 0 && delta > 0 && more && i+delta >= len(row) {
			p.SetLayout(s, stepMore(asp, prm, row))
			return
		}
		// Пробел и ‹ › после ряда «Авто» идут по «Другим вариантам» (решение 0.11 №36).
		inOrder = slices.Concat(row, model.ExtraRow(asp, prm))
		if i < 0 && layout.IsRearranged(cur.Key) {
			rearranged, _ = layout.CachedRearranged(asp, prm, model.VariantLimit)
		}
	case "left", "right":
		side := 0
		if scope == "right" {
			side = 1
		}
		if inOrder, rearranged, _, ok = layout.CachedPages(cur.Key, asp, prm, side, model.VariantLimit); !ok {
			return
		}
	default:
		return
	}
	if key := step(inOrder, rearranged, cur.Key, delta); key != "" {
		p.SetLayout(s, key)
	}
}

// stepMore — шаг вперёд за конец ряда «Авто», который можно досчитать (17+ фото): ряд
// растёт на порцию, встаём на её первый вариант; порция пуста — к началу ряда.
func stepMore(asp []float64, prm layout.Params, row []layout.Result) string {
	if next, _ := model.MoreRow(asp, prm, len(row)); len(next) > len(row) {
		return next[len(row)].Key
	}
	return row[0].Key
}

// step — следующий ключ по кругу в ряду, где стоит cur.
func step(inOrder, rearranged []layout.Result, cur string, delta int) string {
	for _, row := range [][]layout.Result{inOrder, rearranged} {
		if i := model.IndexOf(row, cur); i >= 0 {
			n := len(row)
			return row[((i+delta)%n+n)%n].Key
		}
	}
	if len(inOrder) == 0 {
		return ""
	}
	if delta < 0 {
		return inOrder[len(inOrder)-1].Key
	}
	return inOrder[0].Key
}

// LayoutChoices — чтение: ряды раскладок разворота; Rev выставляет Editor. Ряд «Авто» на
// 17+ фото — до порции с текущей раскладкой (как счётчик в View), More — можно досчитать.
func LayoutChoices(p *model.Project, spreadID string) model.Choices {
	return choices(p, spreadID, -1)
}

// MoreLayouts — чтение: то же, но ряд «Авто» досчитан ещё на порцию после have вариантов
// (плитка «Ещё» в галерее). До 16 фото — как LayoutChoices.
func MoreLayouts(p *model.Project, spreadID string, have int) model.Choices {
	return choices(p, spreadID, max(have, 0))
}

func choices(p *model.Project, spreadID string, have int) model.Choices {
	c := model.Choices{Spread: spreadID, Whole: model.ChoiceRows{InOrder: []model.Choice{}, Rearranged: []model.Choice{}, Extra: []model.Choice{}}}
	_, s := p.Find(spreadID)
	if s == nil {
		return c
	}
	cur, _, ok := p.Layout(s)
	if !ok {
		return c
	}
	asp, prm := p.Aspects(s.Photos), p.Params()
	row, _, more := model.AutoRow(asp, prm, cur.Key)
	if have >= 0 {
		if r, m := model.MoreRow(asp, prm, have); len(r) > len(row) {
			row, more = r, m
		}
	}
	rr, block := layout.CachedRearranged(asp, prm, model.VariantLimit)
	c.Whole = choiceRows(row, rr, block, prm)
	c.Whole.More = more
	c.Whole.Extra = extraChoices(asp, prm)
	for side, dst := range []**model.ChoiceRows{&c.Left, &c.Right} {
		if in, rr, block, ok := layout.CachedPages(cur.Key, asp, prm, side, model.VariantLimit); ok {
			rows := choiceRows(in, rr, block, prm)
			*dst = &rows
		}
	}
	return c
}

// choiceRows — ряды подборки; у плиток семейство и похожие (041), похожие — в пределах ряда.
func choiceRows(inOrder, rearranged []layout.Result, block layout.Block, prm layout.Params) model.ChoiceRows {
	conv := func(rs []layout.Result) []model.Choice {
		out := make([]model.Choice, len(rs))
		for i, r := range rs {
			out[i] = model.Choice{Key: r.Key, Rects: r.Rects}
		}
		describeRows(out, prm)
		return out
	}
	// Extra не nil: UI ждёт массив, а не null; «Другие варианты» — только у разворота.
	return model.ChoiceRows{InOrder: conv(inOrder), Rearranged: conv(rearranged), Block: string(block), Extra: []model.Choice{}}
}
