package app

import (
	"bytes"
	"errors"
	"image/jpeg"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
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
	// Итог чтения папок (rescan в фоне) — обычная строка; остального быть не должно. Буфер
	// читается после конца фоновых rescan: иначе их log.Printf пишет в него одновременно (гонка).
	a.bg.Wait()
	for line := range strings.Lines(buf.String()) {
		if !strings.Contains(line, "папок 0: 0 фото") {
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

// coverEnv — альбом из n разворотов с обложкой: фото p0 на лицевой, плашка корешка красная.
func coverEnv(t *testing.T, n int, photo string) *exportEnv {
	t.Helper()
	e := newExportEnv(t, n)
	if _, err := e.a.CreateCover("", photo); err != nil {
		t.Fatal(err)
	}
	if v := e.a.SetCoverSpineBg("#ff0000"); v.Cover.SpineBg != "#ff0000" {
		t.Fatalf("плашка %q", v.Cover.SpineBg)
	}
	return e
}

// jpegAt читает 00.jpg и возвращает размер и цвет точки (x, y) мм листа при dpi.
func jpegAt(t *testing.T, path string, dpi int, x, y float64) (w, h int, c [3]uint8) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(int(math.Round(x/25.4*float64(dpi))), int(math.Round(y/25.4*float64(dpi)))).RGBA()
	return img.Bounds().Dx(), img.Bounds().Dy(), [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// 00.jpg: размер листа обложки при dpi (как pxW×pxH вида), плашка корешка видна, развороты рядом.
func TestExportCoverFile(t *testing.T) {
	e := coverEnv(t, 3, "p0")
	v := e.a.editor().View()
	var spine model.CoverZone
	for _, z := range v.Cover.Zones {
		if z.Kind == "spine" {
			spine = z
		}
	}
	// dpi 0 — из настроек альбома: размер файла совпадает с pxW×pxH из вида.
	ok, err := e.a.Export(ExportOptions{Cover: true})
	if err != nil || !ok {
		t.Fatalf("Export: %v %v", ok, err)
	}
	if msg := <-e.done; msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	dpi := v.Settings.DPI
	w, h, c := jpegAt(t, filepath.Join(e.dir, "00.jpg"), dpi, spine.Rect.X+spine.Rect.W/2, 5)
	if w != v.Cover.PxW || h != v.Cover.PxH {
		t.Errorf("00.jpg %d×%d px, ждали %d×%d", w, h, v.Cover.PxW, v.Cover.PxH)
	}
	if c[0] < 230 || c[1] > 25 || c[2] > 25 {
		t.Errorf("плашка корешка не видна: %v", c)
	}
	// Повтор: наш 00.jpg заменяется после вопроса.
	e.asked = nil
	if msg := e.export(t, ExportOptions{Cover: true, Range: "2"}); msg != "" {
		t.Fatal(msg)
	}
	if len(e.asked) != 1 || !strings.Contains(e.asked[0], "Заменить обложку и файлы разворота 2 (2 шт.)?") {
		t.Fatalf("вопрос: %q", e.asked)
	}
}

// «Только обложка» — один файл; развороты, Range и хвосты прежнего экспорта не трогаются.
func TestExportCoverOnly(t *testing.T) {
	e := coverEnv(t, 3, "p0")
	e.export(t, ExportOptions{})
	e.oldify(t)
	e.asked = nil
	if msg := e.export(t, ExportOptions{CoverOnly: true, Range: "garbage"}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	if !e.rewritten(t, "00.jpg") || e.rewritten(t, "01.jpg") || e.rewritten(t, "03.jpg") || len(e.asked) != 0 {
		t.Fatalf("тронуты развороты или вопрос %q", e.asked)
	}
	// Чистая папка: только 00.jpg; без Cover «только обложка» тоже включает обложку.
	e.dir = filepath.Join(e.dir, "only")
	if msg := e.export(t, ExportOptions{CoverOnly: true}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	// Альбом без разворотов, но с обложкой, выгружается.
	e0 := coverEnv(t, 0, "")
	if msg := e0.export(t, ExportOptions{Cover: true}); msg != "" || !slices.Equal(e0.names(t), []string{"00.jpg"}) {
		t.Fatalf("без разворотов: %q %v", msg, e0.names(t))
	}
}

// Обложки нет — флаги игнорируются и выгружаются развороты; чужой 00.jpg при выгрузке
// обложки — отказ без вопроса; без обложки в плане он не задевается.
func TestExportCoverIgnoredAndForeign(t *testing.T) {
	e := newExportEnv(t, 2)
	if msg := e.export(t, ExportOptions{Cover: true, CoverOnly: true}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("без обложки: %v", e.names(t))
	}
	c := coverEnv(t, 2, "p0")
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, c.dir, "00.jpg", 10, 10)
	ok, err := c.a.Export(ExportOptions{DPI: 72, Cover: true})
	if ok || err == nil || !strings.Contains(err.Error(), "00.jpg") || len(c.asked) != 0 {
		t.Fatalf("чужой 00.jpg: %v, %v, вопросы %q", ok, err, c.asked)
	}
	if msg := c.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	if st, err := os.Stat(filepath.Join(c.dir, "00.jpg")); err != nil || st.Size() > 5000 {
		t.Fatalf("чужой 00.jpg тронут: %v", err)
	}
}

// Пустая обложка — то же предупреждение, что о пустых разворотах, до выбора папки.
func TestExportCoverBlankAsks(t *testing.T) {
	e := coverEnv(t, 1, "")
	e.a.SetCoverSpineBg("") // плашка — тоже печать
	e.answer = false
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App) (string, error) { picked++; return pick(a) }
	ok, err := e.a.Export(ExportOptions{DPI: 72, Cover: true})
	if ok || err != nil || picked != 0 || len(e.asked) != 1 || !strings.Contains(e.asked[0], "Обложка пустая") {
		t.Fatalf("Export = %v, %v; папка %d; вопросы %q", ok, err, picked, e.asked)
	}
	// Без галочки обложки о ней не спрашиваем.
	e.asked = nil
	if msg := e.export(t, ExportOptions{}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("без обложки: %q, вопросы %q", msg, e.asked)
	}
}

// Подстановки (030) в тексте обложки: на холсте «[Заголовок]» (прежнее {Школа} — синоним), в печати — пусто.
func TestCoverTextExpands(t *testing.T) {
	e := coverEnv(t, 1, "p0")
	ed := e.a.editor()
	v, err := ed.AddText("cover", 0, "")
	if err != nil || len(v.Cover.Sheet.Texts) != 1 {
		t.Fatalf("текст обложки: %v %+v", err, v.Cover)
	}
	v = ed.SetText(v.Cover.Sheet.Texts[0].ID, "{Школа}", "")
	if got := v.Cover.Sheet.Texts[0].Text; got != "[Заголовок]" {
		t.Errorf("на холсте %q", got)
	}
	p, pv := ed.Snapshot()
	if got := pv.Cover.Sheet.Texts[0]; got.Text != "" || printed(got.Lines) != "" {
		t.Errorf("в печати %q, строки %q", got.Text, printed(got.Lines))
	}
	job := export.NewCoverJob(p, pv, 72, 95)
	if job == nil || len(job.Spread.Texts) != 1 || printed(job.Spread.Texts[0].Box.Lines) != "" {
		t.Errorf("задание обложки: %+v", job)
	}
}

// printed — текст всех строк блока подряд.
func printed(lines []text.Line) (out string) {
	for _, l := range lines {
		out += l.Text
	}
	return out
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
