package guidevideo

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Cancel, пришедший после входа в Download, но до begin (гонка 0.31-tails-5), отменяет начинающуюся
// загрузку: ErrCancelled, ни запроса, ни файла; следующая загрузка идёт как обычно.
func TestCancelBeforeBeginCancelsStart(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	entered, gate := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	state := func() string {
		return fmt.Sprintf("сверок согласия %d, сеть %d", calls.Load(), l.hits.Load())
	}
	l.host.Allowed = func() bool {
		if calls.Add(1) == 1 { // первая сверка — сразу после входа в Download, до begin
			close(entered)
			<-gate
		}
		return true
	}
	l.rebuild()
	res := make(chan error, 1)
	go func() { _, err := l.svc.Download([]string{"cover"}); res <- err }()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "Download не дошёл до сверки согласия", state); !ok {
		t.FailNow()
	}
	l.svc.Cancel() // загрузка уже вошла, begin ещё нет
	close(gate)
	err, ok := testkit.WaitCh(t, res, testkit.Limit(10*time.Second), "Download завис", state)
	if !ok {
		t.FailNow()
	}
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("ошибка: %v", err)
	}
	if l.hits.Load() != 0 || len(l.listDir()) != 0 {
		t.Errorf("сеть %d, в папке %v", l.hits.Load(), l.listDir())
	}
	// Метка снята: ни отмена прошлого раза, ни Cancel без загрузки следующую не заденет.
	l.svc.Cancel()
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Errorf("следующая загрузка: %v", err)
	}
	l.svc.netMu.Lock()
	n := len(l.svc.starting)
	l.svc.netMu.Unlock()
	if n != 0 {
		t.Errorf("меток начала осталось %d", n)
	}
}

// Две загрузки: первая ждёт в окне старта, вторая проходит целиком (замок ещё свободен); отмена
// после неё задевает только вошедшую первую, меток не остаётся.
func TestCancelBeforeBeginTwoCalls(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	entered, gate := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	state := func() string {
		return fmt.Sprintf("сверок согласия %d, сеть %d", calls.Load(), l.hits.Load())
	}
	l.host.Allowed = func() bool {
		if calls.Add(1) == 1 {
			close(entered)
			<-gate
		}
		return true
	}
	l.rebuild()
	res := make(chan error, 2)
	go func() { _, err := l.svc.Download([]string{"cover"}); res <- err }()
	if _, ok := testkit.WaitCh(t, entered, testkit.Limit(10*time.Second), "Download не дошёл до сверки согласия", state); !ok {
		t.FailNow()
	}
	go func() { _, err := l.svc.Download([]string{"cover"}); res <- err }()
	err, ok := testkit.WaitCh(t, res, testkit.Limit(10*time.Second), "вторая загрузка зависла", state)
	if !ok {
		t.FailNow()
	}
	if err != nil {
		t.Fatalf("вторая: %v", err)
	}
	l.svc.Cancel()
	close(gate)
	if err, ok = testkit.WaitCh(t, res, testkit.Limit(10*time.Second), "первая загрузка зависла", state); !ok {
		t.FailNow()
	}
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("первая: %v", err)
	}
	l.svc.netMu.Lock()
	n := len(l.svc.starting)
	l.svc.netMu.Unlock()
	if n != 0 {
		t.Errorf("меток начала осталось %d", n)
	}
}
