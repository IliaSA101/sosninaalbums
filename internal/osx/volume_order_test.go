package osx

import (
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ждущий Reachable той же пробы просыпается только после обработчика (157-7): пересчёт по фокусу
// иначе лёг бы раньше снимка обработчика, и тост потерялся бы.
func TestBackBeforeWake(t *testing.T) {
	gate := make(chan struct{})
	v := newVolumes(func(string) (fs.FileInfo, error) { <-gate; return nil, nil }, testkit.Limit(5*time.Second))
	v.vols["E:"] = &volState{until: time.Now().Add(-time.Second)} // был мёртв, срок вышел
	var handled atomic.Bool
	off := v.OnBack(func(string) { time.Sleep(30 * time.Millisecond); handled.Store(true) }) // медленный: ждущий не должен его обогнать
	defer off()
	woke := make(chan bool, 1)
	go func() {
		ok := v.Reachable(`E:\a.jpg`) // ждёт пробу: таймаут велик
		woke <- ok && handled.Load()
	}()
	// Ждущий встал на пробе: идущая проба видна в состоянии тома.
	deadline := time.Now().Add(testkit.Limit(5 * time.Second))
	for {
		v.mu.Lock()
		s := v.vols["E:"]
		waiting := s != nil && s.probe != nil
		v.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("проба не началась")
		}
		time.Sleep(time.Millisecond)
	}
	close(gate)
	got, ok := testkit.WaitCh(t, woke, testkit.Limit(5*time.Second), "пробуждение ждущего Reachable", nil)
	if !ok || !got {
		t.Fatalf("ждущий проснулся раньше обработчика или не увидел живой том: %v", got)
	}
}
