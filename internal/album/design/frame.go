package design

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Кадр фото, зеркало, наклон и цвет страниц разворота (спека 006). Операция —
// функция над *model.Project; метод Editor — обёртка в ядре (ops_design.go, спека 023).

// SetFrame ставит кадр фото: s 1..4, x/y −1..1 (зажимается, NaN — по умолчанию).
// s=1, x=y=0 — сброс. Поворот фото R остаётся: он, как и зеркало, не кадр. Каждый
// вызов — шаг отмены.
func SetFrame(p *model.Project, photoID string, s, x, y float64) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	r.Frame = model.ClampFrame(&model.Frame{S: s, X: x, Y: y, R: r.FrameOrDefault().R})
	p.Photos[photoID] = r
}

// SetFrameAngle — наклон фото листа spreadID в рамке tilt (градусы, −44,9…44,9 при текущих
// четвертях; шаг 0.1) от кадра base (начало сессии жеста; nil — текущий). Наклон держит
// приближение (057, geom.TiltCrop): S, X, Y считаются от base, у «Целиком» меняется только
// R. Пропорции стоят, «Авто» не закрепляется. Фото не на листе и тот же наклон — холостая.
func SetFrameAngle(p *model.Project, spreadID, photoID string, tilt float64, base *model.Frame) {
	s := p.Sheet(spreadID)
	r, ok := p.Photos[photoID]
	if s == nil || !ok || !slices.Contains(s.Photos, photoID) {
		return
	}
	cur := r.FrameOrDefault()
	turn, _ := model.SplitR(cur.R)
	b := cur
	if base != nil {
		if t0, _ := model.SplitR(base.R); t0 == turn { // четверти сменились мимо сессии — от текущего
			b = *base
		}
	}
	f := cur
	f.R = model.ClampR(float64(90*turn) + geom.ClampF(model.Round1(tilt), -model.MaxTilt, model.MaxTilt))
	if cell, o, ok := tiltCell(p, s, photoID); ok {
		c := geom.TiltCrop(cell, p.RawAspect(photoID), geom.Crop{S: b.S, X: b.X, Y: b.Y, R: b.R}, f.R, o)
		f = model.Frame{S: c.S, X: c.X, Y: c.Y, R: c.R}
	}
	r.Frame = model.ClampFrame(&f)
	p.Photos[photoID] = r
}

// tiltCell — место фото для правила наклона (057, решение 12): сетка и обложка — рамка места
// и его форма, «Авто» — ячейка действующей раскладки (прямоугольник). «Целиком» и фото без
// места — ok = false: правило к ним не относится, меняется только R.
func tiltCell(p *model.Project, s *model.Spread, photoID string) (layout.Rect, geom.Outline, bool) {
	if len(s.Grid) > 0 {
		pl := s.PlaceRef(photoID)
		if pl == nil || pl.Fit == model.FitContain {
			return layout.Rect{}, geom.Outline{}, false
		}
		return layout.Rect{W: pl.W, H: pl.H}, pl.Outline(), true
	}
	res, _, ok := p.Layout(s)
	i := slices.Index(s.Photos, photoID)
	if !ok || i < 0 || i >= len(res.Rects) {
		return layout.Rect{}, geom.Outline{}, false
	}
	return res.Rects[i], geom.Outline{}, true
}

// BaseFrame — кадр фото photoID в снимке p0 (начало сессии наклона, 057). Снимка нет или
// фото в нём нет — nil: база — текущий кадр. Кадр по умолчанию — не nil, а {S: 1}.
func BaseFrame(p0 *model.Project, photoID string) *model.Frame {
	if p0 == nil {
		return nil
	}
	r, ok := p0.Photos[photoID]
	if !ok {
		return nil
	}
	f := r.FrameOrDefault()
	return &f
}

// StraightenPhoto — «Выпрямить по линии» (045): линия (dx, dy) мм листа встаёт параллельно
// краю места (у «Авто» — краю листа). Четверти стоят, наклон упирается в ±44,9 — пропорции
// те же, «Авто» не закрепляется и не пересобирается; приближение держится, как у наклона
// (057, от текущего кадра). Фото не на листе, нет линии или она уже ровная — холостая правка.
func StraightenPhoto(p *model.Project, spreadID, photoID string, dx, dy float64) {
	s := p.Sheet(spreadID)
	if s == nil || !slices.Contains(s.Photos, photoID) {
		return
	}
	rot := 0.0
	if pl := s.PlaceRef(photoID); pl != nil {
		rot = pl.Rot
	}
	d, ok := geom.StraightenDelta(dx, dy, rot)
	if !ok {
		return
	}
	_, tilt := model.SplitR(p.Photos[photoID].FrameOrDefault().R)
	SetFrameAngle(p, spreadID, photoID, tilt+d, nil)
}

// TurnPhotos поворачивает фото ids на quarters·90° (quarters ∈ {−1, 1, 2}, иначе
// холостая правка), одной правкой. Наклон сохраняется; чужие и повторные ID
// пропускаются. Действующие пропорции меняются — раскладку «Авто» пересоберёт keepLayouts.
func TurnPhotos(p *model.Project, ids []string, quarters int) {
	done := map[string]bool{}
	for _, id := range ids {
		r, ok := p.Photos[id]
		if !ok || done[id] {
			continue
		}
		done[id] = true
		f := r.FrameOrDefault()
		f.R = float64(90*quarters) + f.R
		r.Frame = model.ClampFrame(&f)
		p.Photos[id] = r
	}
}

// ResetFrame — кадр по умолчанию; у нетронутого кадра это холостая правка.
func ResetFrame(p *model.Project, photoID string) { SetFrame(p, photoID, 1, 0, 0) }

// SetFlip зеркалит фото ids по оси axis ("h" | "v"): если включено не у всех — всем
// включается, иначе у всех выключается. Видимая часть кадра остаётся той же, только
// отражённой (X → −X / Y → −Y, наклон R → −R: горизонт остаётся ровным). Раскладку
// не трогает: «Авто» не закрепляется.
// Пустые и чужие ID пропускаются; нет ни одного или неверная ось — холостая правка.
func SetFlip(p *model.Project, ids []string, axis string) {
	field := func(r *model.PhotoRef) *bool {
		if axis == "h" {
			return &r.FlipH
		}
		return &r.FlipV
	}
	var known []string
	all := true
	for _, id := range ids {
		r, ok := p.Photos[id]
		if !ok || slices.Contains(known, id) {
			continue
		}
		known = append(known, id)
		all = all && *field(&r)
	}
	for _, id := range known {
		r := p.Photos[id]
		f := field(&r)
		if *f == !all {
			continue
		}
		*f = !all
		fr := r.FrameOrDefault()
		// Нулевой ход не трогаем: иначе в JSON попал бы «-0».
		if fr.R != 0 {
			fr.R = -fr.R
		}
		if axis == "h" && fr.X != 0 {
			fr.X = -fr.X
		} else if axis == "v" && fr.Y != 0 {
			fr.Y = -fr.Y
		}
		r.Frame = model.ClampFrame(&fr)
		p.Photos[id] = r
	}
}

// SetBackground — цвет страниц разворота: scope "spread" (обе), "left", "right".
// c — уже зажатый цвет (ClampColor); "" — снять свой (действует цвет альбома).
// Неверный цвет или scope — холостая правка; у листа 031 страниц нет — только "spread".
func SetBackground(p *model.Project, spreadID, scope, c string) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	if _, ok := p.AreaOf(s, scope); !ok {
		return
	}
	switch scope {
	case "spread":
		s.BgL, s.BgR = c, c
	case "left":
		s.BgL = c
	case "right":
		s.BgR = c
	}
}
