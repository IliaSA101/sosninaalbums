package mcpbridge

import (
	"errors"
	"net"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/peercred"
)

// maybeProbe — проба сокета, пока программы нет; не чаще раза в probeEvery (на входящем
// сообщении и по таймеру). force — первая проба при запуске.
func (b *bridge) maybeProbe(force bool) {
	if b.live() {
		return
	}
	now := time.Now()
	if !force && now.Sub(b.last) < b.probeInterval() {
		return
	}
	b.last = now
	conn, why, note := b.dial()
	if conn == nil {
		b.why = why
		if note == "" {
			b.refused = 0 // сокета нет или не годен — это не отказ программы
			if isPeerNote(b.loggedWhy) {
				b.loggedWhy = "" // чужой ушёл: вернётся — строка в stderr снова нужна
			}
			return
		}
		b.refused++ // слушает чужой: пауза проб растёт, как при отказе программы (148-3)
		b.logPeer(note)
		return
	}
	b.attach(conn)
}

// dial обходит список путей: перед каждым dial — проверка папки и файла (checkSock); небезопасный
// путь не открывается. После dial — сверка учётной записи слушателя: чужой или ошибка запроса к ОС
// (в том числе «не поддерживается») — закрыть, не прочитав и не записав ни байта (148, design §4).
// Причина отказа — для заглушки; note — строка stderr, если отказал собеседник сокета.
func (b *bridge) dial() (conn net.Conn, why, note string) {
	if len(b.cfg.socks) == 0 {
		return nil, textNoDir, ""
	}
	why = textClosed
	for _, p := range b.cfg.socks {
		present, release, err := checkSock(p)
		if err != nil {
			why = textUnsafe
			continue
		}
		if !present {
			continue
		}
		c, w, n := b.dialHeld(p, release)
		if c != nil {
			return c, "", ""
		}
		if n != "" {
			why, note = w, n
		}
	}
	return nil, why, note
}

// dialHeld — dial и сверка собеседника, пока checkSock держит папку сокета (Windows: дескриптор
// без FILE_SHARE_DELETE, 0.32-148-11): подменить предка между проверкой и сверкой нельзя. release
// звать ровно здесь, после сверки. conn == nil и пустая note — dial не удался, не отказ собеседника.
func (b *bridge) dialHeld(p string, release func()) (conn net.Conn, why, note string) {
	defer release()
	c, err := net.DialTimeout("unix", p, dialTimeout)
	if err != nil {
		return nil, "", ""
	}
	if w, n := b.checkPeer(c); n != "" {
		_ = c.Close()
		return nil, w, n
	}
	return c, "", ""
}

// checkPeer — слушатель сокета той же учётной записи? Иначе текст заглушки и строка stderr.
// Без функции сверки (нулевой config) подключение не годится: закрыто по умолчанию.
func (b *bridge) checkPeer(c net.Conn) (why, note string) {
	if b.cfg.peer == nil {
		return textUnsafe, logPeerFailed
	}
	ok, err := b.cfg.peer(c)
	switch {
	case err == nil && ok:
		return "", ""
	case err == nil:
		return textUnsafe, logPeerForeign
	case errors.Is(err, peercred.ErrTokenDenied):
		return textElevated, textElevated // Windows: процесс «от администратора» (Г-в)
	case errors.Is(err, peercred.ErrUnsupported):
		return textNoCheck, logPeerUnsupported // «выключите и включите» не поможет: нужна другая Windows
	default:
		return textUnsafe, logPeerFailed
	}
}

// isPeerNote — причина в loggedWhy записана отказом по собеседнику сокета.
func isPeerNote(s string) bool {
	return s == logPeerForeign || s == logPeerFailed || s == logPeerUnsupported || s == textElevated
}

// logPeer пишет в stderr об отказе по собеседнику, только если причина сменилась.
func (b *bridge) logPeer(note string) {
	if b.loggedWhy != note {
		b.loggedWhy = note
		b.logf(note)
	}
}

// attach включает пересылку. Клиент уже прошёл рукопожатие с заглушкой — повторяем его
// программе (replay.go); затем клиенту list_changed: список инструментов сменился.
func (b *bridge) attach(conn net.Conn) {
	b.conn = conn
	b.gen++
	b.first = true
	go b.readServer(conn, b.gen)
	if b.initParams != nil && b.initDelivered && b.negotiated != "" {
		if !b.replay() {
			return
		}
		b.connected() // повтор прошёл: программа приняла сессию
	}
	if b.ready() {
		b.emit(notifyLine("notifications/tools/list_changed"))
	}
}

// abandon — подключение не состоялось (повтор рукопожатия не сошёлся): остаёмся заглушкой,
// клиенту ничего не сообщаем — для него ничего не менялось.
func (b *bridge) abandon(why string) {
	b.closeConn()
	b.why = why
	b.noteRefusal(why)
	b.logDown(why)
}

// connected — программа приняла подключение: отказы забыты, в stderr одна строка.
func (b *bridge) connected() {
	b.refused = 0
	b.loggedWhy = ""
	b.logf("программа подключена")
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
		if id, ok := extractID(ev.head, false); ok {
			if key, ok := idKey(id); ok {
				if _, waits := b.pending[key]; waits {
					delete(b.pending, key)
					b.emit(errorLine(id, codeServer, textBigResp))
				}
			}
		}
		return
	}
	h, kind := parseHeader(ev.line, false)
	if kind != 0 {
		return // не JSON-объект: клиенту не пересылаем
	}
	if first && isNullError(&h) {
		b.toStub(textTooMany, textTooMany) // лишнее подключение: программа закроет сокет
		return
	}
	if first {
		b.connected()
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
