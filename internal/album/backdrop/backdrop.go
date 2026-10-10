package backdrop

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Фото на фоне разворота или страницы (спека 009, design-backdrop). Рамка — как у
// места, поворот — как у текста (geom.TextBox); кадр и зеркало свои, из PhotoRef не берутся.

// SetBackdrop — тело Editor.SetBackdrop над проектом. Неизвестная область — ErrScope
// и проект не тронут: тело зовут и мимо обёртки, проверяющей область.
func SetBackdrop(p *model.Project, spreadID, scope string, ph model.PhotoInfo) error {
	_, s := p.Find(spreadID)
	if s == nil {
		return nil
	}
	slot := s.Slot(scope)
	if slot == nil {
		return model.ErrScope
	}
	model.Remember(p, []model.PhotoInfo{ph})
	if scope == model.ScopeSpread {
		s.BgPhotoL, s.BgPhotoR = nil, nil
	} else {
		s.BgPhoto = nil
	}
	if *slot == nil {
		b := &model.Backdrop{Photo: ph.ID}
		b.FitArea(p.BdArea(scope))
		*slot = b
		return nil
	}
	b := (*slot).Clone()
	b.Photo, b.Frame = ph.ID, nil
	*slot = b
	return nil
}

// changeBackdrop — правка фона области: fn меняет копию, затем фон зажимается;
// нет фона — холостая правка.
func changeBackdrop(p *model.Project, spreadID, scope string, fn func(p *model.Project, b *model.Backdrop)) {
	if !model.ValidScope(scope) {
		return
	}
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	slot := s.Slot(scope)
	if *slot == nil {
		return
	}
	b := (*slot).Clone()
	fn(p, b)
	*slot = p.ClampBackdrop(b, scope)
}

// SetBackdropBox — тело Editor.SetBackdropBox над проектом.
func SetBackdropBox(p *model.Project, spreadID, scope string, x, y, w, h, rot float64) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, b *model.Backdrop) {
		b.X, b.Y, b.W, b.H, b.Rot = x, y, w, h, rot
	})
}

// SetBackdropFrame — тело Editor.SetBackdropFrame над проектом.
func SetBackdropFrame(p *model.Project, spreadID, scope string, s, x, y float64) {
	f := model.ClampFrame(&model.Frame{S: s, X: x, Y: y})
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, b *model.Backdrop) { b.Frame = f })
}

// FlipBackdrop — тело Editor.FlipBackdrop над проектом (axis уже проверена).
func FlipBackdrop(p *model.Project, spreadID, scope, axis string) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, b *model.Backdrop) {
		fr := b.FrameOrDefault()
		if axis == "h" {
			b.FlipH = !b.FlipH
			if fr.X != 0 { // нулевой ход не трогаем: иначе в JSON попал бы «-0»
				fr.X = -fr.X
			}
		} else {
			b.FlipV = !b.FlipV
			if fr.Y != 0 {
				fr.Y = -fr.Y
			}
		}
		b.Frame = model.ClampFrame(&fr)
	})
}

// SetBackdropFade — тело Editor.SetBackdropFade над проектом (fade — число).
func SetBackdropFade(p *model.Project, spreadID, scope string, fade float64) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, b *model.Backdrop) { b.Fade = fade })
}

// FitBackdrop — тело Editor.FitBackdrop над проектом.
func FitBackdrop(p *model.Project, spreadID, scope string) {
	changeBackdrop(p, spreadID, scope, func(p *model.Project, b *model.Backdrop) { b.FitArea(p.BdArea(scope)) })
}

// ClearBackdrop — тело Editor.ClearBackdrop над проектом; неизвестная область — холостая правка.
func ClearBackdrop(p *model.Project, spreadID, scope string) {
	if _, s := p.Find(spreadID); s != nil {
		if slot := s.Slot(scope); slot != nil {
			*slot = nil
		}
	}
}

// Rescale — поменялся формат страницы: рамки остаются на том же месте
// страницы пропорционально (как места), затем зажим.
func Rescale(p *model.Project, before *model.Project) {
	ow, oh := before.Settings.PageW, before.Settings.PageH
	nw, nh := p.Settings.PageW, p.Settings.PageH
	if ow == nw && oh == nh || !(ow > 0) || !(oh > 0) {
		return
	}
	kx, ky := nw/ow, nh/oh
	for _, s := range p.Spreads {
		for _, scope := range model.BdScopes() {
			slot := s.Slot(scope)
			if b := *slot; b != nil {
				b.X, b.W, b.Y, b.H = b.X*kx, b.W*kx, b.Y*ky, b.H*ky
				*slot = p.ClampBackdrop(b, scope)
			}
		}
	}
}
