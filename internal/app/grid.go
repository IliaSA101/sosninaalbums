package app

import (
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Режим «Сетка» (спека 007, design §4.2): правки мест под guardEdit.

// GridResult — вид после правки и сколько фото не встало или ушло в ленту
// (интерфейс показывает тост с «Вернуть» = Undo).
type GridResult struct {
	View model.View `json:"view"`
	Left int        `json:"left"`
}

// SetGrid: on — «Закрепить сетку», иначе «Вернуть авто».
func (a *App) SetGrid(spreadID string, on bool) (v model.View) {
	defer a.guardEdit("SetGrid", &v, nil)
	return a.changed(a.editor().SetGrid(spreadID, on))
}

// AddPlace — пустое место на страницу page (0/1); у «Авто» — закрепление той же правкой.
func (a *App) AddPlace(spreadID string, page int) (v model.View, err error) {
	defer a.guardEdit("AddPlace", &v, &err)
	return a.changedOrErr(a.editor().AddPlace(spreadID, page))
}

// SetPlaces: перенос и размер мест (мм); boxes[].id — ref (ID места или ID фото разворота).
// У «Авто» закрепляет разворот той же правкой. merge — сессия стрелок, один шаг отмены.
func (a *App) SetPlaces(spreadID string, boxes []model.PlaceBox, merge string) (v model.View) {
	defer a.guardEdit("SetPlaces", &v, nil)
	return a.changed(a.editor().SetPlaces(spreadID, boxes, merge))
}

// SetPlaceAngle: угол места ref (ID места или фото), градусы по часовой вокруг центра;
// место не сдвигается. У «Авто» закрепляет разворот той же правкой.
func (a *App) SetPlaceAngle(spreadID, ref string, deg float64) (v model.View) {
	defer a.guardEdit("SetPlaceAngle", &v, nil)
	return a.changed(a.editor().SetPlaceAngle(spreadID, ref, deg))
}

// DeletePlaces: ids — ref; фото удалённых мест уходят в ленту; без мест разворот — «Авто».
// У «Авто» холостая (фото убирает RemovePhotos).
func (a *App) DeletePlaces(spreadID string, ids []string) (v model.View) {
	defer a.guardEdit("DeletePlaces", &v, nil)
	return a.changed(a.editor().DeletePlaces(spreadID, ids))
}

// DeleteSelection: «Del» по группе одним шагом undo — тексты, фото (места остаются
// пустыми) и места по ref. Что убирать вместе, решает UI.
func (a *App) DeleteSelection(spreadID string, photos, places, texts []string) (v model.View) {
	defer a.guardEdit("DeleteSelection", &v, nil)
	return a.changed(a.editor().DeleteSelection(spreadID, photos, places, texts))
}

// ArrangePlaces: op same | sameW | sameH | fitPhoto | splitH | splitV | front | back.
// ids — ref; у «Авто» закрепляет (кроме front/back). Малое для деления место или лимит мест — ошибка.
func (a *App) ArrangePlaces(spreadID, op string, ids []string) (v model.View, err error) {
	defer a.guardEdit("ArrangePlaces", &v, &err)
	return a.changedOrErr(a.editor().ArrangePlaces(spreadID, op, ids))
}

// ArrangeLayers: op front | up | down | back; ids — ref мест и ID текстов разворота.
// Выше/Ниже — через ближайшее перекрывающееся; у «Авто» места не трогаются, тексты работают.
func (a *App) ArrangeLayers(spreadID, op string, ids []string) (v model.View) {
	defer a.guardEdit("ArrangeLayers", &v, nil)
	return a.changed(a.editor().ArrangeLayers(spreadID, op, ids))
}

// AlignItems: выравнивание и распределение мест и текстов одной правкой. ids — ref мест
// и фото, ID текстов; op: left | hcenter | right | top | vcenter | bottom | distH | distV;
// to: sel | page | spread. Мало объектов и неизвестное — холостая; у «Авто» с фото
// в выборе закрепляет разворот той же правкой.
func (a *App) AlignItems(spreadID string, ids []string, op, to string) (v model.View) {
	defer a.guardEdit("AlignItems", &v, nil)
	return a.changed(a.editor().AlignItems(spreadID, ids, op, to))
}

// AlignOverlap: только чтение — распределение (op distH | distV) не помещается без
// наложения; UI показывает подсказку. Аргументы — как у AlignItems.
func (a *App) AlignOverlap(spreadID string, ids []string, op, to string) bool {
	defer guard("AlignOverlap")
	return a.editor().AlignOverlap(spreadID, ids, op, to)
}

// TransformGroup: перенос и пропорциональный масштаб группы (места и тексты) одной правкой,
// p → a + (p−a)·k + d, мм разворота. ids — ref мест и ID текстов. k и d зажимаются в Go;
// у «Авто» с фото в группе закрепляет разворот той же правкой. merge — сессия стрелок.
func (a *App) TransformGroup(spreadID string, ids []string, k, ax, ay, dx, dy float64, merge string) (v model.View) {
	defer a.guardEdit("TransformGroup", &v, nil)
	return a.changed(a.editor().TransformGroup(spreadID, ids, k, ax, ay, dx, dy, merge))
}

// RotateGroup: поворот группы (места и тексты) на deg по часовой вокруг (cx, cy), мм
// разворота, одной правкой. Угол не упирается: группа сдвигается внутрь целиком, крайние
// поджимаются по одному. У «Авто» с фото в группе закрепляет разворот той же правкой.
func (a *App) RotateGroup(spreadID string, ids []string, deg, cx, cy float64) (v model.View) {
	defer a.guardEdit("RotateGroup", &v, nil)
	return a.changed(a.editor().RotateGroup(spreadID, ids, deg, cx, cy))
}

// ScaleGroupAxes: масштаб группы по осям рамки под углом phi (градусы): kx, ky вокруг
// (ax, ay), без перекоса; кегль — по меньшему множителю. kx = ky — как TransformGroup.
// Множители зажимаются в Go; у «Авто» с фото в группе закрепляет разворот той же правкой.
func (a *App) ScaleGroupAxes(spreadID string, ids []string, kx, ky, phi, ax, ay float64) (v model.View) {
	defer a.guardEdit("ScaleGroupAxes", &v, nil)
	return a.changed(a.editor().ScaleGroupAxes(spreadID, ids, kx, ky, phi, ax, ay))
}

// SetPlaceFit: ids — ref; fit "" — с обрезкой, "contain" — целиком (у «Авто» закрепляет).
func (a *App) SetPlaceFit(spreadID string, ids []string, fit string) (v model.View) {
	defer a.guardEdit("SetPlaceFit", &v, nil)
	return a.changed(a.editor().SetPlaceFit(spreadID, ids, fit))
}

// MirrorGrid: scope left | right | spread. Симметричная сетка — тот же rev.
func (a *App) MirrorGrid(spreadID, scope string) (v model.View) {
	defer a.guardEdit("MirrorGrid", &v, nil)
	return a.changed(a.editor().MirrorGrid(spreadID, scope))
}

// PlacePhotos — бросок фото (из ленты или альбома) на место.
func (a *App) PlacePhotos(spreadID, placeID string, ids []string) (r GridResult, err error) {
	defer a.guardEdit("PlacePhotos", &r.View, &err)
	v, left, err := a.editor().PlacePhotos(spreadID, placeID, a.infosOrRefs(ids))
	r.View, err = a.changedOrErr(v, err)
	r.Left = left
	return r, err
}

// DropResult — итог DropPhotos (042). Ask: правки не было (rev тот же) — UI задаёт вопрос
// и зовёт DropPhotos ещё раз с rest "add" | "skip".
type DropResult struct {
	View  model.View `json:"view"`
	Ask   bool       `json:"ask"`
	Fit   int        `json:"fit"`   // сколько встанет (встало) в имеющиеся места, с целевым
	Total int        `json:"total"` // сколько фото пришло на лист (без тех, что уже в его местах)
	Fresh []string   `json:"fresh"` // ID новых мест плитки: UI выделяет их, как после «Место»
	Grown int        `json:"grown"` // на сколько выросло «Учеников» нетронутой виньетки
	Out   bool       `json:"out"`   // прежнее фото целевого места ушло в ленту (замена)
	Left  int        `json:"left"`  // не встало: «Не добавлять» или обложка
	Small bool       `json:"small"` // место плитки мельче 1/10 меньшей стороны страницы
}

// DropPhotos — бросок фото на лист spreadID (042): placeID "" — на лист, иначе на место.
// at — доля ширины листа под курсором 0…1, < 0 — точки нет. rest: "" — спросить, если
// 0 < Fit < Total; "add" — остальные плиткой (нетронутая виньетка — ученики); "skip" — не
// добавлять. «Авто» — как AddPhotos в конец; обложка — как PlacePhotos. Отказ — ошибка,
// проект прежний.
func (a *App) DropPhotos(spreadID, placeID string, ids []string, at float64, rest string) (r DropResult, err error) {
	defer a.guardEdit("DropPhotos", &r.View, &err)
	v, o, err := a.editor().DropPhotos(spreadID, placeID, a.infosOrRefs(ids), at, rest)
	if o.Ask && err == nil { // правки нет (v — нынешний вид): не рассылать, автосохранение не трогать
		r.View = v
	} else if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	fresh := o.Fresh
	if fresh == nil {
		fresh = []string{} // в JSON — [], а не null: UI не проверяет на null
	}
	return DropResult{View: r.View, Ask: o.Ask, Fit: o.Fit, Total: o.Total, Fresh: fresh, Grown: o.Grown,
		Out: o.Out, Left: o.Left, Small: o.Small}, nil
}

// FillPlaces — «Заполнить пустые места» фото из ленты, начиная с разворота fromSpread.
func (a *App) FillPlaces(ids []string, fromSpread string) (r GridResult) {
	defer a.guardEdit("FillPlaces", &r.View, nil)
	v, left := a.editor().FillPlaces(a.infos(ids), fromSpread)
	return GridResult{View: a.changed(v), Left: left}
}

// CopyGrid копирует места области from на цели; design — с текстом и фоном,
// mirror — на страницы другой стороны зеркально.
func (a *App) CopyGrid(spreadID, from string, to []model.GridTarget, design, mirror bool) (r GridResult, err error) {
	defer a.guardEdit("CopyGrid", &r.View, &err)
	v, left, err := a.editor().CopyGrid(spreadID, from, to, design, mirror)
	r.View, err = a.changedOrErr(v, err)
	r.Left = left
	return r, err
}

// PlanCopyGrid — что сделает CopyGrid, без правки (вопрос до копирования).
func (a *App) PlanCopyGrid(spreadID, from string, to []model.GridTarget, design, mirror bool) model.GridPlan {
	defer guard("PlanCopyGrid")
	return a.editor().PlanCopyGrid(spreadID, from, to, design, mirror)
}

// DuplicateGrid — count новых разворотов с местами spreadID без фото.
func (a *App) DuplicateGrid(spreadID string, count int, design bool) (v model.View, err error) {
	defer a.guardEdit("DuplicateGrid", &v, &err)
	return a.changedOrErr(a.editor().DuplicateGrid(spreadID, count, design))
}

// ApplyGrid ставит сетку шаблона или встроенную: boxes — доли области tplScope
// (page | spread) с углом места, scope — куда ставить (left | right | spread).
func (a *App) ApplyGrid(spreadID, scope, tplScope string, boxes []model.Box) (r GridResult, err error) {
	defer a.guardEdit("ApplyGrid", &r.View, &err)
	v, left, err := a.editor().ApplyGrid(spreadID, scope, tplScope, boxes)
	r.View, err = a.changedOrErr(v, err)
	r.Left = left
	return r, err
}

// infosOrRefs — сведения о фото из ленты, а для фото альбома, чьей папки нет в
// ленте, — из проекта (как MovePhoto).
func (a *App) infosOrRefs(ids []string) []model.PhotoInfo {
	known := a.infos(ids)
	byID := make(map[string]model.PhotoInfo, len(known))
	for _, p := range known {
		byID[p.ID] = p
	}
	out := make([]model.PhotoInfo, 0, len(ids))
	ed := a.editor()
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		} else if r, ok := ed.Ref(id); ok {
			out = append(out, model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H, Size: r.Size})
		}
	}
	return out
}

// loadTemplates читает templates.json при старте (хранилище — internal/prefs).
func (a *App) loadTemplates() {
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	a.tpls = prefs.LoadTemplates(a.cfgDir)
}

// templateList — копия текущего списка шаблонов.
func (a *App) templateList() []model.Template {
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	return slices.Clone(a.tpls.List())
}

// storeTemplates пишет новый список в templates.json и принимает его в память. Вызывать под tplMu.
func (a *App) storeTemplates(list []model.Template) error {
	return a.tpls.Store(a.cfgDir, list, a.rename)
}

// GridChoices — вкладка «Сетки»: встроенные сетки на n мест и «Мои шаблоны» с n
// местами (7 — от 7); scope ("left"/"right"/"spread") — куда ставить, по нему Drop.
// Только чтение; на 7 фото холодный счёт занимает до ≈ 0.4 с, интерфейс ждёт.
func (a *App) GridChoices(spreadID string, n int, scope string) model.GridChoices {
	defer guard("GridChoices")
	return a.editor().GridChoices(spreadID, n, scope, a.templateList())
}

// SaveGridTemplate сохраняет места области scope ("left"/"right"/"spread") разворота
// как шаблон (страницы или разворота). Пустое имя — «Шаблон N». Нет мест — ошибка.
func (a *App) SaveGridTemplate(spreadID, scope, name string) (err error) {
	defer guardErr("SaveGridTemplate", &err)
	boxes, tplScope, err := a.editor().PlaceBoxes(spreadID, scope)
	if err != nil {
		return err
	}
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	if len(cur) >= model.MaxTemplates {
		return model.ErrTooManyTpl
	}
	list := append(slices.Clone(cur), model.Template{
		ID: model.NewID(), Name: model.CleanTemplateName(name, len(cur)+1), Scope: tplScope, Boxes: boxes,
	})
	return a.storeTemplates(list)
}

// SaveVignetteTemplate сохраняет виньетку листа (лист или разворот) как свой шаблон (038):
// места с ролями, шапку, пропорции и стиль подписей — без фото, имён и текста шапки. Пустое
// имя — «Шаблон N». Не виньетка, нет мест людей, слишком много шаблонов или файл больше 1 МБ —
// ошибка. Шаблон общий для всех альбомов этого компьютера.
func (a *App) SaveVignetteTemplate(spreadID, name string) (err error) {
	defer guardErr("SaveVignetteTemplate", &err)
	t, err := a.editor().VigTemplate(spreadID)
	if err != nil {
		return err
	}
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	if len(cur) >= model.MaxTemplates {
		return model.ErrTooManyTpl
	}
	t.ID, t.Name = model.NewID(), model.CleanTemplateName(name, len(cur)+1)
	return a.storeTemplates(append(slices.Clone(cur), t))
}

// RenameGridTemplate меняет имя шаблона (пустое — «Шаблон N»). id — как у карточки: у шаблона
// виньетки с префиксом «my:» (VigChoice.id).
func (a *App) RenameGridTemplate(id, name string) (err error) {
	defer guardErr("RenameGridTemplate", &err)
	err = a.editTemplates(id, func(list []model.Template, i int) []model.Template {
		list[i].Name = model.CleanTemplateName(name, i+1)
		return list
	})
	if err == nil {
		a.syncOwnNames()
	}
	return err
}

// DeleteGridTemplate удаляет шаблон (id — как у RenameGridTemplate); вопрос «удалить?» задаёт
// интерфейс. Листы, где шаблон виньетки применён, остаются: геометрия не меняется (в Vig.Own),
// имя — последнее из файла.
func (a *App) DeleteGridTemplate(id string) (err error) {
	defer guardErr("DeleteGridTemplate", &err)
	// Имя копии на листах — последнее из файла: после удаления его больше неоткуда взять
	// (альбом мог быть закрыт, когда шаблон переименовали).
	a.syncOwnNames()
	return a.editTemplates(id, func(list []model.Template, i int) []model.Template {
		return slices.Delete(list, i, i+1)
	})
}

// syncOwnNames — имена своих шаблонов виньетки на листах открытого альбома — по файлу шаблонов,
// без шага отмены (Editor.SyncOwnNames); сменились — вид рассылается, альбом сохранится сам.
func (a *App) syncOwnNames() {
	if v, changed := a.editor().SyncOwnNames(a.templateList()); changed {
		a.changed(v)
	}
}

// editTemplates применяет fn к копии списка (i — индекс шаблона id) и записывает результат.
func (a *App) editTemplates(id string, fn func(list []model.Template, i int) []model.Template) error {
	id = strings.TrimPrefix(id, model.VigOwnPrefix)
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	i := slices.IndexFunc(cur, func(t model.Template) bool { return t.ID == id })
	if i < 0 {
		return model.ErrTplNotFound
	}
	return a.storeTemplates(fn(slices.Clone(cur), i))
}
