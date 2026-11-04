package mcpbridge

import (
	"bytes"
	"encoding/json"
)

// onClient — одна строка от клиента (stdin). Программа доступна — строка уходит в сокет как есть;
// нет — отвечает заглушка. Из строки читаются только id, method и params у initialize.
func (b *bridge) onClient(ev cliEv) {
	if ev.big {
		// Строка длиннее предела: не хранится и не пересылается; id — из её начала.
		if id, ok := extractID(ev.head, true); ok {
			b.emit(errorLine(id, codeServer, textBigReq))
		}
		return
	}
	h, kind := parseHeader(ev.line, true)
	switch kind {
	case codeParse:
		b.emit(errorLine(nil, codeParse, textParse))
		return
	case codeInvalid:
		b.emit(errorLine(nil, codeInvalid, textBadReq))
		return
	}
	if !h.isRequest() {
		b.oneWay(&h, ev.line)
		return
	}
	key, ok := idKey(h.ID)
	if _, dup := b.pending[key]; !ok || dup {
		b.emit(errorLine(nil, codeInvalid, textBadID))
		return
	}
	if h.Method == "initialize" {
		b.saveInit(&h)
	} else if h.Method != "ping" {
		b.sawOther = true
	}
	if !b.live() {
		b.stub(&h)
		b.maybeProbe(false) // клиент заговорил — не ждём таймер, но не чаще раза в probeEvery
		return
	}
	if len(b.pending) >= maxPending {
		b.emit(errorLine(h.ID, codeServer, textBusy))
		return
	}
	b.pending[key] = bytes.Clone(h.ID)
	if h.Method == "initialize" {
		b.initKey = key
	}
	if !b.toProgram(ev.line) {
		b.toStub(textClosed, textGone) // ждущим, в том числе этому, уйдёт ошибка
	}
}

// oneWay — уведомление или ответ клиента на запрос программы: в заглушке гасятся (кроме
// notifications/initialized, он нужен для повтора), в живой сессии идут в сокет как есть.
func (b *bridge) oneWay(h *header, line []byte) {
	if h.Method == "notifications/initialized" {
		b.sawInitialized = true
	}
	if b.live() && !b.toProgram(line) {
		b.toStub(textClosed, textGone)
	}
}

// saveInit запоминает initialize клиента для повтора после запуска программы. Годятся только
// params-объект: иначе повторять нечего.
func (b *bridge) saveInit(h *header) {
	b.sawInitialized = false
	b.initDelivered = false
	b.initParams = nil
	b.negotiated = ""
	var probe map[string]json.RawMessage
	if json.Unmarshal(h.Params, &probe) == nil && probe != nil {
		b.initParams = bytes.Clone(h.Params)
	}
}

// noteInit — ответ программы на initialize клиента: договорённая версия протокола.
func (b *bridge) noteInit(h *header) {
	if len(h.Error) > 0 && string(h.Error) != "null" {
		b.initParams = nil
		return
	}
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(h.Result, &res) != nil || res.ProtocolVersion == "" {
		b.initParams = nil
		return
	}
	b.negotiated = res.ProtocolVersion
	b.initDelivered = true
}
