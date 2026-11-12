package sharer

import (
	"sync"
	"testing"
	"time"
)

// fakeClock — часы и таймеры теста: время идёт только по advance, отложенные отправки срабатывают
// только по fireAll (ни реальных пауз, ни зависимости от нагрузки машины).
type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool { was := !t.stopped; t.stopped = true; return was }

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

func (c *fakeClock) advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func (c *fakeClock) after(d time.Duration, f func()) stopper {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.t.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// fireAll запускает не погашенные таймеры, чьё время пришло.
func (c *fakeClock) fireAll() {
	c.mu.Lock()
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.t) {
			t.stopped = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

// progressRig — служба с записью событий хода загрузки на заданных часах и таймерах.
func progressRig(gap time.Duration, now func() time.Time, after func(time.Duration, func()) stopper) (*progress, func() []int) {
	var (
		mu   sync.Mutex
		done []int
	)
	s := &Service{gap: gap, now: now, after: after, h: Host{Emit: func(name string, data ...any) {
		if pe, ok := data[0].(progressEvent); ok && name == "share:progress" {
			mu.Lock()
			done = append(done, pe.Done)
			mu.Unlock()
		}
	}}}
	pr := &progress{s: s, total: 10, totalBytes: 100}
	pr.emit()
	return pr, func() []int {
		mu.Lock()
		defer mu.Unlock()
		return append([]int(nil), done...)
	}
}

// fakeRig — progressRig на fakeClock.
func fakeRig(gap time.Duration) (*progress, func() []int, *fakeClock) {
	c := &fakeClock{t: time.Unix(1000, 0)}
	pr, get := progressRig(gap, c.now, c.after)
	return pr, get, c
}

func same(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 0.32-146-T6: пачка подтверждений внутри gap не оставляет окно на старом счёте — отброшенное
// значение уходит, как только gap истёк, и ровно одним событием.
func TestProgressTrailingEdge(t *testing.T) {
	const gap = 150 * time.Millisecond
	pr, get, clk := fakeRig(gap)
	for range 3 {
		pr.add(10)
	}
	if d := get(); !same(d, []int{0}) {
		t.Fatalf("внутри gap ушло лишнее: %v", d)
	}
	clk.fireAll() // gap ещё не истёк: таймер не должен стрелять
	if d := get(); !same(d, []int{0}) {
		t.Fatalf("до конца gap: %v", d)
	}
	clk.advance(gap)
	clk.fireAll()
	clk.advance(10 * gap) // нового ничего нет — новых событий тоже
	clk.fireAll()
	if got := get(); !same(got, []int{0, 3}) {
		t.Errorf("события: %v, ждали [0 3]", got)
	}
}

// После flush (конец или остановка потоков) таймер не стреляет: события идут только до share:done.
func TestProgressNoEventAfterFlush(t *testing.T) {
	const gap = 60 * time.Millisecond
	pr, get, clk := fakeRig(gap)
	pr.add(10)
	pr.add(10) // отброшено из-за gap, таймер взведён
	pr.flush()
	before := get()
	if !same(before, []int{0, 2}) {
		t.Fatalf("после flush: %v, ждали [0 2]", before)
	}
	pr.add(10) // поздний ответ после остановки не взводит таймер
	clk.advance(10 * gap)
	clk.fireAll()
	if got := get(); !same(got, before) {
		t.Errorf("событие после flush: %v", got)
	}
}

// Таймер A сработал и ждёт mu; emit его гасит (Stop уже поздно), add взводит B. Опоздавший fire(A)
// не должен ни потерять B, ни слать событие в обход gap.
func TestProgressStaleFire(t *testing.T) {
	const gap = 100 * time.Millisecond
	pr, get, clk := fakeRig(gap)
	pr.add(1) // done 1 внутри gap: взведён A
	stale := clk.timers[0].f
	clk.advance(gap)
	pr.add(1) // since >= gap: emit(2), A погашен
	pr.add(1) // внутри нового gap: взведён B
	if len(clk.timers) != 2 {
		t.Fatalf("таймеров %d, ждали 2 (A и B)", len(clk.timers))
	}
	stale() // fire(A), опоздавший
	if got := get(); !same(got, []int{0, 2}) {
		t.Fatalf("опоздавший fire послал событие: %v", got)
	}
	pr.mu.Lock()
	armed := pr.timer != nil
	pr.mu.Unlock()
	if !armed {
		t.Fatal("опоздавший fire потерял таймер B")
	}
	clk.advance(gap)
	clk.fireAll()
	if got := get(); !same(got, []int{0, 2, 3}) {
		t.Errorf("события: %v, ждали [0 2 3]", got)
	}
}

// Потоки подтверждают одновременно: последнее событие — итоговое, после flush тишина (гонки ловит -race).
func TestProgressConcurrentFlush(t *testing.T) {
	const gap = 5 * time.Millisecond
	pr, get := progressRig(gap, time.Now, realAfter)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for range 3 {
				pr.add(1)
				time.Sleep(time.Millisecond)
			}
		})
	}
	wg.Wait()
	pr.flush()
	before := get()
	if last := before[len(before)-1]; last != 9 {
		t.Fatalf("последнее событие %d, ждали 9: %v", last, before)
	}
	time.Sleep(4 * gap)
	if got := get(); len(got) != len(before) {
		t.Errorf("событие после flush: %v -> %v", before, got)
	}
}
