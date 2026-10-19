package view

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
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
	flat := p.Geo(s).Layflat    // обложка — всегда: место через корешок законно
	var wins []layout.Rect      // окна рамочек мест с фото, рядом с Rects (подписи «на фото», 079)
	var outlines []geom.Outline // формы окон, рядом с Rects
	for i, pl := range s.Grid {
		o := pl.Outline()
		sv.Places[i] = model.PlaceView{ID: pl.ID, X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Photo: pl.Photo, Fit: pl.Fit,
			Spine: !flat && p.CrossesSpine(s, pl), Rot: pl.Rot, Role: pl.Role, Shape: pl.Shape, Round: pl.Round, Radius: o.Radius,
			Border: borderView(p, pl)}
		if pl.Photo == "" {
			continue
		}
		r := layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}
		// Фото садится в окно рамочки (079-3): кадр, «Целиком» и dpi — по окну; без рамочки окно = место.
		win, wo := p.BorderWindow(pl)
		ref := p.Photos[pl.Photo]
		a, f := p.RawAspect(pl.Photo), ref.FrameOrDefault()
		var fv model.FrameView
		if pl.Fit == model.FitContain {
			// Целиком, без S/X/Y: ползунка у такого места нет; поворот фото действует.
			fv = newFrameView(geom.ContainImageIn(win, a, f.R, wo), model.Frame{S: 1, R: f.R})
		} else {
			// Фото закрывает форму, а не рамку (044): у круга наклон его не увеличивает.
			fv = newFrameView(model.PlaceImageIn(win, a, f, wo), f)
		}
		fv.FlipH, fv.FlipV, fv.CellRot = ref.FlipH, ref.FlipV, pl.Rot
		toneView(&fv, ref)
		fv.Shape, fv.Radius = pl.Shape, o.Radius
		fv.Win, fv.WinRadius = win, wo.Radius
		if wo.Oval {
			fv.WinShape = model.ShapeOval
		}
		fv.DPI = geom.FrameDPI(ref.W, fv.Img)
		sv.Rects = append(sv.Rects, r)
		sv.Frames = append(sv.Frames, fv)
		wins, outlines = append(wins, win), append(outlines, wo)
	}
	sv.Captions, sv.Stubs = captionViews(f, s, sv.Photos, sv.Rects, wins, outlines)
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
	sv.Waiting = waiting(s) // и в печати: вопрос перед экспортом (071)
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

// zView — Z у ячеек, мест, подписей, заглушек, стикеров и текстов: проход стека снизу вверх
// с 1, подпись и заглушка — сразу над своим местом. «Авто» — ячейки по порядку, затем стек
// стикеров и текстов (Stack: мест в нём нет).
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
	stickAt := make(map[string]int, len(sv.Stickers))
	for i, st := range sv.Stickers {
		stickAt[st.ID] = i
	}
	placeAt := make(map[string]int, len(sv.Places))
	for i, pv := range sv.Places {
		placeAt[pv.ID] = i
	}
	frameOf := make(map[string]int, len(sv.Frames)) // в сетке Frames[i] — фото Photos[i]
	for i := range sv.Frames {
		if sv.Grid {
			frameOf[sv.Photos[i]] = i
		} else {
			sv.Frames[i].Z = cell(sv.Photos[i], sv.Photos[i])
		}
	}
	for _, id := range s.Stack() {
		if i, ok := placeAt[id]; ok {
			pv := &sv.Places[i]
			pv.Z = cell(pv.Photo, pv.ID)
			if f, ok := frameOf[pv.Photo]; ok && pv.Photo != "" {
				sv.Frames[f].Z = pv.Z
			}
		} else if i, ok := stickAt[id]; ok {
			z++
			sv.Stickers[i].Z = z
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
			FlipH: b.FlipH, FlipV: b.FlipV, DPI: geom.FrameDPI(ref.W, img), Fade: b.Fade, Lib: ref.Lib})
	}
	return out
}

// patternViews — узоры листа по областям (048): геометрия области — та же, что у фото фона,
// поэтому холст и экспорт кладут узор в одно место. Цвета и масштаб — как их нарисует gen.
func patternViews(p *model.Project, s *model.Spread) []model.PatternView {
	out := []model.PatternView{}
	for _, scope := range model.PatScopes() {
		pt := *s.PatSlot(scope)
		if pt == nil {
			continue
		}
		area := p.BdArea(s, scope)
		spec, known := gen.Resolve(gen.Spec{Kind: pt.Kind, Inks: pt.Inks, Scale: pt.Scale, Seed: pt.Seed})
		if spec.Scale == 0 { // неизвестный генератор Resolve не дополняет
			spec.Scale = 1
		}
		pv := model.PatternView{Scope: scope, Area: area, Known: known, Preset: pt.Preset,
			Pat: model.Pattern{Kind: pt.Kind, Inks: spec.Inks, Scale: spec.Scale, Seed: spec.Seed, Fade: pt.Fade}}
		if m, ok := gen.Lookup(pt.Kind); ok {
			pv.Name, pv.Inks, pv.Random = m.Name, len(m.Inks), m.Random
			pv.URL = gen.Query(spec, area.W, area.H)
		}
		out = append(out, pv)
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
		out[i].DPI, out[i].FlipH, out[i].FlipV, out[i].Win = geom.FrameDPI(ref.W, img), ref.FlipH, ref.FlipV, r
		toneView(&out[i], ref)
	}
	return out
}

// toneView — тон фото (068) в виде кадра: числа для карточки и строка CSS для <img>; и
// прозрачность (079). Одно место для «Авто» и сетки — иначе экран и экспорт разошлись бы на
// одном из путей.
func toneView(fv *model.FrameView, ref model.PhotoRef) {
	fv.Fade = ref.Fade
	if t := ref.Adj; t != nil {
		fv.Gray, fv.Bright, fv.Contrast, fv.Filter = t.Gray, t.Bright, t.Contrast, t.CSS()
	}
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
