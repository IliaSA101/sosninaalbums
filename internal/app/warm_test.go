package app

// Ход прогрева раскладок при открытии (073): событие open:progress, GetState вне App.mu.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// warmProject — n разворотов «Авто» по k фото (у каждого свой состав: кэш не общий) и пустой.
func warmProject(n, k, seed int) *model.Project {
	p := model.New()
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator) // локальный абсолютный путь
	for i := range n {
		s := &model.Spread{ID: fmt.Sprintf("s%d-%03d", seed, i)}
		for j := range k {
			id := fmt.Sprintf("p%d-%03d-%02d", seed, i, j)
			w, h := 3000, 2000+seed*13+i*7+j
			if j%3 == 0 {
				w, h = 2000, 3000
			}
			p.Photos[id] = model.PhotoRef{Path: root + id + ".jpg", W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		p.Spreads = append(p.Spreads, s)
	}
	p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d-empty", seed)})
	return p
}

// openProgress — разобранное событие open:progress: контракт — два числа (done, total int).
type openProgress struct{ Done, Total int }

// progressApp — App с записью событий open:progress; onProgress (может быть nil) зовётся на каждом.
func progressApp(t *testing.T, onProgress func()) (*App, func() []openProgress) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var got []openProgress
	a.emit = func(name string, data ...any) {
		if name != "open:progress" {
			return
		}
		if onProgress != nil {
			onProgress()
		}
		// Контракт events.md: ровно два int; один объект (как было) — провал.
		d, ok1 := argInt(data, 0)
		n, ok2 := argInt(data, 1)
		if len(data) != 2 || !ok1 || !ok2 {
			t.Errorf("open:progress: аргументы %#v, ждали (done int, total int)", data)
			return
		}
		mu.Lock()
		got = append(got, openProgress{d, n})
		mu.Unlock()
	}
	take := func() []openProgress {
		mu.Lock()
		defer mu.Unlock()
		out := got
		got = nil
		return out
	}
	return a, take
}

// argInt — i-й аргумент события как int (false, если нет или другой тип).
func argInt(data []any, i int) (int, bool) {
	if i >= len(data) {
		return 0, false
	}
	v, ok := data[i].(int)
	return v, ok
}

// setWarmTimes — подмена порогов хода на время теста.
func setWarmTimes(t *testing.T, first, every time.Duration) {
	t.Helper()
	f, e := warmFirst, warmEvery
	warmFirst, warmEvery = first, every
	t.Cleanup(func() { warmFirst, warmEvery = f, e })
}

func coldLayouts(t *testing.T) {
	t.Helper()
	layout.ResetCache()
	variety.ResetCache()
	t.Cleanup(func() { layout.ResetCache(); variety.ResetCache() })
}

// checkRun — ход одного открытия: total один и тот же, done растёт, последнее — (total, total).
func checkRun(t *testing.T, got []openProgress, total int) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("нет событий хода, ждали до (%d, %d)", total, total)
	}
	for i, e := range got {
		if e.Total != total || e.Done < 1 || e.Done > total || i > 0 && e.Done <= got[i-1].Done {
			t.Fatalf("ход %v: не монотонный или чужой total (ждали %d)", got, total)
		}
	}
	if last := got[len(got)-1]; last.Done != total {
		t.Fatalf("последнее событие %v, ждали (%d, %d)", last, total, total)
	}
}

// TestOpenProgressEvents: без порогов — ход 1…n по разворотам «Авто» с фото (пустой не в счёт),
// последнее (n, n); с настоящими порогами на тёплом кэше событий нет. Повторное открытие другого
// альбома — свой ход с начала; повтор того же — без событий.
func TestOpenProgressEvents(t *testing.T) {
	coldLayouts(t)
	a, take := progressApp(t, nil)
	a.ed.Store(mustEditor(t, warmProject(6, 7, 1), ""))
	setWarmTimes(t, 0, 0)
	a.GetState()
	got := take()
	checkRun(t, got, 6)
	if len(got) != 6 {
		t.Fatalf("без порогов ход %v, ждали 6 событий", got)
	}
	warmFirst, warmEvery = 150*time.Millisecond, 100*time.Millisecond
	a.GetState()
	if got := take(); len(got) != 0 {
		t.Fatalf("тёплый кэш: события %v", got)
	}
	a.ed.Store(mustEditor(t, warmProject(4, 8, 2), "")) // другой альбом — холодный
	warmFirst, warmEvery = 0, 0
	a.GetState()
	checkRun(t, take(), 4)
	a.ed.Store(mustEditor(t, model.New(), "")) // без разворотов — ход не нужен
	a.GetState()
	if got := take(); len(got) != 0 {
		t.Fatalf("пустой альбом: события %v", got)
	}
}

// TestOpenProgressThrottle: первое событие — только после warmFirst (час — ни одного, даже на
// холодном), дальше не чаще warmEvery: первое и обязательное последнее (n, n), без повтора.
func TestOpenProgressThrottle(t *testing.T) {
	coldLayouts(t)
	a, take := progressApp(t, nil)
	a.ed.Store(mustEditor(t, warmProject(5, 7, 3), ""))
	setWarmTimes(t, time.Hour, 0)
	a.GetState()
	if got := take(); len(got) != 0 {
		t.Fatalf("до первого порога: события %v", got)
	}
	layout.ResetCache()
	variety.ResetCache()
	warmFirst, warmEvery = 0, time.Hour
	a.GetState()
	got := take()
	checkRun(t, got, 5)
	if len(got) != 2 || got[0].Done != 1 {
		t.Fatalf("прореженный ход %v, ждали (1, 5) и (5, 5)", got)
	}
}

// TestGetStateWarmOutsideLock: на время прогрева App.mu свободен — лента и события library/scan
// не ждут счёта раскладок; View после прогрева тот же, что без него.
func TestGetStateWarmOutsideLock(t *testing.T) {
	coldLayouts(t)
	var a *App
	var mu sync.Mutex
	held := 0
	a, take := progressApp(t, func() {
		if !a.mu.TryLock() {
			mu.Lock()
			held++
			mu.Unlock()
			return
		}
		a.mu.Unlock()
	})
	p := warmProject(5, 9, 4)
	a.ed.Store(mustEditor(t, p, ""))
	setWarmTimes(t, 0, 0)
	st := a.GetState()
	checkRun(t, take(), 5)
	if held != 0 {
		t.Fatalf("App.mu занят во время прогрева: %d событий из 5", held)
	}
	layout.ResetCache()
	variety.ResetCache()
	cold := mustEditor(t, p, "").View()
	if fmt.Sprint(cold.Spreads) != fmt.Sprint(st.View.Spreads) {
		t.Fatal("вид GetState после прогрева отличается от холодного View")
	}
}

// TestWarmEmitPanic: паника отправки события хода (воркеры раскладки) не роняет программу —
// GetState отдаёт альбом целиком.
func TestWarmEmitPanic(t *testing.T) {
	coldLayouts(t)
	a, _ := progressApp(t, func() { panic("сбой окна") })
	a.ed.Store(mustEditor(t, warmProject(4, 7, 5), ""))
	setWarmTimes(t, 0, 0)
	if st := a.GetState(); len(st.View.Spreads) != 5 {
		t.Fatalf("GetState после паники хода: %d разворотов", len(st.View.Spreads))
	}
}

// TestGetStateConcurrentWarm: окно и браузер (dev) открывают альбом одновременно — оба ответа
// приходят целиком, а прогревы идут по очереди (свой замок Warm, не замок Editor): ход каждого —
// монотонный 1…n, последнее событие (n, n). Под -race.
func TestGetStateConcurrentWarm(t *testing.T) {
	const n = 12
	coldLayouts(t)
	a, take := progressApp(t, nil)
	a.ed.Store(mustEditor(t, warmProject(n, 9, 6), ""))
	setWarmTimes(t, 0, 0)
	var wg sync.WaitGroup
	spreads := make([]int, 2)
	for i := range spreads {
		wg.Go(func() { spreads[i] = len(a.GetState().View.Spreads) })
	}
	wg.Wait()
	for i, got := range spreads {
		if got != n+1 { // n разворотов «Авто» и пустой
			t.Errorf("ответ %d: %d разворотов, ждали %d", i, got, n+1)
		}
	}
	got := take()
	if len(got) < n || len(got) > 2*n {
		t.Fatalf("событий %d, ждали от %d до %d", len(got), n, 2*n)
	}
	// Делим на прогоны по сбросу done: каждый должен кончаться на (n, n).
	for i, e := range got {
		if e.Total != n {
			t.Fatalf("событие %d: total %d, ждали %d", i, e.Total, n)
		}
		if next := i + 1; next == len(got) || got[next].Done <= e.Done {
			if e.Done != n {
				t.Fatalf("прогон оборван на (%d, %d): %v", e.Done, e.Total, got)
			}
		}
	}
}

// TestFoldersDuringWarm (073-11): посреди холодного прогрева GetState Folders (rescan) и /media фото
// не из ленты (Ref) отвечают сразу — замок Editor прогрев держит только на снимок входа. Прогрев
// стоит на первом событии хода, пока идут замеры; до 073-11 оба ждали конца прогрева.
func TestFoldersDuringWarm(t *testing.T) {
	coldLayouts(t)
	limit := testkit.Limit(50 * time.Millisecond)
	started, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	a, _ := progressApp(t, func() {
		once.Do(func() {
			close(started)
			select { // страховка: упавший тест не держит GetState вечно
			case <-resume:
			case <-time.After(10 * time.Second):
			}
		})
	})
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	old := reachable
	reachable = func(string) bool { return false } // Ref пройден, файл не читаем
	t.Cleanup(func() { reachable = old })
	p := warmProject(6, 9, 7)
	p.Folders = []string{testkit.TempDir(t), testkit.TempDir(t)}
	ed := mustEditor(t, p, "")
	a.ed.Store(ed)
	setWarmTimes(t, 0, 0)
	state := make(chan State, 1)
	go func() { state <- a.GetState() }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("прогрев не дошёл до первого события хода")
	}
	// Ответ — в канал, а не t.Errorf из горутины: упавший замер договаривает после теста.
	timed := func(f func() string) (string, chan string) {
		ch := make(chan string, 1)
		go func() { ch <- f() }()
		select {
		case got := <-ch:
			return got, nil
		case <-time.After(limit):
			return "", ch
		}
	}
	folders, late1 := timed(func() string { return fmt.Sprint(len(ed.Folders())) })
	h := a.media(http.NotFoundHandler())
	media, late2 := timed(func() string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/media/t/p7-000-00", nil))
		return fmt.Sprint(rec.Code)
	})
	close(resume)
	if st := <-state; len(st.View.Spreads) != 7 {
		t.Fatalf("GetState: %d разворотов, ждали 7", len(st.View.Spreads))
	}
	if late1 != nil {
		t.Errorf("Folders во время прогрева дольше %v (ответ после прогрева: %s)", limit, <-late1)
	} else if folders != "2" {
		t.Errorf("Folders во время прогрева: %s папок, ждали 2", folders)
	}
	if late2 != nil {
		t.Errorf("/media фото не из ленты во время прогрева дольше %v (ответ после: %s)", limit, <-late2)
	} else if media != "404" {
		t.Errorf("/media недоступного фото: код %s, ждали 404", media)
	}
}

// TestThumbsWaitLayouts (073-11): прогрев миниатюр не начинается, пока идёт прогрев раскладок, и
// доходит до конца после него; отмена (rescan) отпускает ожидание без единого фото.
func TestThumbsWaitLayouts(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	var mu sync.Mutex
	var thumbs [][2]int
	a.emit = func(name string, data ...any) {
		if name == "thumbs" {
			mu.Lock()
			thumbs = append(thumbs, [2]int{data[0].(int), data[1].(int)})
			mu.Unlock()
		}
	}
	taken := func() [][2]int {
		mu.Lock()
		defer mu.Unlock()
		return append([][2]int(nil), thumbs...)
	}
	dir := testkit.TempDir(t)
	var photos []library.Photo
	for i := range 3 {
		name := fmt.Sprintf("p%d.jpg", i)
		photos = append(photos, library.Photo{ID: name, Path: testkit.JPEG(t, dir, name, 60, 40), Name: name})
	}
	run := func() chan struct{} {
		done := make(chan struct{})
		gen := a.scanGen
		go func() { defer close(done); a.warmThumbs(gen, photos) }()
		return done
	}
	wait := func(done chan struct{}, what string) {
		t.Helper()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: прогрев миниатюр не кончился", what)
		}
	}

	a.layouts.enter()
	a.layouts.enter() // два GetState сразу: ждём оба
	done := run()
	time.Sleep(100 * time.Millisecond)
	a.layouts.leave()
	time.Sleep(50 * time.Millisecond)
	if got := taken(); len(got) != 0 {
		t.Fatalf("миниатюры грелись во время прогрева раскладок: %v", got)
	}
	a.layouts.leave()
	wait(done, "после прогрева раскладок")
	if got := taken(); len(got) == 0 || got[len(got)-1] != [2]int{3, 3} {
		t.Fatalf("после прогрева раскладок события %v, ждали в конце (3, 3)", got)
	}

	mu.Lock()
	thumbs = nil
	mu.Unlock()
	a.mu.Lock()
	a.warm = nil // отмена прошлого круга: ждём новую
	a.mu.Unlock()
	a.layouts.enter()
	defer a.layouts.leave()
	done = run()
	for { // ждём, когда warmThumbs поставил отмену
		a.mu.Lock()
		cancel := a.warm
		a.mu.Unlock()
		if cancel != nil {
			cancel()
			break
		}
		time.Sleep(time.Millisecond)
	}
	wait(done, "после отмены")
	if got := taken(); len(got) != 0 {
		t.Fatalf("отменённый прогрев миниатюр прислал события %v", got)
	}
}

// TestBusyGate: свободный шлюз не ждёт (и нулевой, и после выхода всех); занятый — до выхода
// последнего или до отмены ctx.
func TestBusyGate(t *testing.T) {
	var g busyGate
	within := func(what string, f func()) {
		t.Helper()
		done := make(chan struct{})
		go func() { defer close(done); f() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: wait не вернулся", what)
		}
	}
	within("нулевой", func() { g.wait(t.Context()) })
	g.enter()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	within("отмена", func() { g.wait(ctx) })
	waiting := make(chan struct{})
	go func() { defer close(waiting); g.wait(t.Context()) }()
	select {
	case <-waiting:
		t.Fatal("занятый шлюз не ждал")
	case <-time.After(30 * time.Millisecond):
	}
	g.leave()
	within("после выхода", func() { <-waiting })
	g.enter() // второй круг: новый канал, прежний закрыт
	g.leave()
	within("второй круг", func() { g.wait(t.Context()) })
}

// TestRescanEditorBusyOutsideLock: rescan ждёт замок редактора (его держит правка; у прогрева —
// снимок входа) не под App.mu — publish, infos и /media не встают; после правки сканирование
// идёт своим чередом, и папки — уже из правки.
func TestRescanEditorBusyOutsideLock(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	p := model.New()
	p.Photos["own"] = model.PhotoRef{Lib: "my:0123456789abcdef", W: 3, H: 2}
	p.Spreads = []*model.Spread{{ID: "s", Photos: []string{"own"}}}
	p.Folders = []string{testkit.TempDir(t)}
	ed := mustEditor(t, p, "")
	a.ed.Store(ed)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	edited := make(chan struct{})
	go func() {
		defer close(edited)
		// Искатель своего элемента зовётся внутри правки — под замком редактора.
		ed.RelinkLib(func(string) (string, bool) {
			once.Do(func() { close(entered) })
			select {
			case <-release:
			case <-time.After(10 * time.Second):
			}
			return "", false
		})
	}()
	<-entered
	done := make(chan struct{})
	go func() { a.rescan(); close(done) }()
	held := 0
	// Окно, за которое rescan до правки успевал взять App.mu и встать на замке редактора.
	for end := time.Now().Add(testkit.Limit(150 * time.Millisecond)); time.Now().Before(end); {
		if !a.mu.TryLock() {
			held++
			continue
		}
		a.mu.Unlock()
	}
	close(release)
	<-edited
	<-done
	if held != 0 {
		t.Fatalf("App.mu занят rescan, пока замок редактора у правки: %d проб", held)
	}
	a.mu.Lock()
	gen := a.scanGen
	a.mu.Unlock()
	if gen != 1 {
		t.Fatalf("поколение сканирования %d, ждали 1", gen)
	}
	waitScan(t, a)
}

// TestRescanThumbsAfterLayouts (073-10 + 073-11): rescan во время прогрева раскладок сканирует
// папку сразу, а миниатюры начинает только после прогрева и доводит до конца.
func TestRescanThumbsAfterLayouts(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	var mu sync.Mutex
	var thumbs [][2]int
	a.emit = func(name string, data ...any) {
		if name == "thumbs" {
			mu.Lock()
			thumbs = append(thumbs, [2]int{data[0].(int), data[1].(int)})
			mu.Unlock()
		}
	}
	taken := func() [][2]int {
		mu.Lock()
		defer mu.Unlock()
		return append([][2]int(nil), thumbs...)
	}
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "a.jpg", 60, 40)
	testkit.JPEG(t, dir, "b.jpg", 60, 40)
	if _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.layouts.enter()
	a.rescan()
	waitScan(t, a)
	if n := len(a.GetState().Photos); n != 2 {
		t.Fatalf("в ленте %d фото, ждали 2", n)
	}
	time.Sleep(50 * time.Millisecond)
	if got := taken(); len(got) != 0 {
		t.Fatalf("миниатюры грелись во время прогрева раскладок: %v", got)
	}
	a.layouts.leave()
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if got := taken(); len(got) > 0 && got[len(got)-1] == [2]int{2, 2} {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("после прогрева раскладок события %v, ждали в конце (2, 2)", taken())
		}
	}
}
