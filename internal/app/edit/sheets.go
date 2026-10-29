package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// CoverPresets — пресеты типографий для списка «Типография» (статично).
func (m *Methods) CoverPresets() []model.CoverPreset { return model.CoverPresets() }

// PagePresets — кнопки размера страницы «Настроек альбома»: общий ряд и ряд Фотосферы (047, статично).
func (m *Methods) PagePresets() []model.PagePreset { return model.PagePresets() }

// CreateCover создаёт обложку: preset "" — пресет по умолчанию. photoID непуст — фото из
// библиотеки или альбома встаёт фоном лицевой (бросок на плитку «Обложка»), тем же шагом
// отмены; фото остаётся в своём развороте. Обложка уже есть или проект-лист — ошибка.
func (m *Methods) CreateCover(preset, photoID string) (v model.View, err error) {
	defer m.guardEdit("CreateCover", &v, &err)
	var ph *model.PhotoInfo
	if photoID != "" {
		infos := m.infosOrRefs([]string{photoID})
		if len(infos) == 0 {
			return m.editor().View(), ErrNoPhoto
		}
		ph = &infos[0]
	}
	return m.changedOrErr(m.editor().CreateCover(preset, ph))
}

// DeleteCover снимает обложку; фото её мест уходят из альбома, как при удалении разворота.
func (m *Methods) DeleteCover() (v model.View) {
	defer m.guardEdit("DeleteCover", &v, nil)
	return m.changed(m.editor().DeleteCover())
}

// ApplyCoverPreset — числа пресета типографии; сторонка пользователя остаётся.
func (m *Methods) ApplyCoverPreset(id string) (v model.View, err error) {
	defer m.guardEdit("ApplyCoverPreset", &v, &err)
	return m.changedOrErr(m.editor().ApplyCoverPreset(id))
}

// ApplyPrint — кнопка формата типографии и изделие обложки одним шагом отмены (123 Б1): настройки
// страницы, обложка по шаблону прежнего формата идёт за страницей, затем числа изделия preset.
// Неизвестное изделие — ошибка тостом, без правки.
func (m *Methods) ApplyPrint(s model.Settings, preset string) (v model.View, err error) {
	defer m.guardEdit("ApplyPrint", &v, &err)
	return m.changedOrErr(m.editor().ApplyPrint(s, preset))
}

// SetCoverSpec — свои числа обложки (зажим в Go); не совпали с пресетом — «Свои размеры».
func (m *Methods) SetCoverSpec(s model.CoverSpec) (v model.View, err error) {
	defer m.guardEdit("SetCoverSpec", &v, &err)
	return m.changedOrErr(m.editor().SetCoverSpec(s))
}

// SetCoverPx — «Файл обложки, px»: сторонка, при которой лист обложки выходит ровно w × h px
// при 300 dpi (123); изделие остаётся, если в допуске шаблона. Ошибка подбора — тостом.
func (m *Methods) SetCoverPx(w, h int) (v model.View, err error) {
	defer m.guardEdit("SetCoverPx", &v, &err)
	return m.changedOrErr(m.editor().SetCoverPx(w, h))
}

// SetCoverLayout — раскладка обложки: front | both | wrap.
func (m *Methods) SetCoverLayout(kind string) (v model.View, err error) {
	defer m.guardEdit("SetCoverLayout", &v, &err)
	return m.changedOrErr(m.editor().SetCoverLayout(kind))
}

// CoverSpineText — «Надпись на корешке» с плашкой; уже есть — без изменений.
func (m *Methods) CoverSpineText() (v model.View, err error) {
	defer m.guardEdit("CoverSpineText", &v, &err)
	return m.changedOrErr(m.editor().CoverSpineText())
}

// SetCoverSpineBg — цвет плашки корешка #rrggbb; "" — без плашки.
func (m *Methods) SetCoverSpineBg(color string) (v model.View) {
	defer m.guardEdit("SetCoverSpineBg", &v, nil)
	return m.changed(m.editor().SetCoverSpineBg(color))
}

// ClearCoverGuide убирает разметку обложки.
func (m *Methods) ClearCoverGuide() (v model.View) {
	defer m.guardEdit("ClearCoverGuide", &v, nil)
	return m.changed(m.editor().ClearCoverGuide())
}

// ClearSpreadGuide убирает разметку разворотов (066); её нет — без изменений.
func (m *Methods) ClearSpreadGuide() (v model.View) {
	defer m.guardEdit("ClearSpreadGuide", &v, nil)
	return m.changed(m.editor().ClearSpreadGuide())
}

// StepCoverGrid — ‹ › и Пробел над сторонкой обложки (127): scope "left" | "right", delta ±1; листает
// «По фото», если форма мест узнана, иначе правки нет (rev прежний — UI открывает ▦ по Known и Alone).
// Серия по сторонке — один шаг отмены.
func (m *Methods) StepCoverGrid(scope string, delta int) (v model.View) {
	defer m.guardEdit("StepCoverGrid", &v, nil)
	return m.changed(m.editor().StepCoverGrid(scope, delta))
}

// VignetteChoices — превью шаблонов виньетки для чисел и пропорции панели (без правки).
func (m *Methods) VignetteChoices(spreadID string, teachers, pupils int, ratio string) []model.VigChoice {
	defer Guard("VignetteChoices")
	return m.editor().VignetteChoices(spreadID, teachers, pupils, ratio, m.templateList())
}

// PreviewVignette — предпросмотр окна «Настроить виньетку»: лист после «Применить» с теми же
// параметрами и полями и сколько фото уйдёт в ленту. Проект не меняется, события нет.
func (m *Methods) PreviewVignette(spreadID string, s model.VigSetup) (pr model.VigPreview, err error) {
	defer GuardErr("PreviewVignette", &err)
	return m.editor().PreviewVignette(spreadID, s, m.templateList())
}

// AddVignette — «Виньетка» в колонке «+ Новый разворот» и меню: разворот по умолчанию на at.
func (m *Methods) AddVignette(at int) (v model.View, err error) {
	defer m.guardEdit("AddVignette", &v, &err)
	return m.changedOrErr(m.editor().AddVignette(at))
}

// ClearVignette — «Обычный разворот»: места и фото остаются.
func (m *Methods) ClearVignette(spreadID string) (v model.View) {
	defer m.guardEdit("ClearVignette", &v, nil)
	return m.changed(m.editor().ClearVignette(spreadID))
}

// SortVignette — «Сортировать по фамилии».
func (m *Methods) SortVignette(spreadID string) (v model.View) {
	defer m.guardEdit("SortVignette", &v, nil)
	return m.changed(m.editor().SortVignette(spreadID))
}
