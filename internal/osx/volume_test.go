package osx

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeVol — том по префиксу «E:» в строке пути: буквы дисков настоящего VolumeName есть только
// в Windows, а проверку ведёт ведущая машина.
func fakeVol(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return strings.ToUpper(p[:2])
	}
	return ""
}

func newVolumes(stat func(string) (fs.FileInfo, error), timeout time.Duration) *Volumes {
	v := NewVolumes(stat, timeout)
	v.volOf = fakeVol
	return v
}

// shiftClock сдвигает часы кэша вперёд на d (срок «мёртвого» без ожидания).
func shiftClock(v *Volumes, d time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	base := v.now
	v.now = func() time.Time { return base().Add(d) }
}

// backs собирает буквы томов, на которых сработал обработчик.
func backs(v *Volumes) (ch chan string, off func()) {
	ch = make(chan string, 64)
	return ch, v.OnBack(func(vol string) { ch <- vol })
}

func waitVol(t *testing.T, ch <-chan string, what string) string {
	t.Helper()
	vol, ok := testkit.WaitCh(t, ch, testkit.Limit(2*time.Second), what, nil)
	if !ok {
		t.Fatalf("не дождались: %s", what)
	}
	return vol
}

// Висящий корень тома: ответ «недоступен» не позже 2×таймаута, второй вызов — из кэша, сразу.
func TestReachableTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	v := newVolumes(func(string) (fs.FileInfo, error) { calls.Add(1); <-release; return nil, nil }, 100*time.Millisecond)

	start := time.Now()
	if v.Reachable(`Z:\Съёмка\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Fatalf("ждали %v при таймауте 100 мс", d)
	}
	start = time.Now()
	if v.Reachable(`z:\другое\b.jpg`) {
		t.Fatal("висящий том доступен")
	}
	if d := time.Since(start); d > testkit.Limit(50*time.Millisecond) {
		t.Fatalf("второй вызов ждал %v, а не взял кэш", d)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("проб %d, ждали одну", n)
	}
	// Кэш устарел, а проба всё висит: новая не запускается, ответ сразу.
	shiftClock(v, time.Minute)
	if v.Reachable(`Z:\a`) || calls.Load() != 1 {
		t.Fatalf("после кэша: проб %d", calls.Load())
	}
	// Пути без тома проверять нечем.
	if !v.Reachable("относительный") {
		t.Fatal("путь без тома недоступен")
	}
}

// Десять параллельных вызовов на один том — одна проба; ответ доступного тома кэшируется.
func TestReachableSingleFlight(t *testing.T) {
	var calls atomic.Int32
	gate, started := make(chan struct{}), make(chan struct{})
	var once sync.Once
	v := newVolumes(func(string) (fs.FileInfo, error) {
		calls.Add(1)
		once.Do(func() { close(started) })
		<-gate
		return nil, nil
	}, testkit.Limit(time.Second))
	var wg sync.WaitGroup
	res := make([]bool, 10)
	for i := range res {
		wg.Go(func() { res[i] = v.Reachable(`C:\x`) })
	}
	<-started
	close(gate)
	wg.Wait()
	for i, ok := range res {
		if !ok {
			t.Errorf("вызов %d: недоступен", i)
		}
	}
	if !v.Reachable(`c:\y`) {
		t.Error("кэш доступного тома")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("проб %d, ждали одну", n)
	}
}

// Проба, не уложившаяся в таймаут, по окончании обновляет кэш.
func TestReachableLateProbeUpdates(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	if v.Reachable(`Y:\a`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	deadline := time.Now().Add(testkit.Limit(2 * time.Second))
	for !v.Reachable(`Y:\a`) {
		if time.Now().After(deadline) {
			t.Fatal("поздний ответ пробы не попал в кэш")
		}
		time.Sleep(time.Millisecond)
	}
}

// Настоящий VolumeKey: на Unix том пути пуст — Reachable «да», Kick ничего не делает.
func TestVolumeKeyAndNoVolume(t *testing.T) {
	if got := VolumeKey("относительный/путь"); got != "" {
		t.Fatalf("VolumeKey относительного пути: %q", got)
	}
	var calls atomic.Int32
	v := newVolumes(func(string) (fs.FileInfo, error) { calls.Add(1); return nil, nil }, time.Second)
	v.Kick("")
	v.Kick("/home/x")
	if calls.Load() != 0 {
		t.Fatal("Kick пути без тома запустил пробу")
	}
}

// Том, не ответивший в срок, при позднем ответе даёт обработчик ровно один раз.
func TestBackAfterLateProbe(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	ch, off := backs(v)
	defer off()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	if got := waitVol(t, ch, "обработчик позднего ответа"); got != "E:" {
		t.Fatalf("том %q, ждали E:", got)
	}
	if !v.Reachable(`E:\a.jpg`) { // кэш «жив»: вторая проба не нужна
		t.Fatal("после позднего ответа том недоступен")
	}
	if len(ch) != 0 {
		t.Fatalf("лишних вызовов: %d", len(ch))
	}
}

// Том, ответивший сразу, обработчика не зовёт; ответ в срок после ожидания — тоже.
func TestNoBackOnFirstLive(t *testing.T) {
	v := newVolumes(func(string) (fs.FileInfo, error) { return nil, nil }, testkit.Limit(time.Second))
	ch, off := backs(v)
	defer off()
	if !v.Reachable(`E:\a.jpg`) {
		t.Fatal("живой том недоступен")
	}
	// Протухший ответ живого и новая успешная проба — тоже не «вернулся».
	shiftClock(v, time.Minute)
	if !v.Reachable(`E:\a.jpg`) {
		t.Fatal("живой том недоступен после срока")
	}
	if len(ch) != 0 {
		t.Fatalf("обработчик на живом томе: %d вызовов", len(ch))
	}
}

// После срока «мёртвого» проба снова с ошибкой — обработчика нет; следующая удачная — есть.
func TestNoBackWhileDead(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	v := newVolumes(func(string) (fs.FileInfo, error) {
		if fail.Load() {
			return nil, errors.New("диск не отвечает")
		}
		return nil, nil
	}, testkit.Limit(time.Second))
	ch, off := backs(v)
	defer off()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("том с ошибкой доступен")
	}
	shiftClock(v, time.Minute) // срок «мёртвого» вышел
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("том с ошибкой доступен после срока")
	}
	if len(ch) != 0 {
		t.Fatalf("обработчик при ошибке пробы: %d вызовов", len(ch))
	}
	fail.Store(false)
	shiftClock(v, time.Minute)
	if !v.Reachable(`E:\a.jpg`) {
		t.Fatal("ожившая буква недоступна")
	}
	if got := waitVol(t, ch, "обработчик после ожившей пробы"); got != "E:" {
		t.Fatalf("том %q", got)
	}
}

// Kick не ждёт; до срока пробу не запускает, после срока запускает — ожившая буква даёт обработчик.
func TestKickRevives(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int32
	gate := make(chan struct{})
	started := make(chan struct{}, 8)
	v := newVolumes(func(string) (fs.FileInfo, error) {
		calls.Add(1)
		started <- struct{}{}
		if fail.Load() {
			return nil, errors.New("нет диска")
		}
		<-gate
		return nil, nil
	}, testkit.Limit(time.Second))
	ch, off := backs(v)
	defer off()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("выдернутый диск доступен")
	}
	v.Kick(`E:\b.jpg`) // срок не вышел
	if n := calls.Load(); n != 1 {
		t.Fatalf("Kick до срока запустил пробу: %d", n)
	}
	shiftClock(v, time.Minute)
	fail.Store(false)
	<-started // проба первого Reachable
	t0 := time.Now()
	v.Kick(`E:\b.jpg`) // проба повисла на gate, Kick обязан вернуться
	if d := time.Since(t0); d > testkit.Limit(time.Second) {
		t.Fatalf("Kick ждал %v", d)
	}
	<-started
	v.Kick(`E:\c.jpg`) // идущая проба — новая не запускается
	if n := calls.Load(); n != 2 {
		t.Fatalf("проб %d, ждали 2", n)
	}
	close(gate)
	if got := waitVol(t, ch, "обработчик после Kick"); got != "E:" {
		t.Fatalf("том %q", got)
	}
}

// Обработчик вправе звать Reachable того же тома: кэш уже «жив», ожидания нет.
func TestBackHandlerReenters(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	got := make(chan bool, 1)
	off := v.OnBack(func(string) { got <- v.Reachable(`E:\a.jpg`) })
	defer off()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	ok, fin := testkit.WaitCh(t, got, testkit.Limit(2*time.Second), "Reachable из обработчика", nil)
	if !fin || !ok {
		t.Fatalf("Reachable из обработчика: fin=%v ok=%v", fin, ok)
	}
}

// Паника обработчика не мешает остальным и не оставляет ждущих без пробуждения.
func TestBackHandlerPanic(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	offA := v.OnBack(func(string) { panic("сбой обработчика") })
	defer offA()
	ch, offB := backs(v)
	defer offB()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	waitVol(t, ch, "второй обработчик после паники первого")
}

// Десять томов, параллельные вызовы: поздний ответ — ровно один вызов на каждую из пяти букв
// E, G, I, K, M; остальные пять отвечают сразу — ноль. Счёт по буквам, а не по длине списка.
func TestBackManyVolumes(t *testing.T) {
	late := map[string]bool{"E:": true, "G:": true, "I:": true, "K:": true, "M:": true}
	all := []string{"B:", "D:", "E:", "F:", "G:", "H:", "I:", "J:", "K:", "M:"}
	gate := make(chan struct{})
	v := newVolumes(func(p string) (fs.FileInfo, error) {
		if late[strings.ToUpper(p[:2])] {
			<-gate
		}
		return nil, nil
	}, testkit.Limit(500*time.Millisecond))
	var mu sync.Mutex
	count := map[string]int{}
	seen := make(chan string, 64)
	off := v.OnBack(func(vol string) {
		mu.Lock()
		count[vol]++
		mu.Unlock()
		seen <- vol
	})
	defer off()

	var wg sync.WaitGroup
	for _, vol := range all {
		for range 3 { // параллельные вызовы на каждый том
			wg.Go(func() { v.Reachable(vol + `\x.jpg`) })
		}
	}
	wg.Wait() // сонные тома — по таймауту пробы, живые — сразу
	close(gate)
	for range late {
		waitVol(t, seen, "обработчик сонного тома")
	}
	for _, vol := range all { // все пробы кончены: кэш жив, новых проб нет
		if !v.Reachable(vol + `\x.jpg`) {
			t.Errorf("том %s недоступен после ответа", vol)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, vol := range all {
		want := 0
		if late[vol] {
			want = 1
		}
		if count[vol] != want {
			t.Errorf("том %s: вызовов %d, ждали %d", vol, count[vol], want)
		}
	}
}

// off снимает только свой обработчик; повторный off безопасен.
func TestOnBackOff(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	var a atomic.Int32
	offA := v.OnBack(func(string) { a.Add(1) })
	chB, offB := backs(v)
	defer offB()
	offA()
	offA()
	if v.Reachable(`E:\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	waitVol(t, chB, "чужой обработчик после off соседа")
	if a.Load() != 0 {
		t.Fatal("снятый обработчик вызван")
	}
}
