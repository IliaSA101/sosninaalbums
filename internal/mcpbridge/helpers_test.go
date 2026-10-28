package mcpbridge

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Режим «тестовый бинарник сам себе посредник»: TestMain видит переменную и ведёт себя как
// `SosninaAlbums --mcp`, но с быстрой пробой и подменой «дома» (иначе пустое окружение нашло бы
// настоящую папку пользователя и настоящий сокет программы).
const asBridgeEnv = "SA_MCPBRIDGE_TEST_AS_BRIDGE"

func TestMain(m *testing.M) {
	if os.Getenv(asBridgeEnv) == "1" {
		home := os.Getenv("SA_MCPBRIDGE_TEST_USERHOME")
		userHome = func() string { return home }
		cfg := defaultConfig(SockPaths(osx.DataName("")))
		cfg.probeEvery = 100 * time.Millisecond
		os.Exit(run(cfg, os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// shortDir — папка, где путь <dir>/mcp/sock короче предела sun_path.
func shortDir(t *testing.T) string {
	t.Helper()
	return shortDirFor(t, "mcp/sock")
}

// shortDirFor — папка, где путь <dir>/<rel> не длиннее 100 знаков (sun_path — 108).
func shortDirFor(t *testing.T, rel string) string {
	t.Helper()
	fits := func(d string) bool { return len(filepath.Join(d, filepath.FromSlash(rel))) <= 100 }
	if d := testkit.TempDir(t); fits(d) {
		return d
	}
	base := os.TempDir()
	if runtime.GOOS != "windows" {
		base = "/tmp"
	}
	d, err := os.MkdirTemp(base, "sa")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	if !fits(d) {
		t.Fatalf("нет достаточно короткой папки для сокета: %d знаков", len(d))
	}
	return d
}

// sockIn создаёт папку mcp (0700) в dir и возвращает путь сокета в ней.
func sockIn(t *testing.T, dir string) string {
	t.Helper()
	mcp := filepath.Join(dir, "mcp")
	if err := os.MkdirAll(mcp, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(mcp, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(mcp, "sock")
}

func newSock(t *testing.T) string { return sockIn(t, shortDir(t)) }

// ---- сторона клиента: stdin/stdout посредника в памяти ----

type harness struct {
	t     *testing.T
	in    *io.PipeWriter
	lines chan []byte
	errw  *syncBuf
	code  chan int
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// testCfg — быстрая проба, пределы поменьше при желании правит тест.
func testCfg(socks ...string) config {
	c := defaultConfig(socks)
	c.probeEvery = 30 * time.Millisecond
	c.replayWait = testkit.Limit(2 * time.Second)
	return c
}

func startBridge(t *testing.T, cfg config) *harness {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	h := &harness{t: t, in: inW, lines: make(chan []byte, 256), errw: &syncBuf{}, code: make(chan int, 1)}
	go func() {
		lr := newLineReader(outR, 16<<20)
		for {
			line, _, _, err := lr.next()
			if err != nil {
				close(h.lines)
				return
			}
			h.lines <- line
		}
	}()
	go func() {
		c := run(cfg, inR, outW, h.errw)
		_ = outW.Close()
		h.code <- c
	}()
	t.Cleanup(func() { _ = inW.Close() })
	return h
}

func (h *harness) send(line string) {
	h.t.Helper()
	if _, err := io.WriteString(h.in, line+"\n"); err != nil {
		h.t.Fatalf("stdin: %v", err)
	}
}

// recv ждёт очередную строку stdout и разбирает её как объект.
func (h *harness) recv() map[string]any {
	h.t.Helper()
	select {
	case l, ok := <-h.lines:
		if !ok {
			h.t.Fatal("stdout закрыт")
		}
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil {
			h.t.Fatalf("в stdout не JSON-объект: %.200q", l)
		}
		return m
	case <-time.After(testkit.Limit(5 * time.Second)):
		h.t.Fatal("не дождались строки в stdout")
	}
	return nil
}

// recvRaw — как recv, но байты строки.
func (h *harness) recvRaw() []byte {
	h.t.Helper()
	select {
	case l, ok := <-h.lines:
		if !ok {
			h.t.Fatal("stdout закрыт")
		}
		return l
	case <-time.After(testkit.Limit(5 * time.Second)):
		h.t.Fatal("не дождались строки в stdout")
	}
	return nil
}

// none: за d в stdout ничего не пришло.
func (h *harness) none(d time.Duration) {
	h.t.Helper()
	select {
	case l, ok := <-h.lines:
		if ok {
			h.t.Fatalf("лишняя строка в stdout: %.200q", l)
		}
	case <-time.After(d):
	}
}

// finish закрывает stdin и ждёт код выхода.
func (h *harness) finish() int {
	h.t.Helper()
	_ = h.in.Close()
	select {
	case c := <-h.code:
		return c
	case <-time.After(testkit.Limit(5 * time.Second)):
		h.t.Fatal("посредник не вышел по EOF stdin")
	}
	return -1
}

// ---- сторона программы: голый сокет ----

type fakeProg struct {
	t     *testing.T
	ln    net.Listener
	conns chan *progConn
}

type progConn struct {
	t  *testing.T
	c  net.Conn
	lr *lineReader
}

func listenProg(t *testing.T, sock string) *fakeProg {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProg{t: t, ln: ln, conns: make(chan *progConn, 16)}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			p.conns <- &progConn{t: t, c: c, lr: newLineReader(c, 16<<20)}
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return p
}

func (p *fakeProg) accept() *progConn {
	p.t.Helper()
	select {
	case c := <-p.conns:
		p.t.Cleanup(func() { _ = c.c.Close() })
		return c
	case <-time.After(testkit.Limit(5 * time.Second)):
		p.t.Fatal("посредник не подключился к сокету")
	}
	return nil
}

// noConn: за d никто не подключился.
func (p *fakeProg) noConn(d time.Duration) {
	p.t.Helper()
	select {
	case <-p.conns:
		p.t.Fatal("посредник подключился, хотя не должен")
	case <-time.After(d):
	}
}

func (c *progConn) read() map[string]any {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	line, _, _, err := c.lr.next()
	if err != nil {
		c.t.Fatalf("чтение сокета: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		c.t.Fatalf("в сокет ушёл не JSON-объект: %.200q", line)
	}
	return m
}

func (c *progConn) write(s string) {
	c.t.Helper()
	if _, err := io.WriteString(c.c, s+"\n"); err != nil {
		c.t.Fatalf("запись в сокет: %v", err)
	}
}

// waitEOF: посредник закрыл сокет.
func (c *progConn) waitEOF() {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	if _, _, _, err := c.lr.next(); err == nil {
		c.t.Fatal("ждали закрытия сокета, пришла строка")
	}
}

// initialize отвечает программой на initialize, который посредник повторяет/пересылает:
// возвращает его params.
func (c *progConn) answerInit() map[string]any {
	c.t.Helper()
	m := c.read()
	if m["method"] != "initialize" {
		c.t.Fatalf("ждали initialize, пришло %v", m["method"])
	}
	params, _ := m["params"].(map[string]any)
	id, _ := json.Marshal(m["id"])
	ver, _ := params["protocolVersion"].(string)
	c.write(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"protocolVersion":"` + ver +
		`","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"fake","version":"1"}}}`)
	return params
}

// toolText достаёт content[0].text из result.
func toolText(t *testing.T, m map[string]any) string {
	t.Helper()
	s := toolText2(m)
	if s == "" {
		t.Fatalf("нет content[0].text в %v", m)
	}
	return s
}

func errMessage(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	s, _ := e["message"].(string)
	return s
}

func contains(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
