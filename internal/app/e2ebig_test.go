//go:build e2ebig

package app

// e2e L1 (089): замеры Go на большом наборе (600 фото по 24 Мп) в обычной сборке. В обычный
// `go test ./...` не входит — нужен тег e2ebig и набор (scripts/e2e-photos.go). Пороги — 089-e2e/design.md
// §3.1, отчёт — §6, команда — docs/qa/e2e.md:
//
//	SA_E2E_PHOTOS=<набор> SA_E2E_OUT=<папка отчёта> go test -tags e2ebig -run TestE2EBig -timeout 20m ./internal/app
//
// SA_E2E_COUNT=120 — быстрый вариант (папка a/ набора).
// Занятая машина — SA_TEST_SLOW=1.5 (параллельный агент), 4 — сборка и стенды.

import (
	"encoding/json"
	"fmt"
	"image/jpeg"
	"io/fs"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const (
	e2eFull  = 600 // кадров в полном наборе
	e2eQuick = 120 // кадров в папке a/ (быстрый прогон)
	e2eMB    = 1 << 20
)

// Пороги L1 (мс, МБ), по одной строке на id — сверять с design.md §3.1 (089-D12; экспорт — 091-D6).
const (
	limScanMs              = 1000
	limVisibleWaitMs       = 10000 // 090 G2: было 20000
	limVisibleWaitTargetMs = 5000
	limVisibleJumpMs       = 10000 // 090 design §4
	limVisibleJumpTargetMs = 5000
	limThumbsPerPhotoMs    = 400
	limSigsMs              = 1000
	limAutobuild4Ms        = 500
	limAutobuild4SimilarMs = 500
	limAutobuild7Ms        = 1500
	limSaveMs              = 500
	limOpenMs              = 1000
	limExportPerSpreadMs   = 3000
	limExportTargetMs      = 2000
	limPeakSysMb           = 2048
	limHeapAfterMb         = 100
)

// e2eEvent — событие Go → UI с временем отправки.
type e2eEvent struct {
	at   time.Time
	name string
	data []any
}

// e2eBus собирает события, которые нужны замерам (library и view — тяжёлые, их не храним).
type e2eBus struct {
	mu  sync.Mutex
	evs []e2eEvent
}

func (b *e2eBus) add(name string, data ...any) {
	switch name {
	case "scan", "thumbs", "sigs:progress", "open:progress", "export:done":
		b.mu.Lock()
		b.evs = append(b.evs, e2eEvent{time.Now(), name, data})
		b.mu.Unlock()
	}
}

func (b *e2eBus) mark() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.evs)
}

// await ждёт первое событие с индекса from, для которого pred истинно; не дождался — t.Fatalf.
func (b *e2eBus) await(t testing.TB, from int, timeout time.Duration, what string, pred func(e2eEvent) bool) e2eEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for i := from; ; {
		b.mu.Lock()
		for ; i < len(b.evs); i++ {
			if pred(b.evs[i]) {
				e := b.evs[i]
				b.mu.Unlock()
				return e
			}
		}
		b.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s (за %v)", what, timeout)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// e2eThumbs — событие thumbs (done, total) с условием cond.
func e2eThumbs(cond func(done, total int) bool) func(e2eEvent) bool {
	return func(e e2eEvent) bool {
		d, ok1 := argInt(e.data, 0)
		n, ok2 := argInt(e.data, 1)
		return e.name == "thumbs" && ok1 && ok2 && n > 0 && cond(d, n)
	}
}

// e2eScanOver — конец чтения папок: scan с Active = false.
func e2eScanOver(e e2eEvent) bool {
	if e.name != "scan" || len(e.data) != 1 {
		return false
	}
	pr, ok := e.data[0].(library.ScanProgress)
	return ok && !pr.Active
}

// e2eMetric — строка отчёта (design.md §6). Limit — базовый порог (без SA_TEST_SLOW), ok считает Go.
type e2eMetric struct {
	ID    string   `json:"id"`
	Value float64  `json:"value"`
	Limit *float64 `json:"limit"`
	Kind  string   `json:"kind"` // gate | target | info
	OK    bool     `json:"ok"`
	Note  string   `json:"note"`
}

type e2eReport struct {
	Level   string      `json:"level"`
	Started string      `json:"started"`
	Quick   bool        `json:"quick"`
	Count   int         `json:"count"`
	Slow    float64     `json:"slow"`
	Machine e2eMachine  `json:"machine"`
	Metrics []e2eMetric `json:"metrics"`
}

type e2eMachine struct {
	CPU     string `json:"cpu"`
	Threads int    `json:"threads"`
	RAMGB   int    `json:"ramGB"`
}

// e2eRec копит метрики; времена gate умножаются на slow (RaceSlowdown × SA_TEST_SLOW, как testkit.Limit).
type e2eRec struct {
	mu   sync.Mutex
	rep  e2eReport
	slow float64
}

func e2eRound(v float64) float64 { return math.Round(v*10) / 10 }

func (r *e2eRec) put(m e2eMetric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m.Value = e2eRound(m.Value)
	r.rep.Metrics = append(r.rep.Metrics, m)
}

// gate — метрика с порогом: провал — t.Errorf (тест красный, остальные замеры продолжаются).
// timed — порог в миллисекундах, масштабируется на slow.
func (r *e2eRec) gate(t testing.TB, id string, v, limit float64, timed bool, note string) {
	t.Helper()
	eff := limit
	if timed {
		eff = limit * r.slow
	}
	ok := v <= eff
	if !ok {
		t.Errorf("%s = %.1f, порог %.1f", id, v, eff)
	}
	r.put(e2eMetric{ID: id, Value: v, Limit: &limit, Kind: "gate", OK: ok, Note: note})
}

// target — желаемое время (мс, как gate умножается на slow): не достигнуто — только пометка в отчёте.
func (r *e2eRec) target(id string, v, limit float64, note string) {
	ok := v <= limit*r.slow
	if !ok {
		note = strings.TrimSpace(note + " цель не достигнута")
	}
	r.put(e2eMetric{ID: id, Value: v, Limit: &limit, Kind: "target", OK: ok, Note: note})
}

func (r *e2eRec) info(id string, v float64, note string) {
	r.put(e2eMetric{ID: id, Value: v, Kind: "info", OK: true, Note: note})
}

func e2eMs(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// e2eMem опрашивает MemStats раз в 100 мс: пик Sys и HeapInuse за весь тест.
type e2eMem struct {
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
	sys, heap  uint64
	sampleLock sync.Mutex
}

func startE2EMem() *e2eMem {
	m := &e2eMem{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(m.done)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			m.sample()
			select {
			case <-m.stop:
				return
			case <-tick.C:
			}
		}
	}()
	return m
}

func (m *e2eMem) sample() {
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	m.sampleLock.Lock()
	m.sys, m.heap = max(m.sys, s.Sys), max(m.heap, s.HeapInuse)
	m.sampleLock.Unlock()
}

// finish останавливает опрос и отдаёт пики (повторный вызов безопасен).
func (m *e2eMem) finish() (sys, heap uint64) {
	m.once.Do(func() { close(m.stop); <-m.done })
	m.sampleLock.Lock()
	defer m.sampleLock.Unlock()
	return m.sys, m.heap
}

// e2eHeap — HeapInuse после двух сборок мусора.
func e2eHeap() uint64 {
	runtime.GC()
	runtime.GC()
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	return s.HeapInuse
}

// e2eSet — набор из окружения: папки для чтения и число фото в них.
type e2eSet struct {
	folders []string
	have    int
	want    int
	out     string
}

// e2eEnv читает SA_E2E_PHOTOS, SA_E2E_OUT, SA_E2E_COUNT. Пустые переменные — ошибка (не «текущая папка»),
// набора на диске нет — пропуск с причиной.
func e2eEnv(t *testing.T) e2eSet {
	t.Helper()
	root, out := strings.TrimSpace(os.Getenv("SA_E2E_PHOTOS")), strings.TrimSpace(os.Getenv("SA_E2E_OUT"))
	if root == "" || out == "" {
		t.Fatal("SA_E2E_PHOTOS и SA_E2E_OUT обязательны (набор — go run scripts/e2e-photos.go, отчёт — своя папка)")
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(out) || filepath.Dir(out) == out {
		t.Fatalf("SA_E2E_PHOTOS и SA_E2E_OUT — абсолютные пути, отчёт не в корне диска (%q, %q)", root, out)
	}
	want := e2eFull
	if s := strings.TrimSpace(os.Getenv("SA_E2E_COUNT")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > e2eFull {
			t.Fatalf("SA_E2E_COUNT = %q: нужно число 1…%d", s, e2eFull)
		}
		want = n
	}
	folders := []string{filepath.Join(root, "a")}
	if want > e2eQuick {
		folders = append(folders, filepath.Join(root, "b"))
	}
	have := 0
	for _, f := range folders {
		have += e2eCountJPEG(f)
	}
	if have == 0 {
		t.Skipf("набора фото нет: в папках набора (a/, b/) нет JPEG (go run scripts/e2e-photos.go, docs/qa/e2e.md «Подготовка»)")
	}
	if have != want {
		t.Fatalf("набор неполный или лишний: %d фото, ждали %d (SA_E2E_COUNT)", have, want)
	}
	return e2eSet{folders: folders, have: have, want: want, out: out}
}

func e2eCountJPEG(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if ext := strings.ToLower(filepath.Ext(p)); ext == ".jpg" || ext == ".jpeg" {
				n++
			}
		}
		return nil
	})
	return n
}

func e2ePhotos(a *App) []library.Photo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.photos)
}

// TestE2EBig — подтесты идут по порядку на одном App; жёсткая ошибка (t.Fatal) останавливает остальные.
// visible стоит перед thumbs: он выполняется на 10 % прогрева, который стартует сразу после чтения папок.
func TestE2EBig(t *testing.T) {
	applyMemoryLimit() // как в Run: без предела L1 мерит пик не той программы
	set := e2eEnv(t)
	slow := float64(testkit.Limit(time.Second)) / float64(time.Second)
	rec := &e2eRec{slow: slow, rep: e2eReport{Level: "go", Started: time.Now().Format("2006-01-02T15:04:05"),
		Quick: set.want < e2eFull, Count: set.have, Slow: slow,
		Machine: e2eMachine{CPU: os.Getenv("PROCESSOR_IDENTIFIER"), Threads: runtime.NumCPU(), RAMGB: e2eRAMGB()},
		Metrics: []e2eMetric{}}}
	if rec.rep.Machine.CPU == "" {
		rec.rep.Machine.CPU = runtime.GOARCH
	}
	defer writeE2EReport(t, set.out, rec)

	mem := startE2EMem()
	defer mem.finish()

	work := testkit.TempDir(t)
	bus := &e2eBus{}
	a := newApp(t)
	a.cfgDir = filepath.Join(work, "cfg")
	cacheDir := filepath.Join(work, "thumbs")
	a.emit = bus.add
	if err := os.MkdirAll(a.cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	c, err := library.NewCache(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c

	oldAsk, oldPick := askYes, pickExportDir
	outDir := filepath.Join(work, "export")
	askYes = func(*App, string) (bool, error) { return true, nil }
	pickExportDir = func(*App, string) (string, error) { return outDir, os.MkdirAll(outDir, 0o750) }
	t.Cleanup(func() { askYes, pickExportDir = oldAsk, oldPick })

	st := &e2eState{set: set, rec: rec, a: a, bus: bus, mem: mem, work: work, cacheDir: cacheDir, outDir: outDir}
	steps := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"scan", st.scan}, {"visible", st.visible}, {"jump", st.jump}, {"thumbs", st.thumbs}, {"sigs", st.sigs},
		{"autobuild", st.autobuild}, {"saveopen", st.saveopen}, {"export", st.export}, {"memory", st.memory},
	}
	for _, s := range steps {
		aborted := true
		t.Run(s.name, func(t *testing.T) {
			s.fn(t)
			aborted = false // t.Fatal выходит из fn раньше; провал gate (t.Errorf) — нет
		})
		if aborted { // t.Fatal в подтесте: дальше на этом состоянии мерить нечего
			t.Fatalf("подтест %s прерван, остальные не запускались", s.name)
		}
	}
}

// e2eState — общее состояние подтестов.
type e2eState struct {
	set      e2eSet
	rec      *e2eRec
	a        *App
	bus      *e2eBus
	mem      *e2eMem
	work     string
	cacheDir string
	outDir   string

	heapBefore uint64
	scanEnd    time.Time
	mark       int // индекс событий на начало чтения
	spreads    int // развороты альбома после автосборки
	albumPath  string
}

func (s *e2eState) scan(t *testing.T) {
	s.heapBefore = e2eHeap()
	s.mark = s.bus.mark()
	t0 := time.Now()
	for _, f := range s.set.folders[:len(s.set.folders)-1] { // чтение одно, на последней папке: оно берёт все
		if _, _, err := s.a.editor().AddFolder(f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.a.AddFolderPath(s.set.folders[len(s.set.folders)-1]); err != nil {
		t.Fatal(err)
	}
	e := s.bus.await(t, s.mark, testkit.Limit(5*time.Minute), "конец чтения папок", e2eScanOver)
	s.scanEnd = e.at
	if got := len(e2ePhotos(s.a)); got != s.set.have {
		t.Fatalf("прочитано %d фото, в наборе %d", got, s.set.have)
	}
	s.rec.gate(t, "scan_ms", e2eMs(e.at.Sub(t0)), limScanMs, true, fmt.Sprintf("%d фото", s.set.have))
}

// visible: на 10 % прогрева просим миниатюры 30 последних фото (как лента, прокрученная в конец).
func (s *e2eState) visible(t *testing.T) {
	s.bus.await(t, s.mark, testkit.Limit(10*time.Minute), "10 % прогрева миниатюр",
		e2eThumbs(func(d, n int) bool { return d*10 >= n }))
	photos := e2ePhotos(s.a)
	last := photos[max(0, len(photos)-30):]
	h := s.a.media(http.NotFoundHandler())
	codes := make([]int, len(last))
	var mu sync.Mutex
	var done []time.Time // момент каждого ответа — для visible_first_ms
	var wg sync.WaitGroup
	t0 := time.Now()
	for i, p := range last {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", "/media/t/"+p.ID, nil))
			codes[i] = rec.Code
			mu.Lock()
			done = append(done, time.Now())
			mu.Unlock()
		})
	}
	wg.Wait()
	d := e2eMs(time.Since(t0))
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("/media/t фото %d из %d: код %d", i+1, len(codes), c)
		}
	}
	note := fmt.Sprintf("%d фото ленты", len(last))
	if len(done) > 0 {
		first := slices.MinFunc(done, func(a, b time.Time) int { return a.Compare(b) })
		s.rec.info("visible_first_ms", e2eMs(first.Sub(t0)), note)
	}
	s.rec.gate(t, "visible_wait_ms", d, limVisibleWaitMs, true, note)
	s.rec.target("visible_wait_target_ms", d, limVisibleWaitTargetMs, note)
}

// jump: быстрая прокрутка (090 design §4) — 30 плиток, мимо которых пролистали, сняты через 300 мс,
// сразу за ними — 30 на экране; visible_jump_ms — от начала вторых до последнего ответа.
func (s *e2eState) jump(t *testing.T) {
	photos := e2ePhotos(s.a)
	n := len(photos)
	if n < 90 {
		s.rec.info("visible_jump_ms", 0, fmt.Sprintf("пропущено: %d фото, нужно 90", n))
		return
	}
	passed, shown := photos[n-90:n-60], photos[n-60:n-30]
	d, codes, dropped := jumpWave(s.a.media(http.NotFoundHandler()), passed, shown, 300*time.Millisecond)
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("/media/t фото %d из %d после прокрутки: код %d", i+1, len(codes), c)
		}
	}
	note := fmt.Sprintf("%d фото ленты, снято %d из %d", len(shown), dropped, len(passed))
	s.rec.gate(t, "visible_jump_ms", e2eMs(d), limVisibleJumpMs, true, note)
	s.rec.target("visible_jump_target_ms", e2eMs(d), limVisibleJumpTargetMs, note)
}

func (s *e2eState) thumbs(t *testing.T) {
	n := s.set.have
	e := s.bus.await(t, s.mark, testkit.Limit(20*time.Minute), "конец прогрева миниатюр",
		e2eThumbs(func(d, total int) bool { return d == total && total == n }))
	wall := e2eMs(e.at.Sub(s.scanEnd))
	s.rec.info("thumbs_ms", wall, "")
	s.rec.gate(t, "thumbs_per_photo_ms", wall/float64(n), limThumbsPerPhotoMs, true, "стена / число фото")
}

func (s *e2eState) sigs(t *testing.T) {
	photos := e2ePhotos(s.a)
	ids := make([]string, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	mark := s.bus.mark()
	t0 := time.Now()
	st := s.a.PrepareSigs(ids)
	if st.Total != len(ids) {
		t.Fatalf("PrepareSigs: всего %d, ждали %d", st.Total, len(ids))
	}
	end := time.Now()
	if st.Done < st.Total {
		e := s.bus.await(t, mark, testkit.Limit(5*time.Minute), "подписи всех кадров", func(e e2eEvent) bool {
			sg, ok := e2eArg[SigState](e.data)
			return e.name == "sigs:progress" && ok && sg.Done == sg.Total
		})
		end = e.at
	}
	infos := s.a.infos(ids)
	okSigs := 0
	for _, in := range infos {
		if in.Sig.Ok {
			okSigs++
		}
	}
	if okSigs != len(ids) {
		t.Errorf("подписей готово %d из %d", okSigs, len(ids))
	}
	s.rec.gate(t, "sigs_ms", e2eMs(end.Sub(t0)), limSigsMs, true, fmt.Sprintf("%d подписей", okSigs))
}

func e2eArg[T any](data []any) (v T, ok bool) {
	if len(data) == 0 {
		return v, false
	}
	v, ok = data[0].(T)
	return v, ok
}

// build — автосборка всех фото по per на разворот, возвращает время; всё откатывается Undo.
func (s *e2eState) build(t *testing.T, ids []string, per, similar int) (time.Duration, int) {
	t.Helper()
	spreads := (len(ids) + per - 1) / per
	o := model.AutoOpts{Spreads: spreads, MaxPer: per, Similar: similar}
	t0 := time.Now()
	v, err := s.a.AutoBuild(ids, -1, o)
	d := time.Since(t0)
	if err != nil {
		t.Fatalf("AutoBuild по %d: %v", per, err)
	}
	if len(v.Spreads) != spreads {
		t.Fatalf("AutoBuild по %d: %d разворотов, ждали %d", per, len(v.Spreads), spreads)
	}
	return d, len(v.Spreads)
}

func (s *e2eState) undoBuild(t *testing.T) {
	t.Helper()
	if v := s.a.Undo(); len(v.Spreads) != 0 {
		t.Fatalf("после Undo осталось %d разворотов", len(v.Spreads))
	}
}

func (s *e2eState) autobuild(t *testing.T) {
	photos := e2ePhotos(s.a)
	ids := make([]string, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	d4, n4 := s.build(t, ids, 4, 0)
	s.rec.gate(t, "autobuild4_ms", e2eMs(d4), limAutobuild4Ms, true, fmt.Sprintf("%d разворотов", n4))
	s.undoBuild(t)
	d7, n7 := s.build(t, ids, 7, 0)
	s.rec.gate(t, "autobuild7_ms", e2eMs(d7), limAutobuild7Ms, true, fmt.Sprintf("%d разворотов", n7))
	s.undoBuild(t)
	dSim, nSim := s.build(t, ids, 4, 2) // с отбором похожих: подписи берутся из памяти
	s.rec.gate(t, "autobuild4_similar_ms", e2eMs(dSim), limAutobuild4SimilarMs, true, fmt.Sprintf("%d разворотов, похожие: очень", nSim))
	s.undoBuild(t)
	_, s.spreads = s.build(t, ids, 4, 0) // альбом для сохранения, открытия и экспорта
}

func (s *e2eState) saveopen(t *testing.T) {
	s.albumPath = filepath.Join(s.work, "e2e.salbum")
	ed := s.a.editor()
	t0 := time.Now()
	if err := ed.Save(s.albumPath); err != nil {
		t.Fatal(err)
	}
	saveMs := e2eMs(time.Since(t0))
	st, err := os.Stat(s.albumPath)
	if err != nil {
		t.Fatal(err)
	}
	s.a.rememberProject(s.albumPath) // OpenRecent открывает только альбомы из «последних»
	s.rec.gate(t, "save_ms", saveMs, limSaveMs, true, fmt.Sprintf("%d разворотов, файл %d КБ", s.spreads, st.Size()/1024))

	// Второй App с теми же папками данных и кэша — как новый запуск программы.
	bus2 := &e2eBus{}
	b := newApp(t)
	b.cfgDir = s.a.cfgDir
	b.emit = bus2.add
	c, err := library.NewCache(s.cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	b.cache = c
	t0 = time.Now()
	state, err := b.OpenRecent(s.albumPath)
	openMs := e2eMs(time.Since(t0))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.View.Spreads) != s.spreads {
		t.Fatalf("открыто %d разворотов, сохраняли %d", len(state.View.Spreads), s.spreads)
	}
	note := "ход открытия не показывали"
	bus2.mu.Lock()
	for _, e := range bus2.evs {
		if d, ok := argInt(e.data, 0); e.name == "open:progress" && ok {
			if n, _ := argInt(e.data, 1); d == n {
				note = fmt.Sprintf("open:progress (%d, %d) через %.0f мс", d, n, e2eMs(e.at.Sub(t0)))
			}
		}
	}
	bus2.mu.Unlock()
	s.rec.gate(t, "open_ms", openMs, limOpenMs, true, note)
	// Фоновое чтение и прогрев второго App кончаются до следующих замеров, сам App отпускает память.
	bus2.await(t, 0, testkit.Limit(20*time.Minute), "прогрев второго App",
		e2eThumbs(func(d, n int) bool { return d == n }))
	mark2 := bus2.mark()
	b.NewProject("album")
	bus2.await(t, mark2, testkit.Limit(time.Minute), "чтение второго App после «Нового»", e2eScanOver)
}

func (s *e2eState) export(t *testing.T) {
	n := min(10, s.spreads)
	rng := "1"
	if n > 1 {
		rng = fmt.Sprintf("1-%d", n)
	}
	mark := s.bus.mark()
	t0 := time.Now()
	ok, err := s.a.Export(ExportOptions{Range: rng, DPI: 300, Quality: 95})
	if err != nil || !ok {
		t.Fatalf("Export: %v, %v", ok, err)
	}
	e := s.bus.await(t, mark, testkit.Limit(10*time.Minute), "конец экспорта",
		func(e e2eEvent) bool { return e.name == "export:done" })
	if len(e.data) < 2 {
		t.Fatalf("export:done: %d аргументов", len(e.data))
	}
	if msg, _ := e.data[1].(string); msg != "" {
		t.Fatalf("экспорт: %s", msg)
	}
	files, err := filepath.Glob(filepath.Join(s.outDir, "*.jpg"))
	if err != nil || len(files) != n {
		t.Fatalf("в папке экспорта %d файлов, ждали %d (%v)", len(files), n, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if e2eAbs(cfg.Width-7087) > 2 || e2eAbs(cfg.Height-3543) > 2 { // 600×300 мм при 300 dpi
		t.Errorf("размер файла экспорта %d×%d, ждали 7087×3543", cfg.Width, cfg.Height)
	}
	per := e2eMs(e.at.Sub(t0)) / float64(n)
	s.rec.gate(t, "export_per_spread_ms", per, limExportPerSpreadMs, true, fmt.Sprintf("%d разворотов, 300 dpi", n))
	s.rec.target("export_per_spread_target_ms", per, limExportTargetMs, "бюджет qa/05 C9")
}

func e2eAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (s *e2eState) memory(t *testing.T) {
	sys, heap := s.mem.finish()
	s.rec.gate(t, "peak_sys_mb", float64(sys)/e2eMB, limPeakSysMb, false, "MemStats.Sys, опрос 100 мс")
	s.rec.info("peak_heap_mb", float64(heap)/e2eMB, "HeapInuse, опрос 100 мс")

	mark := s.bus.mark()
	s.a.NewProject("album")
	s.bus.await(t, mark, testkit.Limit(time.Minute), "чтение после «Нового»", e2eScanOver)
	after := e2eHeap()
	s.rec.gate(t, "heap_after_mb", (float64(after)-float64(s.heapBefore))/e2eMB, limHeapAfterMb, false,
		"HeapInuse после «Нового» минус до чтения папок")
}

// writeE2EReport пишет report.json в out — и при провале, и при t.Fatal (defer в TestE2EBig).
func writeE2EReport(t *testing.T, out string, rec *e2eRec) {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	raw, err := json.MarshalIndent(rec.rep, "", "  ")
	if err == nil {
		err = os.MkdirAll(out, 0o750)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(out, "report.json"), raw, 0o600)
	}
	if err != nil {
		t.Errorf("report.json: %v", err)
		return
	}
	for _, m := range rec.rep.Metrics {
		t.Logf("%-24s %10.1f  %-6s ok=%v %s", m.ID, m.Value, m.Kind, m.OK, m.Note)
	}
}
