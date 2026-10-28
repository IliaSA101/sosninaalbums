package mcpbridge

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const initLine = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

func initWith(id int, version string) string {
	return `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"initialize","params":{"protocolVersion":"` + version +
		`","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestBridgeStub: программы нет — отвечает заглушка (design §4.2): initialize не выше 2025-11-25,
// один инструмент program_status, ping, ошибка на прочее, server/discover — «нет метода»;
// уведомления гасятся; битые строки — ошибка без пересылки. И ни в одном ответе нет пути-метки
// сокета: нет папки, нет файла, небезопасные права, чужой файл, старый сокет без слушателя.
func TestBridgeStub(t *testing.T) {
	label := filepath.Join(shortDir(t), "ZQmarkZQ")
	cases := map[string]func(t *testing.T) string{
		"нет папки": func(t *testing.T) string { return filepath.Join(label, "mcp", "sock") },
		"нет файла": func(t *testing.T) string { return sockIn(t, label) },
		"чужой файл": func(t *testing.T) string {
			s := sockIn(t, label)
			if err := os.WriteFile(s, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			return s
		},
		"старый сокет без слушателя": func(t *testing.T) string {
			s := sockIn(t, label)
			ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: s, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			ln.SetUnlinkOnClose(false)
			_ = ln.Close()
			return s
		},
		"путь длиннее предела": func(t *testing.T) string {
			return filepath.Join(label, strings.Repeat("d", 150), "mcp", "sock")
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			_ = os.RemoveAll(label)
			t.Cleanup(func() { _ = os.RemoveAll(label) })
			sock := mk(t)
			h := startBridge(t, testCfg(sock))
			out := stubDialog(t, h)
			if code := h.finish(); code != 0 {
				t.Errorf("код выхода %d", code)
			}

			all := strings.Join(append(out, h.errw.String()), "\n")
			if contains(all, "ZQmark", os.TempDir(), "/tmp", "sock") {
				t.Errorf("в ответах путь или его часть: %q", all)
			}
			if out[0] == "" {
				t.Error("пустой текст program_status")
			}
		})
	}
}

// stubDialog гоняет клиента по заглушке и собирает тексты ответов: program_status первым.
func stubDialog(t *testing.T, h *harness) []string {
	t.Helper()
	h.send(initWith(1, "2026-07-28"))
	r := h.recv()
	res, _ := r["result"].(map[string]any)
	if res["protocolVersion"] != "2025-11-25" {
		t.Errorf("версия заглушки: %v", res["protocolVersion"])
	}
	if caps, _ := res["capabilities"].(map[string]any); caps["tools"] == nil {
		t.Errorf("нет capabilities.tools: %v", res)
	}
	h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	h.none(50 * time.Millisecond)

	h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools, _ := h.recv()["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "program_status" {
		t.Errorf("tools/list: %v", tools)
	}
	h.send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"program_status","arguments":{}}}`)
	txt := toolText(t, h.recv())
	h.send(`{"jsonrpc":"2.0","id":4,"method":"ping"}`)
	if h.recv()["result"] == nil {
		t.Error("ping без result")
	}
	h.send(`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`)
	other := errMessage(h.recv())
	h.send(`{"jsonrpc":"2.0","id":6,"method":"server/discover"}`)
	disc := h.recv()
	if e, _ := disc["error"].(map[string]any); e == nil || e["code"].(float64) != -32601 {
		t.Errorf("server/discover: %v", disc)
	}
	h.send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"drop_table"}}`)
	unk := errMessage(h.recv())
	h.send(`не json`)
	bad := h.recv()
	h.send(`[1,2]`)
	arr := h.recv()
	h.send(`{"jsonrpc":"2.0","id":{"a":1},"method":"ping"}`)
	badID := h.recv()
	if bad["id"] != nil || arr["id"] != nil || badID["id"] != nil {
		t.Errorf("на битую строку id должен быть null: %v %v %v", bad, arr, badID)
	}
	return []string{txt, other, unk, errMessage(bad), errMessage(arr), errMessage(badID)}
}

// TestBridgeNoDataDir: папки данных нет (пустой список путей) — свой текст, dial не было.
func TestBridgeNoDataDir(t *testing.T) {
	h := startBridge(t, testCfg())
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`)
	if got := toolText(t, h.recv()); got != textNoDir {
		t.Errorf("текст: %q", got)
	}
}

// TestBridgeEOF: конец stdin — сокет закрыт, код 0.
func TestBridgeEOF(t *testing.T) {
	sock := newSock(t)
	p := listenProg(t, sock)
	h := startBridge(t, testCfg(sock))
	c := p.accept()
	if code := h.finish(); code != 0 {
		t.Errorf("код выхода %d", code)
	}
	c.waitEOF()
}
