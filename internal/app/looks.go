package app

import (
	"image"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/looks"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Макеты альбома (спека 131, 163): тонкие обёртки над internal/app/looks. Здесь только то, что
// принадлежит App: guardErr/guardEdit для биндингов, типы ответов (имена в биндингах прежние) и Host службы.

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

// LookPlanSave — сухой прогон SaveLook для окна «Сохранить альбом как макет…».
type LookPlanSave struct {
	Ok     bool   `json:"ok"`     // сохранить можно (имя и тема не проверяются: это поля окна)
	Steps  int    `json:"steps"`  // шагов в макете после слияния повторяемых; 0, если Ok = false
	Max    int    `json:"max"`    // предел шагов (MaxLookSteps)
	Reason string `json:"reason"` // почему нельзя — текст ошибки SaveLook как есть; пусто при Ok
}

// errLookSheet — лист для печати (031): макеты альбома ему не нужны.
var errLookSheet = looks.ErrSheet

// LookInfo — карточка макета id. Число разворотов серии — от минимума (начало, конец и один повтор)
// до MaxSpreads; без повтора серия фиксирована. Тело — looks.Service.Info.
func (a *App) LookInfo(id string) (info LookInfo, err error) {
	defer guardErr("LookInfo", &err)
	c, err := a.looks.Info(id)
	if err != nil {
		return info, err
	}
	info = LookInfo{Steps: make([]LookStepInfo, len(c.Steps)), Cover: c.Cover, Min: c.Min, Max: c.Max}
	for i, st := range c.Steps {
		info.Steps[i] = LookStepInfo(st)
	}
	return info, nil
}

// LookCount — серия макета id из n разворотов (зажим 1…MaxSpreads): их число, мест и n под k фото.
func (a *App) LookCount(id string, n, k int) (c LookCount, err error) {
	defer guardErr("LookCount", &err)
	s, err := a.looks.Count(id, n, k)
	return LookCount(s), err
}

// PlanLook — вопрос до ApplyLook: сухой прогон на копии (ничего не меняет). Лист 031 — ошибка.
func (a *App) PlanLook(id string, o model.LookOpts) (plan model.LookPlan, err error) {
	defer guardErr("PlanLook", &err)
	return a.looks.Plan(id, o)
}

// ApplyLook — «Применить» окна макетов: одна правка, один шаг отмены; плитка — в недавние.
func (a *App) ApplyLook(id string, o model.LookOpts) (r LookResult, err error) {
	defer a.guardEdit("ApplyLook", &r.View, &err)
	res, err := a.looks.Apply(id, o)
	return LookResult(res), err
}

// PlanLookStep — вопрос до ApplyLookStep: сколько фото уйдёт в ленту, что заменится. Обложка
// ('cover') и шаг 0 — оформление обложки (ApplyLookCover; макет без обложки — ErrLookStep).
func (a *App) PlanLookStep(spreadID, id string, step int) (plan model.GridPlan, err error) {
	defer guardErr("PlanLookStep", &err)
	return a.looks.PlanStep(spreadID, id, step)
}

// ApplyLookStep — «Поставить на разворот N»: шаг step макета id на лист spreadID, как плитка 067.
func (a *App) ApplyLookStep(spreadID, id string, step int) (r TplResult, err error) {
	defer a.guardEdit("ApplyLookStep", &r.View, &err)
	res, err := a.looks.ApplyStep(spreadID, id, step)
	return TplResult(res), err
}

// NewLookSpread — «Разворот макета» в меню «+»: следующий повторяемый шаг макета альбома.
// Альбом не по макету — ErrNoLook; макета нет на этом ПК — отдельная ошибка с именем.
func (a *App) NewLookSpread() (v model.View, err error) {
	defer a.guardEdit("NewLookSpread", &v, &err)
	return a.looks.NewSpread()
}

// SaveLook — «Сохранить альбом как макет…»: серия из разворотов o.Ids (повторяемые — позиции
// RepFrom…RepTo, одинаковые сливаются), обложка при o.Cover, места при o.Places, надписи
// заготовками при o.Drafts — в свои макеты темы o.Theme под именем o.Name. Альбом не меняется (чтение
// под замком); фото и фон из съёмки в макет не идут. Лист 031 — errLookSheet. Больше 24 разных
// шагов — ошибка для тоста окна.
func (a *App) SaveLook(o model.SaveLookOpts) (it DecorItem, err error) {
	defer guardErr("SaveLook", &err)
	got, err := a.looks.Save(o)
	return DecorItem(got), err
}

// PlanSaveLook — сухой прогон SaveLook: тот же путь до записи, но без библиотеки и без правки
// альбома. Звать при смене выбора, границ «Повторяемые» и галочек окна. Ошибка Go — только сбой
// самого вызова; отказ сохранения лежит в Reason.
func (a *App) PlanSaveLook(o model.SaveLookOpts) (r LookPlanSave, err error) {
	defer guardErr("PlanSaveLook", &err)
	c, err := a.looks.PlanSave(o)
	return LookPlanSave(c), err
}

// CreateCoverLook — «Создать обложку» (плитка, бросок фото): как CreateCover, но у альбома по макету
// оформление макета встаёт той же правкой и тем же шагом отмены (131 G5). Макета нет у альбома, нет
// на этом ПК или у него нет обложки — обычная обложка, Styled = false. «Без оформления» — Undo и
// CreateCover (она оформления не ставит).
func (a *App) CreateCoverLook(preset, photoID string) (r CoverResult, err error) {
	defer a.guardEdit("CreateCoverLook", &r.View, &err)
	res, err := a.looks.CoverLook(preset, photoID)
	return CoverResult(res), err
}

// coverLookLibs — запись макета альбома и файлы фонов его обложки; общая для CreateCoverLook и
// set_cover_photo агента.
func (a *App) coverLookLibs(ed *album.Editor) (*model.LookRef, map[string]model.PhotoInfo) {
	return a.looks.CoverLibs(ed)
}

// lookHost — библиотека макетов для Editor (SetLookHost).
func (a *App) lookHost() album.LookHost { return a.looks.LookHost() }

// looksHost — что службе макетов нужно от App. Всё, что тесты подменяют после NewApp (кэш превью,
// папка настроек, лента), читается в момент вызова, а не при сборке. Замок App берёт только Feed
// (копия ленты) и mediaPhoto, как раньше — порядок «замок Editor, затем a.mu» прежний.
func (a *App) looksHost() looks.Host {
	return looks.Host{
		Editor:  a.editor,
		Store:   a.decorStore,
		LibsFor: a.libsFor,
		Infos:   a.infosOrRefs,
		Changed: a.changed,
		CfgDir:  func() string { return a.cfgDir },
		Photo:   a.mediaPhoto,
		Cache:   func() *library.Cache { return a.cache },
		Feed: func() []library.Photo {
			a.mu.Lock()
			defer a.mu.Unlock()
			return slices.Clone(a.photos)
		},
		Cancelled: func() { a.mediaSt.cancelled.Add(1) },
		Image: func(t model.SpreadTpl, px int, r float64, libs map[string]model.PhotoInfo, photos []model.PhotoInfo) (image.Image, error) {
			return tplImageWith(t, tplQuery{px: px, r: r}, libs, photos)
		},
		PNG:      encodeTile,
		Keys:     tplKeys,
		FreePick: freePick,
	}
}
