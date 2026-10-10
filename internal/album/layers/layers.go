package layers

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Порядок слоёв: места и тексты — один стек (спеки 009, 010).

// ArrangeLayers — тело Editor.ArrangeLayers над проектом.
func ArrangeLayers(p *model.Project, spreadID, op string, ids []string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	var sel []string
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil && !slices.Contains(sel, pl.ID) {
			sel = append(sel, pl.ID)
		}
	}
	for _, t := range s.Texts {
		if slices.Contains(ids, t.ID) {
			sel = append(sel, t.ID)
		}
	}
	s.RestackLayers(op, sel)
}
