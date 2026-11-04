package mcpbridge

import (
	"encoding/json"
	"time"
)

// replay повторяет программе initialize клиента (id replayID), пока он говорил с заглушкой:
// программа должна считать сессию начатой. Версия протокола — та, о которой договорились с
// клиентом (ответ заглушки не выше 2025-11-25, решение 0.31-143-44): иначе программа ответила бы
// новее, и клиент оказался бы на другой ревизии, чем думает. Ответ не пересылается — клиент
// его уже получил; сверяется версия. Не сошлась, ошибка, срок или конец сокета — false,
// остаёмся заглушкой. Клиент ревизии без initialize (2026-07-28) сюда не попадает.
func (b *bridge) replay() bool {
	params, ok := withVersion(b.initParams, b.negotiated)
	if !ok {
		return true // повторять нечего
	}
	line, _ := json.Marshal(reqMsg{JSONRPC: "2.0", ID: replayID, Method: "initialize", Params: params})
	if !b.toProgram(line) {
		b.abandon(textClosed)
		return false
	}
	timer := time.NewTimer(b.cfg.replayWait)
	defer timer.Stop()
	for {
		select {
		case ev := <-b.srv:
			if ev.gen != b.gen {
				continue
			}
			if done, ok := b.replayEvent(ev); done {
				return ok
			}
		case <-timer.C:
			b.abandon(textClosed)
			return false
		}
	}
}

// replayEvent разбирает строку программы во время повтора: done — повтор закончен, ok — удачно.
func (b *bridge) replayEvent(ev srvEv) (done, ok bool) {
	if ev.eof {
		b.abandon(textClosed)
		return true, false
	}
	if ev.big {
		return false, false
	}
	h, kind := parseHeader(ev.line, false)
	if kind != 0 {
		return false, false
	}
	first := b.first
	b.first = false
	if first && isNullError(&h) {
		b.abandon(textTooMany)
		return true, false
	}
	if h.Method != "" || string(h.ID) != `"`+replayID+`"` {
		return false, false
	}
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(h.Error) > 0 || json.Unmarshal(h.Result, &res) != nil || res.ProtocolVersion != b.negotiated {
		b.abandon(textVersion)
		return true, false
	}
	if b.sawInitialized && !b.toProgram(notifyLine("notifications/initialized")) {
		b.abandon(textClosed)
		return true, false
	}
	return true, true
}

// withVersion подменяет protocolVersion в params клиента.
func withVersion(params json.RawMessage, version string) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(params, &m) != nil || m == nil {
		return nil, false
	}
	v, _ := json.Marshal(version)
	m["protocolVersion"] = v
	return m, true
}
