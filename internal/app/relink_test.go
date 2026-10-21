package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// relinkApp — App с альбомом из двух фото: «a» лежит на месте, «b» пропало (папка
// съёмки переехала в moved). Проект сохранён и открыт заново, как при старте.
func relinkApp(t *testing.T) (a *App, moved string, events func() []emitted) {
	t.Helper()
	root := testkit.TempDir(t)
	shoot := filepath.Join(root, "Съёмка")
	moved = filepath.Join(root, "Новая", "Съёмка")
	pa := testkit.JPEG(t, shoot, "a.jpg", 40, 30)
	pb := testkit.JPEG(t, moved, "b.jpg", 50, 30)
	oldB := filepath.Join(shoot, "b.jpg") // его в старой папке нет
	st, err := os.Stat(pb)
	if err != nil {
		t.Fatal(err)
	}
	e := mustEditor(t, model.New(), "")
	if _, _, err := e.AddFolder(shoot); err != nil {
		t.Fatal(err)
	}
	if _, err := e.NewSpread(-1, []model.PhotoInfo{
		{ID: library.IDFor(pa), Path: pa, W: 40, H: 30, Size: fileLen(t, pa)},
		{ID: library.IDFor(oldB), Path: oldB, W: 50, H: 30, Size: st.Size()},
	}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "альбом.salbum")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var evs []emitted
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		evs = append(evs, emitted{name, data})
	}
	a.ed.Store(mustEditor(t, p, file))
	// Копия под замком: события шлёт горутина сканирования, пока тест их читает.
	return a, moved, func() []emitted {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(evs)
	}
}

func fileLen(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func waitScan(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for a.GetState().Scan.Active {
		if time.Now().After(deadline) {
			t.Fatal("сканирование не закончилось")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitMissing ждёт конца фоновых пересчётов недостающих (refreshMissing).
func waitMissing(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for a.missPending.Load() > 0 {
		if time.Now().After(deadline) {
			t.Fatal("пересчёт недостающих не закончился")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestMissingAfterLoad(t *testing.T) {
	a, _, events := relinkApp(t)
	a.rescan()
	waitScan(t, a)
	waitMissing(t, a)
	st := a.GetState()
	if len(st.Missing) != 1 || st.Missing[0] != st.View.Spreads[0].Photos[1] {
		t.Fatalf("недостающие: %v, фото %v", st.Missing, st.View.Spreads[0].Photos)
	}
	last := []string(nil)
	for _, e := range events() {
		if e.name == "missing" {
			last = e.data[0].([]string)
		}
	}
	if !slices.Equal(last, st.Missing) {
		t.Errorf("событие missing: %v, ждали %v", last, st.Missing)
	}
}

func TestRelinkFolderPath(t *testing.T) {
	a, moved, _ := relinkApp(t)
	a.rescan()
	waitScan(t, a)
	res, err := a.RelinkFolderPath(moved)
	if err != nil {
		t.Fatal(err)
	}
	if res.Found != 1 || res.Missing != 0 || res.Ambiguous != 0 {
		t.Fatalf("итог: %+v", res)
	}
	waitScan(t, a)
	waitMissing(t, a)
	if m := a.GetState().Missing; len(m) != 0 {
		t.Errorf("после перепривязки недостающих нет, а есть %v", m)
	}
	if !slices.Contains(res.View.Folders, moved) {
		t.Errorf("найденная папка не добавлена: %v", res.View.Folders)
	}
	// Один шаг отмены возвращает недостающее фото.
	a.Undo()
	waitMissing(t, a)
	if m := a.GetState().Missing; len(m) != 1 {
		t.Errorf("после отмены недостающих %v", m)
	}
}

func TestRelinkFolderPathRejectsUNC(t *testing.T) {
	a, _, _ := relinkApp(t)
	for _, dir := range []string{`\\srv\share\фото`, `\\?\C:\x`, "относительная", ""} {
		before := a.editor().View().Rev
		if _, err := a.RelinkFolderPath(dir); err == nil {
			t.Errorf("%q принят", dir)
		}
		if a.editor().View().Rev != before {
			t.Errorf("%q изменил проект", dir)
		}
	}
	if _, err := a.RelinkFolderPath(filepath.Join(testkit.TempDir(t), "нет")); err == nil {
		t.Error("несуществующая папка принята")
	}
}

func TestRelinkDriveBadLetters(t *testing.T) {
	a, _, _ := relinkApp(t)
	for _, c := range [][2]string{{"E", "E"}, {"", "F"}, {"EE", "F"}, {"1", "F"}} {
		if _, err := a.RelinkDrive(c[0], c[1]); err == nil {
			t.Errorf("%v принято", c)
		}
	}
}

// missingApp — App с одним фото в альбоме (путь вымышленный) и записью событий missing.
func missingApp(t *testing.T) (a *App, id string, last func() ([]string, int)) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var got []string
	n := 0
	a.emit = func(name string, data ...any) {
		if name == "missing" {
			mu.Lock()
			got, n = data[0].([]string), n+1
			mu.Unlock()
		}
	}
	path := filepath.Join(a.cfgDir, "нет.jpg")
	id = library.IDFor(path)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: path, W: 30, H: 20}}); err != nil {
		t.Fatal(err)
	}
	return a, id, func() ([]string, int) {
		mu.Lock()
		defer mu.Unlock()
		return got, n
	}
}

// Проверка файлов висит (мёртвый сетевой том): rescan не ждёт её, событие приходит после.
func TestRescanDoesNotWaitMissing(t *testing.T) {
	a, id, last := missingApp(t)
	gate := make(chan struct{})
	a.exists = func(string) bool { <-gate; return false }

	start := time.Now()
	a.rescan()
	// Проверка висит навсегда, так что любое ожидание — секунды; порог для нагрузки масштабируется.
	if d := time.Since(start); d > testkit.Limit(300*time.Millisecond) {
		t.Fatalf("rescan ждал проверку файлов %v", d)
	}
	if m := a.GetState().Missing; len(m) != 0 {
		t.Fatalf("до конца проверки недостающие %v", m)
	}
	close(gate)
	waitMissing(t, a)
	if got, n := last(); n != 1 || !slices.Equal(got, []string{id}) {
		t.Fatalf("событие missing: %v (%d раз)", got, n)
	}
	if m := a.GetState().Missing; !slices.Equal(m, []string{id}) {
		t.Fatalf("GetState: %v", m)
	}
}

// Первый пересчёт медленный, второй быстрый: остаётся второй, отчёт первого отброшен.
func TestMissingStaleDropped(t *testing.T) {
	a, _, last := missingApp(t)
	gate, entered := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	a.exists = func(string) bool {
		if calls.Add(1) == 1 {
			close(entered)
			<-gate
			return false // устаревшее: «нет файла»
		}
		return true
	}
	a.refreshMissing()
	<-entered
	a.refreshMissing()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, n := last(); n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("второй пересчёт не прислал событие")
		}
		time.Sleep(2 * time.Millisecond)
	}
	close(gate)
	waitMissing(t, a)
	if got, n := last(); n != 1 || len(got) != 0 {
		t.Fatalf("последнее событие %v (%d раз), ждали [] один раз", got, n)
	}
	if m := a.GetState().Missing; len(m) != 0 {
		t.Fatalf("недостающие %v: победил устаревший пересчёт", m)
	}
}

// Последний альбом на мёртвом томе: Load не зовётся (файл есть и открылся бы), открыт черновик,
// тост с буквой диска.
func TestStartupDeadLastProject(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	last := filepath.Join(testkit.TempDir(t), "альбом"+projectExt)
	e := mustEditor(t, model.New(), "")
	if _, err := e.NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.Save(last); err != nil {
		t.Fatal(err)
	}
	old := reachable
	reachable = func(p string) bool { return p != last }
	t.Cleanup(func() { reachable = old })

	p, path, notice := a.loadStartup(last)
	if path != "" || len(p.Spreads) != 0 {
		t.Fatalf("открыт %q с %d разворотами: Load звался", path, len(p.Spreads))
	}
	want := "Диск " + filepath.VolumeName(last) + " с последним альбомом недоступен. Открыт черновик."
	if notice != want {
		t.Fatalf("тост %q, ждали %q", notice, want)
	}
	// «Открыть» и «Последние» тоже не читают мёртвый том.
	if _, err := a.openPath(last); err == nil || !strings.Contains(err.Error(), "недоступен") {
		t.Fatalf("openPath: %v", err)
	}
}

// /media для недостающего фото — 404 без чтения файла; фото на мёртвом томе — тоже.
func TestMediaMissingNoRead(t *testing.T) {
	a, id, _ := missingApp(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	var reads atomic.Int32
	oldColor, oldReach := readColor, reachable
	readColor = func(string) (imgfile.Color, string) { reads.Add(1); return imgfile.Color(""), "" }
	t.Cleanup(func() { readColor, reachable = oldColor, oldReach })
	h := a.media(http.NotFoundHandler())
	get := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/media/t/"+id, nil))
		return rec.Code
	}

	a.mu.Lock()
	a.missing = []string{id}
	a.mu.Unlock()
	if code := get(); code != http.StatusNotFound || reads.Load() != 0 {
		t.Fatalf("недостающее: код %d, чтений %d", code, reads.Load())
	}
	// Пересчёт ещё не прошёл (missing пуст), но том не отвечает.
	a.mu.Lock()
	a.missing = nil
	a.mu.Unlock()
	reachable = func(string) bool { return false }
	if code := get(); code != http.StatusNotFound || reads.Load() != 0 {
		t.Fatalf("мёртвый том: код %d, чтений %d", code, reads.Load())
	}
}

// revealApp — App с альбомом из фото «p» по пути path; запуск проводника подменён
// записью вызовов.
func revealApp(t *testing.T, path string) (a *App, calls *[]string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	// Фото кладётся правкой, а не в проект до NewEditor: нелокальный путь NewEditor не пропустит
	// (Ш7), а тест держит собственную проверку ShowPhotoInExplorer.
	e := mustEditor(t, model.New(), "")
	if _, err := e.NewSpread(-1, []model.PhotoInfo{{ID: "p", Path: path, W: 300, H: 200}}); err != nil {
		t.Fatal(err)
	}
	a.ed.Store(e)
	calls = new([]string)
	old := revealPath
	t.Cleanup(func() { revealPath = old })
	revealPath = func(path string, file bool) error {
		if file {
			*calls = append(*calls, "file:"+path)
		} else {
			*calls = append(*calls, "dir:"+path)
		}
		return nil
	}
	return a, calls
}

// Неизвестный ID и нелокальный путь — ошибка, процесс не запускается.
func TestShowPhotoInExplorerRefuses(t *testing.T) {
	unc := `\\server\share\a.jpg`
	if runtime.GOOS != "windows" {
		unc = "//server/share/a.jpg"
	}
	a, calls := revealApp(t, unc)
	for _, id := range []string{"нет", "", "p"} {
		if err := a.ShowPhotoInExplorer(id); err == nil {
			t.Errorf("id %q: ждали ошибку", id)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("процесс запущен: %v", *calls)
	}
}

func TestShowPhotoInExplorer(t *testing.T) {
	dir := testkit.TempDir(t)
	path := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	a, calls := revealApp(t, path)
	if err := a.ShowPhotoInExplorer("p"); err != nil || len(*calls) != 1 || (*calls)[0] != "file:"+path {
		t.Fatalf("файл есть: err %v, вызовы %v", err, *calls)
	}
	// Файла нет — открывается папка.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	*calls = nil
	if err := a.ShowPhotoInExplorer("p"); err != nil || len(*calls) != 1 || (*calls)[0] != "dir:"+dir {
		t.Fatalf("файла нет: err %v, вызовы %v", err, *calls)
	}
	// Нет и папки — ошибка, без запуска.
	a2, calls2 := revealApp(t, filepath.Join(dir, "нет", "b.jpg"))
	if err := a2.ShowPhotoInExplorer("p"); err == nil || len(*calls2) != 0 {
		t.Fatalf("нет папки: err %v, вызовы %v", err, *calls2)
	}
}

// fileExists: только обычный файл; папка, пропавший файл и недоступный том — «нет».
func TestFileExists(t *testing.T) {
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	if !fileExists(file) {
		t.Error("обычный файл не найден")
	}
	if fileExists(dir) {
		t.Error("папка принята за файл")
	}
	if fileExists(filepath.Join(dir, "нет.jpg")) {
		t.Error("пропавший файл найден")
	}
	// Мёртвый том: ответ «нет» без обращения к диску (иначе os.Stat ждал бы таймаут SMB).
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(string) bool { return false }
	if fileExists(file) {
		t.Error("файл на недоступном томе найден")
	}
}

// goneLetter — буква диска, которой нет на машине и которая не равна letter (диску временной папки).
func goneLetter(t *testing.T, letter string) string {
	t.Helper()
	for _, l := range []string{"Q", "P", "O", "N", "M", "Z", "Y"} {
		if l == letter {
			continue
		}
		// Не os.Stat: отключённая сетевая буква вешает его. reachable ограничен таймаутом, а
		// «нет ответа» (нет тома или он мёртв) для теста и есть пропавший диск.
		if !reachable(l + `:\`) {
			return l
		}
	}
	t.Skip("нет свободной буквы диска")
	return ""
}

// SuggestDrive и RelinkDrive через App: альбом ссылается на пропавший диск, файлы лежат на реальном —
// подсказка находит пару, перепривязка переносит фото и папку, Undo возвращает, а после переноса
// подсказки нет.
func TestAppSuggestRelinkDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("буквы дисков")
	}
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from := goneLetter(t, to)
	tail := root[len(vol):]
	folder := filepath.Join(root, "Съёмка")
	file := testkit.JPEG(t, folder, "1.jpg", 40, 30)
	oldFolder := from + ":" + filepath.Join(tail, "Съёмка")
	oldPath := filepath.Join(oldFolder, "1.jpg")

	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	ed := a.editor()
	if _, _, err := ed.AddFolder(oldFolder); err != nil {
		t.Fatal(err)
	}
	oldID := library.IDFor(oldPath)
	if _, err := ed.NewSpread(-1, []model.PhotoInfo{{ID: oldID, Path: oldPath, W: 40, H: 30, Size: fileLen(t, file)}}); err != nil {
		t.Fatal(err)
	}

	hint := a.SuggestDrive()
	if hint != (DriveHint{From: from, To: to, Found: 1, Total: 1}) {
		t.Fatalf("подсказка: %+v", hint)
	}
	// Подсказка только читает: проект не менялся.
	if got := ed.Refs()[oldID].Path; got != oldPath {
		t.Fatalf("SuggestDrive изменил путь: %s", got)
	}

	// Буква допускается с двоеточием и в любом регистре.
	v, err := a.RelinkDrive(from+":", strings.ToLower(to))
	if err != nil {
		t.Fatal(err)
	}
	newID := library.IDFor(file)
	if len(v.Spreads) != 1 || v.Spreads[0].Photos[0] != newID || !slices.Equal(v.Folders, []string{folder}) {
		t.Fatalf("после смены диска: фото %v, папки %v", photosOf(v), v.Folders)
	}
	waitScan(t, a)
	waitMissing(t, a)
	if m := a.GetState().Missing; len(m) != 0 {
		t.Errorf("недостающие после смены диска: %v", m)
	}
	if h := a.SuggestDrive(); h.From != "" {
		t.Errorf("подсказка после переноса: %+v", h)
	}

	// Одна правка: Undo возвращает старые пути и подсказка появляется снова.
	a.Undo()
	waitScan(t, a)
	if h := a.SuggestDrive(); h.From != from {
		t.Errorf("подсказка после Undo: %+v", h)
	}
}

// SuggestDrive без недостающих фото — пустая подсказка.
func TestAppSuggestDriveNothingMissing(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if h := a.SuggestDrive(); h != (DriveHint{}) {
		t.Fatalf("в пустом альбоме подсказка %+v", h)
	}
}

// Фото убрали, пока шёл пересчёт по старому набору: его отчёт фото не возвращает.
func TestMissingRemovedDuringRefresh(t *testing.T) {
	a, x, y, events := missSetApp(t)
	gate, entered := make(chan struct{}), make(chan struct{})
	var once sync.Once
	a.exists = func(p string) bool {
		once.Do(func() { close(entered); <-gate })
		return filepath.Base(p) == "ok.jpg"
	}
	a.refreshMissing()
	<-entered
	a.RemovePhotos([]string{x})
	close(gate)
	waitMissing(t, a)
	if m := a.GetState().Missing; !slices.Equal(m, []string{y}) {
		t.Fatalf("недостающие %v: вернулось убранное фото", m)
	}
	if evs := events(); !slices.Equal(evs[len(evs)-1], []string{y}) {
		t.Fatalf("последнее событие %v", evs[len(evs)-1])
	}
}

// Пропавшее фото стоит только фоном: «Убрать из альбома» снимает и фон — полоса уходит; отмена возвращает.
func TestMissingBackdropRemoved(t *testing.T) {
	a, x, y, events := missSetApp(t)
	bg := filepath.Join(a.cfgDir, "фон.jpg")
	bid := library.IDFor(bg)
	sp := a.editor().View().Spreads[0].ID
	if _, err := a.editor().SetBackdrop(sp, model.ScopeSpread, model.PhotoInfo{ID: bid, Path: bg, W: 30, H: 20}); err != nil {
		t.Fatal(err)
	}
	a.refreshMissing()
	waitMissing(t, a)
	all := sorted(x, y, bid)
	if m := a.GetState().Missing; !slices.Equal(m, all) {
		t.Fatalf("недостающие с фоном %v", m)
	}
	v := a.RemovePhotos(all)
	if len(v.Spreads) != 1 || len(v.Spreads[0].Backdrops) != 0 {
		t.Fatalf("фон не снят: %+v", v.Spreads)
	}
	evs := events()
	if m := a.GetState().Missing; len(m) != 0 || len(evs[len(evs)-1]) != 0 {
		t.Fatalf("после «Убрать из альбома» недостающие %v, событие %v", m, evs[len(evs)-1])
	}
	a.Undo()
	waitMissing(t, a)
	if m := a.GetState().Missing; !slices.Equal(m, all) {
		t.Fatalf("отмена: недостающие %v", m)
	}
}

// Имена пропавших — из путей проекта (пропавшего файла нет в ленте), без os.Stat; убранное фото
// уходит из имён, без пропавших — пустая карта (в JSON «{}», не null).
func TestMissingNames(t *testing.T) {
	a, x, y, _ := missSetApp(t)
	a.exists = func(string) bool { t.Error("MissingNames проверяет файлы"); return false }
	if n := a.MissingNames(); len(n) != 2 || n[x] != "x.jpg" || n[y] != "y.jpg" {
		t.Fatalf("имена %v", n)
	}
	a.RemovePhotos([]string{x})
	if n := a.MissingNames(); len(n) != 1 || n[y] != "y.jpg" {
		t.Fatalf("убрали x: имена %v", n)
	}
	a.RemovePhotos([]string{y})
	if n := a.MissingNames(); n == nil || len(n) != 0 {
		t.Fatalf("без пропавших: имена %#v", n)
	}
	if n := newApp(t).MissingNames(); n == nil || len(n) != 0 {
		t.Fatalf("пустой альбом: имена %#v", n)
	}
}

// Пропавший диск: копия не пытается читать файл.
func TestBackupSkipsUnreachable(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	old := reachable
	reachable = func(string) bool { return false }
	t.Cleanup(func() { reachable = old })
	a.backupBefore(file, backup.Key(file), true)
	if list := backups(t, a); len(list) != 0 {
		t.Fatalf("копия с недоступного диска: %+v", list)
	}
}
