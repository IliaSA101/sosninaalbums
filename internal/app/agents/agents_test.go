package agents

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestOffNoSocket: доступ выключен (по умолчанию) — StartIfOn ничего не делает: ни папки mcp,
// ни сокета; Close без запуска безвреден.
func TestOffNoSocket(t *testing.T) {
	r := newRig(t, Options{}, "")
	if err := r.svc.StartIfOn(); err != nil {
		t.Fatal(err)
	}
	if st, e := r.svc.State(); st != StateOff || e != "" {
		t.Fatalf("State = %q %q", st, e)
	}
	if _, err := os.Lstat(filepath.Dir(r.sock)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("папка mcp создана без включения: %v", err)
	}
	if _, err := net.Dial("unix", r.sock); err == nil {
		t.Fatal("сокет слушает при выключенном доступе")
	}
	// Включено в настройках, но папки настроек нет — тоже ничего.
	r.cfg = ""
	if err := r.svc.StartIfOn(); err != nil || len(r.svc.Clients()) != 0 {
		t.Fatalf("без папки настроек: %v", err)
	}
	r.svc.Close()
}

// TestStartOn: включено в config.json — сокет есть, права 0600/0700, клиент SDK проходит
// initialize, виден в списке с именем из clientInfo; повторный Start ничего не ломает.
func TestStartOn(t *testing.T) {
	r := newRig(t, Options{}, "")
	prefs.UpdateAgents(r.cfg, func(a *prefs.Agents) { a.On = true })
	if err := r.svc.StartIfOn(); err != nil {
		t.Fatalf("StartIfOn: %v", err)
	}
	if st, e := r.svc.State(); st != StateOn || e != "" {
		t.Fatalf("State = %q %q", st, e)
	}
	fi, err := os.Lstat(r.sock)
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("сокет: %v %v", fi, err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("права сокета %v, ждали 0600", fi.Mode().Perm())
	}
	if err := r.svc.Start(); err != nil {
		t.Fatalf("повторный Start: %v", err)
	}
	cs := client(t, r.sock, "test-client")
	if err := cs.Ping(context.Background(), nil); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if res := cs.InitializeResult(); res == nil || res.ServerInfo.Name != "sosnina-albums" || res.ServerInfo.Version != "test" {
		t.Fatalf("InitializeResult: %+v", res)
	}
	waitFor(t, "клиент в списке", func() bool { return len(r.svc.Clients()) == 1 })
	if c := r.svc.Clients()[0]; c.Name != "test-client" || time.Since(c.Since) > time.Minute {
		t.Fatalf("клиент: %+v", c)
	}
	_ = cs.Close()
	waitFor(t, "клиент ушёл", func() bool { return len(r.svc.Clients()) == 0 && r.conns() == 0 })
}

// TestDisableCloses: Close убирает сокет и закрывает подключённых; службу можно запустить снова.
func TestDisableCloses(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	cs := client(t, r.sock, "a")
	raw1 := raw(t, r.sock)
	waitFor(t, "два места", func() bool { return r.conns() == 2 })

	start := time.Now()
	r.svc.Close()
	if d := time.Since(start); d > testkit.Limit(1500*time.Millisecond) {
		t.Fatalf("Close занял %v", d)
	}
	if _, err := os.Lstat(r.sock); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("файл сокета остался: %v", err)
	}
	if st, _ := r.svc.State(); st != StateOff {
		t.Fatalf("State = %q", st)
	}
	if !closedWithin(raw1, testkit.Limit(2*time.Second)) {
		t.Fatal("подключение живо после Close")
	}
	done := make(chan struct{})
	go func() { _ = cs.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(testkit.Limit(3 * time.Second)):
		t.Fatal("клиент SDK не заметил Close")
	}
	r.svc.Close() // повтор безвреден

	// Круг выключил → включил: новый запуск работает.
	r.start(t)
	if err := client(t, r.sock, "b").Ping(context.Background(), nil); err != nil {
		t.Fatalf("после перезапуска: %v", err)
	}
}

// TestNoDataDir: папки данных нет (Sock отдаёт "") — отказ словами, на диске ничего.
func TestNoDataDir(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.svc.h.Sock = func() string { return "" }
	err := r.svc.Start()
	if !errors.Is(err, errNoDataDir) {
		t.Fatalf("Start = %v", err)
	}
	if st, e := r.svc.State(); st != StateError || e != errNoDataDir.Error() {
		t.Fatalf("State = %q %q", st, e)
	}
	if got := r.svc.Clients(); got == nil || len(got) != 0 {
		t.Fatalf("Clients = %#v, ждали пустой список", got)
	}
}

// TestChangedPanic: паника обратного вызова App не роняет ни службу, ни горутину SDK; значение
// паники (в нём бывает путь) в log.txt не попадает.
func TestChangedPanic(t *testing.T) {
	buf := captureLog(t)
	r := newRig(t, Options{}, "")
	r.svc.h.Changed = func() { panic("секрет /home/user/альбом") }
	r.start(t)
	if err := client(t, r.sock, "x").Ping(context.Background(), nil); err != nil {
		t.Fatalf("Ping после паники Changed: %v", err)
	}
	waitFor(t, "клиент записан несмотря на панику", func() bool { return len(r.svc.Clients()) == 1 })
	r.svc.Close()
	out := buf.String()
	if !strings.Contains(out, "ПАНИКА в agents") || strings.Contains(out, "секрет") || strings.Contains(out, "/home/user") {
		t.Fatalf("журнал: %q", out)
	}
}

// TestNewNilField: New называет незаполненное поле Host.
func TestNewNilField(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "Host.Sock") {
			t.Fatalf("паника: %v", r)
		}
	}()
	New(Host{CfgDir: func() string { return "" }, Version: func() string { return "" }, Changed: func() {}}, Options{})
}
