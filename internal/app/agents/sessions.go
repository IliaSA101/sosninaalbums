package agents

import (
	"context"
	"log"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxClientName — предел имени клиента в списке подключённых, знаков.
const maxClientName = 40

// tooMany — ответ лишнему подключению: одна строка JSON-RPC; посредник покажет её текст
// в program_status вместо «программа не запущена».
const tooMany = `{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"слишком много подключений к Sosnina Albums — закройте лишние окна агентов"}}` + "\n"

// run — один запуск службы: слушатель, подключения, сервер SDK. Close делает его одноразовым:
// следующий Start собирает новый.
type run struct {
	s      *Service
	o      Options
	ln     net.Listener
	srv    *mcp.Server
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // приём, сессии и ответы лишним

	mu     sync.Mutex
	closed bool
	conns  map[net.Conn]struct{}           // занятые места
	seen   map[*mcp.ServerSession]Client   // прошедшие initialize
	gone   map[*mcp.ServerSession]struct{} // ушли, обработчики ещё идут
}

// admit принимает подключение: место есть — сессия в своей горутине, нет — текст и закрытие.
func (r *run) admit(c net.Conn) {
	defer func() {
		if recover() != nil { // паника приёма не должна оставить открытое подключение
			_ = c.Close()
			log.Printf("ПАНИКА в agents admit")
		}
	}()
	r.mu.Lock()
	switch {
	case r.closed:
		r.mu.Unlock()
		_ = c.Close()
	case len(r.conns) >= r.o.MaxConns:
		r.wg.Add(1)
		r.mu.Unlock()
		go r.reject(c)
	default:
		r.conns[c] = struct{}{}
		r.wg.Add(1)
		r.mu.Unlock()
		go r.session(c)
	}
}

// reject пишет лишнему подключению текст и закрывает. Сначала закрывается запись и дочитывается
// то, что клиент успел отправить: close с непрочитанными данными шлёт RST и клиент теряет текст.
func (r *run) reject(c net.Conn) {
	defer r.wg.Done()
	defer guardPanic("reject")
	defer func() { _ = c.Close() }()
	_ = c.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := c.Write([]byte(tooMany)); err != nil {
		return
	}
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
	_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 4096)
	for n := 0; n < maxLine; {
		k, err := c.Read(buf)
		if err != nil {
			return
		}
		n += k
	}
}

// watch — читающая сторона подключения: первая ошибка Read (клиент ушёл, сокет закрыт, предел строки)
// освобождает место и убирает клиента из списка сразу. Ждать ss.Wait нельзя: он ждёт и обработчики,
// а застрявший (рендер до 20 с) держал бы место отключившегося агента.
type watch struct {
	net.Conn
	r    *run
	ss   atomic.Pointer[mcp.ServerSession]
	lost atomic.Bool
}

func (w *watch) Read(p []byte) (int, error) {
	n, err := w.Conn.Read(p)
	if err != nil {
		w.drop()
	}
	return n, err
}

func (w *watch) drop() {
	if w.lost.CompareAndSwap(false, true) {
		w.r.release(w.Conn, w.ss.Load())
	}
}

// session ведёт подключение до его конца.
func (r *run) session(c net.Conn) {
	defer r.wg.Done()
	defer guardPanic("session")
	w := &watch{Conn: c, r: r}
	defer w.drop() // и при панике: место не должно остаться занятым
	defer func() { _ = c.Close() }()
	ss, err := r.srv.Connect(r.ctx, &mcp.IOTransport{
		Reader: w, Writer: deadlineConn{c, r.o.WriteTimeout}, MaxLineLength: maxLine,
	}, nil)
	if err != nil {
		log.Printf("agents: сессия не открылась: %v", bare(err))
		return
	}
	w.ss.Store(ss)
	if w.lost.Load() { // клиент ушёл, пока шёл Connect
		r.release(c, ss)
	}
	defer r.forget(ss)
	// Без initialize подключение молчуна не живёт вечно.
	t := time.AfterFunc(r.o.InitTimeout, func() {
		if ss.InitializeParams() == nil {
			_ = c.Close()
		}
	})
	defer t.Stop()
	_ = ss.Wait()
	_ = c.Close()
	w.drop()
	_ = ss.Close() // ждёт обработчики; место уже свободно
}

// release возвращает место и убирает клиента из списка; повтор безвреден. Ушедший клиент
// помечен: запоздалый note от ещё идущего обработчика его не вернёт.
func (r *run) release(c net.Conn, ss *mcp.ServerSession) {
	r.mu.Lock()
	_, had := r.conns[c]
	delete(r.conns, c)
	if ss != nil {
		_, seen := r.seen[ss]
		delete(r.seen, ss)
		r.gone[ss] = struct{}{}
		had = had || seen
	}
	r.mu.Unlock()
	if had {
		r.s.changed()
	}
}

// forget снимает пометку «ушёл», когда сессия закрыта окончательно (обработчиков больше нет).
func (r *run) forget(ss *mcp.ServerSession) {
	r.mu.Lock()
	delete(r.gone, ss)
	delete(r.seen, ss)
	r.mu.Unlock()
}

// errInternal — ответ клиенту при панике в приёмном слое или обработчике.
var errInternal = newAgentErr("внутренняя ошибка программы")

// noteClient — приёмный слой SDK. Клиент попадает в список, как только у сессии есть параметры
// initialize (старый протокол) или server/discover (2026-07-28): до запроса — если они уже есть
// (долгий первый tools/call виден сразу), после — если запрос их поставил. Отдельного
// «initialized» у нового протокола нет, поэтому InitializedHandler не годится.
//
// Паника: обработчики SDK идут в горутинах jsonrpc2, где recover нет, а паника там роняет
// программу с несохранённым альбомом. Слой добавляется последним, то есть он внешний.
func (r *run) noteClient(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (res mcp.Result, err error) {
		ss, _ := req.GetSession().(*mcp.ServerSession)
		if ss != nil {
			r.note(ss)
		}
		defer func() {
			if p := recover(); p != nil {
				logPanic("handler", p)
				res, err = nil, errInternal
			}
		}()
		res, err = next(ctx, method, req)
		if ss != nil {
			r.note(ss)
		}
		return res, err
	}
}

func (r *run) note(ss *mcp.ServerSession) {
	defer guardPanic("note")
	ip := ss.InitializeParams()
	if ip == nil {
		return
	}
	r.mu.Lock()
	_, had := r.seen[ss]
	_, gone := r.gone[ss]
	add := !had && !gone && !r.closed
	if add {
		name := ""
		if ip.ClientInfo != nil {
			name = cleanName(ip.ClientInfo.Name)
		}
		r.seen[ss] = Client{Name: name, Since: time.Now()}
	}
	r.mu.Unlock()
	if add {
		r.s.changed()
	}
}

// clients — прошедшие initialize, по времени подключения.
func (r *run) clients() []Client {
	r.mu.Lock()
	out := make([]Client, 0, len(r.seen))
	for _, c := range r.seen {
		out = append(out, c)
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Since.Before(out[j].Since) })
	return out
}

// close закрывает слушатель (файл сокета удаляет Go), все подключения и ждёт горутины не
// дольше CloseWait.
func (r *run) close() {
	r.mu.Lock()
	r.closed = true
	conns := make([]net.Conn, 0, len(r.conns))
	for c := range r.conns {
		conns = append(conns, c)
	}
	r.mu.Unlock()
	r.cancel()
	_ = r.ln.Close()
	for _, c := range conns {
		_ = c.Close()
	}
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(r.o.CloseWait):
		log.Printf("agents: часть сессий не закрылась за %v, выход не ждёт", r.o.CloseWait)
	}
}

// deadlineConn — запись с сроком: клиент, который не читает, не держит сессию вечно. Ошибка
// записи (в том числе срок) закрывает подключение.
type deadlineConn struct {
	net.Conn
	d time.Duration
}

func (w deadlineConn) Write(p []byte) (int, error) {
	_ = w.SetWriteDeadline(time.Now().Add(w.d))
	n, err := w.Conn.Write(p)
	if err != nil {
		_ = w.Close()
	}
	return n, err
}

// cleanName — имя клиента для списка: без управляющих и форматных знаков (разворот строки,
// невидимые), не длиннее maxClientName знаков, битый UTF-8 пропускается.
func cleanName(s string) string {
	out := make([]rune, 0, maxClientName)
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if len(out) == maxClientName {
			break
		}
		out = append(out, r)
	}
	return string(out)
}
