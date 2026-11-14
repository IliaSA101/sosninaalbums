// Package facefit — «Подогнать по лицам» (спека 130): снимок мест листа, поиск лиц в фоне,
// кэш точек, память сессии (кадры подгона и норма листа) и силуэт под лицо. Вынесен из
// internal/app: методы App для JS (FitFaces, FaceGuide) — тонкие обёртки с guardEdit, поэтому
// набор методов и биндинги прежние. Правка проекта одна, через Editor.FitFaces; вид после неё
// пропускает Host.Changed (автосохранение, рассылка), как у остальных правок.
package facefit

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"maps"
	"sync"
	"sync/atomic"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Подгон кадров по лицам (спека 130, design §6): снимок мест листа → поиск лиц в фоне (кэш
// точек, ход faces:progress) → одна правка Editor.FitFaces → вид. Расчёт и отчёт — в
// design.FitFaces, детектор — в internal/faces.

// Host — что подгону нужно от App; поля задаёт App, пустое обязательное — паника в New.
type Host struct {
	Editor func() *album.Editor
	// Photo — фото места: ok — известно, gone — файла нет или том не отвечает, cloud — фото проекта в
	// облаке (заглушка OneDrive, 160-M1): скачивать её нельзя, берётся только готовая копия.
	Photo func(id string) (p library.Photo, ok, gone, cloud bool)
	// Copy — файл превью фото id своего размера (расчёт с памятью: чужой размер не годится); для
	// фото в облаке — только готовая копия кэша, иначе library.ErrCloudOnly.
	Copy func(ctx context.Context, id string, p library.Photo, cloud bool) (string, error)
	// Ctx — отмена при закрытии программы: родитель поиска.
	Ctx func() context.Context
	// Go запускает fn в фоне App; false — фон закрыт, программа выходит.
	Go func(fn func()) bool
	// ExportBusy — идёт экспорт: подгон и экспорт оба занимают ядра.
	ExportBusy func() bool
	// Emit шлёт событие окну (faces:progress). Только поле App: тесты подменяют его после NewApp.
	Emit    func(name string, args ...any)
	Changed func(model.View) model.View // после правки: автосохранение и рассылка
	// Rare — редкая строка журнала по счётчику; можно не задавать.
	Rare func(n int64, format string, args ...any)
}

// Тексты уходят пользователю тостом как есть (с заглавной, как ownprint.ErrDup).
var (
	ErrBusy   = errors.New("Уже ищу лица")                                //nolint:staticcheck // текст тоста
	ErrExport = errors.New("Идёт экспорт: подгоните по лицам после него") //nolint:staticcheck // текст тоста
	ErrMode   = errors.New("неизвестный режим подгона")
	ErrSheet  = errors.New("Нет такого разворота")                //nolint:staticcheck // текст тоста
	ErrMoved  = errors.New("Альбом сменился во время поиска лиц") //nolint:staticcheck // текст тоста
)

// Fitter — подгон по лицам одного App: занятость, кэш точек, память сессии.
type Fitter struct {
	h Host
	// busy — поиск идёт: второй вызов отвечает ошибкой, а не встаёт в очередь.
	busy  atomic.Bool
	cache faceCache
	miss  atomic.Int64 // не прочитанные фото — для редкой строки журнала
	// Detect — детектор; nil — faces.Detect. Поле для тестов App: они подставляют свой до вызовов.
	Detect func(image.Image) []faces.Face
	// Ready — проверка модели; nil — faces.Ready. Поле для тестов App (сломанная модель).
	// readyLogged — сбой модели записан в журнал: строка одна на процесс, ответ пользователю —
	// на каждый вызов.
	Ready       func() error
	readyLogged atomic.Bool

	// Память сессии под mu: кадры, что поставил подгон (фото → кадр: они не «правленые»), и норма
	// каждого листа после «Остальные — как это лицо» (0.27-130-17). Принадлежат одному альбому:
	// открытие другого начинает их заново. После перезапуска пусты: окно спросит там, где лицо
	// мимо нормы.
	mu     sync.Mutex
	owner  weak.Pointer[album.Editor]
	fitted map[string]model.Frame
	norm   map[string]geom.FaceNorm
}

// New — подгон поверх h. Пустое обязательное поле Host — паника сразу и по имени, а не
// nil-вызов посреди поиска.
func New(h Host) *Fitter {
	switch {
	case h.Editor == nil:
		panic("facefit.New: не задано поле Host.Editor")
	case h.Photo == nil:
		panic("facefit.New: не задано поле Host.Photo")
	case h.Copy == nil:
		panic("facefit.New: не задано поле Host.Copy")
	case h.Ctx == nil:
		panic("facefit.New: не задано поле Host.Ctx")
	case h.Go == nil:
		panic("facefit.New: не задано поле Host.Go")
	case h.ExportBusy == nil:
		panic("facefit.New: не задано поле Host.ExportBusy")
	case h.Emit == nil:
		panic("facefit.New: не задано поле Host.Emit")
	case h.Changed == nil:
		panic("facefit.New: не задано поле Host.Changed")
	}
	if h.Rare == nil {
		h.Rare = func(int64, string, ...any) {}
	}
	return &Fitter{h: h}
}

// modelReady — модель лиц загружена: первый вызов её разбирает (≈ 0,5 мс, дёшево), сбой — одна
// строка в журнал и понятная ошибка вместо молчаливого «нет фото» на каждом месте.
func (f *Fitter) modelReady() error {
	ready := f.Ready
	if ready == nil {
		ready = faces.Ready
	}
	err := ready()
	if err == nil {
		return nil
	}
	if f.readyLogged.CompareAndSwap(false, true) {
		log.Printf("лица: %v", err)
	}
	cause := errors.Unwrap(err)
	if cause == nil {
		cause = err
	}
	return fmt.Errorf("Поиск лиц недоступен: %w", cause) //nolint:staticcheck // текст тоста
}

// ownLocked сбрасывает память, если альбом другой. Под mu.
func (f *Fitter) ownLocked(ed *album.Editor) {
	if f.owner.Value() == ed && f.fitted != nil {
		return
	}
	f.owner, f.fitted, f.norm = weak.Make(ed), map[string]model.Frame{}, map[string]geom.FaceNorm{}
}

// Memory — копии памяти листа для правки: Editor только читает их под своим замком, а карты
// продолжают жить у Fitter. Экспорт для тестов App.
func (f *Fitter) Memory(ed *album.Editor, spreadID string) (norm *geom.FaceNorm, fitted map[string]model.Frame) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ownLocked(ed)
	if n, ok := f.norm[spreadID]; ok {
		norm = &n
	}
	return norm, maps.Clone(f.fitted)
}

// NormOf — норма листа из памяти (после «Остальные — как это лицо») или nil; без копии кадров.
// Память чужого альбома не трогается и не сбрасывается: для него нормы нет.
func (f *Fitter) NormOf(ed *album.Editor, spreadID string) *geom.FaceNorm {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owner.Value() != ed || f.norm == nil {
		return nil
	}
	if n, ok := f.norm[spreadID]; ok {
		return &n
	}
	return nil
}

// remember запоминает итог поставленной правки: кадры фото и, если шло «как это лицо», норму листа.
func (f *Fitter) remember(ed *album.Editor, spreadID string, set map[string]model.Frame, byRef bool, n geom.FaceNorm) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ownLocked(ed)
	maps.Copy(f.fitted, set)
	if byRef {
		f.norm[spreadID] = n
	}
}

// Fit — «Подогнать по лицам» на листе spreadID (виньетка): кадры фото ставятся по лицам, одна
// правка — один шаг отмены. ref — место-образец «Остальные — как это лицо» ("" — норма листа
// из памяти или начальная 0,35 / 0,32). mode: "" — спросить про правленые кадры (Report.Asked,
// правки нет), "keep" — оставить их, "all" — подогнать и их. Ход — событие faces:progress
// (done, total). Ошибки: уже ищу, идёт экспорт, модель лиц не загрузилась, нет листа, образец
// негоден (правки нет). Вид — после Host.Changed; при Report.Asked правки нет, вид прежний (после Host.Changed).
func (f *Fitter) Fit(spreadID, ref, mode string) (v model.View, rep model.FaceReport, err error) {
	if mode != "" && mode != "keep" && mode != "all" {
		return v, rep, ErrMode
	}
	if f.h.ExportBusy() { // подгон занимает ядра, экспорт тоже
		return v, rep, ErrExport
	}
	if !f.busy.CompareAndSwap(false, true) {
		return v, rep, ErrBusy
	}
	defer f.busy.Store(false)
	if err := f.modelReady(); err != nil {
		return v, rep, err
	}

	ed := f.h.Editor()
	hits, ok := ed.FaceSlots(spreadID) // снимок до поиска: правка во время него — «изменили»
	if !ok {
		return v, rep, ErrSheet
	}
	if err := f.findFaces(ed, hits); err != nil {
		return v, rep, err
	}
	norm, fitted := f.Memory(ed, spreadID)
	v, rep, used := ed.FitFaces(spreadID, hits, ref, mode, norm, fitted)
	if rep.RefErr != "" { // образец негоден: правки нет, норма листа прежняя
		return v, rep, errors.New(model.RefErrText(rep.RefErr))
	}
	if !rep.Asked { // спросили — ничего не поставлено: отмена окна не должна менять норму
		f.remember(ed, spreadID, rep.Set, ref != "", used)
	}
	return f.h.Changed(v), rep, nil
}
