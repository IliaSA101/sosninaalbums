package agents

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSocketPerms: папка mcp, созданная с чужими правами, приводится к 0700, сокет — 0600.
func TestSocketPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права Unix; на Windows: TestSocketDACL (win-qa.yml)")
	}
	r := newRig(t, Options{}, "")
	dir := filepath.Dir(r.sock)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { // umask мог урезать
		t.Fatal(err)
	}
	r.start(t)
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("папка: %v %v", fi, err)
	}
	if fi, err := os.Lstat(r.sock); err != nil || fi.Mode().Perm() != 0o600 || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("сокет: %v %v", fi, err)
	}
}

// TestSocketPermsLink: ссылка на месте mcp — отказ, Chmod по ней не идёт, цель не тронута.
func TestSocketPermsLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права Unix; на Windows: TestSocketDACL (win-qa.yml)")
	}
	r := newRig(t, Options{}, "")
	base := filepath.Dir(filepath.Dir(r.sock))
	target := filepath.Join(base, "чужая")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Dir(r.sock)); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Start(); !errors.Is(err, errNoPerm) {
		t.Fatalf("Start = %v, ждали errNoPerm", err)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o755 {
		t.Fatalf("Chmod ушёл по ссылке: %v", fi.Mode().Perm())
	}
	if _, err := os.Lstat(filepath.Join(target, "sock")); err == nil {
		t.Fatal("сокет создан по ссылке")
	}
	if st, e := r.svc.State(); st != StateError || e == "" {
		t.Fatalf("State = %q %q", st, e)
	}
}

// TestSocketPermsFile: файл на месте папки mcp — отказ, файл цел.
func TestSocketPermsFile(t *testing.T) {
	r := newRig(t, Options{}, "")
	dir := filepath.Dir(r.sock)
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Start(); !errors.Is(err, errOpen) {
		t.Fatalf("Start = %v, ждали errOpen", err)
	}
	if b, _ := os.ReadFile(dir); string(b) != "x" {
		t.Fatal("файл на месте папки изменён")
	}
}

// TestSocketStale: след упавшего запуска (сокет без ответчика) заменяется.
func TestSocketStale(t *testing.T) {
	r := newRig(t, Options{}, "")
	if err := os.MkdirAll(filepath.Dir(r.sock), 0o700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", r.sock)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false) // как после kill -9
	_ = ln.Close()
	if fi, err := os.Lstat(r.sock); err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("след не создан: %v %v", fi, err)
	}
	r.start(t)
	client(t, r.sock, "после-падения")
}

// TestSocketStaleForeign: на месте сокета файл, каталог, ссылка — чужое: отказ, всё цело.
func TestSocketStaleForeign(t *testing.T) {
	for name, mk := range map[string]func(sock string) error{
		"обычный файл": func(s string) error { return os.WriteFile(s, []byte("мои данные"), 0o600) },
		"каталог":      func(s string) error { return os.Mkdir(s, 0o700) },
		"ссылка": func(s string) error {
			if runtime.GOOS == "windows" {
				return errSkip
			}
			return os.Symlink(filepath.Join(filepath.Dir(s), "нет-такого"), s)
		},
	} {
		r := newRig(t, Options{}, "")
		if err := os.MkdirAll(filepath.Dir(r.sock), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := mk(r.sock); err != nil {
			if err == errSkip {
				continue
			}
			t.Fatal(err)
		}
		before, _ := os.Lstat(r.sock)
		if err := r.svc.Start(); !errors.Is(err, errForeign) {
			t.Fatalf("%s: Start = %v, ждали errForeign", name, err)
		}
		after, err := os.Lstat(r.sock)
		if err != nil || after.Mode() != before.Mode() || after.Size() != before.Size() {
			t.Fatalf("%s: чужое тронуто: %v %v", name, after, err)
		}
	}
}

// TestSocketStaleLive: живой ответчик — отказ, его сокет не удалён.
func TestSocketStaleLive(t *testing.T) {
	a := newRig(t, Options{}, "")
	a.start(t)
	b := newRig(t, Options{}, a.sock)
	if err := b.svc.Start(); !errors.Is(err, errBusy) {
		t.Fatalf("вторая копия: Start = %v, ждали errBusy", err)
	}
	if _, err := os.Lstat(a.sock); err != nil {
		t.Fatalf("сокет живого удалён: %v", err)
	}
	client(t, a.sock, "жив")
}

var errSkip = errors.New("пропуск")

// pathOfLen — путь сокета ровно в n байт: <base>/<d…>/sock.
func pathOfLen(base string, n int) string {
	pad := n - len(filepath.Join(base, "x", "sock")) + 1
	return filepath.Join(base, strings.Repeat("d", pad), "sock")
}

// TestSockPathTooLong: длиннее предела — отказ до Listen, ничего на диске; ровно предел — работает.
func TestSockPathTooLong(t *testing.T) {
	base := shortDir(t)
	p := pathOfLen(base, maxSockPath)
	if len(p) != maxSockPath {
		t.Fatalf("помощник: %d", len(p))
	}
	newRig(t, Options{}, p).start(t)
	client(t, p, "на-пределе")

	p = pathOfLen(base, maxSockPath+1)
	r := newRig(t, Options{}, p)
	if err := r.svc.Start(); !errors.Is(err, errPathLong) {
		t.Fatalf("Start = %v, ждали errPathLong", err)
	}
	if st, e := r.svc.State(); st != StateError || e != errPathLong.Error() {
		t.Fatalf("State = %q %q", st, e)
	}
	if _, err := os.Lstat(filepath.Dir(p)); err == nil {
		t.Fatal("папка создана до проверки длины")
	}
}

// TestSockPathAbstract: абстрактное «@…» — отказ, абстрактный сокет не появляется.
func TestSockPathAbstract(t *testing.T) {
	const name = "@sosnina-albums-test-abstract"
	r := newRig(t, Options{}, name)
	if err := r.svc.Start(); !errors.Is(err, errBadPath) {
		t.Fatalf("Start = %v, ждали errBadPath", err)
	}
	if runtime.GOOS == "linux" {
		if c, err := net.Dial("unix", name); err == nil {
			_ = c.Close()
			t.Fatal("абстрактный сокет слушает")
		}
	}
}

// TestSockPathRelative: путь от текущей папки — отказ, папка рядом не создаётся.
func TestSockPathRelative(t *testing.T) {
	r := newRig(t, Options{}, filepath.Join("mcp-rel-test", "sock"))
	if err := r.svc.Start(); !errors.Is(err, errBadPath) {
		t.Fatalf("Start = %v, ждали errBadPath", err)
	}
	if _, err := os.Lstat("mcp-rel-test"); err == nil {
		_ = os.RemoveAll("mcp-rel-test")
		t.Fatal("папка создана от текущей папки процесса")
	}
}
