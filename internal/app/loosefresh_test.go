package app

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestFreshFiles: набор свежих отдельных фото — пересечение со списком проекта (of набор не меняет), снятие по done, reset.
func TestFreshFiles(t *testing.T) {
	var f freshFiles
	if got := f.of([]string{"/a.jpg"}); got != nil {
		t.Errorf("пустой набор: %v", got)
	}
	f.done([]string{"/a.jpg"}) // на пустом наборе — без паники
	f.add([]string{"/a.jpg", "/b.jpg", "/gone.jpg"})
	f.add(nil)
	if got := f.of([]string{"/b.jpg", "/a.jpg", "/other.jpg"}); !slices.Equal(got, []string{"/b.jpg", "/a.jpg"}) {
		t.Errorf("of = %v, ожидались /b.jpg, /a.jpg в порядке списка", got)
	}
	// Чтение со старым снимком источников (без /a.jpg и /gone.jpg) признак не стирает: следующее увидит всё.
	f.of([]string{"/b.jpg"})
	if got := f.of([]string{"/a.jpg", "/b.jpg", "/gone.jpg"}); !slices.Equal(got, []string{"/a.jpg", "/b.jpg", "/gone.jpg"}) {
		t.Errorf("повторный of = %v", got)
	}
	f.done([]string{"/a.jpg"})
	if got := f.of([]string{"/a.jpg", "/b.jpg"}); !slices.Equal(got, []string{"/b.jpg"}) {
		t.Errorf("после done(/a.jpg): %v", got)
	}
	f.reset()
	if got := f.of([]string{"/b.jpg", "/gone.jpg"}); got != nil {
		t.Errorf("после reset: %v", got)
	}
	f.add([]string{"/c.jpg"}) // после reset набор снова принимает
	if got := f.of([]string{"/c.jpg"}); len(got) != 1 {
		t.Errorf("добавление после reset: %v", got)
	}
}

// TestLooseCloudOnlyOnAdd (160-M1, хвост 2): заглушку облака у отдельного фото читает только сканирование, запущенное
// добавлением; пересканирование и открытие проекта — «не читается» с причиной. Шов library.UseDehydratedForTest.
// Блоки независимы.
func TestLooseCloudOnlyOnAdd(t *testing.T) {
	rig, inLibrary := stubRig, inLibraryOf

	t.Run("добавление скачивает", func(t *testing.T) {
		a, p := rig(t)
		if r, err := a.AddFilePaths([]string{p}); err != nil || r.Added != 1 {
			t.Fatalf("добавление: %+v, %v", r, err)
		}
		waitRead(t, a)
		if !inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != "" {
			t.Errorf("заглушка не прочитана: в ленте %v, Unread %v", inLibrary(a, p), a.GetState().Unread)
		}
	})
	t.Run("открытие проекта не скачивает", func(t *testing.T) {
		a, p := rig(t)
		if _, _, err := a.editor().AddLoose([]string{p}, 1); err != nil { // мимо addLoose: как загруженный альбом
			t.Fatal(err)
		}
		a.rescan()
		waitRead(t, a)
		if inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != library.WhyCloudOnly {
			t.Errorf("в ленте %v, Unread %v", inLibrary(a, p), a.GetState().Unread)
		}
	})
	t.Run("пересканирование после добавления не скачивает", func(t *testing.T) {
		a, p := rig(t)
		if _, err := a.AddFilePaths([]string{p}); err != nil {
			t.Fatal(err)
		}
		waitRead(t, a)
		// Файл снова «ушёл в облако» (признак остаётся в шве): прежнее добавление уже прочитано — признак снят.
		a.rescan()
		waitRead(t, a)
		if inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != library.WhyCloudOnly {
			t.Errorf("в ленте %v, Unread %v", inLibrary(a, p), a.GetState().Unread)
		}
	})
	t.Run("несколько файлов: свежий и старый", func(t *testing.T) {
		a, p := rig(t)
		other := testkit.JPEG(t, filepath.Dir(p), "stub.png.jpg", 40, 30) // обычный файл
		if _, _, err := a.editor().AddLoose([]string{p}, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddFilePaths([]string{other}); err != nil {
			t.Fatal(err)
		}
		waitRead(t, a)
		if !inLibrary(a, other) || inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != library.WhyCloudOnly {
			t.Errorf("обычный в ленте %v, заглушка в ленте %v, Unread %v", inLibrary(a, other), inLibrary(a, p), a.GetState().Unread)
		}
	})
}

// gates — ворота на пробе тома splitLive (reachable): n-й вызов для подходящего пути ждёт open(n-1). Вызовы
// reachable из других мест (сверка недостающих) ворот не трогают.
type gates struct {
	ch   []chan struct{}
	once []sync.Once
}

func gateSplitLive(t *testing.T, match func(string) bool, n int) *gates {
	t.Helper()
	g := &gates{ch: make([]chan struct{}, n), once: make([]sync.Once, n)}
	for i := range g.ch {
		g.ch[i] = make(chan struct{})
	}
	var calls atomic.Int32
	old := reachable
	t.Cleanup(func() {
		reachable = old
		for i := range g.ch {
			g.open(i) // не оставлять читателей на воротах при провале теста: synctest иначе паникует
		}
	})
	reachable = func(p string) bool {
		if match(p) && fromSplitLive() {
			if i := int(calls.Add(1)) - 1; i < n {
				<-g.ch[i]
			}
		}
		return true
	}
	return g
}

func (g *gates) open(i int) { g.once[i].Do(func() { close(g.ch[i]) }) }

func fromSplitLive() bool {
	pc := make([]uintptr, 12)
	fr := runtime.CallersFrames(pc[:runtime.Callers(2, pc)])
	for {
		f, more := fr.Next()
		if strings.HasSuffix(f.Function, ".splitLive") {
			return true
		}
		if !more {
			return false
		}
	}
}

// stubRig — приложение и заглушка облака stub.jpg (шов library.UseDehydratedForTest).
func stubRig(t *testing.T) (*App, string) {
	t.Helper()
	a, _ := evApp(t)
	p := testkit.JPEG(t, testkit.TempDir(t), "stub.jpg", 40, 30)
	library.UseDehydratedForTest(t, func(st fs.FileInfo) bool { return st != nil && st.Name() == "stub.jpg" })
	return a, p
}

// inLibraryOf — фото файла p есть в ленте.
func inLibraryOf(a *App, p string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.byID[library.IDFor(p)]
	return ok
}

// TestLooseCloudFreshSet: что именно набор свежих файлов помечает и забывает; порядок двух чтений — воротами на пробе
// тома (synctest). Блоки независимы.
func TestLooseCloudFreshSet(t *testing.T) {
	rig, inLibrary := stubRig, inLibraryOf
	t.Run("два запуска подряд, первый отменён", func(t *testing.T) {
		// Бросок в окно: addLoose запускает rescan, и dropPaths сразу запускает второй — он отменяет первый.
		// Ворота держат оба чтения на пробе тома, пока первое не закончится целиком (done при отмене сняло бы признак).
		synctest.Test(t, func(t *testing.T) {
			a, p := rig(t)
			g := gateSplitLive(t, func(s string) bool { return s == p }, 2)
			if _, err := a.addLoose([]string{p}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait() // первое чтение стоит на пробе
			a.rescan()
			synctest.Wait() // второе тоже; первое отменено
			g.open(0)
			synctest.Wait() // первое закончилось (отменённым)
			g.open(1)
			waitRead(t, a)
			if !inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != "" {
				t.Errorf("в ленте %v, Unread %v", inLibrary(a, p), a.GetState().Unread)
			}
		})
	})
	t.Run("устаревшее чтение не стирает признак добавления", func(t *testing.T) {
		// Чтение A сняло источники до добавления (в них только папка) и ждёт пробу тома; пока ждёт, фото добавлено,
		// и чтение B ждёт ту же пробу. A закончилось раньше B: его of по старому списку не должен забыть добавленное.
		synctest.Test(t, func(t *testing.T) {
			a, p := rig(t)
			dir := testkit.TempDir(t)
			testkit.JPEG(t, dir, "plain.jpg", 40, 30)
			if _, _, err := a.editor().AddFolder(dir); err != nil {
				t.Fatal(err)
			}
			g := gateSplitLive(t, func(s string) bool { return s == dir }, 2)
			a.rescan() // A
			synctest.Wait()
			if r, err := a.AddFilePaths([]string{p}); err != nil || r.Added != 1 { // B
				t.Fatalf("добавление: %+v, %v", r, err)
			}
			synctest.Wait()
			g.open(0)
			synctest.Wait() // A закончилось
			g.open(1)
			waitRead(t, a)
			if !inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != "" {
				t.Errorf("в ленте %v, Unread %v", inLibrary(a, p), a.GetState().Unread)
			}
		})
	})
	t.Run("повтор в пакете не помечается", func(t *testing.T) {
		// Заглушка уже добавлена раньше (открытие проекта), её повтор вместе с новым файлом не даёт права её скачать.
		a, p := rig(t)
		other := testkit.JPEG(t, filepath.Dir(p), "new.jpg", 40, 30)
		if _, _, err := a.editor().AddLoose([]string{p}, 1); err != nil {
			t.Fatal(err)
		}
		r, err := a.addLoose([]string{p, other})
		if err != nil || r.Added != 1 || r.Dup != 1 {
			t.Fatalf("добавление: %+v, %v", r, err)
		}
		waitRead(t, a)
		if !inLibrary(a, other) || inLibrary(a, p) || a.GetState().Unread[library.IDFor(p)] != library.WhyCloudOnly {
			t.Errorf("новый в ленте %v, повтор в ленте %v, Unread %v", inLibrary(a, other), inLibrary(a, p), a.GetState().Unread)
		}
	})
	t.Run("другой альбом признак не наследует", func(t *testing.T) {
		a, p := rig(t)
		a.fresh.add([]string{p})
		a.setEditor(blankEditor(""))
		if got := a.fresh.of([]string{p}); got != nil {
			t.Errorf("после смены альбома: %v", got)
		}
	})
}
