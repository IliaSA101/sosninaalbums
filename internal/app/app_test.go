package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/edit"
	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
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
	if p == nil || path != "" || notice != "" {
		t.Fatalf("проект %v, путь %q, уведомление %q", p, path, notice)
	}
	// Вместо заметки — окно «Черновик из новой версии» (112): State.Newer, один раз.
	if n := a.GetState().Newer; n == nil || !n.Draft || n.Name != "" || n.Need != "" || a.GetState().Newer != nil {
		t.Fatalf("State.Newer черновика: %+v", n)
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

// TestNewSheetProject — «Виньетка (лист)» (031): проект-лист без корешка — Settings.Sheet,
// pageW = ширина листа, одна страница во весь лист, layflat, один лист-виньетка без фото; тип изделия
// album (иначе NormProduct пишет в log.txt). SetSettings лист в альбом не превращает.
func TestNewSheetProject(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	v := a.NewVignetteSheet(204, 304).View
	s := v.Settings
	if !s.Sheet || s.PageW != 204 || s.PageH != 304 || !s.Layflat || s.Margin != 8 || s.Gap != 3 || s.Safe != 5 || s.DPI != 300 {
		t.Fatalf("настройки листа %+v", s)
	}
	if v.Product != model.ProductAlbum || len(v.Spreads) != 1 || len(v.Spreads[0].Photos) != 0 || v.Spreads[0].Kind != model.KindVignette {
		t.Fatalf("изделие %q, листов %d", v.Product, len(v.Spreads))
	}
	p, _ := a.editor().Snapshot()
	g := p.Geo(p.Spreads[0])
	if g.W != 204 || g.H != 304 || g.Mid != 0 || len(g.Pages) != 1 || g.Pages[0].W != 204 || !g.Layflat {
		t.Fatalf("геометрия листа %+v", g)
	}
	if prm := p.Params(); prm.PageW != 102 || !prm.Sheet || !prm.Layflat {
		t.Fatalf("layout получил %+v, ждали половину листа и Sheet", prm)
	}
	if len(v.Safe) != 1 || v.Safe[0].X+v.Safe[0].W > 204 {
		t.Fatalf("безопасная зона %+v вне листа", v.Safe)
	}

	// Новый формат из настроек: Sheet из проекта, layflat — всегда.
	s.PageW, s.PageH, s.Layflat, s.Sheet = 304, 404, false, false
	if got := a.SetSettings(s).Settings; !got.Sheet || !got.Layflat || got.PageW != 304 {
		t.Fatalf("после SetSettings %+v", got)
	}
	// И обратно: альбом листом не становится.
	a.NewProject("album")
	al := a.GetState().View.Settings
	al.Sheet = true
	if a.SetSettings(al).Settings.Sheet {
		t.Fatal("альбом стал листом")
	}
	// Итог чтения папок (rescan в фоне) и следом память (089 G3) — обычные строки; остального быть
	// не должно. Буфер читается после конца фоновых rescan: иначе их log.Printf пишет в него
	// одновременно (гонка).
	a.bg.Wait()
	for line := range strings.Lines(buf.String()) {
		if !strings.Contains(line, "папок 0: 0 фото") && !strings.Contains(line, "память: чтение папок — Go ") {
			t.Errorf("в log.txt: %s", line)
		}
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
	// Примера нет: иначе каждому тесту добавилась бы строка «Скачать». Тесты 142 ставят свой.
	a.demoFn = func() libpacks.Demo { return libpacks.Demo{} }
	// Без папки данных prefs пишут config.json в рабочую папку пакета (так в git попал internal/app/config.json).
	a.cfgDir = testkit.TempDir(t)
	// Раннер Windows в CI работает от администратора: настоящий osx.Elevated закрыл бы правки и вопросы.
	a.askM().elevated = func() bool { return false }
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

// Стенды QA (SA_STAND): без стенда замок и все папки данных — прежние побитно; у каждого стенда
// свои, не общие ни с основной копией, ни друг с другом.
func TestStandIsolation(t *testing.T) {
	t.Setenv("APPDATA", testkit.TempDir(t))
	t.Setenv("LOCALAPPDATA", testkit.TempDir(t))
	cfgBase, _ := os.UserConfigDir()
	cacheBase, _ := os.UserCacheDir()
	if NewApp().data != "SosninaAlbums" || instanceKey("") != instanceID {
		t.Fatal("без стенда сменились имя данных или замок")
	}
	cfg, thumbs := dataDirs(osx.DataName(""))
	want := []string{filepath.Join(cfgBase, "SosninaAlbums"), filepath.Join(cacheBase, "SosninaAlbums", "thumbs"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "SosninaAlbums", "WebView2"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "SosninaAlbums", "backups")}
	got := []string{cfg, thumbs, osx.WebViewDir(osx.DataName("")), backup.DefaultRoot(osx.DataName(""))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("без стенда: %q, ждали %q", got, want)
	}
	seen := map[string]string{instanceID: ""}
	for _, p := range want {
		seen[p] = ""
	}
	for _, s := range []string{"b", "c"} {
		name := osx.DataName(s)
		cfg, thumbs := dataDirs(name)
		for _, p := range []string{instanceKey(s), cfg, thumbs, osx.WebViewDir(name), backup.DefaultRoot(name)} {
			if prev, ok := seen[p]; ok {
				t.Fatalf("стенд %q делит %q с %q", s, p, prev)
			}
			seen[p] = s
		}
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
	case <-time.After(testkit.Limit(5 * time.Second)):
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
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("GetState не вернулся после остановки")
	}
}

// «Сохранить как» в папку копий — отказ, файл не создан.
func TestSaveAsIntoBackupsRefused(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	target := filepath.Join(a.backups.Root, "свой"+projectExt)
	if _, err := a.saveAs(a.editor(), target); err == nil {
		t.Fatal("сохранение в папку копий принято")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("файл создан: %v", err)
	}
	// Поверх чужого файла — сначала его копия.
	other := filepath.Join(testkit.TempDir(t), "чужой"+projectExt)
	if err := os.WriteFile(other, []byte(`{"spreads":[{},{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveAs(a.editor(), other); err != nil {
		t.Fatal(err)
	}
	if list, _ := a.backups.List(backup.Key(other)); len(list) != 1 || list[0].Spreads != 3 {
		t.Fatalf("копия перезаписанного файла: %+v", list)
	}
}

// Wails привязывает к JS все экспортируемые методы App, в том числе взятые у встроенного
// *edit.Methods: набор должен совпадать с контрактом биндингов (testdata/bindings-contract.txt).
func TestBoundMethodsMatchContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bindings-contract.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "method "); ok {
			want[rest[:strings.IndexByte(rest, '(')]] = true
		}
	}
	typ := reflect.TypeOf(&App{})
	have := map[string]bool{}
	for i := range typ.NumMethod() {
		have[typ.Method(i).Name] = true
	}
	for name := range want {
		if !have[name] {
			t.Errorf("метода %s нет у App, а в контракте он есть", name)
		}
	}
	for name := range have {
		if !want[name] {
			t.Errorf("метод %s есть у App, но не в контракте (Wails отдаст его интерфейсу)", name)
		}
	}
	if len(want) < 100 {
		t.Fatalf("в контракте только %d методов: файл прочитан не так", len(want))
	}
}

// Метод, объявленный у *App, перекрывает одноимённый встроенный из *edit.Methods: Wails и
// сторож выше увидели бы тот же набор имён, а правка пошла бы другим телом. Имена не пересекаются.
func TestAppMethodsDoNotShadowEdit(t *testing.T) {
	embedded := map[string]bool{}
	typ := reflect.TypeOf(&edit.Methods{})
	for i := range typ.NumMethod() {
		embedded[typ.Method(i).Name] = true
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".go") || strings.HasSuffix(en.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), en.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || !fn.Name.IsExported() {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			if id, ok := recv.(*ast.Ident); !ok || id.Name != "App" {
				continue
			}
			seen++
			if embedded[fn.Name.Name] {
				t.Errorf("(*App).%s перекрывает метод edit.Methods с тем же именем", fn.Name.Name)
			}
		}
	}
	if seen < 50 || len(embedded) < 50 {
		t.Fatalf("разобрано мало методов (App %d, edit %d): сторож смотрит не туда", seen, len(embedded))
	}
}

// editHost отдаёт все поля Host: пустое поле — nil-вызов посреди правки пользователя
// (edit.New тоже отказывается от такого Host паникой с именем поля). Шаблон служб — testkit.HostComplete.
func TestEditHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).editHost())
}

// Фасад обновления задаёт все поля Host службы updater (пустое поле — паника New; не-функция ломает
// проверку IsNil). Шаблон служб — testkit.HostComplete.
func TestUpdaterHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).updHost())
}

// Фасад связи .salbum задаёт все поля Host службы assoc (126 Ш2).
func TestAssocHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).assocHost())
}

// Фасад выгрузки задаёт все поля Host службы exporter (126 Ш3).
func TestExporterHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).expHost())
}

// Фасад наборов задаёт все поля Host службы libpacks (126 Ш4).
func TestLibpacksHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).packsHost())
}

// Фасад типографий задаёт все поля Host службы ownprint (126 Ш5).
func TestOwnprintHostComplete(t *testing.T) {
	testkit.HostComplete(t, newApp(t).printsHost())
}

// AutoOpts 051 из UI по именам JSON; время съёмки из ленты доходит до плана: одна папка, две
// серии с паузой в час — одна граница (без Taken в infos её бы не было), «Авто» даёт From/To.
func TestAutoPlanSeriesFromLibrary(t *testing.T) {
	var o model.AutoOpts
	if err := json.Unmarshal([]byte(`{"spreads":2,"maxPer":9,"minPer":2,"auto":true,"series":true}`), &o); err != nil ||
		!reflect.DeepEqual(o, model.AutoOpts{Spreads: 2, MaxPer: 9, MinPer: 2, Auto: true, Series: true}) {
		t.Fatalf("AutoOpts из JSON: %+v %v", o, err)
	}
	a := newApp(t)
	a.ed.Store(mustEditor(t, model.New(), ""))
	var ids []string
	for i := range 12 {
		id := fmt.Sprint("t", i)
		at := int64(i) * 15
		if i >= 6 {
			at += 3600
		}
		a.byID[id] = library.Photo{ID: id, Path: `C:\свадьба\` + id + `.jpg`, W: 300, H: 200, Taken: 1_700_000_000 + at}
		ids = append(ids, id)
	}
	ap, err := a.AutoPlan(ids, o)
	if err != nil || ap.Cuts != 1 || ap.Spreads != 2 || ap.From < 1 || ap.To < ap.From {
		t.Fatalf("AutoPlan с сериями: %+v %v", ap, err)
	}
	v, err := a.AutoBuild(ids, -1, o)
	if err != nil || len(v.Spreads) != 2 || len(v.Spreads[0].Photos) != 6 {
		t.Fatalf("AutoBuild с сериями: %v, %+v", err, v.Spreads)
	}
}

// Минимум окна по экрану (089-D16): на обычном экране — постоянный, на малом — не больше экрана.
func TestMinSizeFitsScreen(t *testing.T) {
	cases := []struct{ sw, sh, w, h int }{
		{1920, 1080, minWinW, minWinH}, // обычный экран — без изменений
		{1366, 768, minWinW, minWinH},  // 1366×768 при 100 %: 1326×648 — минимум прежний
		{1093, 614, 1053, 494},         // 1366×768 при 125 %
		{1097, 617, 1057, 497},         // 1920×1080 при 175 %
		{800, 600, 760, 480},           // совсем малый
		{30, 100, 1, 1},                // меньше отступов — минимум не 0 и не отрицательный
		{0, 0, minWinW, minWinH},       // размер экрана неизвестен
		{-5, 768, minWinW, minWinH},
	}
	for _, c := range cases {
		w, h := minSize(c.sw, c.sh)
		if w != c.w || h != c.h {
			t.Errorf("minSize(%d, %d) = %d×%d, want %d×%d", c.sw, c.sh, w, h, c.w, c.h)
		}
		if c.sw > 40 && c.sh > 120 && (w > c.sw || h > c.sh) {
			t.Errorf("minSize(%d, %d) = %d×%d — больше экрана", c.sw, c.sh, w, h)
		}
	}
}
