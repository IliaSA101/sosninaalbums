package app

// 090 G2: /media и снятые клиентом запросы миниатюр, плитки прежнего альбома после смены, хвост
// «снято запросов» в строке журнала прогрева.

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cancelApp — App с кэшем миниатюр (папка cacheDir) и n фото в реестре ленты.
func cancelApp(t *testing.T, n int) (a *App, photos []library.Photo, cacheDir string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	cacheDir = testkit.TempDir(t)
	c, err := library.NewCache(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	// Кэш закрывает closeBackground из newApp (a.cache): отдельный хук шёл бы раньше него и закрывал
	// кэш при ещё идущем прогреве фона (гонка под -race).
	a.cache = c
	dir := testkit.TempDir(t)
	a.mu.Lock()
	for i := range n {
		name := fmt.Sprintf("p%d.jpg", i)
		path := testkit.JPEG(t, dir, name, 60, 40)
		p := library.Photo{ID: library.IDFor(path), Path: path, Name: name, W: 60, H: 40}
		photos = append(photos, p)
		a.photos = append(a.photos, p)
		a.byID[p.ID] = p
	}
	a.mu.Unlock()
	return a, photos, cacheDir
}

// getMediaCtx — запрос /media с контекстом клиента ctx.
func getMediaCtx(ctx context.Context, a *App, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
	return rec
}

// unanswered — обработчик ничего не написал: ни заголовков, ни тела (и 404 тоже).
func unanswered(rec *httptest.ResponseRecorder) bool {
	return rec.Body.Len() == 0 && len(rec.Header()) == 0
}

// cacheFiles — файлы копий в папке кэша.
func cacheFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if ext := strings.ToLower(filepath.Ext(p)); ext == ".jpg" || ext == ".png" {
				n++
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestMediaThumbCancelled — снятый клиентом запрос: миниатюра и превью не делаются, ответа нет,
// журнал чист, счётчик снятых растёт на каждый. Готовая копия при снятом запросе тоже не
// отдаётся (как доделанная генерация: клиента уже нет). Живой запрос после — как раньше.
func TestMediaThumbCancelled(t *testing.T) {
	a, photos, cacheDir := cancelApp(t, 3)
	buf := captureLog(t)
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	for _, p := range photos {
		for _, kind := range []string{"t", "p"} {
			if rec := getMediaCtx(gone, a, "/media/"+kind+"/"+p.ID); !unanswered(rec) {
				t.Errorf("снятый /media/%s: ответ %d %q", kind, rec.Code, rec.Body.String())
			}
		}
	}
	if n := cacheFiles(t, cacheDir); n != 0 {
		t.Errorf("снятые запросы сделали %d копий", n)
	}
	if got := a.mediaSt.cancelled.Load(); got != 6 {
		t.Errorf("снято %d, ждали 6", got)
	}
	if a.mediaMiss.Load() != 0 || buf.String() != "" {
		t.Errorf("снятые в журнале: промахов %d, %q", a.mediaMiss.Load(), buf.String())
	}

	// Готовая копия, клиент ушёл: ответа нет, это тоже снятый.
	if rec := getMedia(a, "/media/t/"+photos[0].ID); rec.Code != http.StatusOK {
		t.Fatalf("живой запрос: %d", rec.Code)
	}
	if rec := getMediaCtx(gone, a, "/media/t/"+photos[0].ID); !unanswered(rec) {
		t.Errorf("снятый при готовой копии: ответ %d", rec.Code)
	}
	if got := a.mediaSt.cancelled.Load(); got != 7 {
		t.Errorf("снято %d, ждали 7", got)
	}
	if a.mediaMiss.Load() != 0 || buf.String() != "" {
		t.Errorf("журнал: промахов %d, %q", a.mediaMiss.Load(), buf.String())
	}
}

// jumpWave — быстрая прокрутка (090 design §4): запросы плиток passed снимаются через cancelAfter,
// сразу после снятия — запросы shown разом. d — от начала shown до последнего их ответа; codes —
// коды shown; dropped — запросы passed без ответа (сняты раньше него).
func jumpWave(h http.Handler, passed, shown []library.Photo, cancelAfter time.Duration) (d time.Duration, codes []int, dropped int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recs := make([]*httptest.ResponseRecorder, len(passed))
	var wgPassed sync.WaitGroup
	for i, p := range passed {
		recs[i] = httptest.NewRecorder()
		wgPassed.Go(func() {
			h.ServeHTTP(recs[i], httptest.NewRequest(http.MethodGet, "/media/t/"+p.ID, nil).WithContext(ctx))
		})
	}
	time.Sleep(cancelAfter)
	cancel()
	codes = make([]int, len(shown))
	var wg sync.WaitGroup
	t0 := time.Now()
	for i, p := range shown {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/t/"+p.ID, nil))
			codes[i] = rec.Code
		})
	}
	wg.Wait()
	d = time.Since(t0)
	wgPassed.Wait()
	for _, rec := range recs {
		if unanswered(rec) {
			dropped++
		}
	}
	return d, codes, dropped
}

// TestMediaJumpWave — волна снятых и сразу новых запросов (шаг jump L1 на малом наборе): новые
// все отданы, каждый оставшийся без ответа снятый посчитан, журнал чист.
func TestMediaJumpWave(t *testing.T) {
	a, photos, _ := cancelApp(t, 8)
	buf := captureLog(t)
	for _, after := range []time.Duration{0, time.Millisecond} {
		before := a.mediaSt.cancelled.Load()
		_, codes, dropped := jumpWave(a.media(http.NotFoundHandler()), photos[:4], photos[4:], after)
		for i, c := range codes {
			if c != http.StatusOK {
				t.Errorf("снятие через %v: новая плитка %d — код %d", after, i, c)
			}
		}
		if got := a.mediaSt.cancelled.Load() - before; got != int64(dropped) {
			t.Errorf("снятие через %v: без ответа %d, посчитано %d", after, dropped, got)
		}
	}
	if a.mediaMiss.Load() != 0 || buf.String() != "" {
		t.Errorf("журнал: промахов %d, %q", a.mediaMiss.Load(), buf.String())
	}
}

// TestMediaOldProjectNoMiss — после «Новый» браузер досылает плитки прежнего альбома: 404 без
// промаха в журнале; ID, которого не было никогда, — промах, как раньше.
func TestMediaOldProjectNoMiss(t *testing.T) {
	a, photos, _ := cancelApp(t, 2)
	for _, p := range photos {
		if rec := getMedia(a, "/media/t/"+p.ID); rec.Code != http.StatusOK {
			t.Fatalf("до смены: %d", rec.Code)
		}
	}
	a.NewProject("album")
	miss := a.mediaMiss.Load()
	for _, p := range photos {
		for _, kind := range []string{"t", "p"} {
			if rec := getMedia(a, "/media/"+kind+"/"+p.ID); rec.Code != http.StatusNotFound {
				t.Errorf("прежний /media/%s: %d, ждали 404", kind, rec.Code)
			}
		}
	}
	if got := a.mediaMiss.Load(); got != miss {
		t.Errorf("прежние плитки записаны как промахи: %d → %d", miss, got)
	}
	if rec := getMedia(a, "/media/t/0000never"); rec.Code != http.StatusNotFound {
		t.Errorf("неизвестный: %d", rec.Code)
	}
	if got := a.mediaMiss.Load(); got != miss+1 {
		t.Errorf("неизвестный ID не записан как промах: %d → %d", miss, got)
	}
}

// TestMediaForget — прежний реестр копится через смены, вернувшееся вычёркивается, на пределе
// начинается заново; пустые реестры — без изменений.
func TestMediaForget(t *testing.T) {
	reg := func(ids ...string) map[string]library.Photo {
		m := map[string]library.Photo{}
		for _, id := range ids {
			m[id] = library.Photo{ID: id}
		}
		return m
	}
	has := func(m *mediaState, want ...string) {
		t.Helper()
		if len(m.old) != len(want) {
			t.Fatalf("прежний реестр %v, ждали %v", m.old, want)
		}
		for _, id := range want {
			if _, ok := m.old[id]; !ok {
				t.Fatalf("прежний реестр %v, ждали %v", m.old, want)
			}
		}
	}
	var m mediaState
	m.forget(nil, nil)
	has(&m)
	m.forget(reg("a", "b"), reg("b", "c"))
	has(&m, "a")
	m.forget(reg("b", "c"), reg()) // «Новый»: убраны все
	has(&m, "a", "b", "c")
	m.forget(reg(), reg("a")) // папку вернули: a снова в реестре
	has(&m, "b", "c")

	m.old = make(map[string]struct{}, oldIDsMax)
	for i := range oldIDsMax {
		m.old[fmt.Sprint(i)] = struct{}{}
	}
	m.forget(reg("x"), reg())
	has(&m, "x")
}

// TestThumbsLogLine — хвост «снято запросов» только при снятых; начало строки — прежнее (089 §8).
func TestThumbsLogLine(t *testing.T) {
	cases := []struct {
		n    int
		d    time.Duration
		c    int64
		want string
	}{
		{3, 1500 * time.Millisecond, 0, "миниатюры: 3 за 1.5s"},
		{600, 2*time.Minute + 3500*time.Millisecond + 400*time.Microsecond, 42, "миниатюры: 600 за 2m3.5s, снято запросов 42"},
		{0, 0, -1, "миниатюры: 0 за 0s"},
	}
	for _, c := range cases {
		if got := thumbsLogLine(c.n, c.d, c.c); got != c.want {
			t.Errorf("thumbsLogLine(%d, %v, %d) = %q, ждали %q", c.n, c.d, c.c, got, c.want)
		}
	}
}

// TestWarmLogCancelled — запросы, снятые за время прогрева, — хвостом в его строке журнала.
func TestWarmLogCancelled(t *testing.T) {
	a, photos, _ := cancelApp(t, 3)
	buf := captureLog(t)
	a.layouts.enter() // прогрев ждёт раскладки: снимаем запросы, пока он идёт
	done := make(chan struct{})
	gen := a.scanGen
	go func() { defer close(done); a.warmThumbs(gen, photos) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.mu.Lock()
		started := a.warm != nil
		a.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("прогрев не начался")
		}
		time.Sleep(time.Millisecond)
	}
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	for _, p := range photos[:2] {
		getMediaCtx(gone, a, "/media/t/"+p.ID)
	}
	a.layouts.leave()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("прогрев не кончился")
	}
	if !regexp.MustCompile(`миниатюры: 3 за [^\s,]+, снято запросов 2\n`).MatchString(buf.String()) {
		t.Errorf("нет «снято запросов 2» в строке прогрева: %q", buf.String())
	}
}
