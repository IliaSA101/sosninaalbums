package agents

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/peercred"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// closedSilently — сервер закрыл подключение, не отправив ни байта (Read: 0 байт и EOF/сброс).
func closedSilently(t *testing.T, c net.Conn, info ...func() string) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	buf := make([]byte, 64)
	n, err := c.Read(buf)
	if n != 0 || err == nil {
		t.Fatalf("чужому отправлено %d байт (%q), ошибка %v", n, buf[:n], err)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		extra := ""
		for _, f := range info { // что видел сервер: отличает «не закрыл» от «закрыл, но клиент не увидел»
			extra += "; " + f()
		}
		t.Fatalf("подключение не закрыто: %v%s", err, extra)
	}
}

func count(s, sub string) int { return strings.Count(s, sub) }

// TestAdmitRejectsForeign: чужой закрыт без единого байта и не занимает место; строка в журнале
// одна, остальные за 10 с копятся; свой после чужих проходит (MaxConns 1 — места не съедены).
func TestAdmitRejectsForeign(t *testing.T) {
	buf := captureLog(t)
	var own atomic.Bool
	var calls atomic.Int32
	r := newRig(t, Options{MaxConns: 1, Peer: func(net.Conn) (bool, error) {
		calls.Add(1)
		return own.Load(), nil
	}}, "")
	r.start(t)
	for i := range 4 {
		closedSilently(t, raw(t, r.sock), func() string {
			return fmt.Sprintf("подключение %d из 4, сверок сервера %d, мест %d", i+1, calls.Load(), r.conns())
		})
	}
	if n := r.conns(); n != 0 {
		t.Fatalf("чужие заняли места: %d", n)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("сверок %d, ждали 4", got)
	}
	log := buf.String()
	if n := count(log, "agents: отклонено подключение другой учётной записи"); n != 1 {
		t.Fatalf("строк об отказе %d, ждали 1:\n%s", n, log)
	}
	if strings.Contains(log, r.sock) || strings.Contains(log, "uid") {
		t.Errorf("в журнале путь или uid: %q", log)
	}
	own.Store(true)
	client(t, r.sock, "свой")
	if r.conns() != 1 {
		t.Fatalf("свой не занял место: %d", r.conns())
	}
}

// TestAdmitPeerError: ОС не ответила — отказ (закрыто по умолчанию), причина в журнале без пути.
func TestAdmitPeerError(t *testing.T) {
	buf := captureLog(t)
	r := newRig(t, Options{Peer: func(net.Conn) (bool, error) {
		return true, errors.New("getsockopt: operation not permitted") // «true» вместе с ошибкой — тоже отказ
	}}, "")
	r.start(t)
	closedSilently(t, raw(t, r.sock))
	if r.conns() != 0 {
		t.Fatalf("место занято: %d", r.conns())
	}
	log := buf.String()
	if !strings.Contains(log, "проверка собеседника не удалась: getsockopt: operation not permitted") || strings.Contains(log, r.sock) {
		t.Errorf("журнал: %q", log)
	}
}

// TestAdmitUnsupported: Windows без запроса PID — подключение принято, строка в журнале одна за
// запуск (0.32-148-4).
func TestAdmitUnsupported(t *testing.T) {
	buf := captureLog(t)
	r := newRig(t, Options{Peer: func(net.Conn) (bool, error) { return false, peercred.ErrUnsupported }}, "")
	r.start(t)
	client(t, r.sock, "первый")
	client(t, r.sock, "второй")
	if n := r.conns(); n != 2 {
		t.Fatalf("принято %d, ждали 2", n)
	}
	if n := count(buf.String(), "проверка собеседника недоступна"); n != 1 {
		t.Fatalf("строк %d, ждали 1:\n%s", n, buf.String())
	}
}

// TestAdmitPeerPanic: паника сверки закрывает подключение, служба живёт.
func TestAdmitPeerPanic(t *testing.T) {
	captureLog(t)
	var boom atomic.Bool
	boom.Store(true)
	r := newRig(t, Options{Peer: func(net.Conn) (bool, error) {
		if boom.Load() {
			panic("сбой")
		}
		return true, nil
	}}, "")
	r.start(t)
	closedSilently(t, raw(t, r.sock))
	boom.Store(false)
	client(t, r.sock, "после")
}

// TestAdmitClosed: подключение, пришедшее после Close, закрыто без сверки.
func TestAdmitClosed(t *testing.T) {
	var calls atomic.Int32
	r := newRig(t, Options{Peer: func(net.Conn) (bool, error) { calls.Add(1); return true, nil }}, "")
	r.start(t)
	svc := r.svc
	svc.mu.Lock()
	run := svc.run
	svc.mu.Unlock()
	run.mu.Lock()
	run.closed = true
	run.mu.Unlock()
	a, b := net.Pipe()
	defer func() { _ = b.Close() }()
	run.admit(a)
	if _, err := b.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("после закрытия: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("закрытая служба сверяла собеседника")
	}
}

// TestThrottle: первая строка сразу, остальные в окне 10 с копятся, следующая после окна несёт число.
func TestThrottle(t *testing.T) {
	var th throttle
	t0 := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	step := func(d time.Duration, wantWrite bool, wantSkipped int) {
		t.Helper()
		w, n := th.hit(t0.Add(d))
		if w != wantWrite || n != wantSkipped {
			t.Fatalf("hit(+%v) = %v, %d; ждали %v, %d", d, w, n, wantWrite, wantSkipped)
		}
	}
	step(0, true, 0)
	step(time.Second, false, 0)
	step(9*time.Second, false, 0)
	step(foreignEvery, true, 2) // отсчёт от последней записанной, не от последней пропущенной
	step(foreignEvery+time.Second, false, 0)
	step(2*foreignEvery, true, 1)
	step(3*foreignEvery, true, 0)
}

// TestSocketStaleForeignPeer: на месте сокета отвечает чужая учётная запись — errForeign, файл цел;
// своя копия — errBusy; ошибка сверки — errOpen (ничего не удалено); Windows без PID — errBusy.
func TestSocketStaleForeignPeer(t *testing.T) {
	captureLog(t)
	a := newRig(t, Options{}, "")
	a.start(t)
	cases := []struct {
		name string
		peer func(net.Conn) (bool, error)
		want error
	}{
		{"чужой", func(net.Conn) (bool, error) { return false, nil }, errForeign},
		{"своя копия", func(net.Conn) (bool, error) { return true, nil }, errBusy},
		{"ошибка ОС", func(net.Conn) (bool, error) { return true, errors.New("сбой") }, errOpen},
		{"нет сверки", func(net.Conn) (bool, error) { return false, peercred.ErrUnsupported }, errBusy},
	}
	for _, c := range cases {
		b := newRig(t, Options{Peer: c.peer}, a.sock)
		before, err := os.Lstat(a.sock)
		if err != nil {
			t.Fatal(err)
		}
		if err := b.svc.Start(); !errors.Is(err, c.want) {
			t.Fatalf("%s: Start = %v, ждали %v", c.name, err, c.want)
		}
		if st, e := b.svc.State(); st != StateError || e != c.want.Error() {
			t.Errorf("%s: State = %q %q", c.name, st, e)
		}
		after, err := os.Lstat(a.sock)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("%s: чужой сокет тронут: %v", c.name, err)
		}
		client(t, a.sock, "жив-"+c.name)
	}
}
