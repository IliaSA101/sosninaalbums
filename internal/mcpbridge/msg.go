package mcpbridge

import (
	"bytes"
	"encoding/json"
	"strings"
)

// maxIDLen — длиннейший принимаемый id (байт): длиннее — запрос не пересылается. Предел нужен,
// потому что id хранится в наборе ждущих.
const maxIDLen = 256

// header — то, что посредник читает из строки: не больше этого.
type header struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// hasID — у сообщения есть id, не null (запрос или ответ, не уведомление).
func (h *header) hasID() bool {
	return len(h.ID) > 0 && string(h.ID) != "null"
}

func (h *header) isRequest() bool { return h.Method != "" && h.hasID() }

// parseHeader разбирает строку. kind: 0 — разобрана; codeParse — не JSON; codeInvalid — не объект.
func parseHeader(line []byte) (h header, kind int) {
	t := bytes.TrimLeft(line, " \t")
	if len(t) == 0 || t[0] != '{' {
		if json.Valid(line) {
			return h, codeInvalid
		}
		return h, codeParse
	}
	if err := json.Unmarshal(line, &h); err != nil {
		return header{}, codeParse
	}
	return h, 0
}

// idKey — ключ id для набора ждущих: строка и число различаются, запись не важна (сервер
// перекодирует строки). ok=false — id нельзя использовать (не строка и не число, слишком длинный).
func idKey(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || len(raw) > maxIDLen {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return "s" + x, true
	case json.Number:
		return "n" + x.String(), true
	}
	return "", false
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type errMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   rpcError        `json:"error"`
}

type resMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type reqMsg struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// errorLine — ответ-ошибка; id пуст — null.
func errorLine(id json.RawMessage, code int, msg string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(errMsg{"2.0", id, rpcError{code, msg}})
	return b
}

func resultLine(id json.RawMessage, result any) []byte {
	b, _ := json.Marshal(resMsg{"2.0", id, result})
	return b
}

func notifyLine(method string) []byte {
	b, _ := json.Marshal(reqMsg{JSONRPC: "2.0", Method: method})
	return b
}

// stubVersion — версия, которой заглушка отвечает на initialize.
func stubVersion(requested string) string {
	for _, v := range stubVersions {
		if v == requested {
			return v
		}
	}
	return stubLatest
}

// isNullError — строка вида {"id":null,"error":{…}}: ответ программы лишнему подключению.
func isNullError(h *header) bool {
	return h.Method == "" && len(h.Error) > 0 && !h.hasID() && strings.TrimSpace(string(h.Error)) != "null"
}
