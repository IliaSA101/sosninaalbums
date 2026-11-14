package peercred

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// pair — соединение AF_UNIX в одном процессе: принятый конец и подключённый.
func pair(t *testing.T) (server, client net.Conn) {
	t.Helper()
	l, err := net.Listen("unix", filepath.Join(testkit.TempDir(t), "s"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	type res struct {
		c   net.Conn
		err error
	}
	got := make(chan res, 1)
	go func() {
		c, err := l.Accept()
		got <- res{c, err}
	}()
	client, err = net.DialTimeout("unix", l.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	r := <-got
	if r.err != nil {
		t.Fatal(r.err)
	}
	t.Cleanup(func() { _ = r.c.Close() })
	return r.c, client
}

// Свой процесс на обоих концах — «свой» и для сервера, и для клиента.
func TestSamePair(t *testing.T) {
	server, client := pair(t)
	for name, c := range map[string]net.Conn{"сервер видит клиента": server, "клиент видит сервер": client} {
		start := time.Now()
		ok, err := Same(c)
		t.Logf("%s: ok=%v err=%v за %v", name, ok, err, time.Since(start))
		if err != nil || !ok {
			t.Errorf("%s: Same = %v, %v; ждали true, nil", name, ok, err)
		}
	}
}

// Не AF_UNIX (net.Pipe, TCP, nil) и закрытое соединение — ошибка, а не «свой» и не паника.
func TestNotUnix(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	tcp, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	closed, _ := pair(t)
	_ = closed.Close()
	cases := map[string]net.Conn{"net.Pipe": a, "tcp": tcp, "nil": nil, "типизированный nil": (*net.UnixConn)(nil), "закрытое": closed}
	for name, c := range cases {
		ok, err := Same(c)
		if ok || err == nil {
			t.Errorf("%s: Same = %v, %v; ждали false и ошибку", name, ok, err)
		}
	}
}
