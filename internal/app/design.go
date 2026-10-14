package app

import (
	"errors"
	"log"
	"path/filepath"
	"runtime/debug"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// errInternal — правка упала с паникой: проект не изменён, подробности в log.txt.
var errInternal = errors.New("внутренняя ошибка: правка не применена, подробности в журнале")

// guardEdit — guard для методов правок: паника уходит в лог, интерфейс получает
// текущий вид (а не пустой — иначе он нарисовал бы пустой альбом) и ошибку:
// через err, если метод её возвращает, иначе тостом.
func (a *App) guardEdit(name string, v *model.View, err *error) {
	r := recover()
	if r == nil {
		return
	}
	log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
	func() {
		defer guard(name + ": вид после паники")
		*v = a.editor().View()
	}()
	if err != nil {
		*err = errInternal
		return
	}
	a.notice(errInternal.Error())
}

// SetFrame ставит кадр фото: s 1..4, x/y −1..1 (зажимается); s=1, x=y=0 — сброс.
// Каждый вызов — шаг отмены.
func (a *App) SetFrame(photoID string, s, x, y float64) (v model.View) {
	defer a.guardEdit("SetFrame", &v, nil)
	return a.changed(a.editor().SetFrame(photoID, s, x, y))
}

// ResetFrame — кадр по умолчанию (то же, что SetFrame(id, 1, 0, 0)).
func (a *App) ResetFrame(photoID string) (v model.View) {
	defer a.guardEdit("ResetFrame", &v, nil)
	return a.changed(a.editor().ResetFrame(photoID))
}

// SetFrameAngle: наклон фото в рамке tilt, градусы (−44,9…44,9, шаг 0,1); масштаб,
// сдвиг и пропорции стоят. Вызывать на отпускании ползунка (один шаг отмены).
func (a *App) SetFrameAngle(photoID string, tilt float64) (v model.View) {
	defer a.guardEdit("SetFrameAngle", &v, nil)
	return a.changed(a.editor().SetFrameAngle(photoID, tilt))
}

// TurnPhotos поворачивает фото ids на quarters·90° (−1 — ↺, 1 — ↻, 2 — 180°; иное —
// холостая) одной правкой; «Авто» перестраивается под новые пропорции.
func (a *App) TurnPhotos(ids []string, quarters int) (v model.View) {
	defer a.guardEdit("TurnPhotos", &v, nil)
	return a.changed(a.editor().TurnPhotos(ids, quarters))
}

// SetFlip зеркалит фото ids: axis "h" (↔) | "v" (↕); не у всех включено — всем включить,
// иначе всем выключить. Раскладку не трогает.
func (a *App) SetFlip(ids []string, axis string) (v model.View) {
	defer a.guardEdit("SetFlip", &v, nil)
	return a.changed(a.editor().SetFlip(ids, axis))
}

// SetBackground: scope "spread" (обе страницы) | "left" | "right"; color "" — снять
// свой цвет. Цвет альбома — SetSettings.
func (a *App) SetBackground(spreadID, scope, color string) (v model.View) {
	defer a.guardEdit("SetBackground", &v, nil)
	return a.changed(a.editor().SetBackground(spreadID, scope, color))
}

// AddText: блок «Текст» по центру страницы page (0/1), ширина по тексту. Новый
// блок — последний в SpreadView.Texts. Больше MaxTexts на развороте — ошибка.
func (a *App) AddText(spreadID string, page int, session string) (v model.View, err error) {
	defer a.guardEdit("AddText", &v, &err)
	return a.changedOrErr(a.editor().AddText(spreadID, page, session))
}

// SetText: пустой текст (одни пробелы) удаляет блок; одна session — один шаг отмены.
func (a *App) SetText(textID, text, session string) (v model.View) {
	defer a.guardEdit("SetText", &v, nil)
	return a.changed(a.editor().SetText(textID, text, session))
}

// SetTextBox: x, y — угол рамки до поворота, мм разворота. Другая w закрепляет
// ширину (auto = false), та же — режим не меняется.
func (a *App) SetTextBox(textID string, x, y, w float64) (v model.View) {
	defer a.guardEdit("SetTextBox", &v, nil)
	return a.changed(a.editor().SetTextBox(textID, x, y, w))
}

// SetTextAngle: поворот на deg по часовой вокруг центра блока; Go пересчитывает x, y.
func (a *App) SetTextAngle(textID string, deg float64) (v model.View) {
	defer a.guardEdit("SetTextAngle", &v, nil)
	return a.changed(a.editor().SetTextAngle(textID, deg))
}

// SetTextScale — угловая ручка: новый кегль size (пт) и левый верхний угол рамки (x, y)
// до поворота (считает UI: противоположный тянутому угол стоит). Закреплённая ширина
// растёт пропорционально, «по тексту» пересчитывается; поворот стоит.
func (a *App) SetTextScale(textID string, size, x, y float64) (v model.View, err error) {
	defer a.guardEdit("SetTextScale", &v, &err)
	return a.changed(a.editor().SetTextScale(textID, size, x, y)), nil
}

// SetTextAuto — ширина блока снова по тексту.
func (a *App) SetTextAuto(textID string) (v model.View) {
	defer a.guardEdit("SetTextAuto", &v, nil)
	return a.changed(a.editor().SetTextAuto(textID))
}

// SetSpreadName: "" снимает название; очистка и обрезка до 60 знаков — в Go.
func (a *App) SetSpreadName(spreadID, name string) (v model.View) {
	defer a.guardEdit("SetSpreadName", &v, nil)
	return a.changed(a.editor().SetSpreadName(spreadID, name))
}

func (a *App) SetTextStyle(textID string, st model.TextStyle) (v model.View) {
	defer a.guardEdit("SetTextStyle", &v, nil)
	return a.changed(a.editor().SetTextStyle(textID, st))
}

func (a *App) DeleteText(textID string) (v model.View) {
	defer a.guardEdit("DeleteText", &v, nil)
	return a.changed(a.editor().DeleteText(textID))
}

// SetCaption: пустой текст снимает подпись; одна session — один шаг отмены.
func (a *App) SetCaption(photoID, text, session string) (v model.View) {
	defer a.guardEdit("SetCaption", &v, nil)
	return a.changed(a.editor().SetCaption(photoID, text, session))
}

// SetCaptionStyle: over — подпись на фото у нижнего края, а не под ним; pad — отступ от
// фото, мм (0..20, шаг 0,1; на фото — от нижнего края).
func (a *App) SetCaptionStyle(photoID string, st model.TextStyle, over bool, pad float64) (v model.View) {
	defer a.guardEdit("SetCaptionStyle", &v, nil)
	return a.changed(a.editor().SetCaptionStyle(photoID, st, over, pad))
}

// Поля подписей (спека 030): переменные альбома, имя на фото, список имён, стиль листа.

// NamesResult — вид после вставки списка имён: Extra — лишних строк, Missing — фото листа
// без имени (окно «Имена» показывает «строк лишних: N / фото без имени: M»).
type NamesResult struct {
	View    model.View `json:"view"`
	Extra   int        `json:"extra"`
	Missing int        `json:"missing"`
}

// SetPerson — имя на фото; все поля пусты — снова из имени файла. Одна session — один шаг.
func (a *App) SetPerson(photoID string, p model.Person, session string) (v model.View) {
	defer a.guardEdit("SetPerson", &v, nil)
	return a.changed(a.editor().SetPerson(photoID, p, session))
}

// Person — имя фото для карточки: своё или из имени файла (fromFile). Фото ещё не в
// альбоме — из имени файла ленты.
func (a *App) Person(photoID string) model.PersonView {
	defer guard("Person")
	if pv, ok := a.editor().Person(photoID); ok {
		return pv
	}
	a.mu.Lock()
	ph, ok := a.byID[photoID]
	a.mu.Unlock()
	if !ok {
		return model.PersonView{}
	}
	return model.NameView(model.PhotoRef{Path: ph.Path})
}

// PasteNames — окно «Имена»: строки «Фамилия Имя [Отчество]» по порядку фото листа
// (firstFirst — «Имя Фамилия»), один шаг отмены. Список больше 1000 строк — ошибка.
func (a *App) PasteNames(spreadID, text string, firstFirst bool) (r NamesResult, err error) {
	defer a.guardEdit("PasteNames", &r.View, &err)
	v, extra, missing, err := a.editor().PasteNames(spreadID, text, firstFirst)
	r.View, err = a.changedOrErr(v, err)
	r.Extra, r.Missing = extra, missing
	return r, err
}

// SetSheetCaps — «Один стиль на лист»: on — стиль c у всех подписей листа (свои не
// стираются), off — свои снова действуют. c.text — шаблон подписи к каждому фото.
func (a *App) SetSheetCaps(spreadID string, on bool, c model.SheetCaps) (v model.View) {
	defer a.guardEdit("SetSheetCaps", &v, nil)
	return a.changed(a.editor().SetSheetCaps(spreadID, on, c))
}

// Фото на фоне разворота или страницы (спека 009, design-backdrop §5, §7).
// scope: "spread" (весь разворот) | "left" | "right".

// dirBackdrop — своя последняя папка у диалога «Свой файл…».
const dirBackdrop = "backdrop"

var (
	errNoPhoto       = errors.New("фото не найдено")
	errBackdropScope = errors.New("неизвестная область фона")
)

// openFileDialog — нативный диалог выбора файла; в тестах подменяется.
var openFileDialog = wr.OpenFileDialog

// SetBackdrop ставит на фон фото из библиотеки или альбома. Фона области не было —
// рамка по области; был — меняется только фото (рамка, поворот, зеркало остаются).
func (a *App) SetBackdrop(spreadID, scope, photoID string) (v model.View, err error) {
	defer a.guardEdit("SetBackdrop", &v, &err)
	if !model.ValidScope(scope) {
		return a.editor().View(), errBackdropScope
	}
	infos := a.infosOrRefs([]string{photoID})
	if len(infos) == 0 {
		return a.editor().View(), errNoPhoto
	}
	return a.changedOrErr(a.editor().SetBackdrop(spreadID, scope, infos[0]))
}

// PickBackdropFile — «Свой файл…»: диалог выбора в Go, путь от интерфейса не принимается.
// Отказ от диалога — вид без изменений и без ошибки (фона не появилось).
func (a *App) PickBackdropFile(spreadID, scope string) (v model.View, err error) {
	defer a.guardEdit("PickBackdropFile", &v, &err)
	if !model.ValidScope(scope) {
		return a.editor().View(), errBackdropScope
	}
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Фото для фона",
		DefaultDirectory: a.dialogDir(dirBackdrop),
		Filters:          []wr.FileFilter{{DisplayName: "Фото (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"}},
	})
	if err != nil || path == "" {
		return a.editor().View(), err
	}
	v, err = a.setBackdropFile(spreadID, scope, path)
	if err == nil {
		a.rememberDir(dirBackdrop, filepath.Dir(path))
	}
	return v, err
}

// setBackdropFile проверяет выбранный файл так же строго, как фото папки, и ставит его на фон.
func (a *App) setBackdropFile(spreadID, scope, path string) (model.View, error) {
	if !model.ValidScope(scope) {
		return a.editor().View(), errBackdropScope
	}
	ph, err := library.BackdropFile(path)
	if err != nil {
		return a.editor().View(), err
	}
	return a.changedOrErr(a.editor().SetBackdrop(spreadID, scope, model.PhotoInfo{ID: ph.ID, Path: ph.Path, W: ph.W, H: ph.H, Size: ph.Size}))
}

// SetBackdropBox: рамка фона (мм разворота, угол до поворота) и поворот, градусы по часовой
// вокруг угла. Зажим — в Go. merge — сессия стрелок: серия — один шаг отмены.
func (a *App) SetBackdropBox(spreadID, scope string, x, y, w, h, rot float64, merge string) (v model.View) {
	defer a.guardEdit("SetBackdropBox", &v, nil)
	return a.changed(a.editor().SetBackdropBox(spreadID, scope, x, y, w, h, rot, merge))
}

// SetBackdropFrame: кадр фото в рамке — масштаб s 1..4, сдвиг x/y −1..1.
func (a *App) SetBackdropFrame(spreadID, scope string, s, x, y float64) (v model.View) {
	defer a.guardEdit("SetBackdropFrame", &v, nil)
	return a.changed(a.editor().SetBackdropFrame(spreadID, scope, s, x, y))
}

// SetBackdropFade: прозрачность фото фона 0..1 (0 — непрозрачно), зажим и шаг 0.01 — в Go.
// merge — сессия ползунка: серия — один шаг отмены.
func (a *App) SetBackdropFade(spreadID, scope string, fade float64, merge string) (v model.View) {
	defer a.guardEdit("SetBackdropFade", &v, nil)
	return a.changed(a.editor().SetBackdropFade(spreadID, scope, fade, merge))
}

// FlipBackdrop: axis "h" (↔) | "v" (↕) — переключить зеркало; видимая часть кадра та же.
func (a *App) FlipBackdrop(spreadID, scope, axis string) (v model.View) {
	defer a.guardEdit("FlipBackdrop", &v, nil)
	return a.changed(a.editor().FlipBackdrop(spreadID, scope, axis))
}

// FitBackdrop — «По области»: рамка = область, поворот и кадр сброшены, зеркало остаётся.
func (a *App) FitBackdrop(spreadID, scope string) (v model.View) {
	defer a.guardEdit("FitBackdrop", &v, nil)
	return a.changed(a.editor().FitBackdrop(spreadID, scope))
}

// ClearBackdrop снимает фото фона; цвет страниц остаётся.
func (a *App) ClearBackdrop(spreadID, scope string) (v model.View) {
	defer a.guardEdit("ClearBackdrop", &v, nil)
	return a.changed(a.editor().ClearBackdrop(spreadID, scope))
}
