package album

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Обёртки области grid (023 Ш4): тело ≤ 3 операторов, сама правка — в пакете grid
// над *Project. «Договор» у каждой — что проверяется здесь до правки; остальное
// (неизвестный разворот, пределы мест, NaN в местах) проверяет операция внутри
// change/try, поэтому откат и холостая правка получаются даром.

// SetGrid. Договор: до правки ничего (неизвестный разворот — холостая).
func (e *Editor) SetGrid(spreadID string, on bool) model.View {
	return e.change(func(p *model.Project) { grid.SetGrid(p, spreadID, on) })
}

// AddPlace. Договор: до правки ничего; лимит мест — ошибка try, откат без шага.
func (e *Editor) AddPlace(spreadID string, page int) (model.View, error) {
	return e.try("", func(p *model.Project) error { return grid.AddPlace(p, spreadID, page) })
}

// SetPlaces. Договор: у «Авто» merge сбрасывается до правки (закрепление не должно
// влипнуть в чужую серию); чтение — под замком.
func (e *Editor) SetPlaces(spreadID string, boxes []model.PlaceBox, merge string) model.View {
	e.read(func(p *model.Project) {
		if grid.GridIsAuto(p, spreadID) {
			merge = ""
		}
	})
	return e.apply(model.GridMerge(merge), func(p *model.Project) { grid.SetPlaces(p, spreadID, boxes) })
}

// DeletePlaces. Договор: до правки ничего (у «Авто» и без мест — холостая).
func (e *Editor) DeletePlaces(spreadID string, ids []string) model.View {
	return e.change(func(p *model.Project) { grid.DeletePlaces(p, spreadID, ids) })
}

// SetPlaceFit. Договор: до правки ничего; режим вне ""/FitContain — холостая внутри.
func (e *Editor) SetPlaceFit(spreadID string, ids []string, fit string) model.View {
	return e.change(func(p *model.Project) { grid.SetPlaceFit(p, spreadID, ids, fit) })
}

// ArrangePlaces. Договор: до правки ничего; неизвестный op — холостая, слишком
// мелкое деление и лимит мест — ошибка try.
func (e *Editor) ArrangePlaces(spreadID, op string, ids []string) (model.View, error) {
	return e.try("", func(p *model.Project) error { return grid.ArrangePlaces(p, spreadID, op, ids) })
}

// MirrorGrid. Договор: до правки ничего; симметрия, нет мест, чужая область — холостая внутри.
func (e *Editor) MirrorGrid(spreadID, scope string) model.View {
	return e.change(func(p *model.Project) { grid.MirrorGrid(p, spreadID, scope) })
}

// PlacePhotos. Договор: до правки ничего; неизвестное место и пустой список — холостая.
func (e *Editor) PlacePhotos(spreadID, placeID string, photos []model.PhotoInfo) (model.View, int, error) {
	return e.gridTry(func(p *model.Project) (int, error) { return grid.PlacePhotos(p, spreadID, placeID, photos) })
}

// gridTry — try для операций, что возвращают left (фото, ушедшие в ленту): при
// отказе правки left = 0.
func (e *Editor) gridTry(fn func(p *model.Project) (int, error)) (v model.View, left int, err error) {
	v, err = e.try("", func(p *model.Project) (err error) {
		left, err = fn(p)
		return err
	})
	if err != nil {
		left = 0
	}
	return v, left, err
}

// FillPlaces. Договор: до правки ничего; фото из альбома и с фонов пропускаются внутри.
func (e *Editor) FillPlaces(photos []model.PhotoInfo, fromSpread string) (model.View, int) {
	left := 0
	v := e.change(func(p *model.Project) { left = grid.FillPlaces(p, photos, fromSpread) })
	return v, left
}

// ApplyGrid. Договор: доли зажимаются до правки (NaN или пусто — отказ без шага
// отмены и rev, View текущий).
func (e *Editor) ApplyGrid(spreadID, scope, tplScope string, frac []model.Box) (model.View, int, error) {
	frac, err := model.ClampBoxesStrict(frac)
	if err != nil {
		return e.View(), 0, err
	}
	return e.gridTry(func(p *model.Project) (int, error) { return grid.ApplyGrid(p, spreadID, scope, tplScope, frac) })
}

// CopyGrid. Договор: до правки ничего; область, цели и мест нет — ошибка try.
func (e *Editor) CopyGrid(src, from string, targets []model.GridTarget, design, mirror bool) (model.View, int, error) {
	return e.gridTry(func(p *model.Project) (int, error) {
		plan, err := grid.CopyGrid(p, src, from, targets, design, mirror)
		return plan.Left, err
	})
}

// PlanCopyGrid. Договор: под замком только копия проекта — сухой прогон по всему
// альбому не держит View и автосохранение; снимка, события и rev нет.
func (e *Editor) PlanCopyGrid(src, from string, targets []model.GridTarget, design, mirror bool) (plan model.GridPlan) {
	var c *model.Project
	e.read(func(p *model.Project) { c = p.Clone() })
	return grid.PlanCopyGrid(c, src, from, targets, design, mirror)
}

// DuplicateGrid. Договор: count < 1 — холостая до правки (текущий View).
func (e *Editor) DuplicateGrid(src string, count int, design bool) (model.View, error) {
	if count < 1 {
		return e.View(), nil
	}
	return e.try("", func(p *model.Project) error { return grid.DuplicateGrid(p, src, count, design) })
}

// SetPlaceAngle. Договор: NaN/Inf угла — холостая до правки (текущий View).
func (e *Editor) SetPlaceAngle(spreadID, ref string, deg float64) model.View {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return e.View()
	}
	return e.change(func(p *model.Project) { grid.SetPlaceAngle(p, spreadID, ref, deg) })
}

// PlaceBoxes. Договор: только чтение под замком; область и разворот проверяет grid.
func (e *Editor) PlaceBoxes(spreadID, scope string) (boxes []model.Box, tplScope string, err error) {
	e.read(func(p *model.Project) { boxes, tplScope, err = grid.PlaceBoxes(p, spreadID, scope) })
	return boxes, tplScope, err
}

// GridChoices. Договор: снимок входа под замком (GridInput), layout считается вне него.
func (e *Editor) GridChoices(spreadID string, n int, scope string, tpls []model.Template) model.GridChoices {
	var in grid.GridSource
	e.read(func(p *model.Project) { in = grid.GridInput(p, spreadID, n, scope) })
	return grid.GridChoicesOf(in, scope, tpls)
}
