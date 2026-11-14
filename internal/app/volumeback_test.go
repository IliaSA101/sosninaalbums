package app

import (
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// «Диск снова доступен» (157, volumeback.go). Тома — папки E, G под temp-папкой теста: буква по
// префиксу пути (шов volumeOf), потому что на Linux filepath.VolumeName пуст. Состояние «диск
// не отвечает» и «файла нет» задаёт шов a.exists.

type volFix struct {
	a    *App
	evs  *evLog
	logs *syncBuf
	root string
	dirs map[string]string   // «E:» → папка тома; "L:" — без тома (volumeOf даёт "")
	ids  map[string][]string // «E:» → ID фото тома по порядку
	path map[string]string   // ID → путь

	mu    sync.Mutex
	down  map[string]bool // том не отвечает: файлы его фото «не найдены»
	lost  map[string]bool // путь, файла которого нет, хотя том отвечает
	kicks map[string]int  // «E:» → число Kick
}

// newVolFix — альбом из фото на томах E: (3), G: (2) и в папке L (1, без тома). Швы восстановит cleanup.
func newVolFix(t *testing.T) *volFix {
	t.Helper()
	a := newApp(t)
	f := &volFix{a: a, evs: &evLog{}, logs: &syncBuf{}, root: testkit.TempDir(t),
		dirs: map[string]string{}, ids: map[string][]string{}, path: map[string]string{},
		down: map[string]bool{}, lost: map[string]bool{}, kicks: map[string]int{}}
	a.cfgDir = testkit.TempDir(t)
	a.emit = f.evs.add
	var infos []model.PhotoInfo
	for _, c := range []struct {
		vol string
		n   int
	}{{"E:", 3}, {"G:", 2}, {"L:", 1}} {
		dir := filepath.Join(f.root, strings.TrimSuffix(c.vol, ":"))
		f.dirs[c.vol] = dir
		for i := range c.n {
			p := testkit.JPEG(t, dir, "p"+string(rune('0'+i))+".jpg", 40, 30)
			id := library.IDFor(p)
			f.ids[c.vol] = append(f.ids[c.vol], id)
			f.path[id] = p
			infos = append(infos, model.PhotoInfo{ID: id, Path: p, W: 40, H: 30, Size: fileLen(t, p)})
		}
	}
	if _, err := a.editor().NewSpread(-1, infos); err != nil {
		t.Fatal(err)
	}
	a.exists = func(p string) bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return !f.down[f.volOf(p)] && !f.lost[p]
	}
	oldVol, oldKick, oldRetry, oldTries := volumeOf, kickVolume, backRetry, backTries
	volumeOf, backRetry, backTries = f.volOf, 10*time.Millisecond, 1000 // окно повтора ≈ 10 с
	kickVolume = func(p string) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.kicks[f.volOf(p)]++
	}
	log.SetOutput(f.logs)
	t.Cleanup(func() { // раньше BeforeRemove из newApp: фон читает швы
		a.closeBackground()
		volumeOf, kickVolume, backRetry, backTries = oldVol, oldKick, oldRetry, oldTries
		log.SetOutput(os.Stderr)
	})
	return f
}

// volOf — «E:» для путей под папкой тома E, иначе "" (L и прочее — без тома).
func (f *volFix) volOf(p string) string {
	for vol, dir := range f.dirs {
		if vol != "L:" && strings.HasPrefix(p, dir+string(filepath.Separator)) {
			return vol
		}
	}
	return ""
}

func (f *volFix) setDown(vol string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[vol] = down
}

func (f *volFix) setLost(p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lost[p] = true
}

// seed — полный пересчёт при заданном состоянии дисков; события отбрасываются.
func (f *volFix) seed(t *testing.T, want int) {
	t.Helper()
	loudScan(t, f.a)        // чтение папок и пересчёт недостающих, как при запуске
	runCheck(t, f.a, false) // отпечаток папок снят: следующая проверка не перечитывает их сама
	f.a.check.mu.Lock()
	f.a.check.end = time.Time{} // порог частоты не мешает, «проверка прошла» — end не нулевой
	f.a.check.mu.Unlock()
	if got := len(f.a.GetState().Missing); got != want {
		t.Fatalf("недостающих %d, ждали %d", got, want)
	}
	f.evs.take()
	f.mu.Lock()
	clear(f.kicks)
	f.mu.Unlock()
}

// seen — события name с начала (без сброса).
func (f *volFix) seen(name string) []emitted {
	f.evs.mu.Lock()
	defer f.evs.mu.Unlock()
	return only(f.evs.evs, name)
}

// order — имена событий missing и drive:back по порядку отправки.
func (f *volFix) order() []string {
	f.evs.mu.Lock()
	defer f.evs.mu.Unlock()
	var out []string
	for _, e := range f.evs.evs {
		if e.name == "missing" || e.name == "drive:back" {
			out = append(out, e.name)
		}
	}
	return out
}

// waitEv ждёт n событий name (счёт с начала, после seed).
func (f *volFix) waitEv(t *testing.T, name string, n int) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for len(f.seen(name)) < n {
		if time.Now().After(deadline) {
			t.Fatalf("событий %q: %d, ждали %d", name, len(f.seen(name)), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// waitChecked ждёт, пока пройдёт хотя бы одна проверка папок (backWork закончен) и всё осядет.
func (f *volFix) waitChecked(t *testing.T) {
	t.Helper()
	waitFor(t, f.a, "проверка папок после оживления", func() bool {
		f.a.check.mu.Lock()
		defer f.a.check.mu.Unlock()
		return !f.a.check.end.IsZero() && !f.a.check.running && !f.a.scanning
	})
	waitMissing(t, f.a)
}

// backs — данные событий drive:back.
func (f *volFix) backs() []driveBack {
	var out []driveBack
	for _, e := range f.seen("drive:back") {
		out = append(out, e.data[0].(driveBack))
	}
	return out
}

func (f *volFix) gen() int {
	f.a.mu.Lock()
	defer f.a.mu.Unlock()
	return f.a.missGen
}

func TestVolumeBackClearsMissing(t *testing.T) {
	t.Run("все нашлись", func(t *testing.T) {
		f := newVolFix(t)
		f.setDown("E:", true)
		f.seed(t, 3)
		f.setDown("E:", false)
		f.a.volumeBack("E:")
		f.waitEv(t, "drive:back", 1)
		f.waitChecked(t)
		if got := f.backs(); !slices.Equal(got, []driveBack{{"E", 3}}) {
			t.Fatalf("drive:back %+v", got)
		}
		if ord := f.order(); !slices.Equal(ord, []string{"missing", "drive:back"}) {
			t.Fatalf("порядок событий %v: missing раньше drive:back", ord)
		}
		if m := f.seen("missing")[0].data[0].([]string); len(m) != 0 {
			t.Fatalf("missing после оживления: %v", m)
		}
		if m := f.a.GetState().Missing; len(m) != 0 {
			t.Fatalf("GetState: %v", m)
		}
		if n := strings.Count(f.logs.String(), "диск E: снова отвечает"); n != 1 {
			t.Fatalf("строк в журнале %d:\n%s", n, f.logs.String())
		}
	})
	t.Run("часть найдена", func(t *testing.T) {
		f := newVolFix(t)
		f.setDown("E:", true)
		f.seed(t, 3)
		f.setDown("E:", false)
		f.setLost(f.path[f.ids["E:"][1]]) // одно фото на диске удалено
		f.a.volumeBack("E:")
		f.waitEv(t, "drive:back", 1)
		f.waitChecked(t)
		if got := f.backs(); !slices.Equal(got, []driveBack{{"E", 2}}) {
			t.Fatalf("drive:back %+v", got)
		}
		if m := f.a.GetState().Missing; !slices.Equal(m, []string{f.ids["E:"][1]}) {
			t.Fatalf("остаток недостающих %v", m)
		}
	})
	t.Run("альбом закрыт", func(t *testing.T) {
		f := newVolFix(t)
		f.setDown("E:", true)
		f.seed(t, 3)
		f.setDown("E:", false)
		f.a.closeBackground() // выход из программы
		before := f.gen()
		f.a.volumeBack("E:")
		if f.gen() != before || len(f.evs.take()) != 0 || strings.Contains(f.logs.String(), "снова отвечает") {
			t.Fatalf("после закрытия: событий и строки журнала быть не должно:\n%s", f.logs.String())
		}
	})
}

func TestVolumeBackNoMissingNoEvent(t *testing.T) {
	t.Run("на томе нет недостающих", func(t *testing.T) {
		f := newVolFix(t)
		f.setDown("E:", true)
		f.seed(t, 3)
		before := f.gen()
		f.a.volumeBack("G:") // ожил другой том: пересчёт не нужен, сверка папок идёт
		f.waitChecked(t)
		if f.gen() != before {
			t.Fatalf("пересчёт без недостающих на томе: missGen %d → %d", before, f.gen())
		}
		if len(f.seen("drive:back")) != 0 || len(f.a.GetState().Missing) != 3 {
			t.Fatalf("drive:back %v, недостающих %d", f.backs(), len(f.a.GetState().Missing))
		}
	})
	t.Run("не нашлись", func(t *testing.T) {
		f := newVolFix(t)
		f.setDown("E:", true)
		f.seed(t, 3)
		f.setDown("E:", false)
		for _, id := range f.ids["E:"] {
			f.setLost(f.path[id])
		}
		before := f.gen()
		f.a.volumeBack("E:")
		f.waitChecked(t)
		if f.gen() <= before { // иначе тест проходил бы и без кода
			t.Fatalf("полный пересчёт не прошёл: missGen %d → %d", before, f.gen())
		}
		if len(f.seen("drive:back")) != 0 {
			t.Fatalf("drive:back без найденных: %v", f.backs())
		}
		if len(f.a.GetState().Missing) != 3 {
			t.Fatalf("недостающих %d, ждали прежние 3", len(f.a.GetState().Missing))
		}
	})
}

// holdFirst — a.exists, у которого первый вызов встаёт на gate (пересчёт «до оживления») и после
// ответа видит том мёртвым; entered закрывается, когда вызов встал.
func (f *volFix) holdFirst() (entered <-chan struct{}, release func()) {
	gate, in := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	base := f.a.exists
	f.a.exists = func(p string) bool {
		if first.CompareAndSwap(false, true) {
			close(in)
			<-gate
			return false // устаревший взгляд: том ещё мёртв
		}
		return base(p)
	}
	var once sync.Once
	return in, func() { once.Do(func() { close(gate) }) }
}

func TestVolumeBackStalePendingDiscarded(t *testing.T) {
	f := newVolFix(t)
	f.setDown("E:", true)
	f.seed(t, 3)
	entered, release := f.holdFirst()
	defer release()
	f.a.refreshMissingQuiet() // пересчёт, начатый до оживления: стоит на первой проверке
	<-entered
	f.setDown("E:", false)
	f.a.volumeBack("E:")
	f.waitEv(t, "drive:back", 1)
	release() // старый пересчёт доходит до конца и ничего не затирает
	f.waitChecked(t)
	if got := f.backs(); !slices.Equal(got, []driveBack{{"E", 3}}) {
		t.Fatalf("drive:back %+v", got)
	}
	if m := f.a.GetState().Missing; len(m) != 0 {
		t.Fatalf("устаревший итог вернул недостающих: %v", m)
	}
	for _, e := range f.seen("missing") {
		if len(e.data[0].([]string)) != 0 {
			t.Fatalf("событие missing со старым списком: %v", e.data[0])
		}
	}
}

func TestVolumeBackTwoVolumes(t *testing.T) {
	f := newVolFix(t)
	f.setDown("E:", true)
	f.setDown("G:", true)
	f.seed(t, 5)
	entered, release := f.holdFirst()
	defer release()
	f.a.refreshMissingQuiet() // идёт пересчёт, начатый до оживления
	<-entered
	f.setDown("E:", false)
	f.setDown("G:", false)
	f.a.volumeBack("E:")
	f.a.volumeBack("G:")
	f.waitEv(t, "drive:back", 2)
	release()
	f.waitChecked(t)
	got := f.backs()
	slices.SortFunc(got, func(x, y driveBack) int { return strings.Compare(x.Drive, y.Drive) })
	if !slices.Equal(got, []driveBack{{"E", 3}, {"G", 2}}) {
		t.Fatalf("drive:back %+v: ждали по событию на букву", got)
	}
	if m := f.a.GetState().Missing; len(m) != 0 {
		t.Fatalf("недостающие %v", m)
	}
}

func TestVolumeBackAfterRelinkNoToast(t *testing.T) {
	f := newVolFix(t)
	f.setDown("E:", true)
	f.seed(t, 3)
	// Диск ответил, но фото на нём нет: пользователь нашёл их в другой папке.
	f.setDown("E:", false)
	moved := filepath.Join(f.root, "Новая")
	for _, id := range f.ids["E:"] {
		src := f.path[id]
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(moved, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(moved, filepath.Base(src)), b, 0o644); err != nil {
			t.Fatal(err)
		}
		f.setLost(src)
	}
	entered, release := f.holdFirst()
	defer release()
	f.a.volumeBack("E:") // снимок взят; пересчёт стоит на первой проверке
	<-entered
	res, err := f.a.RelinkFolderPath(moved) // «Найти…»: перепривязал, победил полный пересчёт
	if err != nil || res.Found != 3 {
		t.Fatalf("Найти…: %+v %v", res, err)
	}
	release()
	f.waitChecked(t)
	if len(f.seen("drive:back")) != 0 {
		t.Fatalf("тост о перепривязанных фото: %v", f.backs())
	}
	if m := f.a.GetState().Missing; len(m) != 0 {
		t.Fatalf("недостающие %v", m)
	}
}

// Пересчёт по фокусу ждёт ту же пробу тома, что и оживший том; обработчик срабатывает до её
// пробуждения (контракт osx), поэтому пересчёт по фокусу не съедает тост.
func TestVolumeBackFocusRefreshKeepsToast(t *testing.T) {
	f := newVolFix(t)
	var mu sync.Mutex
	probe := make(chan struct{}) // проба E: — закрыта, когда диск ответил
	up := false
	entered := make(chan struct{}, 8)
	oldReach := reachable
	t.Cleanup(func() { f.a.closeBackground(); reachable = oldReach })
	reachable = func(p string) bool {
		if f.volOf(p) != "E:" {
			return true
		}
		mu.Lock()
		ch := probe
		mu.Unlock()
		entered <- struct{}{}
		<-ch
		mu.Lock()
		defer mu.Unlock()
		return up
	}
	f.a.exists = fileExists
	closedProbe := make(chan struct{})
	close(closedProbe)
	mu.Lock()
	probe = closedProbe // диск мёртв: проба уже ответила «нет»
	mu.Unlock()
	f.seed(t, 3)
	for len(entered) > 0 { // входы пересчёта seed
		<-entered
	}
	mu.Lock()
	probe = make(chan struct{}) // следующая проба ещё висит
	pending := probe
	mu.Unlock()

	f.a.refreshMissingQuiet() // пересчёт по фокусу: встал на пробе
	<-entered
	mu.Lock()
	up = true
	mu.Unlock()
	f.a.volumeBack("E:") // обработчик osx — до пробуждения ждущих
	close(pending)       // проба ответила: ждущий пересчёт идёт дальше
	f.waitEv(t, "drive:back", 1)
	f.waitChecked(t)
	if got := f.backs(); !slices.Equal(got, []driveBack{{"E", 3}}) {
		t.Fatalf("drive:back %+v: ждали один тост", got)
	}
	if m := f.a.GetState().Missing; len(m) != 0 {
		t.Fatalf("недостающие %v", m)
	}
}

// Ворота проверки папок заняты (идёт чтение или другая проверка): проверка повторяется,
// когда они освободятся — тик 10 с идёт только при активном окне.
func TestVolumeBackRecheckWhenGateBusy(t *testing.T) {
	f := newVolFix(t)
	f.setDown("E:", true)
	f.seed(t, 3)
	f.a.check.mu.Lock()
	f.a.check.running = true // ворота заняты
	f.a.check.mu.Unlock()
	f.setDown("E:", false)
	f.a.volumeBack("E:")
	f.waitEv(t, "drive:back", 1) // пересчёт не зависит от ворот
	time.Sleep(5 * backRetry)    // отсутствие проверки при занятых воротах (срок, а не условие)
	f.mu.Lock()
	busyKicks := f.kicks["E:"]
	f.mu.Unlock()
	if busyKicks != 0 {
		t.Fatalf("проверка папок прошла при занятых воротах: %d", busyKicks)
	}
	f.a.check.abort() // ворота свободны
	f.waitChecked(t)
}

func TestCheckFoldersKicksMissingVolumes(t *testing.T) {
	f := newVolFix(t)
	f.setDown("E:", true)
	f.setDown("G:", true)
	f.setLost(f.path[f.ids["L:"][0]]) // фото без тома пропало
	f.seed(t, 6)
	runCheck(t, f.a, false)
	f.mu.Lock()
	got := maps.Clone(f.kicks)
	f.mu.Unlock()
	if len(got) != 2 || got["E:"] != 1 || got["G:"] != 1 {
		t.Fatalf("Kick по томам %v: ждали по одному для E: и G: (3+2 недостающих), без тома — нет", got)
	}
	// Недостающих нет — пробовать нечего.
	f.setDown("E:", false)
	f.setDown("G:", false)
	f.a.mu.Lock()
	f.a.missing = nil
	f.a.mu.Unlock()
	f.mu.Lock()
	clear(f.kicks)
	f.mu.Unlock()
	runCheck(t, f.a, false)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.kicks) != 0 {
		t.Fatalf("Kick без недостающих: %v", f.kicks)
	}
}
