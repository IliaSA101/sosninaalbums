package mcpbridge

import (
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// refusingProg — слушатель, который каждому подключению отвечает «слишком много» и закрывает.
func refusingProg(t *testing.T, sock string) *atomic.Int32 {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var n atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n.Add(1)
			_, _ = c.Write([]byte(tooManyLine + "\n"))
			_ = c.Close()
		}
	}()
	return &n
}

// TestBridgeProbeRate: проба — не чаще раза в probeEvery, а после отказа программы пауза растёт.
// 20 сообщений клиента за 300 мс при probeEvery 300 мс — не больше 2 подключений (решение 52).
func TestBridgeProbeRate(t *testing.T) {
	sock := newSock(t)
	n := refusingProg(t, sock)
	cfg := testCfg(sock)
	cfg.probeEvery = 300 * time.Millisecond
	h := startBridge(t, cfg)
	for range 20 {
		h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
		h.recv()
		time.Sleep(15 * time.Millisecond)
	}
	if got := n.Load(); got > 2 {
		t.Errorf("подключений за окно: %d, ждали не больше 2", got)
	}
	if h.finish() != 0 {
		t.Error("код выхода не 0")
	}
}

// TestBridgeProbeBackoff: пока программа стойко отказывает (много подключений), пауза проб растёт
// вдвое каждый раз, а в stderr — одна строка о недоступности, а не по строке на пробу.
func TestBridgeProbeBackoff(t *testing.T) {
	sock := newSock(t)
	n := refusingProg(t, sock)
	cfg := testCfg(sock)
	cfg.probeEvery = 20 * time.Millisecond
	h := startBridge(t, cfg)
	time.Sleep(700 * time.Millisecond)
	if got := n.Load(); got < 2 || got > 7 {
		t.Errorf("подключений за 700 мс: %d (без роста паузы было бы ~35)", got)
	}
	if h.finish() != 0 {
		t.Error("код выхода не 0")
	}
	if c := strings.Count(h.errw.String(), "недоступна"); c != 1 {
		t.Errorf("строк о недоступности в stderr: %d, ждали 1: %q", c, h.errw.String())
	}
}

// TestProbeInterval: пауза растёт вдвое от каждого отказа и упирается в потолок.
func TestProbeInterval(t *testing.T) {
	b := &bridge{cfg: config{probeEvery: 2 * time.Second}}
	want := []time.Duration{2, 4, 8, 16, 30, 30}
	for i, w := range want {
		b.refused = i
		if got := b.probeInterval(); got != w*time.Second {
			t.Errorf("отказов %d: %v, ждали %vs", i, got, w)
		}
	}
}
