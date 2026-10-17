package backdrop

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Фото на фоне разворота или страницы (спека 009, design-backdrop). Рамка — как у
// места, поворот — как у текста (geom.TextBox); кадр, наклон (045) и зеркало свои, из
// PhotoRef не берутся.

// SetBackdrop — тело Editor.SetBackdrop над проектом. Неизвестная область — ErrScope
// и проект не тронут: тело зовут и мимо обёртки, проверяющей область. То же фото на
// том же фоне — холостая правка; другое фото сбрасывает кадр и наклон (у снимка свой
// горизонт), рамка и зеркало остаются. Своё из библиотеки (ph.Lib, 048): ключ не по
// грамматике или ID не от ключа — ErrLibKey (иначе Rekey при открытии разведёт запись и фон).
func SetBackdrop(p *model.Project, spreadID, scope string, ph model.PhotoInfo) error {
	if ph.Lib != "" && (!model.ValidLibKey(ph.Lib) || ph.ID != model.LibPhotoID(ph.Lib)) {
		return model.ErrLibKey
	}
	s := p.Sheet(spreadID)
	if s == nil {
		return nil
	}
	slot := s.Slot(scope)
	if _, ok := p.AreaOf(s, scope); slot == nil || !ok { // у листа 031 страниц нет, корешок — только у обложки
		return model.ErrScope
	}
	if scope == model.ScopeSpine {
		if err := p.SpineErr(s); err != nil {
			return err
		}
	}
	if *slot != nil && (*slot).Photo == ph.ID { // то же фото — холостая: кадр и наклон не сбрасываем
		return nil
	}
	model.Remember(p, []model.PhotoInfo{ph})
	if scope == model.ScopeSpread { // одно фото через весь лист исключает остальные (061 I2)
		s.BgPhotoL, s.BgPhotoR, s.BgPhotoS = nil, nil, nil
	} else {
		s.BgPhoto = nil
	}
	if *slot == nil {
		b := &model.Backdrop{Photo: ph.ID}
		b.FitArea(p.BdArea(s, scope))
		*slot = b
		return nil
	}
	b := (*slot).Clone()
	b.Photo, b.Frame = ph.ID, nil
	*slot = b
	return nil
}

// changeBackdrop — правка фона области листа (и обложки): fn меняет копию, затем фон
// зажимается; нет фона — холостая правка.
func changeBackdrop(p *model.Project, spreadID, scope string, fn func(p *model.Project, s *model.Spread, b *model.Backdrop)) {
	if !model.ValidBdScope(scope) {
		return
	}
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	slot := s.Slot(scope)
	if *slot == nil {
		return
	}
	b := (*slot).Clone()
	fn(p, s, b)
	*slot = p.ClampBackdrop(s, b, scope)
}

// SetBackdropBox — тело Editor.SetBackdropBox над проектом.
func SetBackdropBox(p *model.Project, spreadID, scope string, x, y, w, h, rot float64) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, _ *model.Spread, b *model.Backdrop) {
		b.X, b.Y, b.W, b.H, b.Rot = x, y, w, h, rot
	})
}

// SetBackdropFrame — тело Editor.SetBackdropFrame над проектом. Наклон прежнего кадра
// остаётся (как design.SetFrame).
func SetBackdropFrame(p *model.Project, spreadID, scope string, s, x, y float64) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, _ *model.Spread, b *model.Backdrop) {
		b.Frame = model.ClampFrame(&model.Frame{S: s, X: x, Y: y, R: b.FrameOrDefault().R})
	})
}

// SetBackdropAngle — тело Editor.SetBackdropAngle: наклон фото в рамке фона tilt, градусы
// (±44,9, шаг 0,1; четвертей у фона нет — 90° даёт «Поворот» рамки) от кадра base (начало
// сессии жеста; nil — текущий). Рамка стоит, наклон держит приближение, как у фото в месте
// (057, решение 14): рамка — прямоугольник b.W×b.H.
func SetBackdropAngle(p *model.Project, spreadID, scope string, tilt float64, base *model.Frame) {
	changeBackdrop(p, spreadID, scope, func(p *model.Project, _ *model.Spread, b *model.Backdrop) { setTilt(p, b, tilt, base) })
}

// StraightenBackdrop — тело Editor.StraightenBackdrop: линия (dx, dy) мм листа встаёт
// параллельно краю рамки фона. Нет линии или она уже ровная — холостая правка.
func StraightenBackdrop(p *model.Project, spreadID, scope string, dx, dy float64) {
	changeBackdrop(p, spreadID, scope, func(p *model.Project, _ *model.Spread, b *model.Backdrop) {
		if d, ok := geom.StraightenDelta(dx, dy, b.Rot); ok {
			setTilt(p, b, b.FrameOrDefault().R+d, nil)
		}
	})
}

// setTilt — наклон кадра фона: зажим ±44,9 и шаг 0,1 (ClampBackdrop потом его не меняет);
// S, X, Y — geom.TiltCrop от base (nil — текущий кадр).
func setTilt(p *model.Project, b *model.Backdrop, tilt float64, base *model.Frame) {
	f := b.FrameOrDefault()
	if base != nil {
		f = *base
	}
	r := geom.ClampF(model.Round1(tilt), -model.MaxTilt, model.MaxTilt)
	c := geom.TiltCrop(layout.Rect{W: b.W, H: b.H}, p.RawAspect(b.Photo), geom.Crop{S: f.S, X: f.X, Y: f.Y, R: f.R}, r, geom.Outline{})
	b.Frame = model.ClampFrame(&model.Frame{S: c.S, X: c.X, Y: c.Y, R: c.R})
}

// BaseFrame — кадр фона области scope листа spreadID в снимке p0 (начало сессии наклона,
// 057). Снимка, листа или фона нет — nil: база — текущий кадр.
func BaseFrame(p0 *model.Project, spreadID, scope string) *model.Frame {
	if p0 == nil {
		return nil
	}
	s := p0.Sheet(spreadID)
	if s == nil {
		return nil
	}
	if sl := s.Slot(scope); sl != nil && *sl != nil {
		f := (*sl).FrameOrDefault()
		return &f
	}
	return nil
}

// FlipBackdrop — тело Editor.FlipBackdrop над проектом (axis уже проверена).
func FlipBackdrop(p *model.Project, spreadID, scope, axis string) {
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, _ *model.Spread, b *model.Backdrop) {
		fr := b.FrameOrDefault()
		if fr.R != 0 { // наклон меняет знак: горизонт остаётся ровным (как design.SetFlip)
			fr.R = -fr.R
		}
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
	changeBackdrop(p, spreadID, scope, func(_ *model.Project, _ *model.Spread, b *model.Backdrop) { b.Fade = fade })
}

// FitBackdrop — тело Editor.FitBackdrop над проектом.
func FitBackdrop(p *model.Project, spreadID, scope string) {
	changeBackdrop(p, spreadID, scope, func(p *model.Project, s *model.Spread, b *model.Backdrop) { b.FitArea(p.BdArea(s, scope)) })
}

// ClearBackdrop — тело Editor.ClearBackdrop над проектом; неизвестная область — холостая правка.
func ClearBackdrop(p *model.Project, spreadID, scope string) {
	if s := p.Sheet(spreadID); s != nil {
		if slot := s.Slot(scope); slot != nil {
			*slot = nil
		}
	}
}

// Rescale — поменялся формат страницы: рамки остаются на том же месте
// страницы пропорционально (как места), затем зажим. Только развороты: обложку
// пересчитывает свой Reflow (032 §3).
func Rescale(p *model.Project, before *model.Project) {
	og, ng := before.Geo(nil), p.Geo(nil)
	ow, oh, nw, nh := og.W, og.H, ng.W, ng.H
	if ow == nw && oh == nh || !(ow > 0) || !(oh > 0) {
		return
	}
	kx, ky := nw/ow, nh/oh
	for _, s := range p.Spreads {
		for _, scope := range model.BdScopes() {
			slot := s.Slot(scope)
			if b := *slot; b != nil {
				b.X, b.W, b.Y, b.H = b.X*kx, b.W*kx, b.Y*ky, b.H*ky
				*slot = p.ClampBackdrop(s, b, scope)
			}
		}
	}
}
