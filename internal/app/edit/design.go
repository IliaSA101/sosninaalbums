package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// SetFrame ставит кадр фото: s 1..4, x/y −1..1 (зажимается); s=1, x=y=0 — сброс.
// Каждый вызов — шаг отмены.
func (m *Methods) SetFrame(photoID string, s, x, y float64) (v model.View) {
	defer m.guardEdit("SetFrame", &v, nil)
	return m.changed(m.editor().SetFrame(photoID, s, x, y))
}

// ResetFrame — кадр по умолчанию (то же, что SetFrame(id, 1, 0, 0)).
func (m *Methods) ResetFrame(photoID string) (v model.View) {
	defer m.guardEdit("ResetFrame", &v, nil)
	return m.changed(m.editor().ResetFrame(photoID))
}

// SetFrameAngle: наклон фото листа spreadID в рамке tilt, градусы (−44,9…44,9, шаг 0,1);
// наклон держит приближение (057: S, X, Y от начала сессии), пропорции стоят. session —
// сессия строки «Наклон» ([A-Za-z0-9-]{1,32}; "" — без слияния): всё до ухода фокуса — один
// шаг отмены. Вызывать на отпускании ползунка, стрелках, вводе числа.
func (m *Methods) SetFrameAngle(spreadID, photoID string, tilt float64, session string) (v model.View) {
	defer m.guardEdit("SetFrameAngle", &v, nil)
	return m.changed(m.editor().SetFrameAngle(spreadID, photoID, tilt, session))
}

// StraightenPhoto — «Выпрямить по линии»: dx, dy — конец минус начало линии, мм листа
// spreadID (порядок точек не важен). Фото поворачивается так, что линия встаёт вдоль края
// места; четверти стоят, наклон упирается в ±44,9. Один шаг отмены; ровная линия — холостая.
func (m *Methods) StraightenPhoto(spreadID, photoID string, dx, dy float64) (v model.View) {
	defer m.guardEdit("StraightenPhoto", &v, nil)
	return m.changed(m.editor().StraightenPhoto(spreadID, photoID, dx, dy))
}

// TurnPhotos поворачивает фото ids на quarters·90° (−1 — ↺, 1 — ↻, 2 — 180°; иное —
// холостая) одной правкой; «Авто» перестраивается под новые пропорции.
func (m *Methods) TurnPhotos(ids []string, quarters int) (v model.View) {
	defer m.guardEdit("TurnPhotos", &v, nil)
	return m.changed(m.editor().TurnPhotos(ids, quarters))
}

// SetFlip зеркалит фото ids: axis "h" (↔) | "v" (↕); не у всех включено — всем включить,
// иначе всем выключить. Раскладку не трогает.
func (m *Methods) SetFlip(ids []string, axis string) (v model.View) {
	defer m.guardEdit("SetFlip", &v, nil)
	return m.changed(m.editor().SetFlip(ids, axis))
}

// SetGray — ч/б фото ids (068): не у всех включено — всем включить, иначе всем выключить.
func (m *Methods) SetGray(ids []string) (v model.View) {
	defer m.guardEdit("SetGray", &v, nil)
	return m.changed(m.editor().SetGray(ids))
}

// SetTone — яркость и контраст фото (068), −50…50 каждое (зажим в Go); ч/б остаётся.
// UI шлёт оба значения сразу: второе — из вида.
func (m *Methods) SetTone(photoID string, bright, contrast int) (v model.View) {
	defer m.guardEdit("SetTone", &v, nil)
	return m.changed(m.editor().SetTone(photoID, bright, contrast))
}

// SetBackground: scope "spread" (обе страницы) | "left" | "right"; color "" — снять
// свой цвет. Цвет альбома — SetSettings.
func (m *Methods) SetBackground(spreadID, scope, color string) (v model.View) {
	defer m.guardEdit("SetBackground", &v, nil)
	return m.changed(m.editor().SetBackground(spreadID, scope, color))
}

// AddText: блок «Текст» по центру страницы page (0/1), ширина по тексту. Новый
// блок — последний в SpreadView.Texts. Больше MaxTexts на развороте — ошибка.
func (m *Methods) AddText(spreadID string, page int, session string) (v model.View, err error) {
	defer m.guardEdit("AddText", &v, &err)
	return m.changedOrErr(m.editor().AddText(spreadID, page, session))
}

// SetText: пустой текст (одни пробелы) удаляет блок; одна session — один шаг отмены.
func (m *Methods) SetText(textID, text, session string) (v model.View) {
	defer m.guardEdit("SetText", &v, nil)
	return m.changed(m.editor().SetText(textID, text, session))
}

// SetTextDraft: признак заготовки надписи (131); false — «Сделать обычной надписью»:
// текст из одной пары «[…]» теряет скобки.
func (m *Methods) SetTextDraft(spreadID, textID string, draft bool) (v model.View) {
	defer m.guardEdit("SetTextDraft", &v, nil)
	return m.changed(m.editor().SetTextDraft(spreadID, textID, draft))
}

// SetTextBox: x, y — угол рамки до поворота, мм разворота. Другая w закрепляет
// ширину (auto = false), та же — режим не меняется.
func (m *Methods) SetTextBox(textID string, x, y, w float64) (v model.View) {
	defer m.guardEdit("SetTextBox", &v, nil)
	return m.changed(m.editor().SetTextBox(textID, x, y, w))
}

// SetTextAngle: поворот на deg по часовой вокруг центра блока; Go пересчитывает x, y.
func (m *Methods) SetTextAngle(textID string, deg float64) (v model.View) {
	defer m.guardEdit("SetTextAngle", &v, nil)
	return m.changed(m.editor().SetTextAngle(textID, deg))
}

// SetTextScale — угловая ручка: новый кегль size (пт) и левый верхний угол рамки (x, y)
// до поворота (считает UI: противоположный тянутому угол стоит). Закреплённая ширина
// растёт пропорционально, «по тексту» пересчитывается; поворот стоит.
func (m *Methods) SetTextScale(textID string, size, x, y float64) (v model.View, err error) {
	defer m.guardEdit("SetTextScale", &v, &err)
	return m.changed(m.editor().SetTextScale(textID, size, x, y)), nil
}

// SetTextAuto — ширина блока снова по тексту.
func (m *Methods) SetTextAuto(textID string) (v model.View) {
	defer m.guardEdit("SetTextAuto", &v, nil)
	return m.changed(m.editor().SetTextAuto(textID))
}

// SetSpreadName: "" снимает название; очистка и обрезка до 60 знаков — в Go.
func (m *Methods) SetSpreadName(spreadID, name string) (v model.View) {
	defer m.guardEdit("SetSpreadName", &v, nil)
	return m.changed(m.editor().SetSpreadName(spreadID, name))
}

func (m *Methods) SetTextStyle(textID string, st model.TextStyle) (v model.View) {
	defer m.guardEdit("SetTextStyle", &v, nil)
	return m.changed(m.editor().SetTextStyle(textID, st))
}

func (m *Methods) DeleteText(textID string) (v model.View) {
	defer m.guardEdit("DeleteText", &v, nil)
	return m.changed(m.editor().DeleteText(textID))
}

// SetCaption: пустой текст снимает подпись; одна session — один шаг отмены.
func (m *Methods) SetCaption(photoID, text, session string) (v model.View) {
	defer m.guardEdit("SetCaption", &v, nil)
	return m.changed(m.editor().SetCaption(photoID, text, session))
}

// SetCaptionStyle: over — подпись на фото у нижнего края, а не под ним; pad — отступ от
// фото, мм (0..20, шаг 0,1; на фото — от нижнего края).
func (m *Methods) SetCaptionStyle(photoID string, st model.TextStyle, over bool, pad float64) (v model.View) {
	defer m.guardEdit("SetCaptionStyle", &v, nil)
	return m.changed(m.editor().SetCaptionStyle(photoID, st, over, pad))
}

// SetPerson — имя на фото; все поля пусты — снова из имени файла. Одна session — один шаг.
func (m *Methods) SetPerson(photoID string, p model.Person, session string) (v model.View) {
	defer m.guardEdit("SetPerson", &v, nil)
	return m.changed(m.editor().SetPerson(photoID, p, session))
}

// SetPlacePerson — табличка места (071): имя ждёт фото на месте; все поля пусты — снять.
// Одна session — один шаг.
func (m *Methods) SetPlacePerson(spreadID, placeID string, p model.Person, session string) (v model.View) {
	defer m.guardEdit("SetPlacePerson", &v, nil)
	return m.changed(m.editor().SetPlacePerson(spreadID, placeID, p, session))
}

// ForgetFileNames — «Убрать имена» тоста броска (060): у фото ids снимается имя, взятое из
// названия файла (DropResult.named, GridResult.named), если его не правили после. Один шаг.
func (m *Methods) ForgetFileNames(ids []string) (v model.View) {
	defer m.guardEdit("ForgetFileNames", &v, nil)
	return m.changed(m.editor().ForgetFileNames(ids))
}

// PreviewNames — предпросмотр окна «Имена»: что ляжет на каждое место листа при PasteNames
// с теми же o, строки без фото, столбцы таблицы. Проект не меняется, события нет.
func (m *Methods) PreviewNames(spreadID string, o model.NamesOpts) (plan model.NamesPlan, err error) {
	defer GuardErr("PreviewNames", &err)
	return m.editor().PreviewNames(spreadID, o)
}

// SetSheetCaps — «Один стиль на лист»: on — стиль c у всех подписей листа (свои не
// стираются), off — свои снова действуют. c.text — шаблон подписи к каждому фото.
func (m *Methods) SetSheetCaps(spreadID string, on bool, c model.SheetCaps) (v model.View) {
	defer m.guardEdit("SetSheetCaps", &v, nil)
	return m.changed(m.editor().SetSheetCaps(spreadID, on, c))
}

// SetBackdrop ставит на фон фото из библиотеки или альбома. Фона области не было —
// рамка по области; был — меняется только фото (рамка, поворот, зеркало остаются).
func (m *Methods) SetBackdrop(spreadID, scope, photoID string) (v model.View, err error) {
	defer m.guardEdit("SetBackdrop", &v, &err)
	if !model.ValidBdScope(scope) {
		return m.editor().View(), ErrBackdropScope
	}
	infos := m.infosOrRefs([]string{photoID})
	if len(infos) == 0 {
		return m.editor().View(), ErrNoPhoto
	}
	return m.changedOrErr(m.editor().SetBackdrop(spreadID, scope, infos[0]))
}

// SetBackdropBox: рамка фона (мм разворота, угол до поворота) и поворот, градусы по часовой
// вокруг угла. Зажим — в Go. merge — сессия стрелок: серия — один шаг отмены.
func (m *Methods) SetBackdropBox(spreadID, scope string, x, y, w, h, rot float64, merge string) (v model.View) {
	defer m.guardEdit("SetBackdropBox", &v, nil)
	return m.changed(m.editor().SetBackdropBox(spreadID, scope, x, y, w, h, rot, merge))
}

// TurnBackdrop: поворот рамки фона на rot° вокруг центра; смена ориентации (около 0/180° ↔ ±90°)
// меняет стороны местами. Любая область, один шаг отмены (merge — сессия поля), тот же угол — холостая.
func (m *Methods) TurnBackdrop(spreadID, scope string, rot float64, merge string) (v model.View) {
	defer m.guardEdit("TurnBackdrop", &v, nil)
	return m.changed(m.editor().TurnBackdrop(spreadID, scope, rot, merge))
}

// SetBackdropFrame: кадр фото в рамке — масштаб s 1..4, сдвиг x/y −1..1.
func (m *Methods) SetBackdropFrame(spreadID, scope string, s, x, y float64) (v model.View) {
	defer m.guardEdit("SetBackdropFrame", &v, nil)
	return m.changed(m.editor().SetBackdropFrame(spreadID, scope, s, x, y))
}

// SetBackdropAngle: наклон фото в рамке фона tilt, градусы (−44,9…44,9, шаг 0,1); рамка
// стоит, фото закрывает её само, наклон держит приближение (057). session — как у
// SetFrameAngle: сессия строки «Наклон», один шаг отмены.
func (m *Methods) SetBackdropAngle(spreadID, scope string, tilt float64, session string) (v model.View) {
	defer m.guardEdit("SetBackdropAngle", &v, nil)
	return m.changed(m.editor().SetBackdropAngle(spreadID, scope, tilt, session))
}

// StraightenBackdrop — «Выпрямить по линии» для фона: dx, dy — конец минус начало линии,
// мм разворота; линия встаёт вдоль края рамки фона. Один шаг отмены; ровная — холостая.
func (m *Methods) StraightenBackdrop(spreadID, scope string, dx, dy float64) (v model.View) {
	defer m.guardEdit("StraightenBackdrop", &v, nil)
	return m.changed(m.editor().StraightenBackdrop(spreadID, scope, dx, dy))
}

// SetBackdropFade: прозрачность фото фона 0..1 (0 — непрозрачно), зажим и шаг 0.01 — в Go.
// merge — сессия ползунка: серия — один шаг отмены.
func (m *Methods) SetBackdropFade(spreadID, scope string, fade float64, merge string) (v model.View) {
	defer m.guardEdit("SetBackdropFade", &v, nil)
	return m.changed(m.editor().SetBackdropFade(spreadID, scope, fade, merge))
}

// SetPhotoFade: прозрачность фото ids (ID фото, не мест; 079-36) 0..1, шаг 0.01 — в Go; у фото
// только на фоне или в рамочке — холостая. Серия с одним merge — один шаг; NaN — тот же rev.
func (m *Methods) SetPhotoFade(ids []string, fade float64, merge string) (v model.View) {
	defer m.guardEdit("SetPhotoFade", &v, nil)
	return m.changed(m.editor().SetPhotoFade(ids, fade, merge))
}

// FlipBackdrop: axis "h" (↔) | "v" (↕) — переключить зеркало; видимая часть кадра та же.
func (m *Methods) FlipBackdrop(spreadID, scope, axis string) (v model.View) {
	defer m.guardEdit("FlipBackdrop", &v, nil)
	return m.changed(m.editor().FlipBackdrop(spreadID, scope, axis))
}

// FitBackdrop — «По области»: рамка = область, поворот рамки и кадр сброшены, наклон фото и зеркало остаются.
func (m *Methods) FitBackdrop(spreadID, scope string) (v model.View) {
	defer m.guardEdit("FitBackdrop", &v, nil)
	return m.changed(m.editor().FitBackdrop(spreadID, scope))
}

// ClearBackdrop снимает фото фона; цвет страниц остаётся.
func (m *Methods) ClearBackdrop(spreadID, scope string) (v model.View) {
	defer m.guardEdit("ClearBackdrop", &v, nil)
	return m.changed(m.editor().ClearBackdrop(spreadID, scope))
}

// Фон альбома (098 §3): ставит его App.SetAlbumBg (нужен каталог библиотеки), здесь — правки строки
// «Фон альбома» и карточки листа.

// ClearAlbumBg — «Убрать» фон альбома; свои фоны листов остаются. Нет фона — холостая.
func (m *Methods) ClearAlbumBg() (v model.View) {
	defer m.guardEdit("ClearAlbumBg", &v, nil)
	return m.changed(m.editor().ClearAlbumBg())
}

// SetAlbumBgFade: прозрачность фона альбома 0..1, зажим и шаг 0.01 — в Go. merge — сессия
// ползунка: серия — один шаг отмены (вызывать на отпускании).
func (m *Methods) SetAlbumBgFade(fade float64, merge string) (v model.View) {
	defer m.guardEdit("SetAlbumBgFade", &v, nil)
	return m.changed(m.editor().SetAlbumBgFade(fade, merge))
}

// SetNoAlbumBg — «Не показывать здесь» (off = true) и «Показать» фона альбома на развороте;
// обложка — холостая.
func (m *Methods) SetNoAlbumBg(spreadID string, off bool) (v model.View) {
	defer m.guardEdit("SetNoAlbumBg", &v, nil)
	return m.changed(m.editor().SetNoAlbumBg(spreadID, off))
}

// SpreadBgToAlbum — карточка разворота: свой узор, фото фона и «Не показывать здесь» сняты, фон
// альбома виден (115). Нет фона альбома, обложка и неизвестный ID — холостая.
func (m *Methods) SpreadBgToAlbum(spreadID string) (v model.View) {
	defer m.guardEdit("SpreadBgToAlbum", &v, nil)
	return m.changed(m.editor().SpreadBgToAlbum(spreadID))
}
