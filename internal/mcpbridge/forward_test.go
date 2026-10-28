package mcpbridge

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// liveBridge — посредник с программой, рукопожатие пройдено; возвращает клиента и сокет программы.
func liveBridge(t *testing.T, cfg func(*config)) (*harness, *progConn) {
	t.Helper()
	sock := newSock(t)
	p := listenProg(t, sock)
	c := testCfg(sock)
	if cfg != nil {
		cfg(&c)
	}
	h := startBridge(t, c)
	pc := p.accept()
	h.send(initLine)
	pc.answerInit()
	if r := h.recv(); r["result"] == nil {
		t.Fatalf("initialize: %v", r)
	}
	h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if m := pc.read(); m["method"] != "notifications/initialized" {
		t.Fatalf("ждали initialized: %v", m)
	}
	return h, pc
}

// TestBridgeForward: строки идут в обе стороны байт в байт (пробелы, эскейпы, юникод);
// запросы и уведомления программы клиенту — как есть; ответ на то, что не спрашивали, не
// пересылается.
func TestBridgeForward(t *testing.T) {
	h, pc := liveBridge(t, nil)

	odd := `{ "jsonrpc" : "2.0", "id" : 5 ,"method":"tools/call","params":{"name":"x","arguments":{"s":"é \" \\ ё"}} }`
	h.send(odd)
	_ = pc.c.SetReadDeadline(time.Now().Add(5 * time.Second))
	got, _, _, err := pc.lr.next()
	if err != nil || string(got) != odd {
		t.Fatalf("в сокет ушло %q (err %v)", got, err)
	}
	reply := `{"jsonrpc":"2.0",  "id":5,"result":{"content":[{"type":"text","text":"ёж  "}]}}`
	pc.write(reply)
	if raw := h.recvRaw(); string(raw) != reply {
		t.Errorf("клиенту пришло %q", raw)
	}

	// Запрос и уведомление от программы; ответ клиента идёт обратно.
	pc.write(`{"jsonrpc":"2.0","id":99,"method":"sampling/createMessage","params":{}}`)
	if m := h.recv(); m["method"] != "sampling/createMessage" {
		t.Errorf("запрос программы: %v", m)
	}
	pc.write(`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info"}}`)
	if m := h.recv(); m["method"] != "notifications/message" {
		t.Errorf("уведомление программы: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":99,"result":{"role":"assistant"}}`)
	if m := pc.read(); m["id"] != float64(99) || m["result"] == nil {
		t.Errorf("ответ клиента: %v", m)
	}

	// Ответ на id, которого никто не ждёт, и строка не-объект — не пересылаются.
	pc.write(`{"jsonrpc":"2.0","id":777,"result":{}}`)
	pc.write(`[1,2,3]`)
	pc.write(`not json`)
	pc.write(`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"last"}}`)
	if m := h.recv(); m["params"].(map[string]any)["level"] != "last" {
		t.Errorf("лишняя строка дошла до клиента: %v", m)
	}
}

// TestBridgeBadJSON: мусор от клиента в живой сессии не уходит в программу (она закрыла бы
// подключение), клиент получает ошибку; негодные id (объект, длинный, повтор) — тоже.
func TestBridgeBadJSON(t *testing.T) {
	h, pc := liveBridge(t, nil)
	h.send("{мусор")
	if m := h.recv(); m["id"] != nil || errMessage(m) != textParse {
		t.Errorf("мусор: %v", m)
	}
	h.send(`"строка"`)
	if m := h.recv(); errMessage(m) != textBadReq {
		t.Errorf("не объект: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":{"a":1},"method":"ping"}`)
	if m := h.recv(); errMessage(m) != textBadID {
		t.Errorf("id-объект: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":"` + strings.Repeat("a", 300) + `","method":"ping"}`)
	if m := h.recv(); errMessage(m) != textBadID {
		t.Errorf("длинный id: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":5,"method":"ping"}`)
	if m := pc.read(); m["id"] != float64(5) {
		t.Fatalf("в программу ушло не то: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":5,"method":"ping"}`) // тот же id, ответа ещё нет
	if m := h.recv(); errMessage(m) != textBadID {
		t.Errorf("повтор id: %v", m)
	}
	pc.write(`{"jsonrpc":"2.0","id":5,"result":{}}`)
	if m := h.recv(); m["id"] != float64(5) {
		t.Errorf("ответ: %v", m)
	}
}

// TestBridgeLineLimit: строка клиента длиннее 1 МБ — ошибка на её id, в программу не уходит,
// подключение живо; id не нашёлся в начале — тихий пропуск; ответ программы 3 МБ проходит, 5 МБ
// (больше 4 МБ) — ошибка на id, подключение живо. Работает и в заглушке.
func TestBridgeLineLimit(t *testing.T) {
	h, pc := liveBridge(t, nil)
	pad := strings.Repeat("A", 1500<<10)

	h.send(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"x","arguments":{"pad":"` + pad + `"}}}`)
	if m := h.recv(); m["id"] != float64(11) || errMessage(m) != textBigReq {
		t.Fatalf("длинная строка клиента: %v", m)
	}
	// id после огромного params: найти нельзя, отвечать некому — молчим, посредник жив.
	h.send(`{"params":{"pad":"` + pad + `"},"id":13,"method":"tools/call"}`)
	h.none(100 * time.Millisecond)

	h.send(`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"x"}}`)
	if m := pc.read(); m["id"] != float64(12) {
		t.Fatalf("длинные строки дошли до программы: %v", m)
	}
	pc.write(`{"jsonrpc":"2.0","id":12,"result":{"content":[{"type":"text","text":"` + strings.Repeat("B", 3<<20) + `"}]}}`)
	if raw := h.recvRaw(); len(raw) < 3<<20 {
		t.Fatalf("ответ 3 МБ усечён: %d", len(raw))
	}

	h.send(`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"x"}}`)
	pc.read()
	pc.write(`{"jsonrpc":"2.0","id":14,"result":{"content":[{"type":"text","text":"` + strings.Repeat("C", 5<<20) + `"}]}}`)
	if m := h.recv(); m["id"] != float64(14) || errMessage(m) != textBigResp {
		t.Fatalf("ответ 5 МБ: %.200v", m)
	}
	// Подключение живо.
	h.send(`{"jsonrpc":"2.0","id":15,"method":"ping"}`)
	if m := pc.read(); m["id"] != float64(15) {
		t.Fatalf("после больших строк: %v", m)
	}
}

// TestBridgeLineLimitStub: тот же предел, пока программы нет.
func TestBridgeLineLimitStub(t *testing.T) {
	h := startBridge(t, testCfg(newSock(t)))
	h.send(`{"jsonrpc":"2.0","id":3,"method":"ping","params":{"pad":"` + strings.Repeat("A", 2<<20) + `"}}`)
	if m := h.recv(); m["id"] != float64(3) || errMessage(m) != textBigReq {
		t.Fatalf("%.200v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":4,"method":"ping"}`)
	if m := h.recv(); m["id"] != float64(4) || m["result"] == nil {
		t.Fatalf("после длинной строки: %v", m)
	}
}

// TestBridgePendingCap: ждущих запросов не больше maxPending — лишний получает ошибку сразу, а
// набор не растёт; после ответов места снова есть.
func TestBridgePendingCap(t *testing.T) {
	h, pc := liveBridge(t, nil)
	go func() { _, _ = io.Copy(io.Discard, pc.c) }() // программа читает, но не отвечает
	for i := 100; i < 100+maxPending; i++ {
		h.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping"}`, i))
	}
	h.send(`{"jsonrpc":"2.0","id":5000,"method":"ping"}`)
	if m := h.recv(); m["id"] != float64(5000) || errMessage(m) != textBusy {
		t.Fatalf("сверх предела: %v", m)
	}
	pc.write(`{"jsonrpc":"2.0","id":100,"result":{}}`)
	if m := h.recv(); m["id"] != float64(100) {
		t.Fatalf("ответ: %v", m)
	}
	h.send(`{"jsonrpc":"2.0","id":5001,"method":"ping"}`)
	h.none(50 * time.Millisecond) // ушёл в программу, а не в ошибку
}

// TestBridgeMany: несколько посредников одновременно к одной программе, у каждого по 20 запросов
// вперемешку и в обратном порядке ответов — каждый клиент получает только свои ответы.
func TestBridgeMany(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	const nb, nq = 3, 20
	hs := make([]*harness, nb)
	pcs := make([]*progConn, nb)
	for i := range hs {
		hs[i] = startBridge(t, testCfg(sock))
		pcs[i] = p.accept()
	}
	// accept отдаёт подключения в порядке прихода; сопоставим по первому сообщению.
	var wg sync.WaitGroup
	for i, h := range hs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for q := range nq {
				h.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"echo","who":%d}}`, q, i))
			}
		}()
	}
	wg.Wait()
	for _, pc := range pcs {
		var ids []float64
		var who float64
		for range nq {
			m := pc.read()
			ids = append(ids, m["id"].(float64))
			who = m["params"].(map[string]any)["who"].(float64)
		}
		for k := len(ids) - 1; k >= 0; k-- {
			pc.write(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"who":%d}}`, int(ids[k]), int(who)))
		}
	}
	for i, h := range hs {
		seen := map[float64]bool{}
		for range nq {
			m := h.recv()
			res := m["result"].(map[string]any)
			if res["who"] != float64(i) {
				t.Fatalf("клиент %d получил чужой ответ: %v", i, m)
			}
			seen[m["id"].(float64)] = true
		}
		if len(seen) != nq {
			t.Errorf("клиент %d: %d разных ответов из %d", i, len(seen), nq)
		}
	}
}
