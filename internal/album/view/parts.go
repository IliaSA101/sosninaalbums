package view

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// gridView заполняет вид разворота-сетки: фото — по местам (Rects[i] — место
// фото i), кадр cover или contain; ряда раскладок нет. Поэтому Cell, кадр,
// подпись и «!» по dpi рисуют сетку без изменений.
func gridView(f *fields, sv *model.SpreadView, s *model.Spread) {
	p := f.p
	sv.Grid, sv.Layout, sv.Variant = true, "", -1
	sv.Photos = model.GridPhotos(s)
	sv.Places = make([]model.PlaceView, len(s.Grid))
	flat := p.Geo(s).Layflat // обложка — всегда: место через корешок законно
	for i, pl := range s.Grid {
		sv.Places[i] = model.PlaceView{ID: pl.ID, X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Photo: pl.Photo, Fit: pl.Fit,
			Spine: !flat && p.CrossesSpine(s, pl), Rot: pl.Rot, Role: pl.Role}
		if pl.Photo == "" {
			continue
		}
		r := layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}
		ref := p.Photos[pl.Photo]
		a, f := p.RawAspect(pl.Photo), ref.FrameOrDefault()
		var fv model.FrameView
		if pl.Fit == model.FitContain {
			// Целиком, без S/X/Y: ползунка у такого места нет; поворот фото действует.
			fv = newFrameView(geom.ContainImage(r, a, f.R), model.Frame{S: 1, R: f.R})
		} else {
			fv = newFrameView(model.PlaceImage(r, a, f), f)
		}
		fv.FlipH, fv.FlipV, fv.CellRot = ref.FlipH, ref.FlipV, pl.Rot
		fv.DPI = geom.FrameDPI(ref.W, fv.Img)
		sv.Rects = append(sv.Rects, r)
		sv.Frames = append(sv.Frames, fv)
	}
	sv.Captions, sv.Stubs = captionViews(f, s, sv.Photos, sv.Rects)
	for i := range sv.Captions {
		if pl := s.PlaceRef(sv.Captions[i].Photo); pl != nil {
			turnCaption(&sv.Captions[i], pl)
		}
	}
	for i := range sv.Stubs {
		if pl := s.PlaceRef(sv.Stubs[i].Photo); pl != nil {
			cv := model.CaptionView{TextView: sv.Stubs[i].TextView}
			turnCaption(&cv, pl)
			sv.Stubs[i].Place, sv.Stubs[i].TextView = pl.ID, cv.TextView
		}
	}
	placeStubs(f, sv, s)
}

// turnCaption — подпись повёрнутого места pl: посчитана для неповёрнутой рамки,
// затем её угол (и строки с ним) поворачивается вокруг центра места, Rot = углу
// места. Overflow — 0: соседи не в осях места, «не помещается» не определить.
func turnCaption(cv *model.CaptionView, pl *model.Place) {
	if pl.Rot == 0 {
		return
	}
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	dx, dy := geom.Rotate(pl.Rot, cv.X-cx, cv.Y-cy)
	mx, my := cx+dx-cv.X, cy+dy-cv.Y
	for k := range cv.Lines {
		cv.Lines[k].X += mx
		cv.Lines[k].Y += my
	}
	cv.X, cv.Y, cv.Rot, cv.Overflow = cv.X+mx, cv.Y+my, pl.Rot, 0
}

// zView — Z у ячеек, мест, подписей, заглушек и текстов: проход стека снизу вверх с 1,
// подпись и заглушка — сразу над своим местом. «Авто» — ячейки по порядку, затем тексты.
// Подпись нижнего места, легшая на верхнее, теперь под ним (в 0.4.0 подписи
// лежали над всеми фото).
func zView(sv *model.SpreadView, s *model.Spread) {
	capOf := make(map[string]int, len(sv.Captions))
	for i, c := range sv.Captions {
		capOf[c.Photo] = i
	}
	stubOf := make(map[string]int, len(sv.Stubs)) // ключ — место в сетке, фото в «Авто»
	for i, st := range sv.Stubs {
		if sv.Grid {
			stubOf[st.Place] = i
		} else {
			stubOf[st.Photo] = i
		}
	}
	z := 0
	cell := func(photo, key string) int {
		z++
		at := z
		if i, ok := capOf[photo]; ok && photo != "" {
			z++
			sv.Captions[i].Z = z
		}
		if i, ok := stubOf[key]; ok && key != "" {
			z++
			sv.Stubs[i].Z = z
		}
		return at
	}
	textAt := make(map[string]int, len(sv.Texts))
	for i, t := range sv.Texts {
		textAt[t.ID] = i
	}
	if !sv.Grid {
		for i := range sv.Frames {
			sv.Frames[i].Z = cell(sv.Photos[i], sv.Photos[i])
		}
		for i := range sv.Texts {
			z++
			sv.Texts[i].Z = z
		}
		return
	}
	placeAt := make(map[string]int, len(sv.Places))
	for i, pv := range sv.Places {
		placeAt[pv.ID] = i
	}
	frameOf := make(map[string]int, len(sv.Frames)) // в сетке Frames[i] — фото Photos[i]
	for i := range sv.Frames {
		frameOf[sv.Photos[i]] = i
	}
	for _, id := range s.Stack() {
		if i, ok := placeAt[id]; ok {
			pv := &sv.Places[i]
			pv.Z = cell(pv.Photo, pv.ID)
			if f, ok := frameOf[pv.Photo]; ok && pv.Photo != "" {
				sv.Frames[f].Z = pv.Z
			}
		} else if i, ok := textAt[id]; ok {
			z++
			sv.Texts[i].Z = z
		}
	}
}

// backdropViews — фоны разворота с рамкой и фото в осях рамки; dpi считается по ширине
// фото в пикселях и размеру Img (входит в счёт lowDPI, 028 В).
func backdropViews(p *model.Project, s *model.Spread) []model.BackdropView {
	out := []model.BackdropView{}
	for _, scope := range model.BdScopes() {
		b := *s.Slot(scope)
		if b == nil {
			continue
		}
		ref := p.Photos[b.Photo]
		fr := b.FrameOrDefault()
		img := model.PlaceImage(layout.Rect{W: b.W, H: b.H}, p.RawAspect(b.Photo), fr)
		out = append(out, model.BackdropView{Scope: scope, Photo: b.Photo, Area: p.BdArea(s, scope),
			Box: model.BackdropBox{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}, Img: img, Frame: fr,
			FlipH: b.FlipH, FlipV: b.FlipV, DPI: geom.FrameDPI(ref.W, img), Fade: b.Fade})
	}
	return out
}

// frameViews — фото целиком для каждой ячейки по кадру из PhotoRef и эффективный dpi.
// Геометрия фото — по пропорциям файла (rawAspect): ячейку под повёрнутое на 90° фото
// уже подогнала раскладка.
func frameViews(p *model.Project, ids []string, rects []layout.Rect) []model.FrameView {
	out := make([]model.FrameView, len(rects))
	for i, r := range rects {
		ref := p.Photos[ids[i]]
		f := ref.FrameOrDefault()
		img := model.PlaceImage(r, p.RawAspect(ids[i]), f)
		out[i] = newFrameView(img, f)
		out[i].DPI, out[i].FlipH, out[i].FlipV = geom.FrameDPI(ref.W, img), ref.FlipH, ref.FlipV
	}
	return out
}

// newFrameView — вид кадра f: Img и поворот, разложенный на четверти и наклон.
func newFrameView(img layout.Rect, f model.Frame) model.FrameView {
	turn, tilt := model.SplitR(f.R)
	return model.FrameView{Img: img, S: f.S, X: f.X, Y: f.Y, R: f.R, Turn: turn, Tilt: tilt}
}

// textViews — блоки разворота с рамкой и строками (переносы — из кэша text); поля
// подставлены (030), исходник — в Src.
func textViews(f *fields, s *model.Spread) []model.TextView {
	out := make([]model.TextView, len(s.Texts))
	for i, t := range s.Texts {
		shown, src := f.text(t.Text)
		out[i] = placeText(t.ID, shown, t.Style, t.X, t.Y, t.W)
		out[i].Rot, out[i].Auto, out[i].Src = t.Rot, t.Auto, src
	}
	return out
}

// placeText — рамка и строки текста с левым верхним углом (x, y) и шириной w, мм разворота.
func placeText(id, s string, st model.TextStyle, x, y, w float64) model.TextView {
	b := text.Cached(s, st, w)
	for i := range b.Lines {
		b.Lines[i].X += x
		b.Lines[i].Y += y
	}
	if b.Lines == nil {
		b.Lines = []text.Line{}
	}
	return model.TextView{ID: id, X: x, Y: y, W: w, H: b.H, Text: s, Style: st, Lines: b.Lines}
}

// pageBg — действующий цвет страниц: свой, иначе цвет альбома, иначе белый.
func pageBg(p *model.Project, s *model.Spread) [2]string {
	album := p.Settings.Bg
	if album == "" {
		album = defaultBg
	}
	out := [2]string{album, album}
	for i, c := range []string{s.BgL, s.BgR} {
		if c != "" {
			out[i] = c
		}
	}
	return out
}
