package design

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Подгон кадров по лицам (спека 130, design §5). Тело правки над *model.Project: лица
// приходят готовыми (model.FaceHit), детектор и кэш — в App. Правит только PhotoRef.Frame.

// Режимы вопроса о правленых кадрах: "" — спросить, keep — оставить, all — подогнать и их.
const (
	FacesAsk  = ""
	FacesKeep = "keep"
	FacesAll  = "all"
)

// Пределы нормы «как это лицо»: доли образца вне них — случайный кадр (лицо вне окна), не вкус.
const (
	minNormSize = 0.05
	maxNormSize = 1.0
)

// BaseFaceNorm — норма листа: из памяти сессии, иначе начальная 0,35 / 0,32 (решение 0.27-130-2).
// Негодная (не число, размер ≤ 0) считается отсутствующей.
func BaseFaceNorm(norm *geom.FaceNorm) geom.FaceNorm {
	if norm != nil && !math.IsNaN(norm.Size+norm.Eye) && !math.IsInf(norm.Size+norm.Eye, 0) && norm.Size > 0 {
		return *norm
	}
	return geom.DefaultFaceNorm
}

// NormFrom — норма по образцу: доли лица в окне места pl при его текущем кадре. Второй ответ —
// код ошибки образца (model.Ref*), "" — норма годится. Образец не меняется. Фото или четверти,
// сменившиеся с поиска, — ошибка (рамка относится к другой картинке); кадр сменился — берётся
// текущий: лицо в долях кадра от него не зависит.
func NormFrom(p *model.Project, pl *model.Place, h model.FaceHit) (geom.FaceNorm, string) {
	if pl == nil || pl.Photo == "" {
		return geom.FaceNorm{}, model.RefNoPlace
	}
	switch {
	case pl.Fit == model.FitContain:
		return geom.FaceNorm{}, model.RefContain
	case h.Missing:
		return geom.FaceNorm{}, model.RefNoFile
	}
	cur := p.Photos[pl.Photo].FrameOrDefault()
	if turn, _ := model.SplitR(cur.R); h.Photo != pl.Photo || h.Turn != turn {
		return geom.FaceNorm{}, model.RefChanged
	}
	switch {
	case h.Box.H <= 0 || h.Box.W <= 0:
		return geom.FaceNorm{}, model.RefNoFace
	case h.Many:
		return geom.FaceNorm{}, model.RefMany
	}
	win, o := p.BorderWindow(pl)
	r := p.Photos[pl.Photo]
	n, ok := geom.FaceShare(win, o, p.RawAspect(pl.Photo), cropOf(cur), r.FlipH, r.FlipV, h.Box, h.Eye)
	if !ok {
		return geom.FaceNorm{}, model.RefNoFace
	}
	n.Size = geom.ClampF(n.Size, minNormSize, maxNormSize)
	n.Eye = geom.ClampF(n.Eye, 0, 1)
	return n, ""
}

// FaceSlots — снимок мест листа spreadID для поиска лиц: места с фото, кроме группового (образец
// «как это лицо» тоже входит): место, фото, четверти и кадр на сейчас, лица ещё не найдены.
// false — листа нет. Только чтение.
func FaceSlots(p *model.Project, spreadID string) ([]model.FaceHit, bool) {
	s := p.Sheet(spreadID)
	if s == nil {
		return nil, false
	}
	out := make([]model.FaceHit, 0, len(s.Grid))
	for _, pl := range s.Grid {
		if pl.Photo == "" || model.RoleOf(pl) == model.RoleGroup {
			continue
		}
		f := p.Photos[pl.Photo].FrameOrDefault()
		turn, _ := model.SplitR(f.R)
		out = append(out, model.FaceHit{Place: pl.ID, Photo: pl.Photo, Turn: turn, Frame: f})
	}
	return out, true
}

// FaceWindow — окно места с фото photoID на листе spreadID для силуэта под лицо (silhouette.md):
// размеры окна рамочки в мм, как их видит FaceCrop. Отбор тот же, что у подгона: лист — виньетка,
// место с этим фото, не групповое и не «Целиком». ok = false — силуэта нет. Только чтение.
func FaceWindow(p *model.Project, spreadID, photoID string) (w, h float64, ok bool) {
	s := p.Sheet(spreadID)
	if s == nil || !s.IsVignette() || photoID == "" {
		return 0, 0, false
	}
	for _, pl := range s.Grid {
		if pl.Photo != photoID {
			continue
		}
		if model.RoleOf(pl) == model.RoleGroup || pl.Fit == model.FitContain {
			return 0, 0, false
		}
		win, _ := p.BorderWindow(pl)
		return win.W, win.H, win.W > 0 && win.H > 0
	}
	return 0, 0, false
}

// FitFaces ставит кадры фото листа spreadID по лицам hits: лица почти одного размера, глаза на
// одной линии, по центру окна рамочки места. Групповое, «Целиком», пустые места и образец ref
// пропускаются. Норма: образец ref (NormFrom), иначе norm (память листа), иначе начальная; вторым
// ответом — норма, по которой считали. fitted — кадры, что поставил прошлый подгон (фото → кадр):
// такой кадр не «правленый». Правленый кадр — S/X/Y не по умолчанию, лицо под ним мимо нормы
// дальше порогов геометрии, кадр не из fitted; mode решает его судьбу (FacesAsk — правки нет,
// Asked). Место, у которого фото, четверти или кадр не те, что в снимке, не трогается (Changed).
// Ошибка образца — RefErr, правки нет. Кадры, что поставлены, — в Report.Set.
func FitFaces(p *model.Project, spreadID string, hits []model.FaceHit, ref, mode string,
	norm *geom.FaceNorm, fitted map[string]model.Frame) (model.FaceReport, geom.FaceNorm) {
	rep := model.NewFaceReport()
	n := BaseFaceNorm(norm)
	s := p.Sheet(spreadID)
	if s == nil || len(s.Grid) == 0 {
		return rep, n
	}
	byPlace := make(map[string]model.FaceHit, len(hits))
	for _, h := range hits {
		byPlace[h.Place] = h
	}
	sample := ""
	if ref != "" {
		rp := s.PlaceRef(ref)
		var h model.FaceHit
		if rp != nil {
			sample, h = rp.ID, byPlace[rp.ID]
		}
		rn, code := NormFrom(p, rp, h)
		if code != "" {
			rep.RefErr = code
			return rep, n
		}
		n = rn
		// Кадр образца — «наш» для следующего нажатия: лицо на нём по X не обязательно в центре
		// окна, и без этого кнопка спросила бы про образец как про правленый кадр.
		rep.Set = map[string]model.Frame{rp.Photo: p.Photos[rp.Photo].FrameOrDefault()}
	}
	return fitPlaces(p, s, byPlace, sample, mode, n, fitted, rep), n
}

// cropOf — кадр модели как кадр геометрии.
func cropOf(f model.Frame) geom.Crop { return geom.Crop{S: f.S, X: f.X, Y: f.Y, R: f.R} }

// faceItem — место, которое будет подогнано (лицо есть), и признак «правленый».
type faceItem struct {
	pl     *model.Place
	h      model.FaceHit
	manual bool
}

// fitPlaces — проход по местам листа: причины «не трогаем», вопрос о правленых, подгон.
func fitPlaces(p *model.Project, s *model.Spread, byPlace map[string]model.FaceHit, sample, mode string,
	n geom.FaceNorm, fitted map[string]model.Frame, rep model.FaceReport) model.FaceReport {
	var work []faceItem
	asked := 0
	for _, pl := range s.Grid {
		if pl.Photo == "" || pl.ID == sample || model.RoleOf(pl) == model.RoleGroup {
			continue
		}
		r := p.Photos[pl.Photo]
		cur := r.FrameOrDefault()
		h, ok := byPlace[pl.ID]
		turn, _ := model.SplitR(cur.R)
		switch {
		case pl.Fit == model.FitContain:
			rep.Contain++
		case !ok || h.Photo != pl.Photo || h.Turn != turn || h.Frame != cur:
			rep.Changed++ // фото, четверти или кадр сменили, пока шёл поиск
		case h.Missing:
			rep.Missing++
			rep.Check = append(rep.Check, pl.ID)
		case h.Box.H <= 0 || h.Box.W <= 0:
			rep.NoFace++
			rep.Check = append(rep.Check, pl.ID)
		default:
			win, o := p.BorderWindow(pl)
			miss := geom.FaceMissAt(win, o, p.RawAspect(pl.Photo), cropOf(cur), r.FlipH, r.FlipV, h.Box, h.Eye, n)
			if miss.Bad {
				rep.NoFace++
				rep.Check = append(rep.Check, pl.ID)
				continue
			}
			it := faceItem{pl: pl, h: h, manual: isManual(cur, miss, fitted[pl.Photo])}
			if it.manual {
				asked++
			}
			work = append(work, it)
		}
	}
	if asked > 0 && mode != FacesKeep && mode != FacesAll {
		out := model.NewFaceReport()
		out.Asked, out.Manual = true, asked
		return out
	}
	for _, it := range work {
		if it.manual && mode == FacesKeep {
			rep.Manual++
			continue
		}
		fitPlace(p, it, n, &rep)
	}
	return rep
}

// isManual — кадр правлен вручную: S/X/Y не по умолчанию, лицо под ним мимо нормы дальше
// порогов (FaceMiss.Flag) и кадр не тот, что поставил прошлый подгон. Поворот, наклон и
// зеркало не в счёт: они не кадр.
func isManual(cur model.Frame, miss geom.FaceMiss, was model.Frame) bool {
	if cur.S == 1 && cur.X == 0 && cur.Y == 0 {
		return false
	}
	if was.S == cur.S && was.X == cur.X && was.Y == cur.Y { // пустой was: S = 0, кадру не равен
		return false
	}
	return miss.Flag() != geom.FaceOK
}

// fitPlace ставит кадр места по лицу и относит место к одной группе отчёта.
func fitPlace(p *model.Project, it faceItem, n geom.FaceNorm, rep *model.FaceReport) {
	pl, h := it.pl, it.h
	r := p.Photos[pl.Photo]
	win, o := p.BorderWindow(pl)
	crop, miss := geom.FaceCrop(win, o, p.RawAspect(pl.Photo), cropOf(r.FrameOrDefault()), r.FlipH, r.FlipV, h.Box, h.Eye, n)
	// miss.Bad здесь не бывает: тот же вход уже прошёл FaceMissAt в fitPlaces.
	f := model.Frame{S: crop.S, X: crop.X, Y: crop.Y, R: crop.R}
	r.Frame = model.ClampFrame(&f)
	p.Photos[pl.Photo] = r
	if rep.Set == nil {
		rep.Set = map[string]model.Frame{}
	}
	rep.Set[pl.Photo] = r.FrameOrDefault()
	switch {
	case h.Many: // кадр по самому крупному лицу, но человеку стоит взглянуть
		rep.Many++
		rep.Check = append(rep.Check, pl.ID)
	case miss.Small():
		rep.Small++
		rep.Short = append(rep.Short, pl.ID)
	case miss.Big():
		rep.Big++
		rep.Short = append(rep.Short, pl.ID)
	case miss.Edge():
		rep.Edge++
		rep.Short = append(rep.Short, pl.ID)
	default:
		rep.Fitted++
	}
}
