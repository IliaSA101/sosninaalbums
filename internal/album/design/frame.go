package design

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
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

// SetFrameAngle — наклон фото в рамке tilt (градусы, −44,9…44,9 при текущих четвертях
// поворота; шаг 0.1). S, X, Y и пропорции стоят, «Авто» не закрепляется. NaN/Inf и
// тот же наклон — холостая правка.
func SetFrameAngle(p *model.Project, photoID string, tilt float64) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	f := r.FrameOrDefault()
	turn, _ := model.SplitR(f.R)
	f.R = model.ClampR(float64(90*turn) + geom.ClampF(model.Round1(tilt), -model.MaxTilt, model.MaxTilt))
	r.Frame = model.ClampFrame(&f)
	p.Photos[photoID] = r
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
// Неверный цвет или scope — холостая правка.
func SetBackground(p *model.Project, spreadID, scope, c string) {
	_, s := p.Find(spreadID)
	if s == nil {
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
