package edit

import (
	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// AddPhotos, NewSpread, MovePhoto: отказ (лимит фото на развороте) — ошибка, проект не меняется.
func (m *Methods) AddPhotos(spreadID string, ids []string, index int) (v model.View, err error) {
	defer m.guardEdit("AddPhotos", &v, &err)
	infos, err := PhotosFor("AddPhotos", ids, m.infosOrRefs)
	if err != nil {
		return m.editor().View(), err
	}
	return m.changedOrErr(m.editor().AddPhotos(spreadID, infos, index))
}

func (m *Methods) NewSpread(at int, ids []string) (v model.View, err error) {
	defer m.guardEdit("NewSpread", &v, &err)
	// Фото пропали при пересканировании во время переноса: пустой разворот здесь не просили —
	// ошибка (092), а не тихая холостая.
	infos, err := PhotosFor("NewSpread", ids, m.infosOrRefs)
	if err != nil {
		return m.editor().View(), err
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

// PurgePhotos — «Убрать из альбома» в полосе «Не найдено»: в отличие от RemovePhotos снимает
// и фоны, и свои стикеры с этими фото, одним шагом отмены (075-13).
func (m *Methods) PurgePhotos(ids []string) (v model.View) {
	defer m.guardEdit("PurgePhotos", &v, nil)
	return m.changed(m.editor().PurgePhotos(ids))
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

// SplitSpread: на пределе MaxSpreads разворотов — ошибка (тост в UI), проект не меняется.
func (m *Methods) SplitSpread(id string, at int) (v model.View, err error) {
	defer m.guardEdit("SplitSpread", &v, &err)
	return m.changedOrErr(m.editor().SplitSpread(id, at))
}

// MergeSpreads: больше MaxPhotos фото или MaxTexts текстов вместе — ничего не меняет.
func (m *Methods) MergeSpreads(id string) (v model.View) {
	defer m.guardEdit("MergeSpreads", &v, nil)
	return m.changed(m.editor().MergeSpreads(id))
}

// MergeSpreadsAuto — «Склеить» из вопроса о ручном развороте (103): один шаг отмены.
func (m *Methods) MergeSpreadsAuto(id string, keep bool) (v model.View, err error) {
	defer m.guardEdit("MergeSpreadsAuto", &v, &err)
	return m.changedOrErr(m.editor().MergeSpreadsAuto(id, keep))
}

// SplitSpreadAuto — «Разрезать» из вопроса о ручном развороте (103); предел 200 — ошибка.
func (m *Methods) SplitSpreadAuto(id string, at int, keep bool) (v model.View, err error) {
	defer m.guardEdit("SplitSpreadAuto", &v, &err)
	return m.changedOrErr(m.editor().SplitSpreadAuto(id, at, keep))
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

// SetSettingsToAll — SetSettings и снятие своего цвета страниц у всех разворотов (115): они идут
// за новым цветом альбома; обложка не трогается. Один шаг отмены.
func (m *Methods) SetSettingsToAll(s model.Settings) (v model.View) {
	defer m.guardEdit("SetSettingsToAll", &v, nil)
	return m.changed(m.editor().SetSettingsToAll(s))
}

// AutoPlan — подсказка окна автосборки на o.Spreads разворотов по o.MinPer…o.MaxPer фото или
// «Авто», с сериями или без (всё зажимается; MaxPer — в 1…64; Variety не влияет; нули новых
// полей — как 0.12). Похожие (078) — без них, число в similar; ч/б — gray; o.again — от альбома
// до прошлой сборки (не жива — ошибка). o.album (107) — «Весь альбом заново»: пул — фото голых
// разворотов, якоря на местах; состав — album, at не действует. Только чтение. Фото нет — пустой
// план; альбом почти полон (MaxSpreads) — ошибка.
func (m *Methods) AutoPlan(ids []string, o model.AutoOpts) (pl model.AutoPlan, err error) {
	defer GuardErr("AutoPlan", &err)
	return m.editor().AutoPlan(m.infos(ids), o)
}

// AutoSimilar — кадры, которых автосборка с теми же аргументами не возьмёт как похожие (078), по
// порядку ленты: like — оставленный, на который похож. similar 0 — пусто. Только чтение.
func (m *Methods) AutoSimilar(ids []string, o model.AutoOpts) (out []model.SimilarDrop, err error) {
	defer GuardErr("AutoSimilar", &err)
	return m.editor().AutoSimilar(m.infos(ids), o)
}

// AutoBuild собирает ровно те развороты, что показал AutoPlan с теми же аргументами.
// Сверх MaxSpreads разворотов — ошибка, проект не меняется. Variety зажимается в 0…2,
// семейства — до 16 строк (065: имена и повторы разбирает сборка). o.again = true — пересборка
// последней сборки (078 §8): тот же шаг отмены, пока после неё не было шагов; не жива — ошибка.
// o.album (107) — пересборка всего альбома: голые развороты заново, якоря и обложка не меняются;
// at не действует, один шаг отмены на всю цепочку «Пересобрать».
func (m *Methods) AutoBuild(ids []string, at int, o model.AutoOpts) (v model.View, err error) {
	defer m.guardEdit("AutoBuild", &v, &err)
	v, err = m.changedOrErr(m.editor().AutoBuild(m.infos(ids), at, o.Clamp()))
	if err == nil {
		album.LogMem("автосборка") // 089: строка для e2e-отчёта; не в Editor — ниже порога журнал молчит
	}
	return v, err
}

// AutoPreview — ряд блока «Раскладки» окна автосборки (065): «Варианты» групп плана и «Что
// соберётся» — AutoBuild с теми же аргументами на копии проекта. Только чтение: rev, undo и
// события не меняются. Фото нет — пустые ряды; ошибка плана — как у AutoPlan.
func (m *Methods) AutoPreview(ids []string, at int, o model.AutoOpts) (pv model.AutoPreview, err error) {
	defer GuardErr("AutoPreview", &err)
	return m.editor().AutoPreview(m.infos(ids), at, o.Clamp())
}

// VaryLayouts — «Разнообразить раскладки» (041) для выделенных разворотов, одним шагом
// отмены; variety зажимается в 0…2, families — фильтр семейств окна автосборки (065; пусто —
// все, до 16 строк). Нечего менять — холостая: rev тот же, UI покажет тост.
// Оценки из XMP берутся до замка Editor: замки не вкладываются.
func (m *Methods) VaryLayouts(spreadIDs []string, variety int, families []string) (v model.View, err error) {
	defer m.guardEdit("VaryLayouts", &v, &err)
	ed := m.editor()
	infos := m.infosOrRefs(ed.PhotosOf(spreadIDs)) // ID — фото разворотов; ошибки «не найдено» нет
	fs := model.ClampFamilies(families)
	return m.changed(ed.VaryLayouts(spreadIDs, model.Variety(variety).Clamp(), fs, infos)), nil
}

// SetMarks ставит свои отметки фото ленты; ID, которых нет в ленте, пропускаются.
// Ошибка — сверх MaxMarks своих отметок; отметки тогда прежние.
func (m *Methods) SetMarks(ids []string, patch model.MarkPatch) (v model.View, err error) {
	defer m.guardEdit("SetMarks", &v, &err)
	return m.changedOrErr(m.editor().SetMarks(m.infos(ids), patch))
}

// PickBest — «Лучший» в сравнении кадров (138): best — «отобрано», rest — «отклонено», один шаг
// отмены. ID вне ленты пропускаются (best неизвестен — холостая без ошибки); ошибка — больше
// трёх остальных или сверх MaxMarks, отметки тогда прежние.
func (m *Methods) PickBest(best string, rest []string) (v model.View, err error) {
	defer m.guardEdit("PickBest", &v, &err)
	b := m.infos([]string{best})
	if len(b) == 0 {
		return m.editor().View(), nil
	}
	return m.changedOrErr(m.editor().PickBest(b[0], m.infos(rest)))
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
