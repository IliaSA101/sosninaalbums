package mcpbridge

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/peercred"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const statusCall = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`

// closedSilently: посредник закрыл подключение, не записав ни байта (вместо EOF — любая запись
// или таймаут — провал).
func closedSilently(t *testing.T, c *progConn) {
	t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	buf := make([]byte, 64)
	n, err := c.c.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("ждали закрытия без байтов, прочитано %d, ошибка %v", n, err)
	}
}

// TestBridgeRefusesForeignPeer: слушатель — чужая учётная запись, ошибка запроса к ОС или «ОС не
// умеет» — посредник закрывает подключение до первого байта, агенту — заглушка textUnsafe (для «ОС не
// умеет» — textNoCheck), в stderr одна строка без пути; когда собеседник стал своим, подключается.
func TestBridgeRefusesForeignPeer(t *testing.T) {
	cases := []struct {
		name string
		res  func() (bool, error)
		text string // текст заглушки агенту
		note string
	}{
		{"чужой", func() (bool, error) { return false, nil }, textUnsafe, logPeerForeign},
		{"ошибка ОС", func() (bool, error) { return false, errors.New("boom /secret/path") }, textUnsafe, logPeerFailed},
		// «выключите и включите» тут не поможет: свой текст про обновление Windows (148-11)
		{"не поддерживается", func() (bool, error) { return false, peercred.ErrUnsupported }, textNoCheck, logPeerUnsupported},
		{"не поддерживается, обёрнута", func() (bool, error) { return false, fmt.Errorf("сверка: %w", peercred.ErrUnsupported) }, textNoCheck, logPeerUnsupported},
		{"ошибка при ok=true", func() (bool, error) { return true, errors.New("boom") }, textUnsafe, logPeerFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sock := newSock(t)
			p := listenProg(t, sock)
			var mine atomic.Bool
			cfg := testCfg(sock)
			cfg.peer = func(net.Conn) (bool, error) {
				if mine.Load() {
					return true, nil
				}
				return tc.res()
			}
			h := startBridge(t, cfg)
			h.send(statusCall)
			if got := toolText(t, h.recv()); got != tc.text {
				t.Errorf("текст: %q, ждали %q", got, tc.text)
			}
			closedSilently(t, p.accept())
			time.Sleep(150 * time.Millisecond) // ещё пробы: строка в stderr всё равно одна
			if got := strings.Count(h.errw.String(), tc.note); got != 1 {
				t.Errorf("строк %q в stderr: %d: %q", tc.note, got, h.errw.String())
			}
			if strings.Contains(h.errw.String(), sock) || strings.Contains(h.errw.String(), "/secret") {
				t.Errorf("в stderr путь или текст ошибки ОС: %q", h.errw.String())
			}
			mine.Store(true) // слушатель «стал своим»
			c := liveConn(t, p)
			h.send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
			if m := c.read(); m["method"] != "ping" {
				t.Errorf("в программу ушло %v", m)
			}
		})
	}
}

// liveConn ждёт подключение, которое посредник не закрыл: прежние, отклонённые, уже закрыты.
func liveConn(t *testing.T, p *fakeProg) *progConn {
	t.Helper()
	deadline := time.After(testkit.Limit(5 * time.Second))
	for {
		select {
		case c := <-p.conns:
			t.Cleanup(func() { _ = c.c.Close() })
			_ = c.c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := c.c.Read(make([]byte, 1)); errors.Is(err, io.EOF) {
				continue // отклонённое
			}
			return c
		case <-deadline:
			t.Fatal("не подключился после того, как собеседник стал своим")
		}
	}
}

// countingProg — слушатель, считающий подключения и закрывающий их, ничего не отвечая.
func countingProg(t *testing.T, sock string) *atomic.Int32 {
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
			_ = c.Close()
		}
	}()
	return &n
}

// TestBridgeForeignPeerBackoff: пока слушает чужой, пауза проб растёт вдвое (как при отказе
// программы), а в stderr остаётся одна строка.
func TestBridgeForeignPeerBackoff(t *testing.T) {
	sock := newSock(t)
	n := countingProg(t, sock)
	cfg := testCfg(sock)
	cfg.probeEvery = 20 * time.Millisecond
	cfg.peer = func(net.Conn) (bool, error) { return false, nil }
	h := startBridge(t, cfg)
	time.Sleep(700 * time.Millisecond)
	if got := n.Load(); got < 2 || got > 7 {
		t.Errorf("подключений за 700 мс: %d (без роста паузы было бы ~35)", got)
	}
	if h.finish() != 0 {
		t.Error("код выхода не 0")
	}
	if c := strings.Count(h.errw.String(), logPeerForeign); c != 1 {
		t.Errorf("строк в stderr: %d: %q", c, h.errw.String())
	}
}

// TestBridgeForeignPeerSecondPath: на первом пути чужой, на втором свой — подключается ко второму;
// первому не уходит ни байта (несколько путей, а не один).
func TestBridgeForeignPeerSecondPath(t *testing.T) {
	foreign := newSock(t)
	good := newSock(t)
	pf := listenProg(t, foreign)
	pg := listenProg(t, good)
	cfg := testCfg(foreign, good)
	cfg.peer = func(c net.Conn) (bool, error) { return c.RemoteAddr().String() == good, nil }
	h := startBridge(t, cfg)
	closedSilently(t, pf.accept())
	c := pg.accept()
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if m := c.read(); m["method"] != "ping" {
		t.Errorf("в программу ушло %v", m)
	}
}

// TestBridgeNoPeerFunc: без функции сверки (нулевой config) подключение не принимается.
func TestBridgeNoPeerFunc(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	cfg := testCfg(sock)
	cfg.peer = nil
	h := startBridge(t, cfg)
	h.send(statusCall)
	if got := toolText(t, h.recv()); got != textUnsafe {
		t.Errorf("текст: %q", got)
	}
	closedSilently(t, p.accept())
}

// TestBridgeElevatedProgram: сверка нашла процесс программы, а токен не открылся (программа «от
// администратора», Windows, Г-в) — отдельный текст заглушки, подключение закрыто без байтов.
// Подмена peer, поэтому проверяется на всех ОС.
func TestBridgeElevatedProgram(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	cfg := testCfg(sock)
	cfg.peer = func(net.Conn) (bool, error) { return false, fmt.Errorf("сверка: %w", peercred.ErrTokenDenied) }
	h := startBridge(t, cfg)
	h.send(statusCall)
	if got := toolText(t, h.recv()); got != textElevated {
		t.Errorf("текст: %q", got)
	}
	closedSilently(t, p.accept())
}

// TestBridgeForeignPeerRelog: чужой, потом сокета нет, потом снова чужой — вторая строка в
// stderr: причина забыта, когда сокет пропал, а не только когда программа подключилась.
func TestBridgeForeignPeerRelog(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	cfg := testCfg(sock)
	cfg.peer = func(net.Conn) (bool, error) { return false, nil }
	h := startBridge(t, cfg)
	waitCount := func(want int) {
		t.Helper()
		deadline := time.Now().Add(testkit.Limit(5 * time.Second))
		for strings.Count(h.errw.String(), logPeerForeign) != want {
			if time.Now().After(deadline) {
				t.Fatalf("строк в stderr: %d, ждали %d: %q", strings.Count(h.errw.String(), logPeerForeign), want, h.errw.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitCount(1)
	_ = p.ln.Close() // Go убирает файл сокета: слушателя нет
	time.Sleep(testkit.Limit(time.Second))
	if c := strings.Count(h.errw.String(), logPeerForeign); c != 1 {
		t.Fatalf("без слушателя строк в stderr: %d", c)
	}
	listenProg(t, sock) // чужой вернулся
	waitCount(2)
}

// TestDialHeldReleasesAfterPeer: папка сокета отпускается ровно один раз и только после сверки
// собеседника (Windows: пока она держится, mcp не подменить, 148-11); при неудавшемся dial тоже.
func TestDialHeldReleasesAfterPeer(t *testing.T) {
	sock := newSock(t)
	listenProg(t, sock)
	var log []string
	cfg := testCfg(sock)
	cfg.peer = func(net.Conn) (bool, error) { log = append(log, "peer"); return true, nil }
	b := &bridge{cfg: cfg}
	c, why, note := b.dialHeld(sock, func() { log = append(log, "release") })
	if c == nil || why != "" || note != "" {
		t.Fatalf("dialHeld = %v, %q, %q", c, why, note)
	}
	_ = c.Close()
	if got := strings.Join(log, ","); got != "peer,release" {
		t.Errorf("порядок: %s, ждали peer,release", got)
	}
	log = nil
	cfg.peer = func(net.Conn) (bool, error) { log = append(log, "peer"); return false, nil }
	b = &bridge{cfg: cfg}
	if c, _, note := b.dialHeld(sock, func() { log = append(log, "release") }); c != nil || note != logPeerForeign {
		t.Errorf("чужой: conn=%v note=%q", c, note)
	}
	if got := strings.Join(log, ","); got != "peer,release" {
		t.Errorf("порядок при отказе: %s", got)
	}
	log = nil
	if c, _, note := b.dialHeld(sock+"-нет", func() { log = append(log, "release") }); c != nil || note != "" {
		t.Errorf("dial не удался: conn=%v note=%q", c, note)
	}
	if got := strings.Join(log, ","); got != "release" {
		t.Errorf("при неудавшемся dial: %s, ждали release", got)
	}
}
