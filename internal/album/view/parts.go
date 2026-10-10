package view

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// gridView заполняет вид разворота-сетки: фото — по местам (Rects[i] — место
// фото i), кадр cover или contain; ряда раскладок нет. Поэтому Cell, кадр,
// подпись и «!» по dpi рисуют сетку без изменений.
func gridView(p *model.Project, sv *model.SpreadView, s *model.Spread) {
	sv.Grid, sv.Layout, sv.Variant = true, "", -1
	sv.Photos = model.GridPhotos(s)
	sv.Places = make([]model.PlaceView, len(s.Grid))
	for i, pl := range s.Grid {
		sv.Places[i] = model.PlaceView{ID: pl.ID, X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Photo: pl.Photo, Fit: pl.Fit,
			Spine: !p.Settings.Layflat && p.CrossesSpine(pl), Rot: pl.Rot}
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
	sv.Captions = captionViews(p, sv.Photos, sv.Rects)
	for i := range sv.Captions {
		if pl := s.PlaceRef(sv.Captions[i].Photo); pl != nil {
			turnCaption(&sv.Captions[i], pl)
		}
	}
}

const (
	// CaptionPad — зазор между фото и подписью, мм.
	CaptionPad = 1.0
	// trimZone — обрезная зона типографии у нижнего края страницы (fsfera 2–3 мм):
	// подпись в неё не заходит.
	trimZone = 3.0
)

// captionViews — подписи фото разворота в порядке ленты. Раскладку подпись не
// раздвигает: не хватило места — Overflow > 0, интерфейс показывает «!».
func captionViews(p *model.Project, ids []string, rects []layout.Rect) []model.CaptionView {
	out := []model.CaptionView{}
	for i := range rects {
		c := p.Photos[ids[i]].Caption
		if c == nil {
			continue
		}
		out = append(out, CaptionViewOf(ids[i], c, rects, i, p.Settings.PageH))
	}
	return out
}

// CaptionViewOf — место подписи к фото в ячейке rects[i] шириной в ячейку.
// Под фото: до края страницы без обрезной зоны и до ячейки, стоящей ниже
// с перекрытием по горизонтали. На фото: у нижнего края ячейки внутри неё.
// Свободные блоки текста препятствием не считаются.
func CaptionViewOf(id string, c *model.Caption, rects []layout.Rect, i int, pageH float64) model.CaptionView {
	r := rects[i]
	tv := placeText("", c.Text, c.Style, r.X, 0, r.W) // по вертикали сдвигается ниже
	h := tv.H
	var top, over float64
	if c.Over {
		top = r.Y + r.H - 3*CaptionPad - h
		over = r.Y + CaptionPad - top
	} else {
		top = r.Y + r.H + CaptionPad
		limit := pageH - trimZone
		for j, o := range rects {
			below := o.Y >= r.Y+r.H-1e-6
			if j != i && below && o.X < r.X+r.W && o.X+o.W > r.X {
				limit = math.Min(limit, o.Y-CaptionPad)
			}
		}
		over = top + h - limit
	}
	tv.Y = top
	for k := range tv.Lines {
		tv.Lines[k].Y += top
	}
	return model.CaptionView{
		Photo:    id,
		TextView: tv,
		Over:     c.Over,
		Overflow: math.Max(0, math.Round(over*10)/10),
	}
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

// zView — Z у ячеек, мест, подписей и текстов: проход стека снизу вверх с 1,
// подпись — сразу над своим фото. «Авто» — ячейки по порядку, затем тексты.
// Подпись нижнего места, легшая на верхнее, теперь под ним (в 0.4.0 подписи
// лежали над всеми фото).
func zView(sv *model.SpreadView, s *model.Spread) {
	capOf := make(map[string]int, len(sv.Captions))
	for i, c := range sv.Captions {
		capOf[c.Photo] = i
	}
	z := 0
	cell := func(photo string) int {
		z++
		at := z
		if i, ok := capOf[photo]; ok && photo != "" {
			z++
			sv.Captions[i].Z = z
		}
		return at
	}
	textAt := make(map[string]int, len(sv.Texts))
	for i, t := range sv.Texts {
		textAt[t.ID] = i
	}
	if !sv.Grid {
		for i := range sv.Frames {
			sv.Frames[i].Z = cell(sv.Photos[i])
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
			pv.Z = cell(pv.Photo)
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
// фото в пикселях и размеру Img (lowDPI фон не считает: размытый фон часто намеренный).
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
		out = append(out, model.BackdropView{Scope: scope, Photo: b.Photo, Area: p.BdArea(scope),
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

// textViews — блоки разворота с рамкой и строками (переносы — из кэша text).
func textViews(p *model.Project, s *model.Spread) []model.TextView {
	out := make([]model.TextView, len(s.Texts))
	for i, t := range s.Texts {
		out[i] = placeText(t.ID, t.Text, t.Style, t.X, t.Y, t.W)
		out[i].Rot, out[i].Auto = t.Rot, t.Auto
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
