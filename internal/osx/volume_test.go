package osx

import (
	"io/fs"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func needVolumes(t *testing.T) {
	t.Helper()
	if filepath.VolumeName(`Z:\x`) == "" {
		t.Skip("буквы дисков есть только в Windows")
	}
}

// Висящий корень тома: ответ «недоступен» не позже 2×таймаута, второй вызов — из кэша, сразу.
func TestReachableTimeout(t *testing.T) {
	needVolumes(t)
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	v := NewVolumes(func(string) (fs.FileInfo, error) { calls.Add(1); <-release; return nil, nil }, 100*time.Millisecond)

	start := time.Now()
	if v.Reachable(`Z:\Съёмка\a.jpg`) {
		t.Fatal("висящий том доступен")
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("ждали %v, предел 2×таймаут", d)
	}
	start = time.Now()
	if v.Reachable(`z:\другое\b.jpg`) {
		t.Fatal("висящий том доступен")
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Fatalf("второй вызов ждал %v, а не взял кэш", d)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("проб %d, ждали одну", n)
	}
	// Кэш устарел, а проба всё висит: новая не запускается, ответ сразу.
	v.mu.Lock()
	v.now = func() time.Time { return time.Now().Add(time.Minute) }
	v.mu.Unlock()
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
	needVolumes(t)
	var calls atomic.Int32
	gate := make(chan struct{})
	v := NewVolumes(func(string) (fs.FileInfo, error) { calls.Add(1); <-gate; return nil, nil }, time.Second)
	var wg sync.WaitGroup
	res := make([]bool, 10)
	for i := range res {
		wg.Go(func() { res[i] = v.Reachable(`C:\x`) })
	}
	time.Sleep(20 * time.Millisecond)
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
	needVolumes(t)
	gate := make(chan struct{})
	v := NewVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, 20*time.Millisecond)
	if v.Reachable(`Y:\a`) {
		t.Fatal("висящий том доступен")
	}
	close(gate)
	deadline := time.Now().Add(2 * time.Second)
	for !v.Reachable(`Y:\a`) {
		if time.Now().After(deadline) {
			t.Fatal("поздний ответ пробы не попал в кэш")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
