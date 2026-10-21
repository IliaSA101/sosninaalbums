package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сверка папок с диском (088, foldercheck.go): тихое перечитывание, порог, мёртвые тома.

// evLog — события App под замком: их шлют фоновые горутины, пока тест читает.
type evLog struct {
	mu  sync.Mutex
	evs []emitted
}

func (l *evLog) add(name string, data ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evs = append(l.evs, emitted{name, data})
}

// take отдаёт накопленное и начинает заново.
func (l *evLog) take() []emitted {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.evs
	l.evs = nil
	return out
}

func only(evs []emitted, name string) []emitted {
	return slices.DeleteFunc(slices.Clone(evs), func(e emitted) bool { return e.name != name })
}

// checkApp — App с папкой проекта dir из n фото p<i>.jpg, ещё не прочитанной; cache — с кэшем
// миниатюр (события thumbs).
func checkApp(t *testing.T, n int, cache bool) (a *App, dir string, evs *evLog) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	evs = &evLog{}
	a.emit = evs.add
	if cache {
		c, err := library.NewCache(testkit.TempDir(t))
		if err != nil {
			t.Fatal(err)
		}
		a.cache = c
	}
	dir = testkit.TempDir(t)
	for i := range n {
		testkit.JPEG(t, dir, fmt.Sprintf("p%d.jpg", i), 20, 10)
	}
	if _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	return a, dir, evs
}

// waitFor ждёт условия cond (под a.mu) до 10 с.
func waitFor(t *testing.T, a *App, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.mu.Lock()
		ok := cond()
		a.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitCheck ждёт конца проверки, чтения папок и пересчёта недостающих (прогрев — waitWarm).
func waitCheck(t *testing.T, a *App) {
	t.Helper()
	waitFor(t, a, "конец сверки и чтения", func() bool {
		a.check.mu.Lock()
		defer a.check.mu.Unlock()
		return !a.check.running && !a.scanning
	})
	waitMissing(t, a)
}

func waitWarm(t *testing.T, a *App) {
	t.Helper()
	waitFor(t, a, "конец прогрева миниатюр", func() bool { return a.warm == nil })
}

// loudScan — первое чтение, как при запуске, до конца прогрева.
func loudScan(t *testing.T, a *App) {
	t.Helper()
	a.rescan()
	waitCheck(t, a)
	waitWarm(t, a)
}

// runCheck снимает порог частоты и проводит одну проверку до конца (без прогрева).
func runCheck(t *testing.T, a *App, full bool) {
	t.Helper()
	a.check.mu.Lock()
	a.check.end = time.Time{}
	a.check.mu.Unlock()
	if !a.beginCheck(full) {
		t.Fatal("проверка не началась")
	}
	waitCheck(t, a)
}

func photoNames(a *App) []string {
	var out []string
	for _, p := range a.GetState().Photos {
		out = append(out, p.Name)
	}
	return out
}

// Ничего не менялось: ни событий library/scan/thumbs/missing, ни строк журнала — и при полной
// проверке, и при обычной.
func TestCheckFoldersNoChangeSilent(t *testing.T) {
	a, _, evs := checkApp(t, 3, true)
	loudScan(t, a)
	evs.take()
	buf := captureLog(t)
	for _, full := range []bool{true, false, true} {
		runCheck(t, a, full)
	}
	waitWarm(t, a)
	if got := evs.take(); len(got) != 0 {
		t.Fatalf("без изменений пришли события: %v", got)
	}
	if s := buf.String(); s != "" {
		t.Fatalf("без изменений строки журнала: %q", s)
	}
	if n := len(a.GetState().Photos); n != 3 {
		t.Fatalf("в ленте %d фото, ждали 3", n)
	}
}

// Удалили 2 файла из 4: их плитки уходят из ленты без полосы хода (scan) и без прогрева.
func TestCheckFoldersRemoved(t *testing.T) {
	a, dir, evs := checkApp(t, 4, true)
	loudScan(t, a)
	evs.take()
	for _, name := range []string{"p1.jpg", "p3.jpg"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	runCheck(t, a, true)
	if d := time.Since(start); d > testkit.Limit(time.Second) {
		t.Errorf("сверка шла %v, ждали ≤ 1 с", d)
	}
	waitWarm(t, a)
	if got := photoNames(a); !slices.Equal(got, []string{"p0.jpg", "p2.jpg"}) {
		t.Fatalf("лента: %v", got)
	}
	got := evs.take()
	if len(only(got, "library")) == 0 {
		t.Fatal("нет события library")
	}
	for _, name := range []string{"scan", "thumbs", "missing"} {
		if e := only(got, name); len(e) != 0 {
			t.Errorf("лишние события %s: %v", name, e)
		}
	}
}

// Новые файлы встают по местам в порядке ленты; thumbs — только по новым, без scan.
func TestCheckFoldersAdded(t *testing.T) {
	a, dir, evs := checkApp(t, 0, true)
	for _, name := range []string{"p1.jpg", "p3.jpg"} {
		testkit.JPEG(t, dir, name, 20, 10)
	}
	loudScan(t, a)
	evs.take()
	for _, name := range []string{"p0.jpg", "p2.jpg", "p10.jpg"} {
		testkit.JPEG(t, dir, name, 20, 10)
	}
	runCheck(t, a, false)
	waitWarm(t, a)
	if got := photoNames(a); !slices.Equal(got, []string{"p0.jpg", "p1.jpg", "p2.jpg", "p3.jpg", "p10.jpg"}) {
		t.Fatalf("лента: %v", got)
	}
	got := evs.take()
	if e := only(got, "scan"); len(e) != 0 {
		t.Errorf("3 новых — полосы хода нет, а пришло %v", e)
	}
	th := only(got, "thumbs")
	if len(th) == 0 {
		t.Fatal("новые не прогреты: событий thumbs нет")
	}
	for _, e := range th {
		if e.data[1].(int) != 3 {
			t.Fatalf("thumbs не только по новым: %v", th)
		}
	}
	if last := th[len(th)-1]; last.data[0].(int) != 3 {
		t.Fatalf("последнее thumbs %v, ждали 3/3", last.data)
	}
}

// Ход тихого чтения виден от quietShowRead новых: 49 — ни одного scan, 50 — ход и финал active:false.
func TestCheckFoldersQuietProgress(t *testing.T) {
	for _, tc := range []struct {
		fresh int
		shown bool
	}{{quietShowRead - 1, false}, {quietShowRead, true}} {
		t.Run(fmt.Sprint(tc.fresh), func(t *testing.T) {
			a, dir, evs := checkApp(t, 1, false)
			loudScan(t, a)
			evs.take()
			for i := range tc.fresh {
				testkit.JPEG(t, dir, fmt.Sprintf("n%03d.jpg", i), 8, 8)
			}
			runCheck(t, a, false)
			if n := len(a.GetState().Photos); n != tc.fresh+1 {
				t.Fatalf("в ленте %d, ждали %d", n, tc.fresh+1)
			}
			sc := only(evs.take(), "scan")
			if !tc.shown {
				if len(sc) != 0 {
					t.Fatalf("ход показан: %v", sc)
				}
				return
			}
			if len(sc) == 0 {
				t.Fatal("ход не показан")
			}
			fin := sc[len(sc)-1].data[0].(library.ScanProgress)
			if fin.Active || fin.Found != tc.fresh+1 {
				t.Fatalf("финал хода %+v", fin)
			}
			if st := a.GetState().Scan; st.Active {
				t.Fatalf("GetState: ход %+v", st)
			}
		})
	}
}

// Ворота: одна за раз, порог max(2 с, 10 обходов), отброшенная полная делает полной следующую.
func TestCheckFoldersGate(t *testing.T) {
	var g folderCheck
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	step := func(what string, full bool, at time.Duration, wantOK, wantFull bool) {
		t.Helper()
		eff, ok := g.enter(full, t0.Add(at))
		if ok != wantOK || ok && eff != wantFull {
			t.Fatalf("%s: пущена %v полная %v, ждали %v %v", what, ok, eff, wantOK, wantFull)
		}
	}
	step("первая", false, 0, true, false)
	step("пока идёт первая", true, 0, false, false)
	g.leave(10*time.Millisecond, t0)
	step("через 1 с", false, time.Second, false, false)
	step("через 2 с — полная за отброшенную", false, 2*time.Second, true, true)
	g.leave(time.Second, t0.Add(2*time.Second)) // медленный обход: порог 10 с
	step("через 9 с после медленной", false, 11*time.Second, false, false)
	step("через 10 с после медленной", false, 12*time.Second, true, false)
	g.leave(0, t0.Add(12*time.Second))
	step("полная после пропущенной обычной", true, 14*time.Second, true, true)
	g.leave(0, t0.Add(14*time.Second))
	step("следующая — снова обычная", false, 16*time.Second, true, false)

	// Потолок: обход, повисший на 60 с, глушит сверку на минуту, а не на 10.
	g.leave(60*time.Second, t0.Add(16*time.Second))
	step("через 59 с после зависшего", false, 75*time.Second, false, false)
	step("через 60 с после зависшего", false, 76*time.Second, true, false)
	g.leave(0, t0.Add(76*time.Second))
	if checkGap(0) != checkMinGap || checkGap(time.Hour) != checkMaxGap || checkGap(3*time.Second) != 30*time.Second {
		t.Fatalf("checkGap: %v %v %v", checkGap(0), checkGap(time.Hour), checkGap(3*time.Second))
	}

	// Через App: вторая сразу за первой не начинается.
	a, _, _ := checkApp(t, 1, false)
	loudScan(t, a)
	if !a.beginCheck(false) {
		t.Fatal("первая не началась")
	}
	if a.beginCheck(true) {
		t.Fatal("вторая началась сразу за первой")
	}
	waitCheck(t, a)
	a.check.mu.Lock()
	pend := a.check.pendFull
	a.check.mu.Unlock()
	if !pend {
		t.Fatal("отброшенная полная не запомнилась")
	}
}

// Во время чтения папок проверка и тихое чтение не начинаются; конец чтения снимает запрет.
func TestCheckFoldersSkipsWhileScanning(t *testing.T) {
	a, dir, evs := checkApp(t, 2, false)
	loudScan(t, a)
	evs.take()
	testkit.JPEG(t, dir, "new.jpg", 20, 10)
	a.mu.Lock()
	a.scanning = true // как идущее «Обновить все»
	gen := a.scanGen
	a.mu.Unlock()
	if a.beginCheck(true) {
		t.Fatal("проверка началась во время чтения")
	}
	a.rescanWith(scanQuiet)
	a.mu.Lock()
	same := a.scanGen == gen
	a.mu.Unlock()
	if !same {
		t.Fatal("тихое чтение началось поверх идущего")
	}
	a.check.mu.Lock()
	idle := !a.check.running && a.check.end.IsZero() && !a.check.pendFull
	a.check.mu.Unlock()
	if !idle {
		t.Fatal("ворота тронуты пропущенной проверкой")
	}
	if got := evs.take(); len(got) != 0 {
		t.Fatalf("события: %v", got)
	}

	// Настоящее чтение: флаг стоит с возврата rescan и снимается с концом Scan. Чтение держит
	// проба тома (gate): без неё 3 фото прочлись бы раньше проверки флага.
	a.mu.Lock()
	a.scanning = false
	a.mu.Unlock()
	gate := make(chan struct{})
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(p string) bool { <-gate; return old(p) }
	a.rescan()
	a.mu.Lock()
	busy := a.scanning
	a.mu.Unlock()
	if !busy {
		t.Fatal("rescan не отметил чтение")
	}
	if a.beginCheck(false) {
		t.Fatal("проверка началась во время rescan")
	}
	close(gate)
	waitCheck(t, a)
	if n := len(a.GetState().Photos); n != 3 {
		t.Fatalf("после чтения в ленте %d, ждали 3", n)
	}
	runCheck(t, a, false) // запрет снят
}

// fcAlbum — альбом с фото path (размеры для PhotoInfo).
func fcAlbum(t *testing.T, a *App, path string) string {
	t.Helper()
	id := library.IDFor(path)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: path, W: 20, H: 10, Size: fileLen(t, path)}}); err != nil {
		t.Fatal(err)
	}
	return id
}

func lastMissing(t *testing.T, evs []emitted) ([]string, int) {
	t.Helper()
	m := only(evs, "missing")
	if len(m) == 0 {
		return nil, 0
	}
	return m[len(m)-1].data[0].([]string), len(m)
}

// Фото альбома вне папок проекта пропало: обычная проверка его не видит (папки прежние), полная —
// присылает missing.
func TestCheckFoldersFullMissing(t *testing.T) {
	a, _, evs := checkApp(t, 1, false)
	out := testkit.JPEG(t, testkit.TempDir(t), "вне.jpg", 20, 10)
	id := fcAlbum(t, a, out)
	loudScan(t, a)
	if m := a.GetState().Missing; len(m) != 0 {
		t.Fatalf("до удаления недостающие %v", m)
	}
	evs.take()
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	runCheck(t, a, false)
	if got, n := lastMissing(t, evs.take()); n != 0 {
		t.Fatalf("обычная проверка прислала missing %v", got)
	}
	runCheck(t, a, true)
	if got, n := lastMissing(t, evs.take()); n != 1 || !slices.Equal(got, []string{id}) {
		t.Fatalf("полная: missing %v (%d раз), ждали [%s]", got, n, id)
	}
	runCheck(t, a, true) // список прежний — события нет
	if got, n := lastMissing(t, evs.take()); n != 0 {
		t.Fatalf("повторная полная прислала missing %v", got)
	}
}

// Файл альбома удалили и вернули тем же путём: missing [id], затем []; rev, отмена и dirty прежние.
func TestCheckFoldersMissingBack(t *testing.T) {
	a, dir, evs := checkApp(t, 2, false)
	path := filepath.Join(dir, "p1.jpg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	id := fcAlbum(t, a, path)
	loudScan(t, a)
	v0 := a.editor().View()
	evs.take()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	runCheck(t, a, true)
	if got, n := lastMissing(t, evs.take()); n != 1 || !slices.Equal(got, []string{id}) {
		t.Fatalf("удалили: missing %v (%d раз)", got, n)
	}
	if got := photoNames(a); !slices.Equal(got, []string{"p0.jpg"}) {
		t.Fatalf("удалили: лента %v", got)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	runCheck(t, a, true)
	if got, n := lastMissing(t, evs.take()); n != 1 || len(got) != 0 {
		t.Fatalf("вернули: missing %v (%d раз), ждали [] один раз", got, n)
	}
	if got := photoNames(a); !slices.Equal(got, []string{"p0.jpg", "p1.jpg"}) {
		t.Fatalf("вернули: лента %v", got)
	}
	v := a.editor().View()
	if v.Rev != v0.Rev || v.Dirty != v0.Dirty || v.CanUndo != v0.CanUndo || v.CanRedo != v0.CanRedo {
		t.Fatalf("сверка тронула альбом: rev %d→%d, dirty %v→%v, отмена %v→%v", v0.Rev, v.Rev, v0.Dirty, v.Dirty,
			v0.CanUndo, v.CanUndo)
	}
}

// Идущий прогрев не теряет фото: тихое чтение его прерывает и греет всё заново (088-7).
func TestCheckFoldersWarmContinues(t *testing.T) {
	a, dir, evs := checkApp(t, 3, true)
	a.layouts.enter() // прогрев миниатюр стоит, пока «греются раскладки»
	released := false
	t.Cleanup(func() {
		if !released {
			a.layouts.leave()
		}
	})
	a.rescan()
	waitCheck(t, a)
	waitFor(t, a, "прогрев выдан", func() bool { return a.warm != nil })
	testkit.JPEG(t, dir, "p9.jpg", 20, 10)
	evs.take()
	runCheck(t, a, false)
	released = true
	a.layouts.leave()
	waitWarm(t, a)
	th := only(evs.take(), "thumbs")
	if len(th) < 2 || th[0].data[0].(int) != 0 || th[0].data[1].(int) != 0 {
		t.Fatalf("прерванный прогрев: ждали сброс 0/0 первым, события %v", th)
	}
	if last := th[len(th)-1]; last.data[0].(int) != 4 || last.data[1].(int) != 4 {
		t.Fatalf("прогрев после сверки %v, ждали 4/4 (все фото)", last.data)
	}
}

// deadUnder подменяет reachable: том папки dir «не отвечает». Возвращает «вернуть том».
func deadUnder(t *testing.T, dir string) (revive func()) {
	t.Helper()
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(p string) bool {
		return !strings.EqualFold(filepath.Clean(p), filepath.Clean(dir)) && !library.Under(p, dir)
	}
	return func() { reachable = old }
}

// naturalPaths — ожидаемый порядок ленты: два списка путей в одном естественном порядке (как Scan).
func naturalPaths(a, b []string) []string {
	toPh := func(ps []string) (out []library.Photo) {
		for _, p := range ps {
			out = append(out, library.Photo{Path: p})
		}
		return out
	}
	var out []string
	for _, p := range library.MergeNatural(toPh(a), toPh(b)) {
		out = append(out, p.Path)
	}
	return out
}

func photoPaths(a *App) []string {
	var out []string
	for _, p := range a.GetState().Photos {
		out = append(out, p.Path)
	}
	return out
}

// Папка недоступного тома не обходится (её новый файл не виден), а её фото остаются в ленте —
// и при сверке, и при «Обновить все»; следующая сверка без изменений молчит.
func TestRescanDeadFolderKeeps(t *testing.T) {
	a, dirA, evs := checkApp(t, 2, false)
	dirB := testkit.TempDir(t)
	testkit.JPEG(t, dirB, "b0.jpg", 20, 10)
	testkit.JPEG(t, dirB, "b1.jpg", 20, 10)
	if _, err := a.editor().AddFolder(dirB); err != nil {
		t.Fatal(err)
	}
	loudScan(t, a)
	evs.take()

	revive := deadUnder(t, dirB)
	testkit.JPEG(t, dirA, "p2.jpg", 20, 10)
	testkit.JPEG(t, dirB, "b2.jpg", 20, 10) // том «не отвечает»: этот файл не увидеть
	pa := func(names ...string) (out []string) {
		for _, n := range names {
			out = append(out, filepath.Join(dirA, n))
		}
		return out
	}
	pb := func(names ...string) (out []string) {
		for _, n := range names {
			out = append(out, filepath.Join(dirB, n))
		}
		return out
	}
	// Порядок ленты — естественный по пути, без пересортировки в тесте.
	want := naturalPaths(pa("p0.jpg", "p1.jpg", "p2.jpg"), pb("b0.jpg", "b1.jpg"))
	check := func(step string) {
		t.Helper()
		got := photoPaths(a)
		if !slices.Equal(got, want) {
			t.Fatalf("%s: лента %v, ждали %v", step, got, want)
		}
	}
	runCheck(t, a, false)
	check("сверка")
	if e := only(evs.take(), "scan"); len(e) != 0 {
		t.Errorf("сверка показала ход: %v", e)
	}

	a.Rescan() // «Обновить все»
	waitCheck(t, a)
	check("Обновить все")
	sc := only(evs.take(), "scan")
	if len(sc) == 0 || sc[len(sc)-1].data[0].(library.ScanProgress).Active {
		t.Fatalf("«Обновить все»: ход %v, ждали финал active:false", sc)
	}

	runCheck(t, a, false)
	if got := evs.take(); len(got) != 0 {
		t.Fatalf("без изменений (мёртвый том не обходится) пришли события %v", got)
	}

	// Том вернулся: следующая сверка подхватывает новый файл (088 п. 8).
	revive()
	runCheck(t, a, false)
	want = naturalPaths(pa("p0.jpg", "p1.jpg", "p2.jpg"), pb("b0.jpg", "b1.jpg", "b2.jpg"))
	check("том вернулся")
}

// Сверка не начинается до готовности запуска и после остановки программы; отказ не занимает ворота.
func TestBeginCheckRefusals(t *testing.T) {
	a, _, evs := checkApp(t, 2, false)
	loudScan(t, a)
	evs.take()

	// Запуск ещё не закончен: ready открыт заново.
	ready := a.ready
	a.ready = make(chan struct{})
	if a.beginCheck(true) {
		t.Fatal("сверка началась до a.ready")
	}
	a.ready = ready

	a.stopCancel()
	if a.beginCheck(true) {
		t.Fatal("сверка началась после остановки программы")
	}
	a.check.mu.Lock()
	running, pend := a.check.running, a.check.pendFull
	a.check.mu.Unlock()
	if running || pend {
		t.Fatalf("отказ занял ворота: running %v, pendFull %v", running, pend)
	}
	if got := evs.take(); len(got) != 0 {
		t.Fatalf("отказ прислал события %v", got)
	}
}

// Фото ленты на недоступном томе: /media — 404 без обращения к файлу (он на месте, а не отдан) и
// без записи в журнал.
func TestMediaDeadVolumeRegistry(t *testing.T) {
	a := newApp(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir := testkit.TempDir(t)
	path := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	id := library.IDFor(path)
	a.mu.Lock()
	a.byID[id] = library.Photo{ID: id, Path: path, Name: "a.jpg", W: 40, H: 30}
	a.mu.Unlock()
	if rec := getMedia(a, "/media/t/"+id); rec.Code != http.StatusOK {
		t.Fatalf("живой том: %d", rec.Code)
	}

	deadUnder(t, dir)
	oldRead := readColor
	t.Cleanup(func() { readColor = oldRead })
	readColor = func(string) (imgfile.Color, string) {
		t.Error("мёртвый том: чтение файла")
		return "", ""
	}
	miss := a.mediaMiss.Load()
	for _, kind := range []string{"t", "p"} {
		if rec := getMedia(a, "/media/"+kind+"/"+id); rec.Code != http.StatusNotFound {
			t.Fatalf("/media/%s мёртвого тома: %d, ждали 404", kind, rec.Code)
		}
	}
	if a.mediaMiss.Load() != miss {
		t.Error("мёртвый том записан в журнал как промах")
	}
}
