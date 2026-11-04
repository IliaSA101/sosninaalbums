package testkit

import (
	"sync"
	"testing"
	"time"
)

// recorder — t, который не валит прогон: Fatalf фиксируется и прерывает горутину теста.
type recorder struct {
	testing.TB
	failed bool
	msg    string
}

func (r *recorder) Helper()    {}
func (r *recorder) Log(...any) {}
func (r *recorder) Fatalf(f string, a ...any) {
	r.failed = true
	r.msg = f
	panic(r) // вместо runtime.Goexit: проверяется recover'ом ниже
}

func fatalOf(fn func(r *recorder)) *recorder {
	r := &recorder{}
	func() {
		defer func() { _ = recover() }()
		fn(r)
	}()
	return r
}

// Bounded отдаёт результат быстрой функции и по бюджету валит тест на зависшей.
func TestBounded(t *testing.T) {
	if got := Bounded(t, time.Second, "быстрая", func() int { return 7 }); got != 7 {
		t.Fatalf("результат %d, ждали 7", got)
	}
	stop := make(chan struct{})
	defer close(stop)
	r := fatalOf(func(r *recorder) {
		Bounded(r, 20*time.Millisecond, "зависшая", func() int { <-stop; return 0 })
	})
	if !r.failed {
		t.Fatal("зависшая функция не провалила тест")
	}
}

// WaitBlocked находит горутину, стоящую на замке, и не находит вернувшуюся или не начавшуюся.
func TestWaitBlocked(t *testing.T) {
	var mu sync.Mutex
	mu.Lock()
	done := make(chan struct{})
	passed := false
	go func() { holdBlocked(&mu, &passed); close(done) }()
	if !WaitBlocked(t, "testkit.holdBlocked", 5*time.Second) {
		t.Fatal("горутина на замке не найдена")
	}
	select {
	case <-done:
		t.Fatal("горутина прошла замок, пока он занят")
	default:
	}
	mu.Unlock()
	<-done
	if WaitBlocked(t, "testkit.holdBlocked", 20*time.Millisecond) {
		t.Fatal("вернувшаяся горутина сочтена стоящей")
	}
	if WaitBlocked(t, "testkit.noSuchFrame", 20*time.Millisecond) {
		t.Fatal("несуществующий кадр найден")
	}
}

func holdBlocked(mu *sync.Mutex, passed *bool) {
	mu.Lock()
	*passed = true
	mu.Unlock()
}
