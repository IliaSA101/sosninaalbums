package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestConnLimit: 17-му подключению — одна строка-ошибка JSON-RPC и закрытие; остальные живут;
// освободилось место — подключение снова принимается.
func TestConnLimit(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	conns := make([]net.Conn, 0, maxConns)
	for range maxConns {
		conns = append(conns, raw(t, r.sock))
	}
	waitFor(t, "16 мест заняты", func() bool { return r.conns() == maxConns })

	extra := raw(t, r.sock)
	_ = extra.SetReadDeadline(time.Now().Add(testkit.Limit(3 * time.Second)))
	br := bufio.NewReader(extra)
	line, err := br.ReadString('\n')
	if err != nil || line != tooMany {
		t.Fatalf("текст лишнему: %q %v", line, err)
	}
	var msg struct {
		ID    *int `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &msg); err != nil || msg.ID != nil || msg.Error.Code != -32000 ||
		!strings.Contains(msg.Error.Message, "слишком много подключений") {
		t.Fatalf("не JSON-RPC ошибка: %q %v", line, err)
	}
	if _, err := br.ReadByte(); err == nil {
		t.Fatal("после текста подключение не закрыто")
	}
	if r.conns() != maxConns {
		t.Fatalf("лишнее заняло место: %d", r.conns())
	}

	_ = conns[0].Close()
	waitFor(t, "место освободилось", func() bool { return r.conns() == maxConns-1 })
	c := raw(t, r.sock)
	handshake(t, c)
	waitFor(t, "снова 16", func() bool { return r.conns() == maxConns })
}

// TestLineLimit: строка запроса длиннее 1 МБ закрывает это подключение; соседнее живёт, место
// освобождается.
func TestLineLimit(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	good := client(t, r.sock, "соседний")
	bad := raw(t, r.sock)
	handshake(t, bad)
	waitFor(t, "два места", func() bool { return r.conns() == 2 })

	go func() { // запись 2 МБ сервер оборвёт: ошибка записи ожидаема
		_ = bad.SetWriteDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
		_, _ = bad.Write([]byte(pingPad(2 << 20)))
	}()
	if !closedWithin(bad, testkit.Limit(5*time.Second)) {
		t.Fatal("подключение со строкой в 2 МБ не закрыто")
	}
	waitFor(t, "место освободилось", func() bool { return r.conns() == 1 })
	if err := good.Ping(context.Background(), nil); err != nil {
		t.Fatalf("соседнее подключение: %v", err)
	}
	// Строка в пределе проходит (ping с длинным, но допустимым телом — это обычный запрос).
	ok := raw(t, r.sock)
	br := handshake(t, ok)
	if _, err := ok.Write([]byte(pingPad(100 << 10))); err != nil {
		t.Fatal(err)
	}
	_ = ok.SetReadDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	if line, err := br.ReadString('\n'); err != nil || !strings.Contains(line, `"id":7`) {
		t.Fatalf("строка 100 КБ: %q %v", line, err)
	}
}

// pingPad — корректный ping на n байт: без предела строки его приняли бы, с пределом — закрывают.
func pingPad(n int) string {
	return `{"jsonrpc":"2.0","id":7,"method":"ping","params":{"_meta":{"pad":"` + strings.Repeat("y", n) + `"}}}` + "\n"
}

// TestSlowReader: клиент, который не читает ответ, не держит место — запись с малым сроком
// рвёт подключение.
func TestSlowReader(t *testing.T) {
	big := strings.Repeat("я", 4<<20)
	r := newRig(t, Options{
		WriteTimeout: 200 * time.Millisecond,
		Setup:        func(s *mcp.Server) { addTool(s, "big", func(context.Context) string { return big }) },
	}, "")
	r.start(t)
	c := raw(t, r.sock)
	handshake(t, c)
	waitFor(t, "место занято", func() bool { return r.conns() == 1 })
	// Дальше не читаем ничего. Объём буфера сокета зависит от ОС (Windows AF_UNIX большой), поэтому
	// просим ответы по одному, пока запись не упрётся: не более slowCalls × 8 МБ.
	const slowCalls = 24
	closed := false
	for i := range slowCalls {
		call := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"big","arguments":{}}}`+"\n", i+2)
		if _, err := c.Write([]byte(call)); err != nil {
			closed = true // сервер уже закрыл подключение
			break
		}
		end := time.Now().Add(testkit.Limit(600 * time.Millisecond))
		for time.Now().Before(end) && r.conns() != 0 {
			time.Sleep(5 * time.Millisecond)
		}
		if r.conns() == 0 {
			t.Logf("срок записи сработал на вызове %d (≈ %d МБ ответов)", i+1, 8*(i+1))
			closed = true
			break
		}
	}
	if !closed {
		t.Fatalf("подключение не закрыто по сроку записи после %d ответов по 8 МБ", slowCalls)
	}
	// Соседний клиент службу видит живой.
	if err := client(t, r.sock, "после").Ping(context.Background(), nil); err != nil {
		t.Fatalf("служба после обрыва: %v", err)
	}
}

// TestCloseDuringRender: инструмент завис (рендер до 20 с) — Close всё равно возвращается за
// секунду с небольшим, файл сокета удалён; выход программы не ждёт обработчик.
func TestCloseDuringRender(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	r := newRig(t, Options{CloseWait: 200 * time.Millisecond, Setup: func(s *mcp.Server) {
		addTool(s, "render", func(context.Context) string {
			close(entered)
			<-release // не слушает ctx: так ведёт себя долгий рендер
			return "готово"
		})
	}}, "")
	t.Cleanup(func() { close(release) })
	r.start(t)
	cs := client(t, r.sock, "render")
	go func() {
		_, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "render", Arguments: map[string]any{}})
	}()
	select {
	case <-entered:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("обработчик не вызван")
	}
	start := time.Now()
	r.svc.Close()
	if d := time.Since(start); d > testkit.Limit(time.Second) {
		t.Fatalf("Close с зависшим обработчиком занял %v", d)
	}
	if _, err := os.Lstat(r.sock); err == nil {
		t.Fatal("файл сокета остался")
	}
	if st, _ := r.svc.State(); st != StateOff {
		t.Fatalf("State = %q", st)
	}
}

// TestInitTimeout: подключение без initialize закрывается по сроку; прошедшее initialize живёт.
func TestInitTimeout(t *testing.T) {
	r := newRig(t, Options{InitTimeout: 200 * time.Millisecond}, "")
	r.start(t)
	silent := raw(t, r.sock)
	good := client(t, r.sock, "живой")
	if !closedWithin(silent, testkit.Limit(3*time.Second)) {
		t.Fatal("молчун не закрыт")
	}
	waitFor(t, "остался один", func() bool { return r.conns() == 1 })
	time.Sleep(3 * 200 * time.Millisecond)
	if err := good.Ping(context.Background(), nil); err != nil {
		t.Fatalf("прошедший initialize закрыт: %v", err)
	}
}

// TestGarbageInput: пустые, усечённые и битые входы — ошибка или закрытие подключения, служба
// жива, места освобождаются.
func TestGarbageInput(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	for _, in := range []string{"", "\n", "{", "не json\n", "{\"jsonrpc\":\"2.0\"\n", "[]\n", "null\n", "\x00\xff\xfe\n", initLine[:40]} {
		c := raw(t, r.sock)
		_, _ = c.Write([]byte(in))
		_ = c.Close()
	}
	waitFor(t, "места свободны", func() bool { return r.conns() == 0 })
	if err := client(t, r.sock, "после-мусора").Ping(context.Background(), nil); err != nil {
		t.Fatalf("служба после мусора: %v", err)
	}
}

// TestCleanName: имя клиента без управляющих и форматных знаков, ≤ 40 знаков, битый UTF-8 пропущен.
func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"":                           "",
		"Claude Desktop":             "Claude Desktop",
		"a\nb\x00c\u202ed\u200be":    "abcde", // перевод строки, NUL, разворот и нулевая ширина
		"\xff\xfeок":                 "ок",
		strings.Repeat("ё", 100):     strings.Repeat("ё", 40),
		strings.Repeat("\u202e", 50): "",
	}
	for in, want := range cases {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// TestDeadlineConn: запись в подключение, которое никто не читает (net.Pipe без буфера — одинаково
// на всех ОС), по сроку возвращает ошибку и закрывает подключение.
func TestDeadlineConn(t *testing.T) {
	a, b := net.Pipe()
	t.Cleanup(func() { _ = b.Close() })
	w := deadlineConn{a, 100 * time.Millisecond}
	start := time.Now()
	if _, err := w.Write([]byte("x")); err == nil {
		t.Fatal("запись без читателя прошла")
	}
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Fatalf("срок записи не сработал: %v", d)
	}
	if _, err := a.Write([]byte("y")); err == nil || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("подключение не закрыто после ошибки записи: %v", err)
	}
}
