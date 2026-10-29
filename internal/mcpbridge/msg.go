package mcpbridge

import (
	"encoding/json"
	"strconv"
	"strings"
)

// maxIDLen — длиннейший принимаемый id (байт): длиннее — запрос не пересылается. Предел нужен,
// потому что id хранится в наборе ждущих.
const maxIDLen = 256

// header — то, что посредник читает из строки: не больше этого.
type header struct {
	Method string
	ID     json.RawMessage
	Params json.RawMessage
	Result json.RawMessage
	Error  json.RawMessage
}

// hasID — у сообщения есть id, не null (запрос или ответ, не уведомление).
func (h *header) hasID() bool {
	return len(h.ID) > 0 && string(h.ID) != "null"
}

func (h *header) isRequest() bool { return h.Method != "" && h.hasID() }

// maxSafeInt — наибольшее целое id: как у go-sdk (float64 → int64), дальше числа теряют точность.
const maxSafeInt = 1 << 53

// goodID — id, который программа вернёт тем же: строка или целое в канонической записи, |n| ≤ 2^53.
// «1.0», «1e1», «-0», «1.5», «1e999» программа (go-sdk читает id через float64) вернёт другой
// записью или оборвёт подключение — ответ не нашёл бы своего ждущего.
func goodID(raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > maxIDLen {
		return false
	}
	if raw[0] == '"' {
		var s string
		return json.Unmarshal(raw, &s) == nil
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	return err == nil && n >= -maxSafeInt && n <= maxSafeInt && strconv.FormatInt(n, 10) == string(raw)
}

// parseHeader разбирает строку. kind: 0 — разобрана; codeParse — не JSON; codeInvalid — не
// объект или не годится. strict — строка клиента, правила разборщика программы (go-sdk,
// segmentio): "jsonrpc":"2.0", ключи с учётом регистра, method — непустая строка, id — строка
// или целое (goodID), ответ — с годным id и result или error. Иначе программа закрыла бы
// подключение или потеряла ответ. Строки программы читаются без строгости (strict=false).
func parseHeader(line []byte, strict bool) (h header, kind int) {
	var m map[string]json.RawMessage // ключи сверяются точно, с учётом регистра
	if err := json.Unmarshal(line, &m); err != nil {
		if json.Valid(line) {
			return h, codeInvalid
		}
		return h, codeParse
	}
	if m == nil { // литерал null
		return h, codeInvalid
	}
	h = header{ID: m["id"], Params: m["params"], Result: m["result"], Error: m["error"]}
	if raw, ok := m["method"]; ok {
		if json.Unmarshal(raw, &h.Method) != nil || (strict && h.Method == "") {
			return header{}, codeInvalid
		}
	}
	if !strict {
		return h, 0
	}
	if string(m["jsonrpc"]) != `"2.0"` {
		return header{}, codeInvalid
	}
	if h.hasID() && !goodID(h.ID) {
		return header{}, codeInvalid
	}
	if h.Method == "" && (!h.hasID() || (len(h.Result) == 0 && len(h.Error) == 0)) {
		return header{}, codeInvalid
	}
	return h, 0
}

// idKey — ключ id для набора ждущих: строка и число различаются. ok=false — id нельзя
// использовать (не строка и не годное целое, слишком длинный).
func idKey(raw json.RawMessage) (string, bool) {
	if !goodID(raw) {
		return "", false
	}
	if raw[0] == '"' {
		var s string
		_ = json.Unmarshal(raw, &s) // сервер перекодирует строки: сверяем значение, не запись
		return "s" + s, true
	}
	return "n" + string(raw), true
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
