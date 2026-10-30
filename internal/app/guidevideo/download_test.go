package guidevideo

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func readyOf(items []Item, id string) (Item, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// Без согласия — ErrNoConsent, ни одного запроса, папка не создана.
func TestDownloadNeedsConsent(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 4000)
	l.allowed.Store(false)
	items, err := l.svc.Download([]string{"cover"})
	if !errors.Is(err, ErrNoConsent) || err.Error() != "нужно разрешить обращение к GitHub" {
		t.Fatalf("ошибка: %v", err)
	}
	if l.hits.Load() != 0 || len(items) != 0 {
		t.Errorf("запросов %d, список %v", l.hits.Load(), items)
	}
	if _, err := os.Stat(l.dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("папка создана без согласия: %v", err)
	}
}

// Круг: скачанный ролик ложится в кэш байт в байт, список его готовым показывает, событие идёт.
func TestDownloadRoundTrip(t *testing.T) {
	l := newLab(t)
	body := l.webm("cover", 1, 200_000)
	items, err := l.svc.Download([]string{"cover"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(l.path("cover", 1, "webm"))
	if !bytes.Equal(got, body) {
		t.Fatalf("файл %d байт, ждали %d", len(got), len(body))
	}
	if it, ok := readyOf(items, "cover"); !ok || !it.Ready || it.Version != 1 || it.Size != int64(len(body)) || it.Seconds != 20 {
		t.Errorf("список: %+v", items)
	}
	if !slices.Equal(l.listDir(), []string{"cover-1.webm"}) {
		t.Errorf("в папке: %v", l.listDir())
	}
	l.mu.Lock()
	ev := slices.Clone(l.events)
	l.mu.Unlock()
	if len(ev) == 0 || ev[len(ev)-1] != (Progress{ID: "cover", Done: int64(len(body)), Total: int64(len(body))}) {
		t.Errorf("события: %v", ev)
	}
	// Быстрый ролик — первое событие и конец, не по событию на кусок (progressGap).
	if len(ev) > 2 {
		t.Errorf("событий %d, ждали не больше 2 на быстрый ролик: %v", len(ev), ev)
	}
	for _, e := range ev {
		if e.ID != "cover" || e.Total != int64(len(body)) || e.Done > e.Total {
			t.Errorf("событие %+v", e)
		}
	}
	// Тот же список без сети.
	hits := l.hits.Load()
	if again := l.svc.List(); !slices.Equal(again, items) || l.hits.Load() != hits {
		t.Errorf("List: %v, запросов +%d", again, l.hits.Load()-hits)
	}
}

// Тело не сошлось с sha256 из каталога (тот же размер) — текст про ролик, нет ни файла, ни *.tmp.
func TestDownloadVerifiesSum(t *testing.T) {
	l := newLab(t)
	body := l.webm("cover", 1, 5000)
	bad := bytes.Clone(body)
	bad[100] ^= 0xff
	l.mu.Lock()
	l.files["/releases/download/videos-1/cover-1.webm"] = bad
	l.mu.Unlock()
	_, err := l.svc.Download([]string{"cover"})
	if !errors.Is(err, ErrVideoBroken) || err.Error() != "ролик скачался с ошибкой — повторите" {
		t.Fatalf("ошибка: %v", err)
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке остались %v", got)
	}
}

// Длина тела не та, что в каталоге: короче, длиннее; размер в каталоге вне предела — строка пропущена.
func TestDownloadOversizeNoFile(t *testing.T) {
	l := newLab(t)
	body := l.webm("cover", 1, 5000)
	for name, served := range map[string][]byte{"длиннее": append(bytes.Clone(body), 1), "короче": body[:4999]} {
		l.mu.Lock()
		l.files["/releases/download/videos-1/cover-1.webm"] = served
		l.mu.Unlock()
		if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrVideoBroken) {
			t.Errorf("%s: %v", name, err)
		}
		if got := l.listDir(); len(got) != 0 {
			t.Errorf("%s: в папке %v", name, got)
		}
	}
	// Каталог объявил больше предела: строка пропущена разбором, ролика для службы нет.
	l.mu.Lock()
	l.videos["cover/1"]["files"].(map[string]any)["webm"].(map[string]any)["size"] = fetch.MaxVideoFile + 1
	l.mu.Unlock()
	hitsBefore := l.hits.Load()
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrGone) {
		t.Errorf("размер больше предела: %v", err)
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
	if l.hits.Load()-hitsBefore != 1 { // только каталог
		t.Errorf("запросов %d, ждали один (каталог)", l.hits.Load()-hitsBefore)
	}
}

// Новая версия заменяет прежнюю: прежний файл удаляется после удачной загрузки новой.
func TestDownloadReplacesOld(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	// Ролик пересняли: в каталоге версия 2 (версия 1 осталась в списке, берётся самая новая).
	body2 := l.webm("cover", 2, 3500)
	l.seq.Add(1)
	items, err := l.svc.Download([]string{"cover"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.listDir(), []string{"cover-2.webm"}) {
		t.Fatalf("в папке: %v", l.listDir())
	}
	if got, _ := os.ReadFile(l.path("cover", 2, "webm")); !bytes.Equal(got, body2) {
		t.Error("новый файл не тот")
	}
	if it, _ := readyOf(items, "cover"); it.Version != 2 || !it.Ready {
		t.Errorf("список: %+v", items)
	}
	// Упавшая загрузка новой версии прежнюю не трогает.
	l.webm("cover", 3, 3600)
	l.mu.Lock()
	delete(l.files, "/releases/download/videos-1/cover-3.webm")
	l.mu.Unlock()
	l.seq.Add(1)
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrGone) {
		t.Fatalf("404 на файле: %v", err)
	}
	if !slices.Equal(l.listDir(), []string{"cover-2.webm"}) {
		t.Errorf("после сбоя в папке: %v", l.listDir())
	}
}

// Чистка прежних версий — по разбору имени: соседние статьи (cover-export) и чужое не задеты.
func TestCleanupKeepsSiblings(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 2, 3000)
	for _, n := range []string{"cover-1.webm", "cover-export-1.webm", "cover-1-x.webm", "cover-1.mp4", "cover-3.webm.bak", "readme.txt", "coverage-1.webm"} {
		l.put(n, []byte("чужое "+n))
	}
	if err := os.Mkdir(filepath.Join(l.dir, "cover-5.webm"), 0o700); err != nil { // папка с похожим именем
		t.Fatal(err)
	}
	link := filepath.Join(l.dir, "cover-4.webm")
	if err := os.Symlink(filepath.Join(l.dir, "readme.txt"), link); err != nil {
		t.Logf("ссылки недоступны: %v", err)
	}
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	got := l.listDir()
	slices.Sort(got)
	want := []string{"cover-1-x.webm", "cover-2.webm", "cover-3.webm.bak", "cover-5.webm", "cover-export-1.webm", "coverage-1.webm", "readme.txt"}
	if _, err := os.Lstat(link); err == nil {
		want = append(want, "cover-4.webm") // ссылка — не обычный файл, чистка её не трогает
		slices.Sort(want)
	}
	if !slices.Equal(got, want) {
		t.Errorf("в папке %v, ждали %v", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(l.dir, "cover-export-1.webm")); string(b) != "чужое cover-export-1.webm" {
		t.Error("ролик соседней статьи тронут")
	}
}

// Готовый файл (размер и sha256 совпали) пропускается без запроса файла; испорченный того же
// размера скачивается заново.
func TestDownloadSkipsReadyAndFixesCorrupt(t *testing.T) {
	l := newLab(t)
	body := l.webm("cover", 1, 6000)
	var assets atomic.Int32
	l.mux.HandleFunc("/count/", func(http.ResponseWriter, *http.Request) {})
	inner := l.srv.Config.Handler
	l.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".webm") {
			assets.Add(1)
		}
		inner.ServeHTTP(w, r)
	})
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.svc.Download([]string{"cover"}); err != nil || assets.Load() != 1 {
		t.Fatalf("повтор: %v, запросов файла %d (ждали 1)", err, assets.Load())
	}
	bad := bytes.Clone(body)
	bad[10] ^= 0xff
	if err := os.WriteFile(l.path("cover", 1, "webm"), bad, 0o600); err != nil {
		t.Fatal(err)
	}
	// Список по размеру считает файл готовым (дёшево), sha256 сверяет Download.
	if it, _ := readyOf(l.svc.List(), "cover"); !it.Ready {
		t.Error("List: файл того же размера должен быть готов")
	}
	if _, err := l.svc.Download([]string{"cover"}); err != nil || assets.Load() != 2 {
		t.Fatalf("испорченный: %v, запросов файла %d (ждали 2)", err, assets.Load())
	}
	if got, _ := os.ReadFile(l.path("cover", 1, "webm")); !bytes.Equal(got, body) {
		t.Error("файл не исправлен")
	}
}

// Ввод от окна: мусор, повторы, слишком много — ErrGone до сети; повторы скачиваются один раз.
func TestDownloadBadIDNoNet(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	many := make([]string, fetch.MaxVideos+1)
	for i := range many {
		many[i] = "video" + strconv.Itoa(1000+i)
	}
	long := strings.Repeat("a", 41)
	for name, ids := range map[string][]string{
		"путь":            {"../x"},
		"регистр":         {"Cover"},
		"короткий":        {"ab"},
		"длинный":         {long},
		"пустой":          {""},
		"слэш":            {"a/b"},
		"устройство":      {"con\x00"},
		"годный и плохой": {"cover", "../x"},
		"101 разных":      many,
	} {
		before := l.hits.Load()
		items, err := l.svc.Download(ids)
		if !errors.Is(err, ErrGone) || err.Error() != "ролика пока нет в библиотеке — повторите позже" {
			t.Errorf("%s: %v", name, err)
		}
		if l.hits.Load() != before || len(items) != 0 {
			t.Errorf("%s: запросов +%d, список %v", name, l.hits.Load()-before, items)
		}
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("после отказов в папке %v", got)
	}
	// 100 разных годных (предел включительно) — не отказ по числу; повторы не считаются.
	dup := make([]string, 0, 300)
	for range 3 {
		dup = append(dup, many[:fetch.MaxVideos]...)
	}
	if ids, ok := cleanIDs(dup); !ok || len(ids) != fetch.MaxVideos {
		t.Errorf("cleanIDs: %d, %v", len(ids), ok)
	}
	if _, ok := cleanIDs(many); ok {
		t.Error("101 разных принято")
	}
	// Повторы одного id — один запрос файла.
	if _, err := l.svc.Download([]string{"cover", "cover", "cover"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.listDir(), []string{"cover-1.webm"}) {
		t.Errorf("в папке %v", l.listDir())
	}
	// Пустой список — не ошибка и не сеть.
	before := l.hits.Load()
	if _, err := l.svc.Download(nil); err != nil || l.hits.Load() != before {
		t.Errorf("пустой список: %v, запросов +%d", err, l.hits.Load()-before)
	}
}

// Первая ошибка обрывает ход: id нет в каталоге — ErrGone, прежние ролики остаются, следующие не качаются.
func TestDownloadGoneStops(t *testing.T) {
	l := newLab(t)
	l.webm("aaa", 1, 3000)
	l.webm("ccc", 1, 3000)
	items, err := l.svc.Download([]string{"aaa", "bbb", "ccc"})
	if !errors.Is(err, ErrGone) {
		t.Fatalf("ошибка: %v", err)
	}
	if !slices.Equal(l.listDir(), []string{"aaa-1.webm"}) {
		t.Errorf("в папке %v", l.listDir())
	}
	if a, _ := readyOf(items, "aaa"); !a.Ready {
		t.Errorf("список при ошибке не обновлён: %+v", items)
	}
}

// Обрыв, 500 и пустой ответ — ни *.tmp, ни целевого файла.
func TestDownloadFailNoTmp(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 50_000)
	for name, handler := range map[string]http.HandlerFunc{
		"500": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", http.StatusInternalServerError) },
		"обрыв": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "50000")
			_, _ = w.Write(make([]byte, 100))
			panic(http.ErrAbortHandler)
		},
		"пусто": func(http.ResponseWriter, *http.Request) {},
	} {
		l.mux = http.NewServeMux()
		l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(l.envelope()) })
		l.mux.HandleFunc("/releases/download/", handler)
		_, err := l.svc.Download([]string{"cover"})
		if err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
		if got := l.listDir(); len(got) != 0 {
			t.Errorf("%s: в папке %v", name, got)
		}
	}
}

// Остатки прошлых запусков: убираются только свои обычные gv-*.tmp; папка, ссылка и чужие *.tmp целы.
func TestTmpCleanOnlyOwn(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	l.put("gv-old123.tmp", []byte("остаток"))
	l.put("gv-other.tmp", []byte("остаток"))
	l.put("other.tmp", []byte("чужой"))
	l.put("agv-x.tmp", []byte("чужой"))
	l.put("gv-x.tmp.keep", []byte("чужой"))
	l.put("gv-.tmp", []byte("слишком короткое имя"))
	if err := os.Mkdir(filepath.Join(l.dir, "gv-dir.tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(testkit.TempDir(t), "victim.txt")
	if err := os.WriteFile(target, []byte("жертва"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(l.dir, "gv-link.tmp")); err != nil {
		t.Logf("ссылки недоступны: %v", err)
	}
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	got := l.listDir()
	slices.Sort(got)
	want := []string{"agv-x.tmp", "cover-1.webm", "gv-.tmp", "gv-dir.tmp", "gv-link.tmp", "gv-x.tmp.keep", "other.tmp"}
	if _, err := os.Lstat(filepath.Join(l.dir, "gv-link.tmp")); err != nil {
		want = slices.DeleteFunc(want, func(s string) bool { return s == "gv-link.tmp" })
	}
	if !slices.Equal(got, want) {
		t.Errorf("в папке %v, ждали %v", got, want)
	}
	if b, _ := os.ReadFile(target); string(b) != "жертва" {
		t.Error("цель ссылки тронута")
	}
}

// Вторая загрузка во время первой — ErrBusy без запроса; первая доходит до конца (под -race).
func TestDownloadBusy(t *testing.T) {
	l := newLab(t)
	l.webm("aaa", 1, 3000)
	l.webm("bbb", 1, 3000)
	started, release := make(chan struct{}), make(chan struct{})
	inner := l.mux
	l.mux = http.NewServeMux()
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, r *http.Request) { inner.ServeHTTP(w, r) })
	l.mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		inner.ServeHTTP(w, r)
	})
	done := make(chan error, 1)
	go func() { _, err := l.svc.Download([]string{"aaa"}); done <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("первая загрузка не дошла до файла")
	}
	before := l.hits.Load()
	items, err := l.svc.Download([]string{"bbb"})
	if !errors.Is(err, ErrBusy) || err.Error() != "ролик уже скачивается" {
		t.Errorf("вторая: %v", err)
	}
	if it, _ := readyOf(items, "bbb"); l.hits.Load() != before || it.Ready {
		t.Errorf("запросов +%d, список %v", l.hits.Load()-before, items)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("первая: %v", err)
	}
	// После конца замок свободен.
	if _, err := l.svc.Download([]string{"bbb"}); err != nil {
		t.Errorf("после конца: %v", err)
	}
}

// Отзыв согласия отменяет идущую загрузку: ErrCancelled, ни файла, ни *.tmp.
func TestConsentRevokeCancelsVideo(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	started := make(chan struct{}, 1)
	inner := l.mux
	l.mux = http.NewServeMux()
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, r *http.Request) { inner.ServeHTTP(w, r) })
	l.mux.HandleFunc("/releases/download/", func(_ http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	})
	res := make(chan error, 1)
	go func() { _, err := l.svc.Download([]string{"cover"}); res <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("загрузка не дошла до файла")
	}
	l.allowed.Store(false) // SetLibraryNet(false): запись согласия, затем Cancel
	l.svc.Cancel()
	select {
	case err := <-res:
		if !errors.Is(err, ErrCancelled) || err.Error() != "скачивание ролика отменено" {
			t.Fatalf("ошибка: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("загрузка не отменилась")
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
	// Служба свободна и снова просит согласие.
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrNoConsent) {
		t.Errorf("после отзыва: %v", err)
	}
	l.svc.Cancel() // без идущей загрузки — тихо
}

// Отзыв между первой проверкой согласия и begin: вторая сверка ловит его до первого запроса.
func TestDownloadRechecksConsent(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	var calls atomic.Int32
	l.host.Allowed = func() bool { return calls.Add(1) == 1 }
	l.rebuild()
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrNoConsent) {
		t.Fatalf("ошибка: %v", err)
	}
	if calls.Load() != 2 || l.hits.Load() != 0 {
		t.Errorf("сверок согласия %d, запросов %d", calls.Load(), l.hits.Load())
	}
	// Замок отпущен, отмена снята.
	l.host.Allowed = func() bool { return true }
	l.rebuild()
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Errorf("после: %v", err)
	}
}

// Контекст выхода программы отменяет загрузку.
func TestDownloadStopCtx(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l.host.StopCtx = func() context.Context { return ctx }
	l.rebuild()
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrCancelled) {
		t.Fatalf("ошибка: %v", err)
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
}

// Сбой Rename: ни временного, ни целевого файла.
func TestDownloadRenameFail(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	l.host.Rename = func(_, _ string) error { return errors.New("rename " + l.dir + ": отказано") }
	l.rebuild()
	_, err := l.svc.Download([]string{"cover"})
	if !errors.Is(err, ErrVideoDisk) || err.Error() != "ролик не записывается на диск" {
		t.Fatalf("ошибка: %v", err)
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
}

// Мало места: отказ до запроса файла, нет файла и *.tmp.
func TestDownloadNoSpace(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	l.host.Free = func(string) (uint64, error) { return 100, nil }
	l.rebuild()
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrVideoDisk) {
		t.Fatalf("ошибка: %v", err)
	}
	if l.hits.Load() != 1 { // каталог
		t.Errorf("запросов %d, ждали один (каталог)", l.hits.Load())
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
	// Не удалось узнать место — не повод отказывать.
	l.host.Free = func(string) (uint64, error) { return 0, errors.New("нет") }
	l.rebuild()
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Errorf("ошибка Free: %v", err)
	}
}

// Папка кэша не абсолютная или пустая: служба выключена, на диск ничего не пишется.
func TestCacheDirNotAbsDisabled(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	l.refresh()
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	for _, dir := range []string{"", ".", "rel/dir", "guide-videos"} {
		l.host.CacheDir = func() string { return dir }
		l.rebuild()
		hits := l.hits.Load()
		items, err := l.svc.Download([]string{"cover"})
		if !errors.Is(err, ErrUnavailable) || err.Error() != "ролики недоступны" || len(items) != 0 {
			t.Errorf("%q: Download %v, %v", dir, items, err)
		}
		if got := l.svc.List(); got == nil || len(got) != 0 {
			t.Errorf("%q: List = %#v", dir, got)
		}
		if _, err := l.svc.RevealPath("cover"); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%q: RevealPath %v", dir, err)
		}
		if l.hits.Load() != hits {
			t.Errorf("%q: запросы при выключенной службе", dir)
		}
		if list, _ := os.ReadDir(cwd); len(list) != 0 {
			t.Errorf("%q: в текущей папке появилось %v", dir, list)
		}
	}
}

// Папка кэша — ссылка: служба в неё не пишет.
func TestCacheDirSymlinkRefused(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	target := filepath.Join(testkit.TempDir(t), "real")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := testkit.DirLink(t, target, filepath.Join(testkit.TempDir(t), "link"))
	l.host.CacheDir = func() string { return link }
	l.rebuild()
	hits := l.hits.Load()
	if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ошибка: %v", err)
	}
	if list, _ := os.ReadDir(target); len(list) != 0 {
		t.Errorf("в цели ссылки %v", list)
	}
	if l.hits.Load() != hits {
		t.Errorf("папка-ссылка: сеть тронута, запросов +%d", l.hits.Load()-hits)
	}
}

// guide:video не чаще progressGap: сотня кусков подряд — первое событие и конец; Done растёт.
func TestProgressThrottled(t *testing.T) {
	l := newLab(t)
	report := l.svc.progress("cover", 1000)
	for done := int64(10); done <= 1000; done += 10 {
		report(done)
	}
	l.mu.Lock()
	ev := append([]Progress(nil), l.events...)
	l.mu.Unlock()
	if len(ev) != 2 || ev[0].Done != 10 || ev[1] != (Progress{ID: "cover", Done: 1000, Total: 1000}) {
		t.Errorf("события: %v", ev)
	}
}

// Адрес файла — из свежего каталога, а не из сохранённого (подмена файла каталога на диске).
func TestDownloadUsesFreshURL(t *testing.T) {
	l := newLab(t)
	body := l.webm("cover", 1, 3000)
	stale := fetch.Catalog{Seq: 1, Videos: []fetch.Video{{ID: "cover", Version: 1, MinApp: "0.30.0", Seconds: 5,
		Files: map[string]fetch.VideoFile{"webm": {Size: int64(len(body)), SHA256: strings.Repeat("0", 64), URL: "https://evil.example/cover.webm"}}}}}
	l.cached.Store(&stale)
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(l.path("cover", 1, "webm")); !bytes.Equal(got, body) {
		t.Error("файл не из свежего каталога")
	}
}

// Свежий каталог не получен: ошибка fetch как есть (без путей), отмена — своим текстом.
func TestDownloadFreshErrors(t *testing.T) {
	l := newLab(t)
	for _, tc := range []struct{ in, want error }{
		{fetch.ErrOffline, fetch.ErrOffline},
		{fetch.ErrSignature, fetch.ErrSignature},
		{fetch.ErrCancelled, ErrCancelled},
	} {
		l.host.Fresh = func(context.Context) (fetch.Catalog, error) { return fetch.Catalog{}, tc.in }
		l.rebuild()
		if _, err := l.svc.Download([]string{"cover"}); !errors.Is(err, tc.want) {
			t.Errorf("%v → %v", tc.in, err)
		}
	}
	if got := l.listDir(); len(got) != 0 {
		t.Errorf("в папке %v", got)
	}
}

// Тексты ошибок про ролик, без путей, адресов и слов про наборы.
func TestVideoErrorsNoPath(t *testing.T) {
	l := newLab(t)
	for _, e := range []error{ErrNoConsent, ErrGone, ErrBusy, ErrUnavailable, ErrVideoBroken, ErrVideoDisk, ErrVideoSlow, ErrCancelled, ErrNoFile, ErrReveal, ErrVideoTLS, ErrVideoAddr} {
		checkText(t, l, e)
	}
	for _, tc := range []struct{ in, want error }{
		{fetch.Broken("ролик"), ErrVideoBroken},
		{&fetch.SlowError{Title: "ролик", Limit: time.Minute}, ErrVideoSlow},
		{fetch.ErrDisk, ErrVideoDisk},
		{fetch.ErrCancelled, ErrCancelled},
		{fetch.ErrTLS, ErrVideoTLS},
		{fetch.ErrAddress, ErrVideoAddr},
		{&fetch.StatusError{Code: 404}, ErrGone},
	} {
		got := human(tc.in, true)
		if !errors.Is(got, tc.want) {
			t.Errorf("human(%v) = %v", tc.in, got)
		}
		checkText(t, l, got)
		// Проверка подлинности и адрес: те же тексты и при ошибке свежего каталога.
		if tc.in == fetch.ErrTLS || tc.in == fetch.ErrAddress {
			if again := human(tc.in, false); !errors.Is(again, tc.want) {
				t.Errorf("human(%v, каталог) = %v", tc.in, again)
			}
		}
	}
	// Сквозная проверка: подложенный путь в ошибке ОС наружу не выходит.
	l.webm("cover", 1, 3000)
	l.host.Rename = func(_, _ string) error { return &os.PathError{Op: "rename", Path: l.dir, Err: os.ErrPermission} }
	l.rebuild()
	_, err := l.svc.Download([]string{"cover"})
	checkText(t, l, err)
}

// Неудачи скачивания попадают в журнал (id и текст ошибки, без путей и адресов); отмена — нет.
func TestDownloadErrorsLogged(t *testing.T) {
	l := newLab(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	fresh := l.host.Fresh
	// Каталог не получен.
	l.host.Fresh = func(context.Context) (fetch.Catalog, error) { return fetch.Catalog{}, fetch.ErrOffline }
	l.rebuild()
	_, _ = l.svc.Download([]string{"cover"})
	if got := buf.String(); !strings.Contains(got, "ролики: каталог — ") {
		t.Fatalf("журнал каталога: %q", got)
	}
	// Файл не скачан: ролик в каталоге, а на сервере его нет.
	buf.Reset()
	l.host.Fresh = fresh
	l.rebuild()
	l.webm("cover", 1, 3000)
	l.mu.Lock()
	l.files = map[string][]byte{}
	l.mu.Unlock()
	if _, err := l.svc.Download([]string{"cover"}); err == nil {
		t.Fatal("ожидалась ошибка")
	}
	got := buf.String()
	if !strings.Contains(got, "ролики: cover — ") {
		t.Fatalf("журнал файла: %q", got)
	}
	for _, bad := range []string{l.dir, filepath.Dir(l.dir), "http", "127.0.0.1"} {
		if strings.Contains(got, bad) {
			t.Errorf("журнал содержит %q: %q", bad, got)
		}
	}
	// Отмена в журнал не пишется.
	buf.Reset()
	l.host.Fresh = func(context.Context) (fetch.Catalog, error) { return fetch.Catalog{}, fetch.ErrCancelled }
	l.rebuild()
	_, _ = l.svc.Download([]string{"cover"})
	if buf.Len() != 0 {
		t.Errorf("отмена в журнале: %q", buf.String())
	}
}

func checkText(t *testing.T, l *lab, err error) {
	t.Helper()
	s := err.Error()
	for _, bad := range []string{l.dir, filepath.Dir(l.dir), "http", "127.0.0.1", "набор", "обновлени", "библиотеки"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(bad)) {
			t.Errorf("текст %q содержит %q", s, bad)
		}
	}
}

// Список: пусто без каталога; ролики по алфавиту; Ready — по размеру файла, не по ссылке и не по папке.
func TestListStates(t *testing.T) {
	l := newLab(t)
	if got := l.svc.List(); got == nil || len(got) != 0 {
		t.Fatalf("без каталога: %#v", got)
	}
	l.webm("bbb", 1, 3000)
	l.webm("aaa", 1, 4000)
	l.webm("ccc", 1, 5000)
	l.webm("ddd", 1, 6000)
	l.refresh()
	l.put("aaa-1.webm", make([]byte, 4000))
	l.put("bbb-1.webm", make([]byte, 2999)) // размер не тот
	if err := os.Mkdir(l.path("ccc", 1, "webm"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := l.svc.List()
	var ids []string
	ready := map[string]bool{}
	for _, it := range got {
		ids = append(ids, it.ID)
		ready[it.ID] = it.Ready
	}
	if !slices.Equal(ids, []string{"aaa", "bbb", "ccc", "ddd"}) {
		t.Errorf("порядок: %v", ids)
	}
	if !ready["aaa"] || ready["bbb"] || ready["ccc"] || ready["ddd"] {
		t.Errorf("готовность: %v", ready)
	}
}
