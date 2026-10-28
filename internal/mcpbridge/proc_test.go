package mcpbridge

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func skipNoUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("подпроцессные проверки с HOME — Unix; Windows гоняет winqa")
	}
}

// tsEnv — окружение, которое передаёт дочернему серверу TS SDK (getDefaultEnvironment) на
// Unix: ни XDG_CONFIG_HOME, ни SA_STAND. Плюс переключатель тестового режима.
func tsEnv(home string) []string {
	return []string{
		"HOME=" + home, "LOGNAME=tester", "PATH=" + os.Getenv("PATH"), "SHELL=/bin/sh", "TERM=dumb", "USER=tester",
		asBridgeEnv + "=1",
	}
}

// homeSock — путь сокета в тестовом «доме»: <home>/.config/SosninaAlbums/mcp/sock; папка mcp 0700.
func homeSock(t *testing.T, home string) string {
	t.Helper()
	return sockIn(t, filepath.Join(home, ".config", "SosninaAlbums"))
}

type proc struct {
	t     *testing.T
	cmd   *exec.Cmd
	in    io.WriteCloser
	lines chan []byte
	eof   chan struct{} // stdout дочитан до конца
	errw  *syncBuf
}

// startProc запускает тестовый бинарник как `--mcp` с заданным окружением и папкой.
func startProc(t *testing.T, env []string, dir string) *proc {
	t.Helper()
	cmd := exec.Command(testkit.SelfExe(t), "--mcp")
	cmd.Env = env
	cmd.Dir = dir
	p := &proc{t: t, cmd: cmd, lines: make(chan []byte, 1024), eof: make(chan struct{}), errw: &syncBuf{}}
	cmd.Stderr = p.errw
	var err error
	if p.in, err = cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		lr := newLineReader(out, 16<<20)
		for {
			l, _, _, err := lr.next()
			if err != nil {
				close(p.lines)
				close(p.eof)
				return
			}
			p.lines <- l
		}
	}()
	t.Cleanup(func() {
		_ = p.in.Close()
		_ = cmd.Process.Kill() // свой процесс по его PID
		_ = cmd.Wait()
	})
	return p
}

func (p *proc) send(line string) {
	p.t.Helper()
	if _, err := io.WriteString(p.in, line+"\n"); err != nil {
		p.t.Fatalf("stdin подпроцесса: %v", err)
	}
}

func (p *proc) recv() map[string]any {
	p.t.Helper()
	select {
	case l, ok := <-p.lines:
		if !ok {
			p.t.Fatalf("stdout закрыт; stderr: %s", p.errw.String())
		}
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil {
			p.t.Fatalf("в stdout не JSON-объект: %.200q", l)
		}
		return m
	case <-time.After(testkit.Limit(10 * time.Second)):
		p.t.Fatalf("нет строки в stdout; stderr: %s", p.errw.String())
	}
	return nil
}

// wait закрывает stdin и ждёт выхода; возвращает код.
func (p *proc) wait() int {
	p.t.Helper()
	_ = p.in.Close()
	select { // Wait закрывает канал вывода: сначала дочитать stdout
	case <-p.eof:
	case <-time.After(testkit.Limit(10 * time.Second)):
		p.t.Fatal("подпроцесс не вышел по EOF stdin")
	}
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return -1
	case <-time.After(testkit.Limit(10 * time.Second)):
		p.t.Fatal("подпроцесс не вышел по EOF stdin")
	}
	return -1
}

// sdkProgram — «программа» на go-sdk: настоящий сервер MCP на сокете.
type sdkProgram struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func startSDKProgram(t *testing.T, sock string) *sdkProgram {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-sa", Version: "1"}, nil)
	type none struct{}
	type spreads struct {
		Spreads int `json:"spreads"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "count_spreads", Description: "число разворотов"},
		func(context.Context, *mcp.CallToolRequest, none) (*mcp.CallToolResult, spreads, error) {
			return nil, spreads{Spreads: 7}, nil
		})
	p := &sdkProgram{ln: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			p.conns = append(p.conns, c)
			p.mu.Unlock()
			if _, err := srv.Connect(context.Background(), &mcp.IOTransport{Reader: c, Writer: c}, nil); err != nil {
				_ = c.Close()
			}
		}
	}()
	t.Cleanup(p.stop)
	return p
}

func (p *sdkProgram) stop() {
	_ = p.ln.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = nil
}

// TestBridgeE2E: тестовый бинарник сам себе посредник (TestMain + переменная окружения), клиент —
// go-sdk через mcp.CommandTransport, программа — go-sdk на сокете. Заглушка → запуск программы
// без перезапуска клиента (list_changed, повтор initialize) → закрытие программы → запуск снова.
func TestBridgeE2E(t *testing.T) {
	skipNoUnix(t)
	home := shortDirFor(t, ".config/SosninaAlbums/mcp/sock")
	sock := homeSock(t, home)

	cmd := exec.Command(testkit.SelfExe(t), "--mcp")
	cmd.Env = tsEnv(home)
	cmd.Dir = testkit.TempDir(t)
	changed := make(chan struct{}, 16)
	cl := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "1"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			select {
			case changed <- struct{}{}:
			default:
			}
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(60*time.Second))
	defer cancel()
	cs, err := cl.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("Connect (заглушка): %v", err)
	}
	defer func() { _ = cs.Close() }()

	toolNames := func() []string {
		t.Helper()
		res, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		var names []string
		for _, tl := range res.Tools {
			names = append(names, tl.Name)
		}
		return names
	}
	waitChanged := func(what string) {
		t.Helper()
		select {
		case <-changed:
		case <-time.After(testkit.Limit(15 * time.Second)):
			t.Fatalf("нет list_changed: %s", what)
		}
	}

	if got := toolNames(); len(got) != 1 || got[0] != "program_status" {
		t.Fatalf("заглушка: %v", got)
	}
	st, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "program_status"})
	if err != nil || len(st.Content) == 0 || !strings.Contains(st.Content[0].(*mcp.TextContent).Text, "Sosnina Albums") {
		t.Fatalf("program_status: %v %v", st, err)
	}

	for round := range 2 { // программа приходит и уходит дважды
		prog := startSDKProgram(t, sock)
		waitChanged("программа запущена, круг " + itoa(round))
		if got := toolNames(); len(got) != 1 || got[0] != "count_spreads" {
			t.Fatalf("круг %d, программа: %v", round, got)
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "count_spreads"})
		if err != nil || res.IsError {
			t.Fatalf("count_spreads: %v %v", res, err)
		}
		sc, _ := json.Marshal(res.StructuredContent)
		if string(sc) != `{"spreads":7}` {
			t.Errorf("structuredContent: %s", sc)
		}
		prog.stop()
		waitChanged("программа закрыта, круг " + itoa(round))
		if got := toolNames(); len(got) != 1 || got[0] != "program_status" {
			t.Fatalf("круг %d, снова заглушка: %v", round, got)
		}
	}
}

// TestBridgeMinimalEnv: окружение как у TS SDK (Claude Desktop, Inspector) — без XDG_CONFIG_HOME
// и SA_STAND; и с чужим XDG_CONFIG_HOME у посредника, тогда как программа пишет в ~/.config:
// посредник находит сокет по списку путей (design §4.1).
func TestBridgeMinimalEnv(t *testing.T) {
	skipNoUnix(t)
	for name, extra := range map[string][]string{
		"как у TS SDK":          nil,
		"XDG_CONFIG_HOME чужой": {"XDG_CONFIG_HOME=/nonexistent-sa-xdg"},
	} {
		t.Run(name, func(t *testing.T) {
			home := shortDirFor(t, ".config/SosninaAlbums/mcp/sock")
			p := listenProg(t, homeSock(t, home))
			pr := startProc(t, append(tsEnv(home), extra...), testkit.TempDir(t))
			c := p.accept()
			pr.send(initLine)
			c.answerInit()
			if r := pr.recv(); r["result"] == nil {
				t.Fatalf("initialize: %v", r)
			}
			if code := pr.wait(); code != 0 {
				t.Errorf("код выхода %d", code)
			}
		})
	}
}

// TestBridgeCwdClean: ни дома, ни папки настроек, пустая текущая папка — пути от текущей папки
// не бывает: в ней ничего не создаётся, ответ — «не найдена папка данных».
func TestBridgeCwdClean(t *testing.T) {
	skipNoUnix(t)
	dir := testkit.TempDir(t)
	pr := startProc(t, []string{"PATH=" + os.Getenv("PATH"), asBridgeEnv + "=1"}, dir)
	pr.send(initLine)
	pr.recv()
	pr.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"program_status"}}`)
	if got := toolText(t, pr.recv()); got != textNoDir {
		t.Errorf("текст: %q", got)
	}
	if code := pr.wait(); code != 0 {
		t.Errorf("код выхода %d", code)
	}
	if es, err := os.ReadDir(dir); err != nil || len(es) != 0 {
		t.Errorf("в текущей папке появились файлы: %v %v", es, err)
	}
}

// TestBridgeStdoutClean: что бы ни пришло в stdin (мусор, нули, огромная строка, обрыв в конце),
// каждая строка stdout — объект JSON-RPC 2.0; тексты ошибок о причинах — без путей; stderr отдельно.
func TestBridgeStdoutClean(t *testing.T) {
	skipNoUnix(t)
	home := shortDirFor(t, ".config/SosninaAlbums/mcp/sock")
	pr := startProc(t, tsEnv(home), testkit.TempDir(t))
	in := []string{
		initLine,
		"\x00\xff\xfe{",
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`[]`,
		`{"jsonrpc":"2.0","id":3,"method":"ping","params":{"pad":"` + strings.Repeat("x", 2<<20) + `"}}`,
		``,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
		strings.Repeat("{", 100000),
	}
	for _, l := range in {
		pr.send(l)
	}
	_, _ = io.WriteString(pr.in, `{"jsonrpc":"2.0","id":5,"method":"pin`) // обрыв без перевода строки
	if code := pr.wait(); code != 0 {
		t.Errorf("код выхода %d", code)
	}
	n := 0
	for l := range pr.lines {
		n++
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil || m["jsonrpc"] != "2.0" {
			t.Errorf("строка stdout не JSON-RPC: %.100q", l)
		}
		if contains(string(l), home, "/tmp") {
			t.Errorf("путь в stdout: %.200q", l)
		}
	}
	if n < 6 {
		t.Errorf("ответов %d, ждали не меньше 6", n)
	}
	if contains(pr.errw.String(), home, "/tmp", ".config") {
		t.Errorf("путь в stderr: %q", pr.errw.String())
	}
}
