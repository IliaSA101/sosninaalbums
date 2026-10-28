package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Подгон по лицам в App (спека 130, G4): детектор подменён полем faceSt.detect, фото — настоящие
// маленькие JPEG, читаются через кэш превью, как в работе. Лицо i-го фото задаёт правило rule:
// по умолчанию размер 0,18…0,25 кадра (i%8), ровно по центру, глаза на 0,38 высоты рамки.

// facesEnv — лист-виньетка 204×304 с n портретами 3:4 (длинная сторона файла 200+i: по ней
// детектор узнаёт фото) и сборщик событий faces:progress.
type facesEnv struct {
	a     *App
	ed    *album.Editor
	sid   string
	dir   string
	files []string
	ids   []string

	calls atomic.Int32
	// rule — лица i-го фото; nil — стандартное.
	rule func(i int) []faces.Face

	mu     sync.Mutex
	events [][2]int
}

// faceBox — рамка и глаза лица высотой h кадра, по центру по X.
func faceBox(h float64) faces.Face {
	b := faces.Rect{X: 0.5 - h*0.375, Y: 0.22, W: h * 0.75, H: h}
	e := faces.Pt{X: 0.5, Y: b.Y + 0.38*h}
	return faces.Face{Score: 0.9, Box: b, Eyes: [2]faces.Pt{{X: e.X - 0.02, Y: e.Y}, {X: e.X + 0.02, Y: e.Y}}}
}

func defaultFace(i int) []faces.Face { return []faces.Face{faceBox(0.18 + 0.01*float64(i%8))} }

func newFacesEnv(t *testing.T, n int) *facesEnv {
	t.Helper()
	e := &facesEnv{a: newApp(t), dir: testkit.TempDir(t)}
	e.a.cfgDir = testkit.TempDir(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	// Кэш закрывает closeBackground из newApp (a.cache): отдельный хук закрыл бы его при идущем фоне.
	e.a.cache = cache
	e.a.emit = func(name string, data ...any) {
		if name == "faces:progress" {
			e.mu.Lock()
			e.events = append(e.events, [2]int{data[0].(int), data[1].(int)})
			e.mu.Unlock()
		}
	}
	e.a.faceSt.detect = func(img image.Image) []faces.Face {
		e.calls.Add(1)
		b := img.Bounds()
		i := max(b.Dx(), b.Dy()) - 200
		if e.rule != nil {
			return e.rule(i)
		}
		return defaultFace(i)
	}
	e.ed = mustEditor(t, album.NewVignetteSheet(204, 304), "")
	e.sid = e.ed.View().Spreads[0].ID
	infos := make([]model.PhotoInfo, n)
	for i := range n {
		long := 200 + i
		path := testkit.JPEG(t, e.dir, fmt.Sprintf("p%02d.jpg", i), long*3/4, long)
		e.files = append(e.files, path)
		e.ids = append(e.ids, library.IDFor(path))
		infos[i] = model.PhotoInfo{ID: e.ids[i], Path: path, W: long * 3 / 4, H: long}
	}
	e.ed.FillPlaces(infos, e.sid)
	e.a.ed.Store(e.ed)
	return e
}

func (e *facesEnv) frame(id string) model.Frame {
	p, _ := e.ed.Snapshot()
	return p.Photos[id].FrameOrDefault()
}

func (e *facesEnv) rev() int { return e.ed.View().Rev }

// placeOf — ID места, где стоит i-е фото.
func (e *facesEnv) placeOf(t *testing.T, i int) string {
	t.Helper()
	p, _ := e.ed.Snapshot()
	for _, pl := range p.Sheet(e.sid).Grid {
		if pl.Photo == e.ids[i] {
			return pl.ID
		}
	}
	t.Fatalf("фото %d не стоит на листе", i)
	return ""
}

func (e *facesEnv) progress() [][2]int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([][2]int(nil), e.events...)
}

// fit зовёт FitFaces и ждёт ответ; ошибка — провал.
func (e *facesEnv) fit(t *testing.T, ref, mode string) FacesResult {
	t.Helper()
	res, err := e.a.FitFaces(e.sid, ref, mode)
	if err != nil {
		t.Fatalf("FitFaces(%q, %q): %v", ref, mode, err)
	}
	return res
}

func wantReport(t *testing.T, r model.FaceReport, fitted, manual int) {
	t.Helper()
	if r.Fitted != fitted || r.Manual != manual || len(r.Check) != 0 || len(r.Short) != 0 || r.Asked {
		t.Fatalf("отчёт %+v: ждали подогнано %d, не тронуто вручную %d, без остальных групп", r, fitted, manual)
	}
}

// TestAppFitFacesReport: лист из 25 мест — одна правка и один шаг отмены, отчёт в нужной форме,
// повтор холостой (rev прежний) и без поиска (кэш), JSON — как ждёт интерфейс.
func TestAppFitFacesReport(t *testing.T) {
	e := newFacesEnv(t, 25)
	rev0 := e.rev()
	res := e.fit(t, "", "")
	wantReport(t, res.Report, 25, 0)
	if e.rev() != rev0+1 || res.View.Rev != rev0+1 {
		t.Fatalf("rev %d → %d (вид %d): ждали одну правку", rev0, e.rev(), res.View.Rev)
	}
	if got := e.calls.Load(); got != 25 {
		t.Fatalf("поисков %d, ждали 25", got)
	}
	for i, id := range e.ids {
		if f := e.frame(id); f.S < 1 || f.S > 4 || f == (model.Frame{S: 1}) && i%8 != 7 {
			t.Fatalf("кадр фото %d не поставлен: %+v", i, f)
		}
	}

	again := e.fit(t, "", "")
	wantReport(t, again.Report, 25, 0)
	if e.rev() != rev0+1 {
		t.Fatalf("повтор не холостой: rev %d", e.rev())
	}
	if got := e.calls.Load(); got != 25 {
		t.Fatalf("повтор искал заново: %d поисков", got)
	}

	a := e.a
	if v := a.Undo(); v.Rev != rev0+2 {
		t.Fatalf("Undo: rev %d", v.Rev)
	}
	for _, id := range e.ids {
		if f := e.frame(id); f != (model.Frame{S: 1}) {
			t.Fatalf("после одной отмены кадр не прежний: %+v", f)
		}
	}

	js, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"view"`, `"report"`, `"fitted":25`, `"noFace":0`, `"many":0`, `"missing":0`, `"small":0`,
		`"big":0`, `"edge":0`, `"contain":0`, `"manual":0`, `"changed":0`, `"check":[]`, `"short":[]`, `"asked":false`} {
		if !strings.Contains(string(js), k) {
			t.Errorf("в JSON ответа нет %s", k)
		}
	}
}

// TestAppFitFacesProgress: faces:progress — (0, N) сразу, затем числа без убывания, последнее (N, N);
// у листа без фото события нет.
func TestAppFitFacesProgress(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.fit(t, "", "")
	ev := e.progress()
	if len(ev) < 2 || ev[0] != [2]int{0, 25} || ev[len(ev)-1] != [2]int{25, 25} {
		t.Fatalf("события %v: ждали (0, 25) … (25, 25)", ev)
	}
	for i, p := range ev {
		if p[1] != 25 || i > 0 && p[0] < ev[i-1][0] {
			t.Fatalf("событие %d: %v после %v", i, p, ev[i-1])
		}
	}
	// Повтор из кэша: ход тот же по форме, поиска нет.
	e.mu.Lock()
	e.events = nil
	e.mu.Unlock()
	e.fit(t, "", "")
	if ev := e.progress(); len(ev) < 2 || ev[len(ev)-1] != [2]int{25, 25} {
		t.Fatalf("повтор: события %v", ev)
	}

	empty := newFacesEnv(t, 0)
	res := empty.fit(t, "", "")
	wantReport(t, res.Report, 0, 0)
	if ev := empty.progress(); len(ev) != 0 {
		t.Fatalf("лист без фото послал события: %v", ev)
	}
}

// TestAppFitFacesBusy: второй вызов во время поиска — ошибка «Уже ищу лица», первый заканчивается
// как обычно, а после него поиск снова доступен.
func TestAppFitFacesBusy(t *testing.T) {
	e := newFacesEnv(t, 6)
	entered, release := make(chan struct{}, 64), make(chan struct{})
	e.a.faceSt.detect = func(img image.Image) []faces.Face {
		entered <- struct{}{}
		<-release
		return defaultFace(0)
	}
	type out struct {
		res FacesResult
		err error
	}
	done := make(chan out, 1)
	go func() {
		res, err := e.a.FitFaces(e.sid, "", "")
		done <- out{res, err}
	}()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "поиск не начался", nil); !ok {
		t.FailNow()
	}
	rev := e.rev()
	if _, err := e.a.FitFaces(e.sid, "", ""); err != errFacesBusy {
		t.Fatalf("второй вызов: %v, ждали %v", err, errFacesBusy)
	}
	if e.rev() != rev {
		t.Fatal("второй вызов изменил проект")
	}
	close(release)
	o, ok := testkit.WaitCh(t, done, testkit.Limit(10*time.Second), "первый вызов не закончился", nil)
	if !ok || o.err != nil || o.res.Report.Fitted != 6 {
		t.Fatalf("первый вызов: %+v, %v", o.res.Report, o.err)
	}
	if _, err := e.a.FitFaces(e.sid, "", ""); err != nil {
		t.Fatalf("после поиска: %v", err)
	}
}

// TestAppFitFacesExportGuard: пока идёт экспорт, подгон отказывает ошибкой, не ищет и не правит.
func TestAppFitFacesExportGuard(t *testing.T) {
	e := newFacesEnv(t, 4)
	entered, gate := make(chan struct{}, 1), make(chan struct{})
	oldAsk, oldPick := askYes, pickExportDir
	askYes = func(*App, string) (bool, error) { return true, nil }
	pickExportDir = func(*App, string) (string, error) { entered <- struct{}{}; <-gate; return "", nil }
	t.Cleanup(func() { askYes, pickExportDir = oldAsk, oldPick })
	exported := make(chan error, 1)
	go func() {
		_, err := e.a.Export(ExportOptions{DPI: 72})
		exported <- err
	}()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "экспорт не дошёл до выбора папки", nil); !ok {
		close(gate)
		t.FailNow()
	}
	rev := e.rev()
	_, err := e.a.FitFaces(e.sid, "", "")
	close(gate)
	if err != errFacesExport || e.calls.Load() != 0 || e.rev() != rev {
		t.Fatalf("во время экспорта: ошибка %v, поисков %d, rev %d → %d", err, e.calls.Load(), rev, e.rev())
	}
	if _, ok := testkit.WaitCh(t, exported, testkit.Limit(10*time.Second), "экспорт не закончился", nil); !ok {
		t.FailNow()
	}
	if _, err := e.a.FitFaces(e.sid, "", ""); err != nil {
		t.Fatalf("после экспорта: %v", err)
	}
}

// TestAppFitFacesErrors: чужой лист и неизвестный режим — ошибка без правки и без поиска.
func TestAppFitFacesErrors(t *testing.T) {
	e := newFacesEnv(t, 3)
	rev := e.rev()
	if _, err := e.a.FitFaces("нет-такого", "", ""); err != errFacesSheet {
		t.Errorf("чужой лист: %v", err)
	}
	if _, err := e.a.FitFaces(e.sid, "", "всё"); err != errFacesMode {
		t.Errorf("режим: %v", err)
	}
	if e.calls.Load() != 0 || e.rev() != rev {
		t.Fatalf("ошибки не холостые: поисков %d, rev %d → %d", e.calls.Load(), rev, e.rev())
	}
}

// TestAppFitFacesModelBroken: модель не загрузилась — «Поиск лиц недоступен: …», правки, событий и
// поиска нет; в журнале одна строка на сколько бы вызовов ни было.
func TestAppFitFacesModelBroken(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	e := newFacesEnv(t, 3)
	e.a.faceSt.ready = func() error {
		return fmt.Errorf("faces: модель не загрузилась: %w", errors.New("битые веса"))
	}
	rev := e.rev()
	for range 3 {
		_, err := e.a.FitFaces(e.sid, "", "")
		if err == nil || err.Error() != "Поиск лиц недоступен: битые веса" {
			t.Fatalf("ошибка %v", err)
		}
	}
	if e.calls.Load() != 0 || e.rev() != rev || len(e.progress()) != 0 {
		t.Fatalf("сломанная модель: поисков %d, rev %d → %d, событий %d", e.calls.Load(), rev, e.rev(), len(e.progress()))
	}
	if n := strings.Count(buf.String(), "модель не загрузилась"); n != 1 {
		t.Fatalf("в журнале %d записей, ждали 1: %q", n, buf.String())
	}
	if e.a.faceSt.busy.Load() {
		t.Fatal("после ошибки поиск остался занятым")
	}
	e.a.faceSt.ready = nil // настоящая модель исправна: подгон идёт
	e.fit(t, "", "")
}

// TestAppFitFacesKeepsManual: правленый кадр — вопрос без правки (ask), keep оставляет его, all
// подгоняет; кадр прошлого подгона правленым не считается.
func TestAppFitFacesKeepsManual(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.fit(t, "", "")
	hand := e.ids[3]
	e.a.SetFrame(hand, 3, 0.4, 0.3) // лицо уехало мимо нормы: вручную
	manual := e.frame(hand)
	rev := e.rev()

	res := e.fit(t, "", "")
	if !res.Report.Asked || res.Report.Manual != 1 || res.Report.Fitted != 0 || e.rev() != rev || e.frame(hand) != manual {
		t.Fatalf("ask: %+v, rev %d → %d, кадр %+v", res.Report, rev, e.rev(), e.frame(hand))
	}
	if n := len(e.ed.View().Spreads); n == 0 || res.View.Rev != rev {
		t.Fatalf("вид в ответе ask: rev %d", res.View.Rev)
	}

	res = e.fit(t, "", "keep")
	wantReport(t, res.Report, 24, 1)
	if e.frame(hand) != manual {
		t.Fatalf("keep тронул правленый кадр: %+v", e.frame(hand))
	}
	if e.rev() != rev {
		t.Fatalf("keep: остальные уже на месте, ждали холостую правку, rev %d → %d", rev, e.rev())
	}

	res = e.fit(t, "", "all")
	wantReport(t, res.Report, 25, 0)
	if e.frame(hand) == manual || e.rev() != rev+1 {
		t.Fatalf("all: кадр %+v, rev %d → %d", e.frame(hand), rev, e.rev())
	}
	// Подогнан — значит снова «наш»: вопроса нет.
	res = e.fit(t, "", "")
	wantReport(t, res.Report, 25, 0)
}

// TestAppFitFacesEmptyFitted: после перезапуска память пуста — спросит только там, где лицо мимо
// нормы; кадры, стоящие по норме, вопроса не вызывают.
func TestAppFitFacesEmptyFitted(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.fit(t, "", "")
	e.a.faceSt.mu.Lock()
	e.a.faceSt.fitted, e.a.faceSt.norm = map[string]model.Frame{}, map[string]geom.FaceNorm{}
	e.a.faceSt.mu.Unlock()
	rev := e.rev()
	res := e.fit(t, "", "")
	wantReport(t, res.Report, 25, 0)
	if e.rev() != rev {
		t.Fatalf("кадры по норме после перезапуска: правка, rev %d → %d", rev, e.rev())
	}
	e.a.SetFrame(e.ids[5], 2.5, 0.5, 0.5)
	e.a.faceSt.mu.Lock()
	e.a.faceSt.fitted = map[string]model.Frame{}
	e.a.faceSt.mu.Unlock()
	if res := e.fit(t, "", ""); !res.Report.Asked || res.Report.Manual != 1 {
		t.Fatalf("правленый кадр при пустой памяти: %+v", res.Report)
	}
}

// TestAppFitFacesByRef: «как это лицо» — норма листа по образцу (± 0,005), образец не трогается,
// кнопка дальше держит эту норму без окна, а память листа живёт отдельно у каждого альбома.
func TestAppFitFacesByRef(t *testing.T) {
	e := newFacesEnv(t, 25)
	const sample = 7 // лицо 0,25 — самое крупное: у остальных S ≥ 1 и норма достижима
	ref := e.placeOf(t, sample)
	before := e.frame(e.ids[sample])
	rev := e.rev()
	res := e.fit(t, ref, "")
	wantReport(t, res.Report, 24, 0)
	if e.frame(e.ids[sample]) != before {
		t.Fatalf("образец тронут: %+v → %+v", before, e.frame(e.ids[sample]))
	}
	if e.rev() != rev+1 {
		t.Fatalf("rev %d → %d", rev, e.rev())
	}
	e.a.faceSt.mu.Lock()
	n, ok := e.a.faceSt.norm[e.sid]
	e.a.faceSt.mu.Unlock()
	if !ok || math.Abs(n.Size-0.25) > 0.005 {
		t.Fatalf("норма листа %+v (есть=%v), ждали размер 0,25 от образца", n, ok)
	}
	// Кнопка без образца берёт норму листа: образец тоже встаёт по ней (он уже на ней), окна нет.
	rev = e.rev()
	res = e.fit(t, "", "")
	wantReport(t, res.Report, 25, 0)
	if e.rev() != rev {
		t.Fatalf("кнопка после «как это лицо»: правка, rev %d → %d", rev, e.rev())
	}

	// Другой альбом начинает память заново.
	other := newFacesEnv(t, 3)
	norm, fitted := e.a.faceSt.memory(other.ed, other.sid)
	if norm != nil || len(fitted) != 0 {
		t.Fatalf("память чужого альбома: норма %v, кадров %d", norm, len(fitted))
	}
}

// TestAppFitFacesRefErrors: образец без лица, с несколькими, «Целиком», без файла, чужое место —
// ошибка с текстом из model.RefErrText, проект и норма листа не тронуты.
func TestAppFitFacesRefErrors(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.rule = func(i int) []faces.Face {
		switch i {
		case 3:
			return nil
		case 4:
			return []faces.Face{faceBox(0.2), faceBox(0.19)}
		}
		return defaultFace(i)
	}
	e.ed.SetPlaceFit(e.sid, []string{e.placeOf(t, 6)}, "contain")
	if err := os.Remove(e.files[5]); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, ref, code string }{
		{"без лица", e.placeOf(t, 3), model.RefNoFace},
		{"несколько лиц", e.placeOf(t, 4), model.RefMany},
		{"нет файла", e.placeOf(t, 5), model.RefNoFile},
		{"целиком", e.placeOf(t, 6), model.RefContain},
		{"чужое место", "нет-такого", model.RefNoPlace},
	}
	rev := e.rev()
	for _, c := range cases {
		_, err := e.a.FitFaces(e.sid, c.ref, "")
		if err == nil || err.Error() != model.RefErrText(c.code) || model.RefErrText(c.code) == "" {
			t.Errorf("%s: ошибка %v, ждали %q", c.name, err, model.RefErrText(c.code))
		}
		if e.rev() != rev {
			t.Errorf("%s: проект изменён, rev %d → %d", c.name, rev, e.rev())
		}
		e.a.faceSt.mu.Lock()
		_, has := e.a.faceSt.norm[e.sid]
		e.a.faceSt.mu.Unlock()
		if has {
			t.Errorf("%s: норма листа обновлена при ошибке образца", c.name)
		}
	}
}

// TestAppFitFacesChangedPlace: кадр правили, пока шёл поиск, — место «изменили», правка ему не
// пишется, остальные подогнаны.
func TestAppFitFacesChangedPlace(t *testing.T) {
	e := newFacesEnv(t, 25)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	e.rule = func(i int) []faces.Face {
		if i == 2 {
			entered <- struct{}{}
			<-release
		}
		return defaultFace(i)
	}
	done := make(chan FacesResult, 1)
	go func() {
		res, _ := e.a.FitFaces(e.sid, "", "")
		done <- res
	}()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "поиск не дошёл до фото 2", nil); !ok {
		close(release)
		t.FailNow()
	}
	e.a.SetFrame(e.ids[2], 1.6, 0.1, 0)
	hand := e.frame(e.ids[2])
	close(release)
	res, ok := testkit.WaitCh(t, done, testkit.Limit(10*time.Second), "подгон не закончился", nil)
	if !ok || res.Report.Changed != 1 || res.Report.Fitted != 24 {
		t.Fatalf("отчёт %+v: ждали 24 подогнано и 1 «изменили»", res.Report)
	}
	if e.frame(e.ids[2]) != hand {
		t.Fatalf("правка во время поиска перезаписана: %+v → %+v", hand, e.frame(e.ids[2]))
	}
}

// TestAppFitFacesSwitchedAlbum: альбом сменили во время поиска — ошибка, прежний проект не тронут,
// память сессии не записана.
func TestAppFitFacesSwitchedAlbum(t *testing.T) {
	e := newFacesEnv(t, 8)
	entered, release := make(chan struct{}, 8), make(chan struct{})
	e.rule = func(i int) []faces.Face {
		entered <- struct{}{}
		<-release
		return defaultFace(i)
	}
	type out struct {
		res FacesResult
		err error
	}
	done := make(chan out, 1)
	go func() {
		res, err := e.a.FitFaces(e.sid, "", "")
		done <- out{res, err}
	}()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "поиск не начался", nil); !ok {
		close(release)
		t.FailNow()
	}
	rev := e.rev()
	e.a.ed.Store(blankEditor(""))
	close(release)
	o, ok := testkit.WaitCh(t, done, testkit.Limit(10*time.Second), "вызов не закончился", nil)
	if !ok || o.err != errFacesMoved {
		t.Fatalf("ошибка %v, ждали %v", o.err, errFacesMoved)
	}
	if e.rev() != rev {
		t.Fatal("прежний проект изменён")
	}
	e.a.faceSt.mu.Lock()
	n := len(e.a.faceSt.fitted)
	e.a.faceSt.mu.Unlock()
	if n != 0 {
		t.Fatalf("в памяти %d кадров от несостоявшегося подгона", n)
	}
}

// TestAppFitFacesCache: повтор и ответ на окно идут без поиска; другой файл (время изменения или
// длина при том же времени) и другие четверти — новая запись; без изменений кэш не растёт.
func TestAppFitFacesCache(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.fit(t, "", "")
	if got, size := e.calls.Load(), e.a.faceSt.cache.size(); got != 25 || size != 25 {
		t.Fatalf("после первого прохода: поисков %d, в кэше %d", got, size)
	}
	e.fit(t, "", "all")
	e.fit(t, "", "keep")
	if got := e.calls.Load(); got != 25 {
		t.Fatalf("ответы на окно искали заново: %d поисков", got)
	}

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(e.files[0], later, later); err != nil {
		t.Fatal(err)
	}
	e.fit(t, "", "")
	if got := e.calls.Load(); got != 26 {
		t.Fatalf("файл с новым временем: поисков %d, ждали 26", got)
	}

	e.a.TurnPhotos([]string{e.ids[1]}, 1)
	e.fit(t, "", "all")
	if got := e.calls.Load(); got != 27 {
		t.Fatalf("фото повернули на четверть: поисков %d, ждали 27", got)
	}

	// Файл переписан другой длиной, время прежнее (копия с сохранением mtime): ключ меняет size.
	old, err := os.Stat(e.files[2])
	if err != nil {
		t.Fatal(err)
	}
	alt, err := os.ReadFile(testkit.JPEG(t, e.dir, "alt.jpg", 140, 202)) // длинная сторона та же: detect узнаёт фото
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(alt)) == old.Size() {
		t.Fatalf("тест не различает длину: обе %d байт", old.Size())
	}
	if err := os.WriteFile(e.files[2], alt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(e.files[2], old.ModTime(), old.ModTime()); err != nil {
		t.Fatal(err)
	}
	e.fit(t, "", "all")
	if got := e.calls.Load(); got != 28 {
		t.Fatalf("файл другой длины при прежнем времени: поисков %d, ждали 28", got)
	}
}

// TestAppFitFacesBadFiles: пустой, обрезанный, мусорный, с раздутым заголовком (900 Мп) и
// пропавший файлы — «нет фото» без паники, остальные подогнаны; фото ленты с относительным путём
// не читается вовсе (второй рубеж paths.CheckLocalPath в faceJob, хотя файл рядом и читался бы).
func TestAppFitFacesBadFiles(t *testing.T) {
	e := newFacesEnv(t, 8)
	good, err := os.ReadFile(e.files[7])
	if err != nil {
		t.Fatal(err)
	}
	write := func(i int, data []byte) {
		t.Helper()
		if err := os.WriteFile(e.files[i], data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(0, nil)
	write(1, good[:len(good)/2])
	write(2, []byte("это не картинка, а просто текст\x00\x01\x02"))
	testkit.HugeJPEG(t, e.dir, "p03.jpg", 30000, 30000)
	if err := os.Remove(e.files[4]); err != nil {
		t.Fatal(err)
	}
	write(5, append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 100)...))
	// Фото 6 — из ленты с относительным путём к существующему файлу (рабочая папка — его папка):
	// без проверки пути его бы прочитали и подогнали.
	t.Chdir(e.dir)
	e.a.mu.Lock()
	e.a.byID[e.ids[6]] = library.Photo{ID: e.ids[6], Path: "p06.jpg", Name: "p06.jpg"}
	e.a.mu.Unlock()
	res := e.fit(t, "", "")
	r := res.Report
	if r.Missing != 7 || r.Fitted != 1 || len(r.Check) != 7 {
		t.Fatalf("отчёт %+v: ждали 7 «нет фото» и 1 подогнанное", r)
	}
	if got := e.calls.Load(); got != 1 {
		t.Fatalf("детектор звали %d раз, ждали 1 (относительный путь читать нельзя)", got)
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6} {
		if f := e.frame(e.ids[i]); f != (model.Frame{S: 1}) {
			t.Errorf("фото %d без файла получило кадр %+v", i, f)
		}
	}
}

// TestAppFitFacesDetectorPanic: паника детектора на одном кадре — «нет фото» для него, остальные
// подогнаны, вызов не падает.
func TestAppFitFacesDetectorPanic(t *testing.T) {
	log.SetOutput(io.Discard) // guard пишет стек паники в журнал: в выводе теста он не нужен
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	e := newFacesEnv(t, 6)
	e.rule = func(i int) []faces.Face {
		if i == 1 {
			panic("сбой сети")
		}
		return defaultFace(i)
	}
	res, err := e.a.FitFaces(e.sid, "", "")
	if err != nil || res.Report.Missing != 1 || res.Report.Fitted != 5 {
		t.Fatalf("отчёт %+v, ошибка %v", res.Report, err)
	}
}

// TestAppFitFacesRealDetector: настоящий детектор на кадре без лиц — «без лица», кадр прежний,
// без паники (градиент не лицо).
func TestAppFitFacesRealDetector(t *testing.T) {
	e := newFacesEnv(t, 2)
	e.a.faceSt.detect = nil
	rev := e.rev()
	res := e.fit(t, "", "")
	if res.Report.NoFace != 2 || len(res.Report.Check) != 2 || e.rev() != rev {
		t.Fatalf("отчёт %+v, rev %d → %d", res.Report, rev, e.rev())
	}
}

// TestAppFitFacesRefAsked: «как это лицо» с вопросом о правленых — правки нет и норма листа не
// меняется (окно можно отменить); после ответа норма записана.
func TestAppFitFacesRefAsked(t *testing.T) {
	e := newFacesEnv(t, 25)
	e.fit(t, "", "")
	e.a.SetFrame(e.ids[3], 3, 0.4, 0.3)
	e.a.SetFrame(e.ids[7], 1, 0, 0) // образец без кадра: его лицо 0,25 кадра
	ref, rev := e.placeOf(t, 7), e.rev()
	norm := func() (float64, bool) {
		e.a.faceSt.mu.Lock()
		defer e.a.faceSt.mu.Unlock()
		n, ok := e.a.faceSt.norm[e.sid]
		return n.Size, ok
	}
	if res := e.fit(t, ref, ""); !res.Report.Asked || e.rev() != rev {
		t.Fatalf("вопрос: %+v, rev %d → %d", res.Report, rev, e.rev())
	}
	if _, ok := norm(); ok {
		t.Fatal("отложенный вопрос записал норму листа")
	}
	res := e.fit(t, ref, "keep")
	if size, ok := norm(); !ok || math.Abs(size-0.25) > 0.005 || res.Report.Manual != 1 || res.Report.Fitted != 23 {
		t.Fatalf("после ответа: норма %.3f (есть=%v), отчёт %+v", size, ok, res.Report)
	}
}

// TestAppFitFacesOwnFlaggedNotManual: кадр, который поставил подгон, но лицо осталось мимо нормы
// (мелкое — предел 400 %), при повторе не «правленый»: вопроса нет, правка холостая, отчёт тот же.
func TestAppFitFacesOwnFlaggedNotManual(t *testing.T) {
	e := newFacesEnv(t, 10)
	e.rule = func(i int) []faces.Face {
		if i%2 == 0 {
			return []faces.Face{faceBox(0.04)} // в рост: до нормы 0,35 не дотянуть
		}
		return defaultFace(i)
	}
	first := e.fit(t, "", "")
	if first.Report.Small != 5 || len(first.Report.Short) != 5 || first.Report.Fitted != 5 {
		t.Fatalf("первый проход %+v", first.Report)
	}
	rev := e.rev()
	again := e.fit(t, "", "")
	if again.Report.Asked || again.Report.Small != 5 || again.Report.Manual != 0 || e.rev() != rev {
		t.Fatalf("повтор %+v, rev %d → %d", again.Report, rev, e.rev())
	}
}
