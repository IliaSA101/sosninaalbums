package app

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// switchApp — App с хранилищем копий и тишиной вместо событий.
func switchApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	return a
}

// switchAlbum — альбом s1…s3 (по фото); path непуст — записан туда.
func switchAlbum(t *testing.T, path string) *album.Editor {
	t.Helper()
	p := model.New()
	for i, id := range []string{"a", "b", "c"} {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + strconv.Itoa(i+1), Photos: []string{id}})
		p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
	}
	ed := mustEditor(t, p, "")
	if path != "" {
		if err := ed.Save(path); err != nil {
			t.Fatal(err)
		}
	}
	return ed
}

// nameOnDisk — имя разворота s1 в файле path.
func nameOnDisk(t *testing.T, path string) string {
	t.Helper()
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return q.Spreads[0].Name
}

// switchFn — смена альбома в тесте; f2 — другой записанный альбом.
type switchFn func(t *testing.T, a *App, f2 string) error

func openSwitch(_ *testing.T, a *App, f2 string) error {
	_, err := a.openPath(f2)
	return err
}

// backupSwitch — открытие копии f2 из папки копий (openBackup).
func backupSwitch(t *testing.T, a *App, f2 string) error {
	if err := a.backups.Force(f2, backup.Key(f2)); err != nil {
		return err
	}
	list, err := a.backups.List(backup.Key(f2))
	if err != nil || len(list) != 1 {
		t.Fatalf("копия: %+v, %v", list, err)
	}
	path, err := a.backups.File(backup.Key(f2), list[0].Name)
	if err != nil {
		return err
	}
	return openSwitch(t, a, path)
}

// Правка, взявшая прежний Editor до смены альбома и пришедшая после записи (087-5), не меняет
// его: на диске и в памяти одно и то же, прежний не dirty; ответ — вид нового альбома.
func TestSwitchLateEdit(t *testing.T) {
	cases := []struct {
		name  string
		file  bool // прежний альбом с файлом (иначе — черновик)
		start switchFn
	}{
		{"openPath", true, openSwitch},
		{"NewProject", true, func(_ *testing.T, a *App, _ string) error { a.NewProject("album"); return nil }},
		{"openBackup", true, backupSwitch},
		{"черновик", false, openSwitch},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { lateEdit(t, c.file, c.start) })
	}
}

// lateEdit — случай TestSwitchLateEdit: правка "early" через App, смена start, поздняя "late".
func lateEdit(t *testing.T, file bool, start switchFn) {
	a := switchApp(t)
	dir := testkit.TempDir(t)
	f1, f2 := "", filepath.Join(dir, "b"+projectExt)
	if file {
		f1 = filepath.Join(dir, "a"+projectExt)
	}
	switchAlbum(t, f2)
	e1 := switchAlbum(t, f1)
	a.ed.Store(e1)
	a.GetState()
	a.SetSpreadName("s1", "early")
	if err := start(t, a, f2); err != nil {
		t.Fatal(err)
	}
	if a.editor() == e1 {
		t.Fatal("альбом не сменился")
	}
	w := a.changed(e1.SetSpreadName("s1", "late")) // Editor взят до смены
	if w.Src() != a.editor().ID() {
		t.Errorf("ответ позднему — вид прежнего альбома")
	}
	v := e1.View()
	// Черновик не снимает dirty (это не файл альбома): сверка — по имени в нём.
	if v.Spreads[0].Name != "early" || v.Dirty && file {
		t.Errorf("прежний в памяти: имя %q, dirty %v", v.Spreads[0].Name, v.Dirty)
	}
	disk := f1
	if !file {
		disk = a.draftPath() // запись прежнего до смены — в черновике (openPath его не удаляет)
	}
	if got := nameOnDisk(t, disk); got != "early" {
		t.Errorf("прежний на диске: %q", got)
	}
}

// Смена альбома под непрерывным потоком правок (параллельные вызовы Wails): каждый раз прежний
// альбом на диске равен памяти. До 087-5 расходились 126 из 150 открытий.
func TestSwitchLateEditStress(t *testing.T) {
	a := switchApp(t)
	dir := testkit.TempDir(t)
	files := [2]string{filepath.Join(dir, "a"+projectExt), filepath.Join(dir, "b"+projectExt)}
	for _, f := range files {
		switchAlbum(t, f)
	}
	if _, err := a.openPath(files[0]); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; !stop.Load(); i++ {
			a.SetSpreadName("s1", strconv.Itoa(i))
		}
	})
	defer func() { stop.Store(true); wg.Wait() }()
	n := 40
	if testkit.Lite() {
		n = 8
	}
	bad := 0
	for k := range n {
		prev := a.editor()
		if _, err := a.openPath(files[(k+1)%2]); err != nil {
			t.Fatal(err)
		}
		if disk, mem := nameOnDisk(t, prev.Path()), prev.View().Spreads[0].Name; disk != mem {
			bad++
			t.Logf("открытие %d: на диске %q, в памяти %q", k, disk, mem)
		}
	}
	if bad > 0 {
		t.Fatalf("прежний альбом на диске ≠ в памяти: %d из %d открытий", bad, n)
	}
}

// watchEditors — горутина-поток правок s1 (как параллельные вызовы Wails); пока крутится, помнит
// каждый Editor, побывавший текущим. Стоп и ожидание — возвращённая функция; она же отдаёт набор.
func watchEditors(a *App) (stop func() []*album.Editor) {
	var done atomic.Bool
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[*album.Editor]bool{}
	wg.Go(func() {
		for i := 0; !done.Load(); i++ {
			mu.Lock()
			seen[a.editor()] = true
			mu.Unlock()
			a.SetSpreadName("s1", strconv.Itoa(i))
		}
	})
	return func() []*album.Editor {
		done.Store(true)
		wg.Wait()
		mu.Lock()
		defer mu.Unlock()
		list := make([]*album.Editor, 0, len(seen))
		for e := range seen {
			list = append(list, e)
		}
		return list
	}
}

// switchRound — один раунд TestSwitchConcurrent: три смены альбома из трёх горутин разом.
func switchRound(t *testing.T, a *App, dir string, k int, second func(a *App, f string)) {
	t.Helper()
	// Файл на каждый открытый Editor: иначе на диске остаётся запись только последнего.
	fb := filepath.Join(dir, "b"+strconv.Itoa(k)+projectExt)
	fc := filepath.Join(dir, "c"+strconv.Itoa(k)+projectExt)
	fd := filepath.Join(dir, "d"+strconv.Itoa(k)+projectExt)
	switchAlbum(t, fb)
	switchAlbum(t, fc)
	switchAlbum(t, fd)
	start := make(chan struct{})
	var wg sync.WaitGroup
	open := func(f string) {
		<-start
		if _, err := a.openPath(f); err != nil {
			t.Error(err)
		}
	}
	wg.Go(func() { open(fb) })
	wg.Go(func() { <-start; second(a, fc) })
	wg.Go(func() { open(fd) })
	close(start)
	wg.Wait()
}

// checkSeenEditors — каждый побывавший текущим Editor: на диске равен памяти и не dirty.
func checkSeenEditors(t *testing.T, seen []*album.Editor) {
	t.Helper()
	for _, e := range seen {
		if e.Path() == "" {
			continue // пустой «Новый» без разворотов: правок, которые терять, нет
		}
		v := e.View()
		if disk, mem := nameOnDisk(t, e.Path()), v.Spreads[0].Name; disk != mem {
			t.Errorf("%s: на диске %q, в памяти %q", e.Path(), disk, mem)
		}
		if v.Dirty {
			t.Errorf("%s: dirty после записи", e.Path())
		}
	}
}

// Две смены альбома подряд из разных горутин под потоком правок (retire держит saveMu от Close
// до Store: вторая закрывает и пишет next первой, а не прежний снова). Каждый Editor, побывавший
// текущим, — на диске равен памяти и не dirty (последнего пишет flushSave); текущий не закрыт —
// правка через App в него проходит.
func TestSwitchConcurrent(t *testing.T) {
	cases := []struct {
		name   string
		second func(a *App, f string) // вторая смена; первая — всегда openPath(b)
	}{
		{"openPath+openPath", func(a *App, f string) { _, _ = a.openPath(f) }},
		{"openPath+NewProject", func(a *App, _ string) { a.NewProject("album") }},
	}
	rounds := testkit.Steps(12, 5)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := switchApp(t)
			dir := testkit.TempDir(t)
			first := filepath.Join(dir, "a"+projectExt)
			switchAlbum(t, first)
			if _, err := a.openPath(first); err != nil {
				t.Fatal(err)
			}
			stop := watchEditors(a)
			for k := range rounds {
				switchRound(t, a, dir, k, c.second)
			}
			seen := stop()
			a.flushSave()
			cur := a.editor()
			checkSeenEditors(t, seen)
			if len(cur.View().Spreads) > 0 { // текущий — альбом из файла: он принимает правки
				a.SetSpreadName("s1", "final")
				if got := cur.View().Spreads[0].Name; got != "final" {
					t.Errorf("текущий закрыт: имя %q", got)
				}
			}
		})
	}
}

// lockedDraftApp — App с прежним альбомом (с разворотами или пустым), нечитаемым черновиком и
// застрявшим замком draftLocked; с содержимым в альбом внесена правка «early».
func lockedDraftApp(t *testing.T, withContent bool) (*App, *album.Editor) {
	t.Helper()
	a := switchApp(t)
	prev := switchAlbum(t, "")
	if !withContent {
		prev = blankEditor("")
	}
	a.ed.Store(prev)
	a.GetState()
	if err := os.WriteFile(a.draftPath(), []byte("{не json"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.draftLocked.Store(true)
	if withContent {
		a.SetSpreadName("s1", "early")
	}
	return a, prev
}

// checkDraftCopies — копии черновика: у пустого прежнего альбома их нет, у содержательного — одна
// с правкой «early».
func checkDraftCopies(t *testing.T, a *App, withContent bool) {
	t.Helper()
	list, err := a.backups.List(backup.DraftKey)
	if err != nil {
		t.Fatal(err)
	}
	if !withContent {
		if len(list) != 0 {
			t.Errorf("копия пустого черновика: %+v", list)
		}
		return
	}
	if len(list) != 1 {
		t.Fatalf("копии черновика: %+v", list)
	}
	path, err := a.backups.File(backup.DraftKey, list[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if got := nameOnDisk(t, path); got != "early" {
		t.Errorf("в копии черновика %q, ждали правку прежнего альбома", got)
	}
}

// startNew при застрявшем черновике (draftLocked), когда unlockDraft удался: правки прежнего
// альбома без файла пишутся в черновик и его копию (design.md §5.4: раньше — никуда), а черновик
// после этого убран; нечитаемый уходит в .bad. Пустой прежний — копии нет (HasContent).
func TestSwitchLockedDraftNew(t *testing.T) {
	for _, withContent := range []bool{true, false} {
		name := "пустой"
		if withContent {
			name = "с разворотами"
		}
		t.Run(name, func(t *testing.T) {
			a, prev := lockedDraftApp(t, withContent)
			a.NewProject("album")
			if a.editor() == prev || a.draftLocked.Load() {
				t.Fatal("новый проект должен создаться, замок сняться")
			}
			if _, err := os.Stat(a.draftPath()); !os.IsNotExist(err) {
				t.Errorf("черновик остался: %v", err)
			}
			bad, _ := filepath.Glob(filepath.Join(a.cfgDir, "черновик.bad-*"+projectExt))
			if len(bad) != 1 {
				t.Fatalf(".bad: %v", bad)
			}
			if data, _ := os.ReadFile(bad[0]); string(data) != "{не json" {
				t.Error(".bad испорчен")
			}
			checkDraftCopies(t, a, withContent)
		})
	}
}
