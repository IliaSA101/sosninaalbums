package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type statusOut struct {
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

type albumOut struct {
	Spreads int `json:"spreads"`
}

// newServer — минимальный сервер: program_status (в заглушке посредника тоже есть, но с другим текстом) и album_info.
func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "sosnina-albums", Version: "spike"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "program_status", Description: "Состояние программы"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, statusOut, error) {
			out := statusOut{Status: "running", PID: os.Getpid()}
			text := fmt.Sprintf("program_status: running pid=%d", out.PID)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "album_info", Description: "Сколько разворотов"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, albumOut, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "spreads=12"}}}, albumOut{Spreads: 12}, nil
		})
	return s
}

// program держит слушатель и подключённые сессии: Close закрывает всех и удаляет файл сокета.
type program struct {
	srv      *mcp.Server
	ln       net.Listener
	mu       sync.Mutex
	sessions map[*mcp.ServerSession]bool
}

func (p *program) serve(ctx context.Context) {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handle(ctx, c)
	}
}

func (p *program) handle(ctx context.Context, c net.Conn) {
	ss, err := p.srv.Connect(ctx, &mcp.IOTransport{Reader: c, Writer: c}, nil)
	if err != nil {
		logf("serve: connect: %v", err)
		_ = c.Close()
		return
	}
	p.mu.Lock()
	p.sessions[ss] = true
	p.mu.Unlock()
	logf("serve: сессия открыта")
	_ = ss.Wait()
	p.mu.Lock()
	delete(p.sessions, ss)
	p.mu.Unlock()
	ver, client := "-", "-"
	if ip := ss.InitializeParams(); ip != nil {
		ver = ip.ProtocolVersion
		if ip.ClientInfo != nil {
			client = ip.ClientInfo.Name
		}
	}
	logf("serve: сессия закрыта client=%s version=%s", client, ver)
}

// Close закрывает слушатель (файл сокета должен исчезнуть — С2 (д)) и сессии.
func (p *program) Close() error {
	err := p.ln.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for ss := range p.sessions {
		_ = ss.Close()
	}
	return err
}

func startProgram(ctx context.Context, path string, clean bool) (*program, bool, error) {
	ln, replaced, err := listenSock(path, clean)
	if err != nil {
		return nil, false, err
	}
	p := &program{srv: newServer(), ln: ln, sessions: map[*mcp.ServerSession]bool{}}
	go p.serve(ctx)
	return p, replaced, nil
}

// runServe — слушатель до --ttl секунд (страховка от висящего процесса) или до сигнала.
// --ready F — после успешного Listen пишет в F «pid sock replaced=...».
func runServe(o opts) int {
	path := sockPath(o)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, replaced, err := startProgram(ctx, path, o["no-clean"] == "")
	if err != nil {
		logf("serve: listen %s: %v", path, err)
		if f := o["ready"]; f != "" {
			_ = os.WriteFile(f+".fail", []byte(err.Error()), 0o666)
		}
		return 2
	}
	logf("serve: слушаю %s (замена старого файла: %v)", path, replaced)
	if f := o["ready"]; f != "" {
		_ = os.MkdirAll(filepath.Dir(f), 0o777)
		_ = os.WriteFile(f, []byte(fmt.Sprintf("%d %s replaced=%v", os.Getpid(), path, replaced)), 0o666)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-time.After(time.Duration(o.num("ttl", 120)) * time.Second):
	}
	_ = p.Close()
	logf("serve: закрыт")
	return 0
}
