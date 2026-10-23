package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// SetGrid: on — «Закрепить сетку», иначе «Вернуть авто».
func (m *Methods) SetGrid(spreadID string, on bool) (v model.View) {
	defer m.guardEdit("SetGrid", &v, nil)
	return m.changed(m.editor().SetGrid(spreadID, on))
}

// BackToAuto — «Вернуть авто» из окна «Вернуть авто?» (098): keep — галочка «Сохранить кадр
// правленых фото», key — раскладка из галереи ("" — сохранённая или лучшая). Один шаг отмены;
// заменяет в окне SetGrid(false) и SetLayout с сетки.
func (m *Methods) BackToAuto(spreadID, key string, keep bool) (v model.View, err error) {
	defer m.guardEdit("BackToAuto", &v, &err)
	return m.changedOrErr(m.editor().BackToAuto(spreadID, key, keep))
}

// CropsToKeep — N для галочки «Сохранить кадр правленых фото: N»; 0 — галочки нет. Только чтение.
func (m *Methods) CropsToKeep(spreadID string) int {
	defer Guard("CropsToKeep")
	return m.editor().CropsToKeep(spreadID)
}

// AddPlace — пустое место на страницу page (0/1); у «Авто» — закрепление той же правкой.
// shape — форма (044, 076): "oval" — круглое, "rounded" — скругление 10 %, иначе прямоугольное.
func (m *Methods) AddPlace(spreadID string, page int, shape string) (v model.View, err error) {
	defer m.guardEdit("AddPlace", &v, &err)
	return m.changedOrErr(m.editor().AddPlace(spreadID, page, shape))
}

// SetPlaces: перенос и размер мест (мм); boxes[].id — ref (ID места или ID фото разворота).
// У «Авто» закрепляет разворот той же правкой. merge — сессия стрелок, один шаг отмены.
func (m *Methods) SetPlaces(spreadID string, boxes []model.PlaceBox, merge string) (v model.View) {
	defer m.guardEdit("SetPlaces", &v, nil)
	return m.changed(m.editor().SetPlaces(spreadID, boxes, merge))
}

// SetPlaceAngle: угол места ref (ID места или фото), градусы по часовой вокруг центра;
// место не сдвигается. У «Авто» закрепляет разворот той же правкой.
func (m *Methods) SetPlaceAngle(spreadID, ref string, deg float64) (v model.View) {
	defer m.guardEdit("SetPlaceAngle", &v, nil)
	return m.changed(m.editor().SetPlaceAngle(spreadID, ref, deg))
}

// DeletePlaces: ids — ref; фото удалённых мест уходят в ленту; без мест разворот — «Авто».
// У «Авто» холостая (фото убирает RemovePhotos).
func (m *Methods) DeletePlaces(spreadID string, ids []string) (v model.View) {
	defer m.guardEdit("DeletePlaces", &v, nil)
	return m.changed(m.editor().DeletePlaces(spreadID, ids))
}

// DeleteSelection: «Del» по группе одним шагом undo — тексты, фото (места остаются
// пустыми) и места по ref. Что убирать вместе, решает UI.
func (m *Methods) DeleteSelection(spreadID string, photos, places, texts []string) (v model.View) {
	defer m.guardEdit("DeleteSelection", &v, nil)
	return m.changed(m.editor().DeleteSelection(spreadID, photos, places, texts))
}

// ArrangePlaces: op same | sameW | sameH | fitPhoto | splitH | splitV | front | back.
// ids — ref; у «Авто» закрепляет (кроме front/back). Малое для деления место или лимит мест — ошибка.
func (m *Methods) ArrangePlaces(spreadID, op string, ids []string) (v model.View, err error) {
	defer m.guardEdit("ArrangePlaces", &v, &err)
	return m.changedOrErr(m.editor().ArrangePlaces(spreadID, op, ids))
}

// ArrangeLayers: op front | up | down | back; ids — ref мест и ID текстов разворота.
// Выше/Ниже — через ближайшее перекрывающееся; у «Авто» места не трогаются, тексты работают.
func (m *Methods) ArrangeLayers(spreadID, op string, ids []string) (v model.View) {
	defer m.guardEdit("ArrangeLayers", &v, nil)
	return m.changed(m.editor().ArrangeLayers(spreadID, op, ids))
}

// AlignItems: выравнивание и распределение мест и текстов одной правкой. ids — ref мест
// и фото, ID текстов; op: left | hcenter | right | top | vcenter | bottom | distH | distV;
// to: sel | page | spread. Мало объектов и неизвестное — холостая; у «Авто» с фото
// в выборе закрепляет разворот той же правкой.
func (m *Methods) AlignItems(spreadID string, ids []string, op, to string) (v model.View) {
	defer m.guardEdit("AlignItems", &v, nil)
	return m.changed(m.editor().AlignItems(spreadID, ids, op, to))
}

// AlignOverlap: только чтение — распределение (op distH | distV) не помещается без
// наложения; UI показывает подсказку. Аргументы — как у AlignItems.
func (m *Methods) AlignOverlap(spreadID string, ids []string, op, to string) bool {
	defer Guard("AlignOverlap")
	return m.editor().AlignOverlap(spreadID, ids, op, to)
}

// TransformGroup: перенос и пропорциональный масштаб группы (места и тексты) одной правкой,
// p → a + (p−a)·k + d, мм разворота. ids — ref мест и ID текстов. k и d зажимаются в Go;
// у «Авто» с фото в группе закрепляет разворот той же правкой. merge — сессия стрелок.
func (m *Methods) TransformGroup(spreadID string, ids []string, k, ax, ay, dx, dy float64, merge string) (v model.View) {
	defer m.guardEdit("TransformGroup", &v, nil)
	return m.changed(m.editor().TransformGroup(spreadID, ids, k, ax, ay, dx, dy, merge))
}

// RotateGroup: поворот группы (места и тексты) на deg по часовой вокруг (cx, cy), мм
// разворота, одной правкой. Угол не упирается: группа сдвигается внутрь целиком, крайние
// поджимаются по одному. У «Авто» с фото в группе закрепляет разворот той же правкой.
func (m *Methods) RotateGroup(spreadID string, ids []string, deg, cx, cy float64) (v model.View) {
	defer m.guardEdit("RotateGroup", &v, nil)
	return m.changed(m.editor().RotateGroup(spreadID, ids, deg, cx, cy))
}

// ScaleGroupAxes: масштаб группы по осям рамки под углом phi (градусы): kx, ky вокруг
// (ax, ay), без перекоса; кегль — по меньшему множителю. kx = ky — как TransformGroup.
// Множители зажимаются в Go; у «Авто» с фото в группе закрепляет разворот той же правкой.
func (m *Methods) ScaleGroupAxes(spreadID string, ids []string, kx, ky, phi, ax, ay float64) (v model.View) {
	defer m.guardEdit("ScaleGroupAxes", &v, nil)
	return m.changed(m.editor().ScaleGroupAxes(spreadID, ids, kx, ky, phi, ax, ay))
}

// SetPlaceFit: ids — ref; fit "" — с обрезкой, "contain" — целиком (у «Авто» закрепляет).
func (m *Methods) SetPlaceFit(spreadID string, ids []string, fit string) (v model.View) {
	defer m.guardEdit("SetPlaceFit", &v, nil)
	return m.changed(m.editor().SetPlaceFit(spreadID, ids, fit))
}

// SetPlaceShape: ids — ref; shape "" — прямые углы, "rounded" — скругление (своё или 10 %),
// "oval" — круг/овал, вписанный в рамку (скругление снимается). "rounded" и "oval" у «Авто»
// закрепляют. Иное — тот же rev.
func (m *Methods) SetPlaceShape(spreadID string, ids []string, shape string) (v model.View) {
	defer m.guardEdit("SetPlaceShape", &v, nil)
	return m.changed(m.editor().SetPlaceShape(spreadID, ids, shape))
}

// SetPlaceRound: ids — ref; скругление углов прямоугольных мест, % меньшей стороны (0…50,
// целое). NaN — тот же rev.
func (m *Methods) SetPlaceRound(spreadID string, ids []string, round float64) (v model.View) {
	defer m.guardEdit("SetPlaceRound", &v, nil)
	return m.changed(m.editor().SetPlaceRound(spreadID, ids, round))
}

// SetBorder: ids — ref; цвета, толщина или размер рамочек мест (079 §8). Inks nil — не менять,
// [] — цвета вида, лишние цвета у вида с меньшим их числом отбрасываются (079-36); 0 — не
// менять; правка снимает отметку плитки. Серия с одним merge — один шаг; NaN — тот же rev.
func (m *Methods) SetBorder(spreadID string, ids []string, p model.BorderPatch, merge string) (v model.View) {
	defer m.guardEdit("SetBorder", &v, nil)
	return m.changed(m.editor().SetBorder(spreadID, ids, p, merge))
}

// MirrorGrid: scope left | right | spread. Симметричная сетка — тот же rev.
func (m *Methods) MirrorGrid(spreadID, scope string) (v model.View) {
	defer m.guardEdit("MirrorGrid", &v, nil)
	return m.changed(m.editor().MirrorGrid(spreadID, scope))
}

// DuplicateGrid — count новых разворотов с местами spreadID без фото.
func (m *Methods) DuplicateGrid(spreadID string, count int, design bool) (v model.View, err error) {
	defer m.guardEdit("DuplicateGrid", &v, &err)
	return m.changedOrErr(m.editor().DuplicateGrid(spreadID, count, design))
}

// PlanCopyGrid — что сделает CopyGrid, без правки (вопрос до копирования).
func (m *Methods) PlanCopyGrid(spreadID, from string, to []model.GridTarget, design, mirror bool) model.GridPlan {
	defer Guard("PlanCopyGrid")
	return m.editor().PlanCopyGrid(spreadID, from, to, design, mirror)
}

// PlanCopyDecor — что заменит CopyDecor (стикеры, надписи), без правки (вопрос до копирования).
func (m *Methods) PlanCopyDecor(spreadID, from string, to []model.GridTarget, texts, mirror bool) model.GridPlan {
	defer Guard("PlanCopyDecor")
	return m.editor().PlanCopyDecor(spreadID, from, to, texts, mirror)
}

// GridChoices — вкладка «Сетки»: встроенные сетки на n мест и «Мои шаблоны» с n
// местами (7 — от 7); scope ("left"/"right"/"spread") — куда ставить, по нему Drop.
// Только чтение; на 7 фото холодный счёт занимает до ≈ 0.4 с, интерфейс ждёт.
func (m *Methods) GridChoices(spreadID string, n int, scope string) model.GridChoices {
	defer Guard("GridChoices")
	return m.editor().GridChoices(spreadID, n, scope, m.templateList())
}
