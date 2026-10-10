package app

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// mustEditor — album.NewEditor для тестов: отказ проверки проекта — Fatal.
func mustEditor(t testing.TB, p *model.Project, path string) *album.Editor {
	t.Helper()
	e, err := album.NewEditor(p, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// Подмена редактора (NewProject) идёт параллельно с GetState, /media и автосохранением;
// под -race тест падает, если a.ed снова станет обычным полем.
func TestNewProjectRace(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	h := a.media(http.NotFoundHandler())
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 50 {
				a.GetState()
				a.flushSave()
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/media/t/x", nil))
			}
		})
	}
	for range 50 {
		a.NewProject("album")
	}
	wg.Wait()
}

// Проект, который NewEditor не принял при запуске (Load такого не отдаёт, 023 Ш7), — пустой
// альбом, уведомление и запрет записи в черновик: автосохранение не затрёт его пустым.
func TestStartEditorRefused(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	p := model.New()
	p.Folders = []string{`\\srv\share`}
	path, notice := a.startEditor(p, filepath.Join(a.cfgDir, "альбом"+projectExt), "Прежнее.")
	if path != "" || !strings.HasPrefix(notice, "Прежнее. Не открылся проект") || !a.draftLocked.Load() ||
		a.editor() == nil || len(a.editor().Folders()) != 0 {
		t.Fatalf("путь %q, уведомление %q, черновик закрыт %v", path, notice, a.draftLocked.Load())
	}
}

func TestLoadStartupBadDraftKept(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	os.WriteFile(a.draftPath(), []byte("{не json"), 0o644)

	p, path, notice := a.loadStartup("")
	if p == nil || path != "" {
		t.Fatalf("ждали пустой проект без пути, получили %v %q", p, path)
	}
	if _, err := os.Stat(a.draftPath()); !os.IsNotExist(err) {
		t.Fatal("сломанный черновик должен быть переименован")
	}
	bad, _ := filepath.Glob(filepath.Join(a.cfgDir, "черновик.bad-*"+projectExt))
	if len(bad) != 1 || !strings.Contains(notice, bad[0]) {
		t.Fatalf("копия %v, уведомление %q", bad, notice)
	}
	// Автосохранение пустого проекта пишет новый черновик и не трогает .bad.
	a.ed.Store(mustEditor(t, p, path))
	a.flushSave()
	data, _ := os.ReadFile(bad[0])
	if string(data) != "{не json" {
		t.Fatal(".bad затёрт")
	}
	if _, err := os.Stat(a.draftPath()); err != nil {
		t.Fatal("новый черновик не записан:", err)
	}
}

// Черновик более новой версии лежит на месте байт-в-байт: не .bad, не затёрт автосохранением,
// не удалён «Новым» проектом.
func TestLoadStartupNewerDraftUntouched(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	raw := []byte(`{"version": 99, "новое": [1, 2]}`)
	os.WriteFile(a.draftPath(), raw, 0o644)

	p, path, notice := a.loadStartup("")
	if p == nil || path != "" || !strings.Contains(notice, "более новой версией программы") {
		t.Fatalf("проект %v, путь %q, уведомление %q", p, path, notice)
	}
	if !a.draftNewer.Load() {
		t.Fatal("флаг draftNewer не поднят")
	}
	check := func(when string) {
		t.Helper()
		if data, _ := os.ReadFile(a.draftPath()); !bytes.Equal(data, raw) {
			t.Fatalf("%s: черновик изменён: %q", when, data)
		}
		if bad, _ := filepath.Glob(filepath.Join(a.cfgDir, "черновик.bad-*")); len(bad) != 0 {
			t.Fatalf("%s: появился %v", when, bad)
		}
	}
	check("старт")
	a.ed.Store(mustEditor(t, p, path))
	a.flushSave()
	check("автосохранение")
	a.NewProject("album")
	check("новый проект")
}

func TestLoadStartupBadLastProjectUntouched(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	foreign := filepath.Join(testkit.TempDir(t), "чужой"+projectExt)
	os.WriteFile(foreign, []byte("{не json"), 0o644)

	p, path, notice := a.loadStartup(foreign)
	if path != "" || notice == "" {
		t.Fatalf("путь %q, уведомление %q", path, notice)
	}
	a.ed.Store(mustEditor(t, p, path))
	a.flushSave() // пишет только черновик
	if data, _ := os.ReadFile(foreign); string(data) != "{не json" {
		t.Fatal("чужой файл изменён")
	}
	if _, err := os.Stat(a.draftPath()); err != nil {
		t.Fatal("черновик не записан")
	}
}

func TestLoadStartupMissingIsSilent(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if _, _, notice := a.loadStartup(""); notice != "" {
		t.Fatalf("отсутствие файлов — не ошибка: %q", notice)
	}
}

func TestLoadStartupMissingLastNotice(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	last := filepath.Join(a.cfgDir, "нет"+projectExt)
	_, _, notice := a.loadStartup(last)
	if !strings.Contains(notice, "Последний проект не найден: "+last) {
		t.Fatalf("уведомление %q", notice)
	}
}

// Застрявший черновик (draftLocked) не удаляется ни «Новым», ни «Сохранить как».
func TestNewProjectKeepsLockedDraft(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	os.WriteFile(a.draftPath(), []byte("{не json"), 0o644)
	a.draftLocked.Store(true)
	a.rename = func(string, string) error { return os.ErrPermission }

	before := a.editor()
	a.NewProject("album")
	if a.editor() != before {
		t.Fatal("проект не должен подменяться, пока черновик не убран")
	}
	if !a.draftLocked.Load() {
		t.Fatal("флаг должен остаться")
	}
	if data, _ := os.ReadFile(a.draftPath()); string(data) != "{не json" {
		t.Fatal("черновик тронут")
	}

	// Переименование получилось: флаг снят, черновик ушёл в .bad, проект новый.
	a.rename = os.Rename
	a.NewProject("album")
	if a.editor() == before || a.draftLocked.Load() {
		t.Fatal("новый проект должен создаться, флаг сняться")
	}
	bad, _ := filepath.Glob(filepath.Join(a.cfgDir, "черновик.bad-*"+projectExt))
	if len(bad) != 1 {
		t.Fatalf("копия %v", bad)
	}
	if data, _ := os.ReadFile(bad[0]); string(data) != "{не json" {
		t.Fatal(".bad испорчен")
	}
}

// Консоль без дескриптора (собранный exe) не должна мешать записи в файл.
func TestLogWriterConsoleFails(t *testing.T) {
	const line = "запуск"
	var file bytes.Buffer
	w := logWriter{file: &file, console: failWriter{}}
	n, err := w.Write([]byte(line))
	if err != nil || n != len(line) || file.String() != line {
		t.Fatalf("n=%d err=%v file=%q", n, err, file.String())
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("нет консоли") }

func TestRotateLog(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "log.txt")
	os.WriteFile(path, make([]byte, 100), 0o644)

	rotateLog(path, 200)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("маленький лог не должен уходить в .old")
	}
	rotateLog(path, 50)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("большой лог должен быть переименован")
	}
	if _, err := os.Stat(filepath.Join(dir, "log.old.txt")); err != nil {
		t.Fatal("нет log.old.txt:", err)
	}
	// Повторная ротация заменяет старый .old.
	os.WriteFile(path, make([]byte, 100), 0o644)
	rotateLog(path, 50)
	if _, err := os.Stat(filepath.Join(dir, "log.old.txt")); err != nil {
		t.Fatal(err)
	}
}

// newApp — App для теста: к очистке теста фоновая работа (сканирование, прогрев, чистка кэша,
// автосохранение) остановлена и дождана, а папки теста удаляются уже после этого —
// иначе горутина программы ещё пишет в удаляемую папку или держит её открытой (Windows:
// «The directory is not empty» на очистке).
func newApp(t testing.TB) *App {
	a := NewApp()
	close(a.ready) // startup в тестах не вызывается
	testkit.BeforeRemove(t, a.closeBackground)
	return a
}

// После closeBackground таймер автосохранения ничего не пишет (иначе он пересоздал бы
// удалённую папку теста), а сканирование и прогрев дождались.
func TestCloseBackgroundSilencesAutosave(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	a.changed(a.editor().View())
	a.rescan()
	a.closeBackground()
	a.autosave() // как сработавший таймер
	if _, err := os.Stat(a.draftPath()); !os.IsNotExist(err) {
		t.Errorf("черновик записан после остановки: %v", err)
	}
	a.flushSave() // явное сохранение (выход из программы) по-прежнему пишет
	if _, err := os.Stat(a.draftPath()); err != nil {
		t.Errorf("flushSave не записал черновик: %v", err)
	}
}

// instanceID — замок второго экземпляра. Смена значения пустит две версии программы
// параллельно, и они поделят config.json и черновик: менять только осознанно.
func TestInstanceIDStable(t *testing.T) {
	if instanceID != "6f1c2b7e-3a4d-4e58-9b0a-5d2e8c7f1a93" {
		t.Fatalf("instanceID сменился: %s", instanceID)
	}
}

func TestAppVersion(t *testing.T) {
	cases := map[string]string{
		`{"info":{"productVersion":"1.2.3"}}`: "1.2.3",
		`{"info":{}}`:                         "dev",
		`не json`:                             "dev",
		``:                                    "dev",
	}
	for in, want := range cases {
		if got := appVersion([]byte(in)); got != want {
			t.Errorf("appVersion(%q) = %q, want %q", in, got, want)
		}
	}
	// wails.json (его встраивает main) обязан давать настоящую версию.
	raw, err := os.ReadFile("../../wails.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := appVersion(raw); got == "dev" {
		t.Error("в wails.json нет info.productVersion")
	}
}

// GetState, вызванный окном раньше конца startup, ждёт его и отдаёт уже загруженный
// проект и ошибку запуска, а не пустой альбом.
func TestGetStateWaitsStartup(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.ready = make(chan struct{}) // как после NewApp: startup ещё идёт

	got := make(chan State, 1)
	go func() { got <- a.GetState() }()
	select {
	case <-got:
		t.Fatal("GetState вернулся до конца startup")
	case <-time.After(50 * time.Millisecond):
	}

	const path = `C:\альбом.salbum`
	a.ed.Store(mustEditor(t, model.New(), path))
	a.mu.Lock()
	a.startNotice = "не открылся"
	a.mu.Unlock()
	close(a.ready)

	select {
	case st := <-got:
		if st.View.Path != path || st.Notice != "не открылся" {
			t.Errorf("View.Path=%q Notice=%q: снимок до startup", st.View.Path, st.Notice)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("GetState не вернулся после startup")
	}
}

// Остановка программы отпускает GetState, даже если startup так и не закончился.
func TestGetStateStopReleases(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.ready = make(chan struct{})
	got := make(chan State, 1)
	go func() { got <- a.GetState() }()
	a.stopBackground()
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("GetState не вернулся после остановки")
	}
}
