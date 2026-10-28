package mcpbridge

import (
	"net"
	"time"
)

// maybeProbe — проба сокета, пока программы нет; не чаще раза в probeEvery (на входящем
// сообщении и по таймеру). force — первая проба при запуске.
func (b *bridge) maybeProbe(force bool) {
	if b.live() {
		return
	}
	now := time.Now()
	if !force && now.Sub(b.last) < b.cfg.probeEvery {
		return
	}
	b.last = now
	conn, why := b.dial()
	if conn == nil {
		b.why = why
		return
	}
	b.attach(conn)
}

// dial обходит список путей: перед каждым dial — проверка папки и файла (checkSock); небезопасный
// путь не открывается. Причина отказа — для заглушки.
func (b *bridge) dial() (net.Conn, string) {
	if len(b.cfg.socks) == 0 {
		return nil, textNoDir
	}
	why := textClosed
	for _, p := range b.cfg.socks {
		present, err := checkSock(p)
		if err != nil {
			why = textUnsafe
			continue
		}
		if !present {
			continue
		}
		if c, err := net.DialTimeout("unix", p, dialTimeout); err == nil {
			return c, ""
		}
	}
	return nil, why
}

// attach включает пересылку. Клиент уже прошёл рукопожатие с заглушкой — повторяем его
// программе (replay.go); затем клиенту list_changed: список инструментов сменился.
func (b *bridge) attach(conn net.Conn) {
	b.conn = conn
	b.gen++
	b.first = true
	go b.readServer(conn, b.gen)
	if b.initParams != nil && b.initDelivered && b.negotiated != "" && !b.replay() {
		return
	}
	b.logf("программа подключена")
	if b.ready() {
		b.emit(notifyLine("notifications/tools/list_changed"))
	}
}

// abandon — подключение не состоялось (повтор рукопожатия не сошёлся): остаёмся заглушкой,
// клиенту ничего не сообщаем — для него ничего не менялось.
func (b *bridge) abandon(why string) {
	b.closeConn()
	b.why = why
	b.logf("программа недоступна")
}

// onServer — одна строка от программы (сокет).
func (b *bridge) onServer(ev srvEv) {
	if ev.gen != b.gen || !b.live() {
		return
	}
	if ev.eof {
		b.toStub(textClosed, textGone)
		return
	}
	first := b.first
	b.first = false
	if ev.big {
		// Ответ длиннее предела: не пересылается; ждущему id — ошибка, подключение живо.
		if id, ok := extractID(ev.head); ok {
			if key, ok := idKey(id); ok {
				if _, waits := b.pending[key]; waits {
					delete(b.pending, key)
					b.emit(errorLine(id, codeServer, textBigResp))
				}
			}
		}
		return
	}
	h, kind := parseHeader(ev.line)
	if kind != 0 {
		return // не JSON-объект: клиенту не пересылаем
	}
	if first && isNullError(&h) {
		b.toStub(textTooMany, textTooMany) // лишнее подключение: программа закроет сокет
		return
	}
	if h.Method == "" && h.hasID() { // ответ: снимаем из ждущих
		key, ok := idKey(h.ID)
		if !ok {
			return
		}
		if _, waits := b.pending[key]; !waits {
			return // на это мы не спрашивали
		}
		delete(b.pending, key)
		if key == b.initKey {
			b.initKey = ""
			b.noteInit(&h)
		}
	}
	b.emit(ev.line)
}
