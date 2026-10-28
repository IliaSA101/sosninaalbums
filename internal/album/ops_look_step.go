package album

import (
	"errors"
	"reflect"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Шаг макета на один лист и новый разворот по серии (131 design §4, G4b): «Поставить на разворот
// N», «Поставить на обложку», «Разворот макета» в меню «+». Каждое — одна e.try.

var (
	// ErrLookStep — в макете нет шага с таким номером.
	ErrLookStep = errors.New("в макете нет такого шага")
	// ErrLookCover — шаг 0 (оформление обложки) встаёт только на обложку, а шаги 1…N — только на
	// развороты.
	ErrLookCover = errors.New("оформление обложки ставится только на обложку, а шаги макета — на развороты")
	// ErrLookChanged — макет альбома сменился, пока App готовил разворот: шаг чужого макета в альбом не встаёт.
	ErrLookChanged = errors.New("макет альбома сменился — повторите")
	// ErrNoLook — альбом не оформлен макетом: следующий шаг серии назвать нельзя.
	ErrNoLook = errors.New("альбом не оформлен макетом")
)

// ApplyLookStep ставит шаг step (1…len(Steps) макета t) на лист spreadID целиком, как плитка 067:
// места по LookBoxes, рамочки, оформление, заготовки; фото — по порядку мест, лишние — в ленту (GridPlan.Left); надписи, которых нет в шаге, и
// название остаются. LookStep листа = step, если ref — макет этого альбома (Project.Look; у
// альбома без макета ref становится им), иначе 0; пустой ref — обычная плитка. missing — ключей
// библиотеки без файла на этом ПК. Одна правка; отказ — без шага; повтор — холостая; неизвестный
// лист — холостая. Обложка — spreadID = CoverID и step 0: оформление обложки макета (фон, места
// сторонок по LookBoxes в безопасной зоне, фото по порядку, лишние — в ленту; корешок не
// трогается); обложки нет — холостая, у макета нет обложки — ErrLookStep; шаг на обложку и шаг 0
// на разворот — ErrLookCover.
func (e *Editor) ApplyLookStep(spreadID string, step int, ref model.LookRef, t model.AlbumTpl, libs map[string]model.PhotoInfo) (v model.View, plan model.GridPlan, missing int, err error) {
	r, c, err := checkStep(ref, t)
	if err != nil {
		return e.View(), plan, 0, err
	}
	v, err = e.try("", func(p *model.Project) (err error) {
		plan, missing, err = applyLookStep(p, spreadID, step, r, c, libs)
		return err
	})
	if err != nil {
		plan, missing = model.GridPlan{}, 0
	}
	return v, plan, missing, err
}

// PlanLookStep — сухой прогон ApplyLookStep той же функцией на копии (вопрос до правки: фото в
// ленту, заменятся стикеры и надписи).
func (e *Editor) PlanLookStep(spreadID string, step int, ref model.LookRef, t model.AlbumTpl, libs map[string]model.PhotoInfo) (plan model.GridPlan, err error) {
	r, c, err := checkStep(ref, t)
	if err != nil {
		return plan, err
	}
	err = e.dryRun(func(cp *model.Project) (err error) {
		plan, _, err = applyLookStep(cp, spreadID, step, r, c, libs)
		return err
	})
	if err != nil {
		plan = model.GridPlan{}
	}
	return plan, err
}

// checkStep — зажим входа ApplyLookStep: ссылка (пустая или негодная — nil: обычная плитка) и макет.
func checkStep(ref model.LookRef, t model.AlbumTpl) (*model.LookRef, model.AlbumTpl, error) {
	c, ok := model.ClampLook(t)
	if !ok {
		return nil, model.AlbumTpl{}, model.ErrTplEmpty
	}
	return model.ClampLookRef(&ref), c, nil
}

// applyLookStep — тело ApplyLookStep и PlanLookStep.
func applyLookStep(p *model.Project, spreadID string, step int, ref *model.LookRef, t model.AlbumTpl, libs map[string]model.PhotoInfo) (plan model.GridPlan, missing int, err error) {
	if (spreadID == model.CoverID) != (step == 0) {
		return plan, 0, ErrLookCover
	}
	if step == 0 {
		return applyLookCoverStep(p, ref, t, libs)
	}
	if step < 0 || step > len(t.Steps) {
		return plan, 0, ErrLookStep
	}
	if _, s := p.Find(spreadID); s == nil {
		return plan, 0, nil
	}
	n := 0 // номер шага запоминается, только если макет — этого альбома (набор обновился — тот же)
	if ref != nil && (p.Look == nil || model.SameLook(p.Look, ref)) {
		n = step
	}
	plan, gone, err := grid.PutLookTile(p, spreadID, t.Steps[step-1], n, libs)
	if err != nil {
		return model.GridPlan{}, 0, err
	}
	if n > 0 && (p.Look == nil || p.Look.Key != ref.Key) {
		p.Look = ref // новая версия набора — запись с новым ключом
	}
	return plan, len(gone), nil
}

// NewLookSpread ставит новый разворот по серии макета t (макет альбома — Project.Look, его
// находит App): пустой разворот перед хвостом серии (в конец, если хвоста нет) и следующий
// повторяемый шаг, одной правкой («Разворот макета» в меню «+»). Альбом без макета — ErrNoLook;
// предел MaxSpreads и отказ постановки — ошибка без шага.
func (e *Editor) NewLookSpread(t model.AlbumTpl, libs map[string]model.PhotoInfo) (model.View, error) {
	c, ok := model.ClampLook(t)
	if !ok {
		return e.View(), model.ErrTplEmpty
	}
	return e.try("", func(p *model.Project) error {
		if p.Look == nil {
			return ErrNoLook
		}
		// Макет альбома мог смениться между чтением App и правкой: сверка под замком с тем, что в реестре
		// (по содержимому, не по SameLook: обновлённый набор — другие шаги).
		if f := e.findLook(p.Look); f != nil && !reflect.DeepEqual(*f, c) {
			return ErrLookChanged
		}
		return lookAddSteps(p, &model.LookPlan{}, c, 1, libs)
	})
}
