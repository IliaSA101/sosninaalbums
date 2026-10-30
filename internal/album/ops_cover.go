package album

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Обёртки раскладок сторонки обложки (127): тела — grid/coverside.go. Editor добавляет память
// последней формы coverLast: свой шаблон или встроенная, поставленная ▦, узнаётся для ‹ › и после
// замены фото (grid.CoverSideStates ищет форму в «По фото», памяти и встроенных).

// rememberCover запоминает форму shape (доли зоны) сторонки scope; зовётся внутри правки, под e.mu,
// после удачного тела. Нет обложки или сторонки — память не пишется. Откат правки (settle) память
// не откатывает: она безвредна, узнавание требует равенства с местами.
func (e *Editor) rememberCover(p *model.Project, scope string, shape []model.Box) {
	if i, ok := grid.CoverSideIndex(scope); ok && p.Cover != nil && p.Cover.Sheet != nil {
		e.coverLast[i] = slices.Clone(shape) // не делим срез с вызывающим
	}
}

// ApplyCoverGrid. Договор: доли зажимаются до правки (NaN или пусто — отказ без шага отмены и rev,
// View текущий); один шаг отмены; та же форма — холостая; не left/right, нет зоны, сверх MaxPlaces —
// ошибка try, откат без шага. left — фото, ушедшие в ленту.
func (e *Editor) ApplyCoverGrid(scope string, frac []model.Box) (model.View, int, error) {
	frac, err := model.ClampBoxesStrict(frac)
	if err != nil {
		return e.View(), 0, err
	}
	return e.gridTry(func(p *model.Project) (int, error) {
		left, err := grid.ApplyCoverGrid(p, scope, frac)
		if err == nil {
			e.rememberCover(p, scope, frac)
		}
		return left, err
	})
}

// StepCoverGrid. Договор: серия ‹ › по сторонке — один шаг отмены (ключ "covergrid:"+scope); форма не
// узнана, в ряду одна раскладка, нет сторонки, delta 0 — холостая (UI решает по Known и Alone).
// Ошибка тела (не left/right) откатывает правку: вид прежний.
func (e *Editor) StepCoverGrid(scope string, delta int) model.View {
	v, _ := e.try("covergrid:"+scope, func(p *model.Project) error {
		shape, err := grid.StepCoverGrid(p, scope, delta, e.coverLast)
		if err == nil && shape != nil {
			e.rememberCover(p, scope, shape)
		}
		return err
	})
	return v
}

// CoverGridChoices. Договор: снимок входа (CoverInput) под замком, layout считается вне него;
// правки, rev и события нет.
func (e *Editor) CoverGridChoices(scope string, n int, tpls []model.Template) model.CoverGridChoices {
	var in grid.CoverSource
	e.read(func(p *model.Project) { in = grid.CoverInput(p, scope) })
	return grid.CoverGridChoicesOf(in, n, tpls)
}
