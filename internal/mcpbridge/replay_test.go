package mcpbridge

import (
	"net"
	"testing"
	"time"
)

// handshakeStub проводит рукопожатие клиента с заглушкой и возвращает версию из ответа.
func handshakeStub(t *testing.T, h *harness, line string, initialized bool) string {
	t.Helper()
	h.send(line)
	res, _ := h.recv()["result"].(map[string]any)
	v, _ := res["protocolVersion"].(string)
	if initialized {
		h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	}
	return v
}

func wantListChanged(t *testing.T, h *harness) {
	t.Helper()
	if m := h.recv(); m["method"] != "notifications/tools/list_changed" || m["id"] != nil {
		t.Fatalf("ждали notifications/tools/list_changed: %v", m)
	}
}

// TestBridgeReplay: программа запустилась после рукопожатия клиента с заглушкой — посредник
// повторяет ей initialize (своим id, версия — та, что получил клиент, не выше 2025-11-25:
// решение 0.31-143-44) и initialized, ответ не пересылает, клиенту шлёт list_changed; потом
// пересылка. Клиенты обеих ревизий.
func TestBridgeReplay(t *testing.T) {
	t.Run("2025-11-25, initialized послан до запуска программы", func(t *testing.T) {
		sock := newSock(t)
		h := startBridge(t, testCfg(sock))
		if v := handshakeStub(t, h, initWith(1, "2025-11-25"), true); v != "2025-11-25" {
			t.Fatalf("заглушка: %q", v)
		}
		p := listenProg(t, sock)
		c := p.accept()
		params := c.answerInit()
		if params["protocolVersion"] != "2025-11-25" || params["clientInfo"].(map[string]any)["name"] != "t" {
			t.Errorf("params повтора: %v", params)
		}
		if m := c.read(); m["method"] != "notifications/initialized" {
			t.Fatalf("после повтора ждали initialized: %v", m)
		}
		wantListChanged(t, h)
		h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		if m := c.read(); m["id"] != float64(2) || m["method"] != "tools/list" {
			t.Fatalf("после повтора: %v", m)
		}
		c.write(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"real"}]}}`)
		tools := h.recv()["result"].(map[string]any)["tools"].([]any)
		if len(tools) != 1 || tools[0].(map[string]any)["name"] != "real" {
			t.Errorf("tools/list: %v", tools)
		}
	})

	t.Run("клиент просит 2026-07-28: discover → ошибка, initialize → 2025-11-25, повтор — 2025-11-25", func(t *testing.T) {
		sock := newSock(t)
		h := startBridge(t, testCfg(sock))
		h.send(`{"jsonrpc":"2.0","id":0,"method":"server/discover"}`)
		if e, _ := h.recv()["error"].(map[string]any); e == nil || e["code"] != float64(-32601) {
			t.Fatal("server/discover должен отвечать ошибкой")
		}
		if v := handshakeStub(t, h, initWith(1, "2026-07-28"), false); v != "2025-11-25" {
			t.Fatalf("заглушка ответила %q, не выше 2025-11-25", v)
		}
		p := listenProg(t, sock)
		c := p.accept()
		if params := c.answerInit(); params["protocolVersion"] != "2025-11-25" {
			t.Errorf("повтор с версией %v: программа ответила бы новее, чем знает клиент", params["protocolVersion"])
		}
		wantListChanged(t, h)
		// Клиент initialized не слал — повтор его не посылает: первой строкой после initialize
		// в сокете должен оказаться запрос клиента, а не лишний initialized.
		h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		if m := c.read(); m["method"] != "tools/list" {
			t.Fatalf("после повтора ждали tools/list клиента, пришло %v", m["method"])
		}
		// initialized клиента придёт позже и уйдёт в программу как есть.
		h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
		if m := c.read(); m["method"] != "notifications/initialized" {
			t.Fatalf("initialized клиента: %v", m)
		}
	})

	t.Run("ревизия без initialize: повтора нет, только list_changed", func(t *testing.T) {
		sock := newSock(t)
		h := startBridge(t, testCfg(sock))
		h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"protocolVersion":"2026-07-28"}}}`)
		h.recv()
		p := listenProg(t, sock)
		c := p.accept()
		wantListChanged(t, h)
		h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		if m := c.read(); m["method"] != "tools/list" {
			t.Fatalf("программа получила не запрос клиента (повтор initialize не нужен): %v", m)
		}
	})

	t.Run("версия не сошлась: остаёмся заглушкой, потом сходится", func(t *testing.T) {
		sock := newSock(t)
		cfg := testCfg(sock)
		cfg.probeEvery = 300 * time.Millisecond
		h := startBridge(t, cfg)
		handshakeStub(t, h, initWith(1, "2025-06-18"), true)
		p := listenProg(t, sock)
		c := p.accept()
		m := c.read()
		c.write(`{"jsonrpc":"2.0","id":"` + replayID + `","result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"f","version":"1"}}}`)
		_ = m
		c.waitEOF() // посредник закрыл подключение
		h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"program_status"}}`)
		if got := toolText(t, h.recv()); got != textVersion {
			t.Errorf("program_status: %q", got)
		}
		c2 := p.accept() // следующая проба
		c2.answerInit()
		c2.read() // initialized
		wantListChanged(t, h)
	})

	t.Run("программа молчит: срок повтора, заглушка остаётся", func(t *testing.T) {
		sock := newSock(t)
		cfg := testCfg(sock)
		cfg.replayWait = 150 * time.Millisecond
		cfg.probeEvery = 400 * time.Millisecond
		h := startBridge(t, cfg)
		handshakeStub(t, h, initWith(1, "2025-11-25"), true)
		p := listenProg(t, sock)
		c := p.accept()
		c.read() // initialize
		c.waitEOF()
		h.none(50 * time.Millisecond) // клиенту ничего не сообщали
	})
}

// TestBridgeProgramGone: программа закрылась посреди сессии — всем ждущим id ошибка «программа
// закрыта», клиенту list_changed; пока её нет, отвечает заглушка; вернулась — повтор initialize
// и снова list_changed.
func TestBridgeProgramGone(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	h := startBridge(t, testCfg(sock))
	c := p.accept()
	h.send(initLine)
	c.answerInit()
	h.recv()
	h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c.read()

	for _, id := range []string{"5", `"six"`} {
		h.send(`{"jsonrpc":"2.0","id":` + id + `,"method":"tools/call","params":{"name":"slow"}}`)
		c.read()
	}
	// Закрываем и слушатель: сокет пропал, как при выходе программы.
	_ = p.ln.Close()
	_ = c.c.Close()
	got := map[any]string{}
	for range 2 {
		m := h.recv()
		got[m["id"]] = errMessage(m)
	}
	if got[float64(5)] != textGone || got["six"] != textGone {
		t.Errorf("ошибки ждущим: %v", got)
	}
	wantListChanged(t, h)

	h.send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"program_status"}}`)
	if got := toolText(t, h.recv()); got != textClosed {
		t.Errorf("заглушка: %q", got)
	}

	// Программа вернулась.
	p2 := listenProg(t, sock)
	c2 := p2.accept()
	if params := c2.answerInit(); params["protocolVersion"] != "2025-11-25" {
		t.Errorf("повтор: %v", params)
	}
	if m := c2.read(); m["method"] != "notifications/initialized" {
		t.Errorf("initialized: %v", m)
	}
	wantListChanged(t, h)
	h.send(`{"jsonrpc":"2.0","id":8,"method":"ping"}`)
	if m := c2.read(); m["id"] != float64(8) {
		t.Errorf("пересылка после возврата: %v", m)
	}
}

// Ответ программы лишнему подключению (agents/sessions.go tooMany): тот же текст, id null.
const tooManyLine = `{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"слишком много подключений к Sosnina Albums — закройте лишние окна агентов"}}`

// TestBridgeTooMany: лишнее подключение — первая строка от программы ошибка с id null: сокет
// закрывается, program_status и ошибки показывают именно этот текст, а не «не запущена»;
// проба сокета продолжается.
func TestBridgeTooMany(t *testing.T) {
	sock := newSock(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accepted := make(chan struct{}, 1000)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte(tooManyLine + "\n"))
			_ = c.Close()
			select {
			case accepted <- struct{}{}:
			default:
			}
		}
	}()
	h := startBridge(t, testCfg(sock))
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`)
		m := h.recv()
		for m["method"] != nil { // list_changed при смене состояния — не ответ
			m = h.recv()
		}
		if m["id"] == nil { // ошибка программы с id null клиенту не предназначена
			t.Fatalf("строка лишнего подключения дошла до клиента: %v", m)
		}
		if toolText2(m) == textTooMany || errMessage(m) == textTooMany {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("текст лишнего подключения не появился: %v", m)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Клиент с прежним списком зовёт get_album: тот же текст результатом, не -32000 (0.31-143-85).
	h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_album"}}`)
	m := h.recv()
	for m["method"] != nil {
		m = h.recv()
	}
	if m["error"] != nil || !isToolError(m) || toolText2(m) != textTooMany {
		t.Errorf("get_album при лишнем подключении: %v", m)
	}
	for len(accepted) > 0 {
		<-accepted
	}
	select { // проба продолжается: новые подключения
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Error("проба сокета прекратилась")
	}
}

func toolText2(m map[string]any) string {
	res, _ := m["result"].(map[string]any)
	cs, _ := res["content"].([]any)
	if len(cs) == 0 {
		return ""
	}
	c0, _ := cs[0].(map[string]any)
	s, _ := c0["text"].(string)
	return s
}
