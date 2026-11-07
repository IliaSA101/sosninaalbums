package app

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Готовые развороты (спека 067, design §3, §6): окно «Развороты» — вид spread библиотеки
// (DecorCatalog, RenameDecor, HideDecor — общие 048), поставить, вопрос до правки, сохранить
// свой и плитки /media/tpl. Шаблон берётся только по ID плитки из decor.Store.

// TplResult — итог ApplySpreadTpl: вид, сколько фото ушло в ленту и сколько ключей библиотеки
// шаблона нет на этом ПК (их слои пропущены).
type TplResult struct {
	View    model.View `json:"view"`
	Left    int        `json:"left"`
	Missing int        `json:"missing"`
}

// spreadTpl — шаблон плитки itemID и библиотека.
func (a *App) spreadTpl(itemID string) (*decor.Store, looktpl.SpreadTpl, error) {
	s, err := a.decorStore()
	if err != nil {
		return nil, looktpl.SpreadTpl{}, err
	}
	t, err := s.SpreadTpl(itemID)
	return s, t, err
}

// PlanSpreadTpl — вопрос до ApplySpreadTpl (как PlanCopyDecor): сколько фото уйдёт в ленту,
// сколько стикеров и надписей заменится, сколько «Авто» станет сеткой.
func (a *App) PlanSpreadTpl(spreadID, scope, itemID string) (plan model.GridPlan, err error) {
	defer guardErr("PlanSpreadTpl", &err)
	s, t, err := a.spreadTpl(itemID)
	if err != nil {
		return plan, err
	}
	return a.editor().PlanSpreadTpl(spreadID, scope, t, a.looks.TplLibs(s, t, false))
}

// ApplySpreadTpl — щелчок по плитке разворота: места и оформление области scope ("spread" |
// "left" | "right") — один шаг отмены; повтор — холостая правка без шага. Плитка — в недавние.
func (a *App) ApplySpreadTpl(spreadID, scope, itemID string) (r TplResult, err error) {
	defer a.guardEdit("ApplySpreadTpl", &r.View, &err)
	ed := a.editor()
	s, t, err := a.spreadTpl(itemID)
	if err != nil {
		return TplResult{View: ed.View()}, err
	}
	v, plan, missing, err := ed.ApplySpreadTpl(spreadID, scope, t, a.looks.TplLibs(s, t, false))
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Left, r.Missing = plan.Left, missing
	prefs.RememberDecor(a.cfgDir, decor.KindSpread, itemID)
	return r, nil
}

// SaveSpreadTpl — «Сохранить разворот…»: снимок области (places — с местами, texts — с
// надписями; фото фона из съёмки не входит, решение 6) в свои шаблоны темы theme под именем
// name. Альбом не меняется.
func (a *App) SaveSpreadTpl(spreadID, scope, theme, name string, places, texts bool) (it DecorItem, err error) {
	defer guardErr("SaveSpreadTpl", &err)
	s, err := a.decorStore()
	if err != nil {
		return it, err
	}
	t, err := a.editor().SpreadTplOf(spreadID, scope, places, texts)
	if err != nil {
		return it, err
	}
	got, err := s.SaveSpreadTpl(theme, name, t)
	if err != nil {
		return it, err
	}
	return DecorItem(got), nil
}
