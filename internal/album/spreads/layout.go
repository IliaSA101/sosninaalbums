package spreads

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// SetLayout — операция над проектом.
func SetLayout(p *model.Project, spreadID, key string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	if _, ok := layout.Build(key, p.Aspects(s.Photos), p.Params()); ok {
		p.Unfreeze(s)
		p.SetLayout(s, key)
	}
}

// StepLayout — операция над проектом.
func StepLayout(p *model.Project, spreadID, scope string, delta int) {
	_, s := p.Find(spreadID)
	if s == nil || delta == 0 || len(s.Grid) > 0 { // в сетке листать нечего
		return
	}
	cur, best, ok := p.Layout(s)
	if !ok {
		return
	}
	asp, prm := p.Aspects(s.Photos), p.Params()
	var inOrder, rearranged []layout.Result
	switch scope {
	case "spread":
		inOrder = best
		if model.IndexOf(best, cur.Key) < 0 && layout.IsRearranged(cur.Key) {
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

// LayoutChoices — чтение: ряды раскладок разворота; Rev выставляет Editor.
func LayoutChoices(p *model.Project, spreadID string) model.Choices {
	c := model.Choices{Spread: spreadID, Whole: model.ChoiceRows{InOrder: []model.Choice{}, Rearranged: []model.Choice{}}}
	_, s := p.Find(spreadID)
	if s == nil {
		return c
	}
	cur, best, ok := p.Layout(s)
	if !ok {
		return c
	}
	asp, prm := p.Aspects(s.Photos), p.Params()
	rr, block := layout.CachedRearranged(asp, prm, model.VariantLimit)
	c.Whole = choiceRows(best, rr, block)
	for side, dst := range []**model.ChoiceRows{&c.Left, &c.Right} {
		if in, rr, block, ok := layout.CachedPages(cur.Key, asp, prm, side, model.VariantLimit); ok {
			rows := choiceRows(in, rr, block)
			*dst = &rows
		}
	}
	return c
}

func choiceRows(inOrder, rearranged []layout.Result, block layout.Block) model.ChoiceRows {
	conv := func(rs []layout.Result) []model.Choice {
		out := make([]model.Choice, len(rs))
		for i, r := range rs {
			out[i] = model.Choice{Key: r.Key, Rects: r.Rects}
		}
		return out
	}
	return model.ChoiceRows{InOrder: conv(inOrder), Rearranged: conv(rearranged), Block: string(block)}
}
