package view

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// ownURL — превью своего стикера: тот же /media/p/<id>, что у фото (путь — от искателя
// библиотеки, пропавшее отвечает 404 без чтения диска).
const ownURL = "/media/p/"

// stickerViews — стикеры листа (048 design §6) по порядку Spread.Stickers; Z ставит zView.
// Процедурный — как его нарисует gen (цвета дополнены, растяжимый — в пропорции рамки), свой —
// фото из Photos с dpi по ширине рамки (входит в счёт lowDPI, как фото фона).
func stickerViews(p *model.Project, s *model.Spread) []model.StickerView {
	out := make([]model.StickerView, 0, len(s.Stickers))
	for _, st := range s.Stickers {
		sv := model.StickerView{ID: st.ID, X: st.X, Y: st.Y, W: st.W, H: st.H, Rot: st.Rot,
			FlipH: st.FlipH, FlipV: st.FlipV, Fade: st.Fade, Photo: st.Photo}
		if a, ok := p.StickerAspect(st); ok {
			sv.Aspect = a
		}
		if st.Art != nil {
			artView(&sv, st)
		} else {
			ref := p.Photos[st.Photo]
			sv.Lib, sv.Known, sv.URL = ref.Lib, true, ownURL+st.Photo
			sv.DPI = geom.FrameDPI(ref.W, layout.Rect{W: st.W, H: st.H})
		}
		out = append(out, sv)
	}
	return out
}

// artView — процедурный стикер: Art после Resolve (что рисуется, то и в карточке); неизвестный
// генератор — Art как в файле, Known false, без имени и URL.
func artView(sv *model.StickerView, st *model.Sticker) {
	a := st.Art
	sv.Gen = true
	// Aspect рамки Clamp оставляет только растяжимой фигуре — у остальных он отбрасывается.
	spec, known := gen.Resolve(gen.Spec{Kind: a.Kind, Inks: a.Inks, Seed: a.Seed, Aspect: st.W / st.H})
	sv.Art = &model.Art{Kind: a.Kind, Inks: spec.Inks, Seed: spec.Seed}
	m, ok := gen.Lookup(a.Kind)
	if !known || !ok {
		sv.Art.Inks = append([]string(nil), a.Inks...)
		return
	}
	sv.Known, sv.Name, sv.Inks, sv.Stretch = true, m.Name, len(m.Inks), m.Stretch
	sv.URL = gen.Query(spec, 0, 0)
}
