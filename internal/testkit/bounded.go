package testkit

import (
	"bufio"
	"regexp"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// Bounded вызывает fn в горутине и ждёт результат не дольше d (бюджет «секунды», не доли секунды).
// Для тестов «не ждёт / отменяется сразу»: блокировку, которую нельзя пройти, тест держит до конца,
// так что регресс («ждёт») не медленнее на пару миллисекунд, а зависает — и бюджет его ловит.
// Время при этом не сравнивается: на занятой машине ложных провалов нет. Не уложился — t.Fatalf
// (отложенные Release/Unlock вызывающего отпустят застрявшую горутину).
func Bounded[T any](t testing.TB, d time.Duration, what string, fn func() T) T {
	t.Helper()
	ch := make(chan T, 1)
	go func() { ch <- fn() }()
	v, ok := WaitCh(t, ch, d, what, nil)
	if !ok {
		t.Fatalf("%s: нет ответа за %v", what, d)
	}
	return v
}

// goroutineHead — первая строка стека горутины в pprof debug=2: «goroutine 7 [sync.Mutex.Lock, 2 minutes]:».
var goroutineHead = regexp.MustCompile(`^goroutine \d+ \[([^\],]+)`)

// WaitBlocked ждёт до d, пока какая-нибудь горутина со стеком, содержащим frame (подстрока имени
// функции), не встанет на ожидание (замок, канал, select). Это «вошла и стоит» без пауз-угадайки:
// после него отрицательную проверку («пока стоит, замок X свободен») можно делать сразу.
// Горутина, вернувшаяся или ещё не начавшая, не считается; false — не встала за d.
func WaitBlocked(t testing.TB, frame string, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		if blockedIn(frame) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func blockedIn(frame string) bool {
	var b strings.Builder
	if p := pprof.Lookup("goroutine"); p != nil {
		_ = p.WriteTo(&b, 2)
	}
	for _, g := range strings.Split(b.String(), "\n\n") {
		if !strings.Contains(g, frame) {
			continue
		}
		m := goroutineHead.FindStringSubmatch(firstLine(g))
		if m == nil {
			continue
		}
		switch m[1] {
		case "running", "runnable", "syscall":
		default:
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	if sc.Scan() {
		return sc.Text()
	}
	return ""
}
