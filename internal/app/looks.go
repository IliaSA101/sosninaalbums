package app

import (
	"errors"
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Макеты альбома (спека 131, design §8): окно «Макеты альбома» — вид album библиотеки
// (DecorCatalog, RenameDecor, HideDecor — общие 048), сведения, числа, вопрос до правки и сама
// постановка. Макет берётся только по ключу плитки из индекса decor.Store; пути от интерфейса не идут.
// Плитки /media/look — looks_media.go.

// errLookSheet — лист для печати (031): макеты альбома ему не нужны.
var errLookSheet = errors.New("Макеты альбома ставятся на альбом, не на лист для печати") //nolint:staticcheck // текст уходит пользователю как есть

// LookStepInfo — шаг серии в карточке макета.
type LookStepInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`   // "" | "repeat" | "vignette"
	Places int    `json:"places"` // мест в шаге
	Pages  int    `json:"pages"`  // 1 — страница, 2 — разворот
}

// LookInfo — карточка макета: шаги, есть ли обложка и в каких пределах число разворотов серии.
type LookInfo struct {
	Steps []LookStepInfo `json:"steps"`
	Cover bool           `json:"cover"`
	Min   int            `json:"min"`
	Max   int            `json:"max"`
}

// LookCount — серия из N разворотов: сколько их на деле, мест в них и сколько разворотов
// нужно под k фото («Подобрать под K»).
type LookCount struct {
	N      int `json:"n"`
	Places int `json:"places"`
	Fit    int `json:"fit"`
}

// LookResult — итог ApplyLook: вид и что вышло.
type LookResult struct {
	View model.View     `json:"view"`
	Plan model.LookPlan `json:"plan"`
}

// CoverResult — итог CreateCoverLook: вид, встало ли оформление макета альбома и его имя для тоста
// «Обложка в стиле «…» · Без оформления».
type CoverResult struct {
	View   model.View `json:"view"`
	Styled bool       `json:"styled"`
	Look   string     `json:"look"` // имя макета; пусто, если оформление не встало
}

// lookByID — макет плитки id и его запись для альбома. Нет в индексе, файл не читается или он
// новее программы — ошибка библиотеки.
func (a *App) lookByID(id string) (*decor.Store, model.LookRef, model.AlbumTpl, error) {
	s, err := a.decorStore()
	if err != nil {
		return nil, model.LookRef{}, model.AlbumTpl{}, err
	}
	ref, ok := s.LookRefOf(id)
	if !ok {
		return s, ref, model.AlbumTpl{}, decor.ErrNotFound
	}
	t, err := s.Look(id)
	return s, ref, t, err
}

// lookHost — библиотека макетов для Editor (SetLookHost): поиск по записи альбома и «есть на ПК».
// Хранилище читается при вызове: оно открывается после первого Editor.
func (a *App) lookHost() album.LookHost {
	return album.LookHost{
		Find: func(ref model.LookRef) *model.AlbumTpl {
			s := a.decorLib.Load()
			if s == nil {
				return nil
			}
			key, ok := s.LookKeyOf(ref)
			if !ok {
				return nil
			}
			t, err := s.Look(key)
			if err != nil {
				return nil
			}
			return &t
		},
		Has: func(ref model.LookRef) bool {
			s := a.decorLib.Load()
			return s != nil && s.HasLook(ref)
		},
		Private: func(lib string) bool {
			s := a.decorLib.Load()
			return s != nil && s.IsQR(lib)
		},
	}
}

// lookLibs — свои из библиотеки для макетов ts, чьи файлы есть на этом ПК.
func (a *App) lookLibs(s *decor.Store, ts ...model.AlbumTpl) map[string]model.PhotoInfo {
	var keys []string
	for _, t := range ts {
		for _, st := range t.Steps {
			keys = append(keys, tplKeys(st.Tpl)...)
		}
		if c := t.Cover; c != nil {
			if c.Bg != nil && c.Bg.Lib != "" {
				keys = append(keys, c.Bg.Lib)
			}
			for _, side := range []*model.SpreadTpl{c.Back, c.Front} {
				if side != nil {
					keys = append(keys, tplKeys(*side)...)
				}
			}
		}
	}
	return a.libsFor(s, keys, false)
}

// lookLibsPrev — lookLibs макета t и прежнего макета альбома (по ним сверяется «своё»).
func (a *App) lookLibsPrev(s *decor.Store, t model.AlbumTpl) map[string]model.PhotoInfo {
	ts := []model.AlbumTpl{t}
	if cur := a.editor().Look(); cur != nil {
		if prev := a.lookHost().Find(*cur); prev != nil {
			ts = append(ts, *prev)
		}
	}
	return a.lookLibs(s, ts...)
}

// lookOpts — зажим параметров: разворотов 1…MaxSpreads, режим известен (пусто — выберет ядро).
func lookOpts(o model.LookOpts) (model.LookOpts, error) {
	switch o.Mode {
	case "", model.LookSeries, model.LookAdd, model.LookBg:
	default:
		return o, fmt.Errorf("неизвестный режим макета %q", o.Mode)
	}
	o.Spreads = max(1, min(o.Spreads, model.MaxSpreads))
	return o, nil
}

// LookInfo — карточка макета id. Число разворотов серии — от минимума (начало, конец и один повтор)
// до MaxSpreads; без повтора серия фиксирована.
func (a *App) LookInfo(id string) (info LookInfo, err error) {
	defer guardErr("LookInfo", &err)
	_, _, t, err := a.lookByID(id)
	if err != nil {
		return info, err
	}
	pre, rep, post := model.LookBlockSizes(t)
	info = LookInfo{Steps: make([]LookStepInfo, len(t.Steps)), Cover: t.Cover != nil, Min: pre + post + min(1, rep), Max: model.MaxSpreads}
	if rep == 0 {
		info.Min, info.Max = len(t.Steps), len(t.Steps)
	}
	for i, st := range t.Steps {
		pages := 2
		if st.Tpl.Pages == 1 {
			pages = 1
		}
		if st.Role == model.RoleVignette {
			pages = 1 // лист 031 — одна страница
		}
		info.Steps[i] = LookStepInfo{Name: st.Name, Role: st.Role, Places: model.LookPlaces(t, i), Pages: pages}
	}
	return info, nil
}

// LookCount — серия макета id из n разворотов (зажим 1…MaxSpreads): их число, мест и n под k фото.
func (a *App) LookCount(id string, n, k int) (c LookCount, err error) {
	defer guardErr("LookCount", &err)
	_, _, t, err := a.lookByID(id)
	if err != nil {
		return c, err
	}
	ids := model.Series(t, max(1, min(n, model.MaxSpreads)))
	c.N = len(ids)
	for _, i := range ids {
		c.Places += model.LookPlaces(t, i)
	}
	c.Fit = model.LookFit(t, max(0, k))
	return c, nil
}

// PlanLook — вопрос до ApplyLook: сухой прогон на копии (ничего не меняет). Лист 031 — ошибка.
func (a *App) PlanLook(id string, o model.LookOpts) (plan model.LookPlan, err error) {
	defer guardErr("PlanLook", &err)
	ed := a.editor()
	if ed.IsSheet() {
		return plan, errLookSheet
	}
	if o, err = lookOpts(o); err != nil {
		return plan, err
	}
	s, ref, t, err := a.lookByID(id)
	if err != nil {
		return plan, err
	}
	return ed.PlanLook(ref, t, a.lookHost().Find, o, a.lookLibsPrev(s, t))
}

// ApplyLook — «Применить» окна макетов: одна правка, один шаг отмены; плитка — в недавние.
func (a *App) ApplyLook(id string, o model.LookOpts) (r LookResult, err error) {
	defer a.guardEdit("ApplyLook", &r.View, &err)
	ed := a.editor()
	if ed.IsSheet() {
		return LookResult{View: ed.View()}, errLookSheet
	}
	if o, err = lookOpts(o); err != nil {
		return LookResult{View: ed.View()}, err
	}
	s, ref, t, err := a.lookByID(id)
	if err != nil {
		return LookResult{View: ed.View()}, err
	}
	v, plan, err := ed.ApplyLook(ref, t, a.lookHost().Find, o, a.lookLibsPrev(s, t))
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Plan = plan
	prefs.RememberDecor(a.cfgDir, decor.KindAlbum, id)
	return r, nil
}

// PlanLookStep — вопрос до ApplyLookStep: сколько фото уйдёт в ленту, что заменится. Обложка
// ('cover') и шаг 0 — оформление обложки (ApplyLookCover; макет без обложки — ErrLookStep).
func (a *App) PlanLookStep(spreadID, id string, step int) (plan model.GridPlan, err error) {
	defer guardErr("PlanLookStep", &err)
	ed := a.editor()
	if ed.IsSheet() {
		return plan, errLookSheet
	}
	s, ref, t, err := a.lookByID(id)
	if err != nil {
		return plan, err
	}
	return ed.PlanLookStep(spreadID, step, ref, t, a.lookLibs(s, t))
}

// ApplyLookStep — «Поставить на разворот N»: шаг step макета id на лист spreadID, как плитка 067.
func (a *App) ApplyLookStep(spreadID, id string, step int) (r TplResult, err error) {
	defer a.guardEdit("ApplyLookStep", &r.View, &err)
	ed := a.editor()
	if ed.IsSheet() {
		return TplResult{View: ed.View()}, errLookSheet
	}
	s, ref, t, err := a.lookByID(id)
	if err != nil {
		return TplResult{View: ed.View()}, err
	}
	v, plan, missing, err := ed.ApplyLookStep(spreadID, step, ref, t, a.lookLibs(s, t))
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Left, r.Missing = plan.Left, missing
	prefs.RememberDecor(a.cfgDir, decor.KindAlbum, id)
	return r, nil
}

// NewLookSpread — «Разворот макета» в меню «+»: следующий повторяемый шаг макета альбома.
// Альбом не по макету — ErrNoLook; макета нет на этом ПК — отдельная ошибка с именем.
func (a *App) NewLookSpread() (v model.View, err error) {
	defer a.guardEdit("NewLookSpread", &v, &err)
	ed := a.editor()
	if ed.IsSheet() {
		return ed.View(), errLookSheet
	}
	cur := ed.Look()
	if cur == nil {
		return ed.View(), album.ErrNoLook
	}
	s, err := a.decorStore()
	if err != nil {
		return ed.View(), err
	}
	key, ok := s.LookKeyOf(*cur)
	if !ok {
		return ed.View(), fmt.Errorf("Макета «%s» нет в библиотеке этого компьютера", cur.Name) //nolint:staticcheck // текст уходит пользователю как есть
	}
	t, err := s.Look(key)
	if err != nil {
		return ed.View(), err
	}
	return a.changedOrErr(ed.NewLookSpread(t, a.lookLibs(s, t)))
}

// SaveLook — «Сохранить альбом как макет…»: серия из разворотов o.Ids (повторяемые — позиции
// RepFrom…RepTo, одинаковые сливаются), обложка при o.Cover, места при o.Places, надписи
// заготовками при o.Drafts — в свои макеты темы o.Theme под именем o.Name. Альбом не меняется (чтение
// под замком); фото и фон из съёмки в макет не идут. Лист 031 — errLookSheet. Больше 24 разных
// шагов — ошибка для тоста окна.
func (a *App) SaveLook(o model.SaveLookOpts) (it DecorItem, err error) {
	defer guardErr("SaveLook", &err)
	t, err := a.saveLookTpl(o)
	if err != nil {
		return it, err
	}
	s, err := a.decorStore()
	if err != nil {
		return it, err
	}
	got, err := s.SaveLook(o.Theme, o.Name, t)
	if err != nil {
		return it, err
	}
	return DecorItem(got), nil
}

// saveLookTpl — макет из разворотов o.Ids до записи: общая часть SaveLook и PlanSaveLook (лист 031,
// пустой выбор, чужие id, больше MaxLookSteps шагов, пустой или слишком большой файл). Чтение под замком.
func (a *App) saveLookTpl(o model.SaveLookOpts) (model.AlbumTpl, error) {
	ed := a.editor()
	if ed.IsSheet() {
		return model.AlbumTpl{}, errLookSheet
	}
	t, err := ed.AlbumTplOf(o)
	if err != nil {
		return t, err
	}
	return decor.CheckLook(t)
}

// LookPlanSave — сухой прогон SaveLook для окна «Сохранить альбом как макет…».
type LookPlanSave struct {
	Ok     bool   `json:"ok"`     // сохранить можно (имя и тема не проверяются: это поля окна)
	Steps  int    `json:"steps"`  // шагов в макете после слияния повторяемых; 0, если Ok = false
	Max    int    `json:"max"`    // предел шагов (MaxLookSteps)
	Reason string `json:"reason"` // почему нельзя — текст ошибки SaveLook как есть; пусто при Ok
}

// PlanSaveLook — сухой прогон SaveLook: тот же путь до записи (saveLookTpl), но без библиотеки и
// без правки альбома. Звать при смене выбора, границ «Повторяемые» и галочек окна. Ошибка Go — только
// сбой самого вызова; отказ сохранения лежит в Reason.
func (a *App) PlanSaveLook(o model.SaveLookOpts) (r LookPlanSave, err error) {
	defer guardErr("PlanSaveLook", &err)
	r.Max = model.MaxLookSteps
	t, terr := a.saveLookTpl(o)
	if terr != nil {
		r.Reason = terr.Error()
		return r, nil
	}
	r.Ok, r.Steps = true, len(t.Steps)
	return r, nil
}

// CreateCoverLook — «Создать обложку» (плитка, бросок фото): как CreateCover, но у альбома по макету
// оформление макета встаёт той же правкой и тем же шагом отмены (131 G5). Макета нет у альбома, нет
// на этом ПК или у него нет обложки — обычная обложка, Styled = false. «Без оформления» — Undo и
// CreateCover (она оформления не ставит).
func (a *App) CreateCoverLook(preset, photoID string) (r CoverResult, err error) {
	defer a.guardEdit("CreateCoverLook", &r.View, &err)
	ed := a.editor()
	var ph *model.PhotoInfo
	if photoID != "" {
		infos := a.infosOrRefs([]string{photoID})
		if len(infos) == 0 {
			return CoverResult{View: ed.View()}, errNoPhoto
		}
		ph = &infos[0]
	}
	// Файлы фонов обложки макета; хранилище недоступно — оформление встанет без «своих» файлов.
	cur := ed.Look()
	var libs map[string]model.PhotoInfo
	if cur != nil {
		if s, serr := a.decorStore(); serr == nil {
			if t := a.lookHost().Find(*cur); t != nil {
				libs = a.lookLibs(s, *t)
			}
		}
	}
	// cur — запись, по которой собраны libs: сменился макет между чтением и правкой — ErrLookChanged.
	v, rec, err := ed.CreateCoverLook(preset, ph, libs, cur)
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	if rec != nil {
		r.Styled, r.Look = true, rec.Name
	}
	return r, nil
}
