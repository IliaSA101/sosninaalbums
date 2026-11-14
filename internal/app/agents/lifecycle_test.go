package agents

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// syncBuf — буфер журнала, безопасный для записи из горутин сессий и чтения теста.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLog направляет log в буфер на время теста.
func captureLog(t *testing.T) *syncBuf {
	t.Helper()
	buf := &syncBuf{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

// gone — число ушедших сессий, чьи обработчики ещё идут.
func (r *rig) gone() int {
	r.svc.mu.Lock()
	run := r.svc.run
	r.svc.mu.Unlock()
	if run == nil {
		return 0
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	return len(run.gone)
}

// TestPanicOutsideTool: паника в приёмном слое (на tools/list) и в обработчике вызова не роняет
// программу: клиенту — «внутренняя ошибка программы», соседнее подключение живо, в журнале тип
// без значения паники. Без recover в noteClient упал бы весь тестовый процесс.
func TestPanicOutsideTool(t *testing.T) {
	buf := captureLog(t)
	r := newRig(t, Options{Setup: func(s *mcp.Server) {
		addTool(s, "boom", func(context.Context) string { panic("секрет-обработчика") })
		s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method == "tools/list" {
					panic("секрет-слоя")
				}
				return next(ctx, method, req)
			}
		})
	}}, "")
	r.start(t)
	neighbor := client(t, r.sock, "соседний")
	bad := client(t, r.sock, "плохой")

	_, err := bad.CallTool(context.Background(), &mcp.CallToolParams{Name: "boom", Arguments: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "внутренняя ошибка программы") {
		t.Fatalf("CallTool: %v", err)
	}
	if _, err := bad.ListTools(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "внутренняя ошибка программы") {
		t.Fatalf("ListTools: %v", err)
	}
	if err := neighbor.Ping(context.Background(), nil); err != nil {
		t.Fatalf("соседнее подключение: %v", err)
	}
	if err := bad.Ping(context.Background(), nil); err != nil {
		t.Fatalf("подключение после паники: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ПАНИКА в agents handler") || strings.Contains(out, "секрет") {
		t.Fatalf("журнал: %q", out)
	}
}

// TestGoneHoldsNoSlot: обработчик завис, клиент закрыл сокет — место и строка в списке
// освобождаются сразу, а запоздалый ответ обработчика клиента в список не возвращает.
func TestGoneHoldsNoSlot(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r := newRig(t, Options{Setup: func(s *mcp.Server) {
		addTool(s, "hang", func(context.Context) string {
			once.Do(func() { close(entered) })
			<-release
			return "поздно"
		})
	}}, "")
	t.Cleanup(func() { close(release) })
	r.start(t)

	c := raw(t, r.sock)
	handshake(t, c)
	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hang","arguments":{}}}` + "\n"
	if _, err := c.Write([]byte(call)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("обработчик не вызван")
	}
	waitFor(t, "клиент в списке", func() bool { return len(r.svc.Clients()) == 1 && r.conns() == 1 })
	_ = c.Close()
	waitFor(t, "место и список свободны", func() bool { return r.conns() == 0 && len(r.svc.Clients()) == 0 })
	if r.gone() != 1 {
		t.Fatalf("пометка «ушёл»: %d", r.gone())
	}
	release <- struct{}{} // обработчик заканчивает ответ
	waitFor(t, "сессия закрыта окончательно", func() bool { return r.gone() == 0 })
	if n := len(r.svc.Clients()); n != 0 {
		t.Fatalf("ушедший клиент вернулся в список: %d", n)
	}
}

// TestCloseStartRace: Close и Start одновременно, 200 раз подряд — Start не отказывает
// («нет файла», «занят другой копией»), в конце служба работает.
func TestCloseStartRace(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	var fails atomic.Int32
	for range 200 {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); r.svc.Close() }()
		go func() {
			defer wg.Done()
			if err := r.svc.Start(); err != nil {
				fails.Add(1)
				t.Errorf("Start: %v", err)
			}
		}()
		wg.Wait()
	}
	if fails.Load() != 0 {
		t.Fatalf("отказов Start: %d", fails.Load())
	}
	r.start(t)
	if err := client(t, r.sock, "после-гонки").Ping(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

// TestLegacyInitialize: старый протокол (initialize + initialized), которым идёт и посредник:
// имя клиента в списке, после закрытия — пусто.
func TestLegacyInitialize(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.start(t)
	c := raw(t, r.sock)
	handshake(t, c)
	waitFor(t, "клиент в списке", func() bool { return len(r.svc.Clients()) == 1 })
	if cl := r.svc.Clients()[0]; cl.Name != "raw" {
		t.Fatalf("клиент: %+v", cl)
	}
	_ = c.Close()
	waitFor(t, "список пуст", func() bool { return len(r.svc.Clients()) == 0 && r.conns() == 0 })
}
