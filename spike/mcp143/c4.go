package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// С4: клиент Go SDK выбранной ревизии протокола ходит через посредника (`--mcp` этого же exe или --bridge).
// Сценарии: live — программа уже слушает; late — клиент начал с заглушкой, программа открылась позже;
// restart — программа убита посреди сессии и открыта снова. Проверки strict=false печатаются как INFO.

type c4 struct {
	rev, scen string
	dir, sock string
	changed   atomic.Int32
	bad       int
	prefix    string
}

func (c *c4) check(name string, ok bool, strict bool, format string, a ...any) {
	st := "OK"
	if !ok {
		if strict {
			st = "FAIL"
			c.bad++
		} else {
			st = "INFO"
		}
	}
	res(c.prefix+"/"+name, st, format, a...)
}

func (c *c4) startServer() (*exec.Cmd, error) {
	ready := filepath.Join(c.dir, "ready")
	_ = os.Remove(ready)
	_ = os.Remove(ready + ".fail")
	cmd := exec.Command(os.Args[0], "--serve", "--sock", c.sock, "--ttl", "90", "--ready", ready, "--out", outDir)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(ready); err == nil {
			return cmd, nil
		}
		if b, err := os.ReadFile(ready + ".fail"); err == nil {
			_ = cmd.Process.Kill()
			return nil, errors.New(string(b))
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	return nil, os.ErrDeadlineExceeded
}

func (c *c4) waitChanged(n int32, d time.Duration) bool {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if c.changed.Load() >= n {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func (c *c4) tools(ctx context.Context, cs *mcp.ClientSession) (int, string) {
	tl, err := cs.ListTools(ctx, nil)
	if err != nil {
		return -1, err.Error()
	}
	var n []string
	for _, t := range tl.Tools {
		n = append(n, t.Name)
	}
	return len(tl.Tools), strings.Join(n, ",")
}

func (c *c4) status(ctx context.Context, cs *mcp.ClientSession) string {
	r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "program_status"})
	if err != nil {
		return "err: " + err.Error()
	}
	for _, x := range r.Content {
		if t, ok := x.(*mcp.TextContent); ok {
			return t.Text
		}
	}
	return "нет текста"
}

func runC4(o opts) int {
	c := &c4{rev: o.str("rev", "2025-11-25"), scen: o.str("scenario", "live")}
	c.prefix = "c4go-" + c.rev + "/" + c.scen
	c.dir = filepath.Join(outDir, "c4-"+c.rev+"-"+c.scen)
	_ = os.MkdirAll(c.dir, 0o777)
	// Сокет — в коротком пути: sun_path всего 104–108 байт, а outDir на раннере глубокий
	sdir := filepath.Join(o.str("sockdir", os.TempDir()), "s143-"+c.rev[2:4]+c.rev[5:7]+"-"+c.scen)
	_ = os.RemoveAll(sdir)
	c.sock = filepath.Join(sdir, "sock")
	defer os.RemoveAll(sdir)
	bridgeExe := o.str("bridge", os.Args[0])
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var srv *exec.Cmd
	var err error
	kill := func() {
		if srv != nil && srv.Process != nil {
			_ = srv.Process.Kill()
			_, _ = srv.Process.Wait()
			srv = nil
		}
	}
	defer kill()
	start := func() bool {
		srv, err = c.startServer()
		if err != nil {
			c.check("server-start", false, true, "%v", err)
			return false
		}
		return true
	}
	if c.scen != "late" && !start() {
		return 1
	}

	cmd := exec.Command(bridgeExe, "--mcp", "--out", outDir)
	cmd.Env = append(os.Environ(), "SPIKE_SOCK="+c.sock, "SPIKE_PROBE_MS=200")
	client := mcp.NewClient(&mcp.Implementation{Name: "c4-" + c.rev, Version: "1"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) { c.changed.Add(1) },
	})
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, &mcp.ClientSessionOptions{ProtocolVersion: c.rev})
	if err != nil {
		c.check("connect", false, true, "%v", err)
		return 1
	}
	ver := cs.InitializeResult().ProtocolVersion
	c.check("connect", true, true, "согласована ревизия %s", ver)

	switch c.scen {
	case "live":
		n, names := c.tools(ctx, cs)
		c.check("tools-live", n == 2, true, "%d: %s", n, names)
		st := c.status(ctx, cs)
		c.check("call-live", strings.Contains(st, "running pid="), true, "%s", st)
		// Ревизия 2026-07-28 при живой программе идёт без initialize (server/discover) — сквозь посредника
		c.check("revision-live", ver == c.rev, true, "запрошена %s, согласована %s", c.rev, ver)
	case "late":
		n, _ := c.tools(ctx, cs)
		c.check("tools-stub", n == 1, true, "%d", n)
		st := c.status(ctx, cs)
		c.check("call-stub", strings.Contains(st, "не запущена"), true, "%s", st)
		if !start() {
			break
		}
		c.check("list-changed", c.waitChanged(1, 10*time.Second), true, "получено %d", c.changed.Load())
		n, names := c.tools(ctx, cs)
		c.check("tools-after-replay", n == 2, true, "%d: %s", n, names)
		st = c.status(ctx, cs)
		c.check("call-after-replay", strings.Contains(st, "running pid="), true, "%s", st)
	case "restart":
		kill()
		c.check("list-changed-down", c.waitChanged(1, 10*time.Second), false, "получено %d", c.changed.Load())
		n, _ := c.tools(ctx, cs)
		c.check("tools-stub-after-kill", n == 1, false, "%d", n)
		if !start() {
			break
		}
		c.check("list-changed-up", c.waitChanged(2, 10*time.Second), false, "получено %d", c.changed.Load())
		n, names := c.tools(ctx, cs)
		c.check("tools-after-restart", n == 2, false, "%d: %s", n, names)
		st := c.status(ctx, cs)
		c.check("call-after-restart", strings.Contains(st, "running pid="), false, "%s", st)
	}

	t0 := time.Now()
	_ = cs.Close()
	d := time.Since(t0)
	c.check("close-time", d <= time.Second, true, "Close занял %v", d.Round(time.Millisecond))
	if c.bad > 0 {
		return 1
	}
	return 0
}
