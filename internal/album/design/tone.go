package design

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Ч/б и тон фото в месте (068): PhotoRef.Adj, едет с фото, как кадр и зеркало; прозрачность
// фото (079) — PhotoRef.Fade.

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

// SetPhotoFade — прозрачность фото ids (079, PhotoRef.Fade, уже зажата ClampPhotoFade): едет с
// фото, как тон; у «Авто» без закрепления. Чужие ID пропускаются; то же значение — холостая.
// У фото только на фоне или в рамочке PruneRefs её снимет — правка там холостая.
func SetPhotoFade(p *model.Project, ids []string, fade float64) {
	for _, id := range ids {
		if r, ok := p.Photos[id]; ok {
			r.Fade = fade
			p.Photos[id] = r
		}
	}
}
