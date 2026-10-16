package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// AddPhotos, NewSpread, MovePhoto: отказ (лимит фото на развороте) — ошибка, проект не меняется.
func (m *Methods) AddPhotos(spreadID string, ids []string, index int) (v model.View, err error) {
	defer m.guardEdit("AddPhotos", &v, &err)
	return m.changedOrErr(m.editor().AddPhotos(spreadID, m.infos(ids), index))
}

func (m *Methods) NewSpread(at int, ids []string) (v model.View, err error) {
	defer m.guardEdit("NewSpread", &v, &err)
	infos := m.infos(ids)
	if len(ids) > 0 && len(infos) == 0 {
		// Фото пропали при пересканировании во время переноса: пустой разворот здесь не просили.
		return m.editor().View(), nil
	}
	return m.changedOrErr(m.editor().NewSpread(at, infos))
}

func (m *Methods) MovePhoto(photoID, toSpread string, index int) (v model.View, err error) {
	defer m.guardEdit("MovePhoto", &v, &err)
	return m.changedOrErr(m.editor().MovePhoto(photoID, toSpread, index))
}

func (m *Methods) SwapPhotos(x, y string) (v model.View) {
	defer m.guardEdit("SwapPhotos", &v, nil)
	return m.changed(m.editor().SwapPhotos(x, y))
}

func (m *Methods) RemovePhotos(ids []string) (v model.View) {
	defer m.guardEdit("RemovePhotos", &v, nil)
	return m.changed(m.editor().RemovePhotos(ids))
}

func (m *Methods) DeleteSpread(id string) (v model.View) {
	defer m.guardEdit("DeleteSpread", &v, nil)
	return m.changed(m.editor().DeleteSpread(id))
}

// DeleteSpreads удаляет выделенные развороты одним шагом отмены; неизвестные ID пропускаются.
func (m *Methods) DeleteSpreads(ids []string) (v model.View) {
	defer m.guardEdit("DeleteSpreads", &v, nil)
	return m.changed(m.editor().DeleteSpreads(ids))
}

func (m *Methods) MoveSpread(id string, to int) (v model.View) {
	defer m.guardEdit("MoveSpread", &v, nil)
	return m.changed(m.editor().MoveSpread(id, to))
}

func (m *Methods) SplitSpread(id string, at int) (v model.View) {
	defer m.guardEdit("SplitSpread", &v, nil)
	return m.changed(m.editor().SplitSpread(id, at))
}

// MergeSpreads: больше MaxPhotos фото или MaxTexts текстов вместе — ничего не меняет.
func (m *Methods) MergeSpreads(id string) (v model.View) {
	defer m.guardEdit("MergeSpreads", &v, nil)
	return m.changed(m.editor().MergeSpreads(id))
}

// SetLayout ставит раскладку из галереи; битый или уже стоящий ключ — ничего не меняет.
func (m *Methods) SetLayout(spreadID, key string) (v model.View) {
	defer m.guardEdit("SetLayout", &v, nil)
	return m.changed(m.editor().SetLayout(spreadID, key))
}

// StepLayout листает раскладки: scope "spread" | "left" | "right".
func (m *Methods) StepLayout(spreadID, scope string, delta int) (v model.View) {
	defer m.guardEdit("StepLayout", &v, nil)
	return m.changed(m.editor().StepLayout(spreadID, scope, delta))
}

// LayoutChoices — варианты для галереи. Только чтение: без правки и без события.
func (m *Methods) LayoutChoices(spreadID string) model.Choices {
	defer Guard("LayoutChoices")
	return m.editor().LayoutChoices(spreadID)
}

// MoreLayouts — плитка «Ещё» галереи: ряд «Авто» (17+ фото) на порцию длиннее have. Только чтение.
func (m *Methods) MoreLayouts(spreadID string, have int) model.Choices {
	defer Guard("MoreLayouts")
	return m.editor().MoreLayouts(spreadID, have)
}

func (m *Methods) SetSettings(s model.Settings) (v model.View) {
	defer m.guardEdit("SetSettings", &v, nil)
	return m.changed(m.editor().SetSettings(s))
}

// AutoPlan — подсказка окна автосборки на o.Spreads разворотов по o.MinPer…o.MaxPer фото или
// «Авто», с сериями или без (всё зажимается; MaxPer — в 1…64; Variety не влияет; нули новых
// полей — как 0.12). Только чтение. Фото нет — пустой
// план; альбом почти полон (MaxSpreads) — ошибка.
func (m *Methods) AutoPlan(ids []string, o model.AutoOpts) (pl model.AutoPlan, err error) {
	defer GuardErr("AutoPlan", &err)
	return m.editor().AutoPlan(m.infos(ids), o)
}

// AutoBuild собирает ровно те развороты, что показал AutoPlan с теми же аргументами.
// Сверх MaxSpreads разворотов — ошибка, проект не меняется. Variety зажимается в 0…2.
func (m *Methods) AutoBuild(ids []string, at int, o model.AutoOpts) (v model.View, err error) {
	defer m.guardEdit("AutoBuild", &v, &err)
	o.Variety = o.Variety.Clamp()
	return m.changedOrErr(m.editor().AutoBuild(m.infos(ids), at, o))
}

// VaryLayouts — «Разнообразить раскладки» (041) для выделенных разворотов, одним шагом
// отмены; variety зажимается в 0…2. Нечего менять — холостая: rev тот же, UI покажет тост.
// Оценки из XMP берутся до замка Editor: замки не вкладываются.
func (m *Methods) VaryLayouts(spreadIDs []string, variety int) (v model.View, err error) {
	defer m.guardEdit("VaryLayouts", &v, &err)
	ed := m.editor()
	infos := m.infos(ed.PhotosOf(spreadIDs))
	return m.changed(ed.VaryLayouts(spreadIDs, model.Variety(variety).Clamp(), infos)), nil
}

// SetMarks ставит свои отметки фото ленты; ID, которых нет в ленте, пропускаются.
// Ошибка — сверх MaxMarks своих отметок; отметки тогда прежние.
func (m *Methods) SetMarks(ids []string, patch model.MarkPatch) (v model.View, err error) {
	defer m.guardEdit("SetMarks", &v, &err)
	return m.changedOrErr(m.editor().SetMarks(m.infos(ids), patch))
}

// ResetMarks — «Как в Lightroom»: у этих фото снова действуют отметки из файлов.
func (m *Methods) ResetMarks(ids []string) (v model.View) {
	defer m.guardEdit("ResetMarks", &v, nil)
	return m.changed(m.editor().ResetMarks(ids))
}

// Marks — свои отметки проекта (ID → заданные поля); UI перечитывает при смене View.marksRev и после state.
func (m *Methods) Marks() map[string]model.MarkView {
	defer Guard("Marks")
	return m.editor().Marks()
}
