package agents

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// shortDir — папка, где путь <dir>/mcp/sock укладывается в maxSockPath: t.TempDir бывает длиннее
// sun_path (108 байт), и Listen отвечает «invalid argument». Сначала папка теста, затем /tmp.
func shortDir(t *testing.T) string {
	t.Helper()
	fits := func(d string) bool { return len(filepath.Join(d, "mcp", "sock")) <= maxSockPath }
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

// rig — служба на сокете в короткой папке и счётчик вызовов Changed.
type rig struct {
	svc     *Service
	sock    string
	cfg     string
	changed atomic.Int32
}

// newRig собирает службу (не запускает). sock "" — по умолчанию <короткая папка>/mcp/sock.
func newRig(t *testing.T, o Options, sock string) *rig {
	t.Helper()
	r := &rig{cfg: testkit.TempDir(t), sock: sock}
	if r.sock == "" {
		r.sock = filepath.Join(shortDir(t), "mcp", "sock")
	}
	r.svc = New(Host{
		CfgDir:  func() string { return r.cfg },
		Version: func() string { return "test" },
		Sock:    func() string { return r.sock },
		Changed: func() { r.changed.Add(1) },
	}, o)
	t.Cleanup(r.svc.Close)
	return r
}

// start запускает службу; отказ — провал теста.
func (r *rig) start(t *testing.T) {
	t.Helper()
	if err := r.svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

// conns — число занятых мест.
func (r *rig) conns() int {
	r.svc.mu.Lock()
	run := r.svc.run
	r.svc.mu.Unlock()
	if run == nil {
		return 0
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	return len(run.conns)
}

// waitFor ждёт условие до testkit.Limit(5 с).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	end := time.Now().Add(testkit.Limit(5 * time.Second))
	for !cond() {
		if time.Now().After(end) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// client подключает клиента SDK к сокету.
func client(t *testing.T, sock, name string) *mcp.ClientSession {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(10*time.Second))
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: name, Version: "1"}, nil).
		Connect(ctx, &mcp.IOTransport{Reader: c, Writer: c}, nil)
	if err != nil {
		t.Fatalf("клиент: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// raw — голое подключение к сокету.
func raw(t *testing.T, sock string) net.Conn {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

const (
	initLine  = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"raw","version":"1"}}}` + "\n"
	readyLine = `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
)

// handshake — initialize и initialized голым подключением; ответ initialize прочитан.
func handshake(t *testing.T, c net.Conn) *bufio.Reader {
	t.Helper()
	br := bufio.NewReader(c)
	_ = c.SetDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	if _, err := c.Write([]byte(initLine + readyLine)); err != nil {
		t.Fatal(err)
	}
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("ответ initialize: %v", err)
	}
	_ = c.SetDeadline(time.Time{})
	return br
}

// closedWithin — подключение закрыто (чтение даёт ошибку, а не данные) за d.
func closedWithin(c net.Conn, d time.Duration) bool {
	_ = c.SetReadDeadline(time.Now().Add(d))
	buf := make([]byte, 1)
	for {
		if _, err := c.Read(buf); err != nil {
			var ne net.Error
			return !errors.As(err, &ne) || !ne.Timeout()
		}
	}
}

// addTool — безвредный инструмент для проверок канала: тело получает контекст запроса.
func addTool(srv *mcp.Server, name string, f func(ctx context.Context) string) {
	mcp.AddTool(srv, &mcp.Tool{Name: name}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: f(ctx)}}}, nil, nil
	})
}
