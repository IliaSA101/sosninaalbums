// Package mcpbridge — посредник `SosninaAlbums --mcp` (спека 0.31/143, design §4): строки JSON-RPC
// между stdio агента и сокетом программы, байт в байт. Пока программа закрыта, отвечает сам
// (заглушка stub.go), потом подключается и повторяет клиенту рукопожатие (replay.go).
//
// SDK не нужен: посредник MCP не разбирает, читает только id и method. Ничего не пишет на диск,
// программу не запускает. В stdout — только строки JSON-RPC, тексты — фиксированные (texts.go),
// без путей: stderr клиент пишет в свой журнал.
package mcpbridge

import (
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/peercred"
)

// Пределы и сроки по умолчанию; тесты подменяют через config.
const (
	maxFromClient  = 1 << 20 // клиент → программа: столько же принимает программа
	maxFromProgram = 4 << 20 // программа → клиент: ответ программы ≤ 1 МБ, запас
	probeEvery     = 2 * time.Second
	dialTimeout    = 500 * time.Millisecond
	writeTimeout   = 10 * time.Second
	replayWait     = 10 * time.Second
	maxPending     = 1024
	maxProbeEvery  = 30 * time.Second // потолок паузы проб при стойком отказе программы
)

type config struct {
	socks      []string
	maxIn      int
	maxOut     int
	probeEvery time.Duration
	replayWait time.Duration
	// peer — чья учётная запись на другом конце сокета (148, design §4); тесты подменяют.
	peer func(net.Conn) (bool, error)
}

func defaultConfig(socks []string) config {
	return config{socks: socks, maxIn: maxFromClient, maxOut: maxFromProgram, probeEvery: probeEvery, replayWait: replayWait, peer: peercred.Same}
}

// Run — тело `--mcp`: код выхода процесса. EOF stdin — закрыть сокет и выйти с 0.
func Run(in io.Reader, out, errw io.Writer, socks []string) int {
	return run(defaultConfig(socks), in, out, errw)
}

type cliEv struct {
	line []byte
	big  bool
	head []byte
	eof  bool
}

type srvEv struct {
	gen  int
	line []byte
	big  bool
	head []byte
	eof  bool // конец сокета или ошибка чтения
}

type bridge struct {
	cfg  config
	out  io.Writer
	errw io.Writer
	cli  chan cliEv
	srv  chan srvEv
	done chan struct{}

	conn  net.Conn
	gen   int
	first bool   // следующая строка от программы — первая в этом подключении
	why   string // текст заглушки: почему программы нет
	last  time.Time

	refused   int    // подряд отказов программы (много подключений, версии), от них растёт пауза проб
	loggedWhy string // причина, о которой уже написано в stderr: журнал клиента не засоряем

	pending map[string]json.RawMessage // id ждущих запросов → исходная запись id

	initParams     json.RawMessage // params первого initialize клиента (повтор после запуска программы)
	initKey        string          // ключ id initialize, ответ которого ещё идёт от программы
	negotiated     string          // версия протокола, о которой договорились с клиентом
	initDelivered  bool            // клиент получил ответ на initialize
	sawInitialized bool            // клиент прислал notifications/initialized
	sawOther       bool            // клиент прислал запросы без initialize (ревизия 2026-07-28)

	outErr bool // stdout закрыт клиентом
}

func run(cfg config, in io.Reader, out, errw io.Writer) int {
	b := &bridge{
		cfg: cfg, out: out, errw: errw,
		cli: make(chan cliEv), srv: make(chan srvEv), done: make(chan struct{}),
		pending: map[string]json.RawMessage{}, why: textClosed,
	}
	defer close(b.done)
	defer b.closeConn()
	go b.readClient(in)
	b.maybeProbe(true)
	tick := time.NewTicker(cfg.probeEvery)
	defer tick.Stop()
	for !b.outErr {
		select {
		case ev := <-b.cli:
			if ev.eof {
				return 0
			}
			b.onClient(ev)
		case ev := <-b.srv:
			b.onServer(ev)
		case <-tick.C:
			b.maybeProbe(false)
		}
	}
	return 1
}

// readClient читает stdin строками с пределом; порядок и ожидание — в главном цикле.
func (b *bridge) readClient(in io.Reader) {
	lr := newLineReader(in, b.cfg.maxIn)
	for {
		line, big, head, err := lr.next()
		ev := cliEv{line: line, big: big, head: head, eof: err != nil}
		select {
		case b.cli <- ev:
		case <-b.done:
			return
		}
		if err != nil {
			return
		}
	}
}

// readServer читает сокет программы; gen отсекает события закрытых подключений.
func (b *bridge) readServer(conn net.Conn, gen int) {
	lr := newLineReader(conn, b.cfg.maxOut)
	for {
		line, big, head, err := lr.next()
		ev := srvEv{gen: gen, line: line, big: big, head: head, eof: err != nil}
		select {
		case b.srv <- ev:
		case <-b.done:
			return
		}
		if err != nil {
			return
		}
	}
}

// emit — строка в stdout. Пишет только главный цикл, поэтому строки не перемешиваются.
func (b *bridge) emit(line []byte) {
	if b.outErr {
		return
	}
	buf := make([]byte, 0, len(line)+1)
	buf = append(append(buf, line...), '\n')
	if _, err := b.out.Write(buf); err != nil {
		b.outErr = true
	}
}

func (b *bridge) logf(msg string) {
	_, _ = io.WriteString(b.errw, "sosnina-mcp: "+msg+"\n")
}

func (b *bridge) live() bool { return b.conn != nil }

func (b *bridge) closeConn() {
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
	b.gen++ // события этого подключения больше не нужны
}

// toProgram — строка в сокет. false — запись не удалась: программа считается закрытой.
func (b *bridge) toProgram(line []byte) bool {
	if b.conn == nil {
		return false
	}
	_ = b.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	buf := make([]byte, 0, len(line)+1)
	buf = append(append(buf, line...), '\n')
	_, err := b.conn.Write(buf)
	return err == nil
}

// failPending отвечает всем ждущим id ошибкой с текстом и очищает набор.
func (b *bridge) failPending(msg string) {
	for k, id := range b.pending {
		b.emit(errorLine(id, codeServer, msg))
		delete(b.pending, k)
	}
	b.initKey = ""
}

// ready — клиент уже закончил рукопожатие или работает без него: ему можно слать list_changed.
func (b *bridge) ready() bool { return b.initDelivered || b.sawOther }

// toStub — программа недоступна: ждущим ошибка, дальше отвечает заглушка; клиенту — list_changed
// (его список инструментов сменился).
func (b *bridge) toStub(why, pendingMsg string) {
	b.closeConn()
	b.why = why
	b.noteRefusal(why)
	b.failPending(pendingMsg)
	if b.ready() {
		b.emit(notifyLine("notifications/tools/list_changed"))
	}
	b.logDown(why)
}

// noteRefusal считает стойкие отказы программы: от них растёт пауза проб.
func (b *bridge) noteRefusal(why string) {
	if why == textTooMany || why == textVersion {
		b.refused++
	}
}

// logDown пишет в stderr о недоступности программы, только если причина сменилась.
func (b *bridge) logDown(why string) {
	if b.loggedWhy != why {
		b.loggedWhy = why
		b.logf("программа недоступна")
	}
}

// probeInterval — пауза между пробами: probeEvery, после отказов программы вдвое дольше каждый
// раз, не больше maxProbeEvery (программа держит 16 подключений или версии не сходятся — звонить
// каждые 2 с незачем).
func (b *bridge) probeInterval() time.Duration {
	d := b.cfg.probeEvery
	for i := 0; i < b.refused && d < maxProbeEvery; i++ {
		d *= 2
	}
	return min(d, maxProbeEvery)
}
