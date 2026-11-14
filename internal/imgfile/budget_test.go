package imgfile

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBudgetLimitsConcurrency(t *testing.T) {
	b := NewBudget(100)
	var cur, peak atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			b.Acquire(40) // помещаются двое
			n := cur.Add(40)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			cur.Add(-40)
			b.Release(40)
		})
	}
	wg.Wait()
	if peak.Load() > 100 {
		t.Fatalf("одновременно %d при лимите 100", peak.Load())
	}
	if b.Used() != 0 {
		t.Fatalf("бюджет не освобождён: %d", b.Used())
	}
}

func TestBudgetOversizeRunsAlone(t *testing.T) {
	b := NewBudget(100)
	b.Acquire(30)
	got := make(chan struct{})
	go func() {
		b.Acquire(500) // больше лимита: ждёт, пока не останется никого
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("большой запрос прошёл при занятом бюджете")
	case <-time.After(30 * time.Millisecond):
	}
	b.Release(30)
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("большой запрос не дождался пустого бюджета")
	}
	// Пока он держит бюджет, другие ждут.
	other := make(chan struct{})
	go func() { b.Acquire(1); close(other) }()
	select {
	case <-other:
		t.Fatal("запрос прошёл рядом с большим")
	case <-time.After(30 * time.Millisecond):
	}
	b.Release(500)
	<-other
}
