package app

import (
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
)

// Подгон кадров по лицам (спека 130, design §6): снимок мест листа → поиск лиц в фоне (кэш
// точек, ход faces:progress) → одна правка Editor.FitFaces → вид. Тонкий фасад без службы
// (0.27-130-9): расчёт и отчёт — в design.FitFaces, детектор — в internal/faces.

// FacesResult — ответ FitFaces: вид после правки (при Report.Asked правки нет — вид прежний) и отчёт.
type FacesResult struct {
	View   model.View       `json:"view"`
	Report model.FaceReport `json:"report"`
}

// Тексты уходят пользователю тостом как есть (с заглавной, как ownprint.ErrDup).
var (
	errFacesBusy   = errors.New("Уже ищу лица")                                //nolint:staticcheck // текст тоста
	errFacesExport = errors.New("Идёт экспорт: подгоните по лицам после него") //nolint:staticcheck // текст тоста
	errFacesMode   = errors.New("неизвестный режим подгона")
	errFacesSheet  = errors.New("Нет такого разворота")                //nolint:staticcheck // текст тоста
	errFacesMoved  = errors.New("Альбом сменился во время поиска лиц") //nolint:staticcheck // текст тоста
)

// faceState — состояние подгона по лицам у App.
type faceState struct {
	// busy — поиск идёт: второй вызов отвечает ошибкой, а не встаёт в очередь.
	busy  atomic.Bool
	cache faceCache
	miss  atomic.Int64 // не прочитанные фото — для редкой строки журнала
	// detect — детектор; nil — faces.Detect (тесты подставляют свой).
	detect func(image.Image) []faces.Face
	// ready — проверка модели; nil — faces.Ready (тесты подставляют сломанную). readyLogged — сбой
	// модели записан в журнал: строка одна на процесс, ответ пользователю — на каждый вызов.
	ready       func() error
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

// modelReady — модель лиц загружена: первый вызов её разбирает (≈ 0,5 мс, дёшево), сбой — одна
// строка в журнал и понятная ошибка вместо молчаливого «нет фото» на каждом месте.
func (a *App) modelReady() error {
	ready := a.faceSt.ready
	if ready == nil {
		ready = faces.Ready
	}
	err := ready()
	if err == nil {
		return nil
	}
	if a.faceSt.readyLogged.CompareAndSwap(false, true) {
		log.Printf("лица: %v", err)
	}
	cause := errors.Unwrap(err)
	if cause == nil {
		cause = err
	}
	return fmt.Errorf("Поиск лиц недоступен: %w", cause) //nolint:staticcheck // текст тоста
}

// ownLocked сбрасывает память, если альбом другой. Под mu.
func (s *faceState) ownLocked(ed *album.Editor) {
	if s.owner.Value() == ed && s.fitted != nil {
		return
	}
	s.owner, s.fitted, s.norm = weak.Make(ed), map[string]model.Frame{}, map[string]geom.FaceNorm{}
}

// memory — копии памяти листа для правки: Editor только читает их под своим замком, а карты
// продолжают жить у App.
func (s *faceState) memory(ed *album.Editor, spreadID string) (norm *geom.FaceNorm, fitted map[string]model.Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ownLocked(ed)
	if n, ok := s.norm[spreadID]; ok {
		norm = &n
	}
	return norm, maps.Clone(s.fitted)
}

// normOf — норма листа из памяти (после «Остальные — как это лицо») или nil; без копии кадров.
// Память чужого альбома не трогается и не сбрасывается: для него нормы нет.
func (s *faceState) normOf(ed *album.Editor, spreadID string) *geom.FaceNorm {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner.Value() != ed || s.norm == nil {
		return nil
	}
	if n, ok := s.norm[spreadID]; ok {
		return &n
	}
	return nil
}

// remember запоминает итог поставленной правки: кадры фото и, если шло «как это лицо», норму листа.
func (s *faceState) remember(ed *album.Editor, spreadID string, set map[string]model.Frame, byRef bool, n geom.FaceNorm) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ownLocked(ed)
	maps.Copy(s.fitted, set)
	if byRef {
		s.norm[spreadID] = n
	}
}

// FitFaces — «Подогнать по лицам» на листе spreadID (виньетка): кадры фото ставятся по лицам,
// одна правка — один шаг отмены. ref — место-образец «Остальные — как это лицо» ("" — норма листа
// из памяти или начальная 0,35 / 0,32). mode: "" — спросить про правленые кадры (Report.Asked,
// правки нет), "keep" — оставить их, "all" — подогнать и их. Ход — событие faces:progress
// (done, total). Ошибки: уже ищу, идёт экспорт, модель лиц не загрузилась, нет листа, образец
// негоден (правки нет).
func (a *App) FitFaces(spreadID, ref, mode string) (res FacesResult, err error) {
	defer a.guardEdit("FitFaces", &res.View, &err)
	if mode != "" && mode != "keep" && mode != "all" {
		return res, errFacesMode
	}
	if running, _ := a.exp.Busy(); running { // подгон занимает ядра, экспорт тоже
		return res, errFacesExport
	}
	if !a.faceSt.busy.CompareAndSwap(false, true) {
		return res, errFacesBusy
	}
	defer a.faceSt.busy.Store(false)
	if err := a.modelReady(); err != nil {
		return res, err
	}

	ed := a.editor()
	hits, ok := ed.FaceSlots(spreadID) // снимок до поиска: правка во время него — «изменили»
	if !ok {
		return res, errFacesSheet
	}
	if err := a.findFaces(ed, hits); err != nil {
		return res, err
	}
	norm, fitted := a.faceSt.memory(ed, spreadID)
	v, rep, used := ed.FitFaces(spreadID, hits, ref, mode, norm, fitted)
	if rep.RefErr != "" { // образец негоден: правки нет, норма листа прежняя
		return res, errors.New(model.RefErrText(rep.RefErr))
	}
	if !rep.Asked { // спросили — ничего не поставлено: отмена окна не должна менять норму
		a.faceSt.remember(ed, spreadID, rep.Set, ref != "", used)
	}
	res.Report = rep
	res.View = a.changed(v)
	return res, nil
}
