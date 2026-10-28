package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"regexp"
	"sync"
	"time"
)

// Посредник (design §4.1, §4.2): строки stdin ↔ сокет как есть; пока сокета нет — заглушка;
// сокет появился — повтор initialize клиента со своим id и tools/list_changed клиенту.

const (
	maxLine      = 1 << 20 // предел строки от клиента (design §4.1)
	replayID     = `"sa-bridge-1"`
	errClosedMsg = "программа закрыта"
	stubText     = "Sosnina Albums не запущена или доступ агентам выключен — откройте программу: Файл ▸ ИИ-агенты"
)

// Версии протокола go-sdk v1.8.0 (shared.go): заглушка отвечает запрошенной, если она из списка.
var sdkVersions = map[string]bool{"2026-07-28": true, "2025-11-25": true, "2025-06-18": true, "2025-03-26": true, "2024-11-05": true}

var idRe = regexp.MustCompile(`"id"\s*:\s*("(?:[^"\\]|\\.)*"|-?\d+)`)

type bridge struct {
	out   io.Writer
	outMu sync.Mutex
	sock  string
	probe time.Duration

	mu        sync.Mutex // состояние ниже
	conn      net.Conn
	pending   map[string]bool
	initParam json.RawMessage // params первого initialize клиента
	stubVer   string          // версия, которой ответила заглушка
	sawInited bool
	wasStub   bool // клиент уже видел заглушку или обрыв — после подключения нужен list_changed
	lastProbe time.Time
	replayCh  chan json.RawMessage
}

func runBridge(in io.Reader, out io.Writer, sock string, probe time.Duration) int {
	b := &bridge{out: out, sock: sock, probe: probe, pending: map[string]bool{}, replayCh: make(chan json.RawMessage, 1)}
	b.mu.Lock()
	b.tryConnectLocked()
	b.mu.Unlock()
	done := make(chan struct{})
	defer close(done)
	go b.ticker(done)
	r := bufio.NewReaderSize(in, 64<<10)
	for {
		line, long, err := readLine(r)
		if err != nil { // EOF stdin: закрыть сокет и выйти с 0 (спецификация stdio)
			b.mu.Lock()
			if b.conn != nil {
				_ = b.conn.Close()
			}
			b.mu.Unlock()
			return 0
		}
		if long {
			id := "null"
			if m := idRe.FindSubmatch(line); m != nil {
				id = string(m[1])
			}
			b.send(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32600,"message":"строка длиннее 1 МБ"}}`)
			continue
		}
		b.fromClient(line)
	}
}

// readLine читает строку до \n; длиннее maxLine — отбрасывает хвост и возвращает первые 512 байт (для id).
func readLine(r *bufio.Reader) (line []byte, long bool, err error) {
	var buf, first []byte
	for {
		chunk, e := r.ReadSlice('\n')
		if first == nil {
			first = append([]byte{}, chunk[:min(len(chunk), 512)]...)
		}
		if !long {
			if len(buf)+len(chunk) > maxLine {
				long, buf = true, first
			} else {
				buf = append(buf, chunk...)
			}
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != nil {
			if len(buf) > 0 {
				return bytes.TrimRight(buf, "\r\n"), long, nil
			}
			return nil, false, e
		}
		return bytes.TrimRight(buf, "\r\n"), long, nil
	}
}

type head struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (b *bridge) send(s string) {
	b.outMu.Lock()
	defer b.outMu.Unlock()
	_, _ = io.WriteString(b.out, s+"\n")
}

func (b *bridge) listChanged() {
	b.send(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`)
}

func (b *bridge) fromClient(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var h head
	if err := json.Unmarshal(line, &h); err != nil {
		b.send(`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"разбор JSON"}}`)
		return
	}
	b.mu.Lock()
	if b.conn == nil && time.Since(b.lastProbe) >= b.probe {
		b.tryConnectLocked()
	}
	if h.Method == "initialize" {
		b.initParam = h.Params
		if b.conn != nil {
			b.stubVer = "" // сессию начала настоящая программа, сверять версию при повторе не с чем
		}
	}
	if h.Method == "notifications/initialized" {
		b.sawInited = true
	}
	if b.conn != nil {
		if len(h.ID) > 0 && h.Method != "" {
			b.pending[string(h.ID)] = true
		}
		_, err := b.conn.Write(append(append([]byte{}, line...), '\n'))
		b.mu.Unlock()
		if err != nil {
			logf("bridge: запись в сокет: %v", err)
		}
		return
	}
	b.wasStub = true
	b.mu.Unlock()
	b.stub(h)
}

// stub — ответы без программы (design §4.2).
func (b *bridge) stub(h head) {
	if len(h.ID) == 0 { // уведомление
		return
	}
	id := string(h.ID)
	reply := func(result string) { b.send(`{"jsonrpc":"2.0","id":` + id + `,"result":` + result + `}`) }
	fail := func(code, msg string) {
		m, _ := json.Marshal(msg)
		b.send(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":` + code + `,"message":` + string(m) + `}}`)
	}
	switch h.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(h.Params, &p)
		// initialize в SDK v1.8.0 договаривается не выше 2025-11-25: ревизия 2026-07-28 идёт через server/discover,
		// поэтому заглушка на initialize с 2026-07-28 отвечает 2025-11-25 (иначе повтор после запуска не сойдётся по версии)
		ver := "2025-11-25"
		if sdkVersions[p.ProtocolVersion] && p.ProtocolVersion != "2026-07-28" {
			ver = p.ProtocolVersion
		}
		b.mu.Lock()
		b.stubVer = ver
		b.mu.Unlock()
		reply(`{"protocolVersion":"` + ver + `","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"sosnina-albums","version":"spike-stub"}}`)
	case "tools/list":
		reply(`{"tools":[{"name":"program_status","description":"Состояние программы","inputSchema":{"type":"object"}}]}`)
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(h.Params, &p)
		if p.Name != "program_status" {
			fail("-32602", "нет такого инструмента")
			return
		}
		t, _ := json.Marshal(stubText)
		reply(`{"content":[{"type":"text","text":` + string(t) + `}],"isError":false,"structuredContent":{"status":"stub"}}`)
	case "ping":
		reply(`{}`)
	default: // в том числе server/discover: клиент 2026-07-28 откатывается на initialize
		fail("-32000", stubText)
	}
}

func (b *bridge) ticker(done <-chan struct{}) {
	t := time.NewTicker(b.probe)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			b.mu.Lock()
			if b.conn == nil && b.wasStub {
				b.tryConnectLocked()
			}
			b.mu.Unlock()
		}
	}
}

// tryConnectLocked пробует сокет; b.mu держит вызывающий. Успех — b.conn установлен, повтор initialize сделан.
func (b *bridge) tryConnectLocked() {
	b.lastProbe = time.Now()
	c, err := net.DialTimeout("unix", b.sock, 500*time.Millisecond)
	if err != nil {
		return
	}
	for len(b.replayCh) > 0 {
		<-b.replayCh
	}
	go b.readSock(c)
	if len(b.initParam) > 0 { // клиент уже начинал сессию с заглушкой или с прежней программой — повторить initialize
		req := `{"jsonrpc":"2.0","id":` + replayID + `,"method":"initialize","params":` + string(b.initParam) + `}`
		if _, err := c.Write([]byte(req + "\n")); err != nil {
			_ = c.Close()
			return
		}
		select {
		case res := <-b.replayCh:
			var r struct {
				Result struct {
					ProtocolVersion string `json:"protocolVersion"`
				} `json:"result"`
			}
			_ = json.Unmarshal(res, &r)
			if b.stubVer != "" && r.Result.ProtocolVersion != b.stubVer {
				logf("bridge: версии не совпали stub=%s server=%s", b.stubVer, r.Result.ProtocolVersion)
				_ = c.Close()
				return
			}
		case <-time.After(3 * time.Second):
			logf("bridge: нет ответа на повтор initialize")
			_ = c.Close()
			return
		}
		if b.sawInited {
			_, _ = c.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"))
		}
	}
	b.conn = c
	logf("bridge: подключился к программе (replay=%v wasStub=%v)", len(b.initParam) > 0, b.wasStub)
	if b.wasStub {
		b.wasStub = false
		go b.listChanged()
	}
}

// readSock пересылает строки сокета клиенту; ответ на повтор initialize уходит в replayCh, не клиенту.
func (b *bridge) readSock(c net.Conn) {
	r := bufio.NewReaderSize(c, 64<<10)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var h head
			_ = json.Unmarshal(line, &h)
			switch {
			case h.Method == "" && string(h.ID) == replayID:
				select {
				case b.replayCh <- json.RawMessage(bytes.Clone(line)):
				default:
				}
			default:
				if h.Method == "" && len(h.ID) > 0 {
					b.mu.Lock()
					delete(b.pending, string(h.ID))
					b.mu.Unlock()
				}
				b.send(string(bytes.TrimRight(line, "\r\n")))
			}
		}
		if err != nil {
			break
		}
	}
	b.mu.Lock()
	if b.conn != c {
		b.mu.Unlock()
		return
	}
	b.conn = nil
	b.wasStub = true
	ids := make([]string, 0, len(b.pending))
	for id := range b.pending {
		ids = append(ids, id)
	}
	b.pending = map[string]bool{}
	b.mu.Unlock()
	_ = c.Close()
	logf("bridge: программа закрылась, ждущих запросов %d", len(ids))
	for _, id := range ids {
		b.send(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32000,"message":"` + errClosedMsg + `"}}`)
	}
	b.listChanged()
}
