package design

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Ч/б и тон фото в месте (068): PhotoRef.Adj, едет с фото, как кадр и зеркало.

// SetGray — ч/б фото ids «всем или никому», как SetFlip: если включено не у всех известных —
// всем включается, иначе у всех выключается. Яркость и контраст остаются. Пустые, чужие и
// повторные ID пропускаются; ни одного известного — холостая правка.
func SetGray(p *model.Project, ids []string) {
	var known []string
	all := true
	for _, id := range ids {
		r, ok := p.Photos[id]
		if !ok || slices.Contains(known, id) {
			continue
		}
		known = append(known, id)
		all = all && r.Adj != nil && r.Adj.Gray
	}
	for _, id := range known {
		r := p.Photos[id]
		var t model.Tone
		if r.Adj != nil {
			t = *r.Adj
		}
		t.Gray = !all
		r.Adj = model.ClampTone(&t)
		p.Photos[id] = r
	}
}

// SetTone ставит яркость и контраст фото из t (уже зажаты; nil — нули), ч/б остаётся.
// Чужое фото пропускается; те же значения — холостая правка (нули — Adj nil).
func SetTone(p *model.Project, photoID string, t *model.Tone) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	var n model.Tone
	if t != nil {
		n.Bright, n.Contrast = t.Bright, t.Contrast
	}
	n.Gray = r.Adj != nil && r.Adj.Gray
	r.Adj = model.ClampTone(&n)
	p.Photos[photoID] = r
}
