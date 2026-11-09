package layers

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Порядок слоёв: места, стикеры и тексты — один стек (спеки 009, 010, 048).

// ArrangeLayers — тело Editor.ArrangeLayers над проектом.
func ArrangeLayers(p *model.Project, spreadID, op string, ids []string) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	var sel []string
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil && !slices.Contains(sel, pl.ID) {
			sel = append(sel, pl.ID)
		}
	}
	for _, st := range s.Stickers {
		if slices.Contains(ids, st.ID) {
			sel = append(sel, st.ID)
		}
	}
	for _, t := range s.Texts {
		if slices.Contains(ids, t.ID) {
			sel = append(sel, t.ID)
		}
	}
	s.RestackLayers(op, sel)
}
