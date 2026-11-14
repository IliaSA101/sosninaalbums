package mcpbridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type lineRes struct {
	line string
	big  bool
	head string
}

func readAll(in string, limit int) ([]lineRes, error) {
	lr := newLineReader(strings.NewReader(in), limit)
	var out []lineRes
	for {
		l, big, head, err := lr.next()
		if err != nil {
			return out, err
		}
		out = append(out, lineRes{string(l), big, string(head)})
	}
}

// TestReadLine: края разбора строк — пустой ввод, без перевода в конце, CRLF, пустые строки,
// ровно предел и предел+1, длинная строка в конце ввода без перевода.
func TestReadLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want []lineRes
	}{
		{"пусто", "", 10, nil},
		{"одна", "abc\n", 10, []lineRes{{"abc", false, ""}}},
		{"без перевода в конце", "abc", 10, []lineRes{{"abc", false, ""}}},
		{"CRLF", "a\r\nb\r\n", 10, []lineRes{{"a", false, ""}, {"b", false, ""}}},
		{"пустые строки пропущены", "\n\na\n\n", 10, []lineRes{{"a", false, ""}}},
		{"ровно предел", "0123456789\n", 10, []lineRes{{"0123456789", false, ""}}},
		{"предел+1", "01234567890\nok\n", 10, []lineRes{{"", true, "01234567890\n"}, {"ok", false, ""}}},
		{"длинная в конце без перевода", "ok\n01234567890123", 10, []lineRes{{"ok", false, ""}, {"", true, "01234567890123"}}},
		{"две длинные подряд", strings.Repeat("x", 30) + "\n" + strings.Repeat("y", 30) + "\nz\n", 10,
			[]lineRes{{"", true, strings.Repeat("x", 30) + "\n"}, {"", true, strings.Repeat("y", 30) + "\n"}, {"z", false, ""}}},
	}
	for _, tc := range tests {
		got, err := readAll(tc.in, tc.max)
		if !errors.Is(err, io.EOF) {
			t.Errorf("%s: ошибка %v, ждали EOF", tc.name, err)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: %d строк, ждали %d: %+v", tc.name, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s[%d]: %+v, ждали %+v", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestReadLineHuge: строка в десятки раз больше предела не копится: голова не больше headKeep,
// следующая строка читается нормально. Байты с нулями и мусор — без паники.
func TestReadLineHuge(t *testing.T) {
	in := bytes.Repeat([]byte{0, 0xff, '{'}, 3<<20) // ~9 МБ без перевода строки внутри
	in = append(in, '\n')
	in = append(in, "{\"id\":1}\n"...)
	lr := newLineReader(bytes.NewReader(in), 1<<20)
	line, big, head, err := lr.next()
	if err != nil || !big || len(line) != 0 || len(head) != headKeep {
		t.Fatalf("big=%v head=%d err=%v", big, len(head), err)
	}
	line, big, _, err = lr.next()
	if err != nil || big || string(line) != `{"id":1}` {
		t.Fatalf("следом: %q big=%v err=%v", line, big, err)
	}
}

// TestExtractID: id верхнего уровня из начала длинной строки; вложенный id не годится; у строки
// клиента (needMethod) нужен ещё верхний method.
func TestExtractID(t *testing.T) {
	tests := []struct {
		name, in, want string
		needMethod     bool
		ok             bool
	}{
		{"число", `{"jsonrpc":"2.0","id":7,"method":"x","params":{"a":"`, "7", true, true},
		{"строка", `{"id":"abc","method":"x"`, `"abc"`, true, true},
		{"после params", `{"params":{"id":5},"id":9,"method":"x"}`, "9", true, true},
		{"id после method", `{"method":"x","id":9,"params":{"a":"`, "9", true, true},
		{"вложенный не считается", `{"params":{"id":5},"method":"x","pa`, "", true, false},
		{"params оборван до id", `{"params":{"a":"` + strings.Repeat("x", 100), "", true, false},
		{"ответ клиента без method — не отвечаем", `{"jsonrpc":"2.0","id":9,"result":{"a":"`, "", true, false},
		{"без method, но он не нужен", `{"jsonrpc":"2.0","id":9,"result":{"a":"`, "9", false, true},
		{"method после обрыва", `{"id":9,"params":{"a":"` + strings.Repeat("x", 100), "", true, false},
		{"null", `{"id":null}`, "", false, false},
		{"объект вместо id", `{"id":{"a":1}}`, "", false, false},
		{"дробный id", `{"id":1.5,"method":"x"}`, "", true, false},
		{"id 1.0", `{"id":1.0,"method":"x"}`, "", true, false},
		{"id 1e999", `{"id":1e999,"method":"x"}`, "", true, false},
		{"массив", `[1,2]`, "", false, false},
		{"мусор", "\x00\xff{", "", false, false},
		{"пусто", ``, "", false, false},
		{"id длиннее предела", `{"id":"` + strings.Repeat("a", 300) + `"}`, "", false, false},
	}
	for _, tc := range tests {
		got, ok := extractID([]byte(tc.in), tc.needMethod)
		if ok != tc.ok || string(got) != tc.want {
			t.Errorf("%s: %q ok=%v, ждали %q ok=%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// TestParseStrict: строка клиента проверяется по правилам разборщика программы (go-sdk): иначе она
// закрыла бы подключение или потеряла ответ (id 1.0 вернётся как 1). Годные и негодные случаи.
func TestParseStrict(t *testing.T) {
	good := []string{
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":"a","method":"ping"}`,
		`{"jsonrpc":"2.0","id":-5,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":0,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":9007199254740992,"method":"ping"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":null,"method":"notifications/x"}`,
		`{"jsonrpc":"2.0","id":7,"result":{}}`,
		`{"jsonrpc":"2.0","id":"s","error":{"code":1,"message":"m"}}`,
	}
	for _, l := range good {
		if _, kind := parseHeader([]byte(l), true); kind != 0 {
			t.Errorf("годная строка отвергнута (%d): %s", kind, l)
		}
	}
	bad := map[string]string{
		"{}":                       "пустой объект",
		`{"id":7,"method":"ping"}`: "нет jsonrpc",
		`{"jsonrpc":"1.0","id":7,"method":"ping"}`:                 "версия не 2.0",
		`{"jsonrpc":2.0,"id":7,"method":"ping"}`:                   "jsonrpc числом",
		`{"JSONRPC":"2.0","id":7,"method":"ping"}`:                 "ключ другого регистра",
		`{"jsonrpc":"2.0","ID":7,"Method":"ping"}`:                 "ключи другого регистра",
		`{"jsonrpc":"2.0","id":1.0,"method":"ping"}`:               "id 1.0",
		`{"jsonrpc":"2.0","id":1.5,"method":"ping"}`:               "id 1.5",
		`{"jsonrpc":"2.0","id":1e1,"method":"ping"}`:               "id 1e1",
		`{"jsonrpc":"2.0","id":-0,"method":"ping"}`:                "id -0",
		`{"jsonrpc":"2.0","id":1e999,"method":"ping"}`:             "id 1e999",
		`{"jsonrpc":"2.0","id":9007199254740993,"method":"p"}`:     "id больше 2^53",
		`{"jsonrpc":"2.0","id":99999999999999999999,"method":"p"}`: "id больше int64",
		`{"jsonrpc":"2.0","id":01,"method":"ping"}`:                "id с нулём впереди",
		`{"jsonrpc":"2.0","id":true,"method":"ping"}`:              "id булев",
		`{"jsonrpc":"2.0","id":[1],"method":"ping"}`:               "id массив",
		`{"jsonrpc":"2.0","id":7,"method":5}`:                      "method числом",
		`{"jsonrpc":"2.0","id":7,"method":""}`:                     "method пустой",
		`{"jsonrpc":"2.0","result":{}}`:                            "ответ без id",
		`{"jsonrpc":"2.0","id":null,"result":{}}`:                  "ответ с id null",
		`{"jsonrpc":"2.0","id":7}`:                                 "ни method, ни result",
		`{"jsonrpc":"2.0","id":7,"result":{}`:                      "оборванный JSON",
		`[1]`:                                                      "массив",
		`null`:                                                     "null",
		`"x"`:                                                      "строка",
	}
	for l, why := range bad {
		if _, kind := parseHeader([]byte(l), true); kind == 0 {
			t.Errorf("негодная строка принята (%s): %s", why, l)
		}
	}
	// Строки программы читаются без строгости: ответ лишнему подключению с id null.
	if h, kind := parseHeader([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"m"}}`), false); kind != 0 || !isNullError(&h) {
		t.Errorf("ответ лишнему подключению: %v %d", h, kind)
	}
}

// FuzzLines: любой ввод и любые пределы — без паники, память не больше предела + буфера,
// каждая отданная строка не длиннее предела.
func FuzzLines(f *testing.F) {
	f.Add([]byte("{\"id\":1}\n\n{\"id\":"), uint8(8))
	f.Add([]byte(strings.Repeat("a", 200)+"\n"), uint8(3))
	f.Add([]byte("\r\n\x00\xff"), uint8(1))
	f.Fuzz(func(t *testing.T, in []byte, lim uint8) {
		m := int(lim) + 1
		lr := newLineReader(bytes.NewReader(in), m)
		for range len(in) + 2 {
			line, big, head, err := lr.next()
			if err != nil {
				return
			}
			if len(line) > m || len(head) > headKeep || (big && len(line) != 0) {
				t.Fatalf("предел нарушен: line=%d head=%d max=%d", len(line), len(head), m)
			}
			if big {
				extractID(head, true)
			}
			if !big {
				parseHeader(line, true)
			}
		}
	})
}

// FuzzStubInput: любой stdin при отсутствующей программе — без паники и зависания, код 0, а
// каждая строка stdout — объект JSON-RPC 2.0 (чужой ввод не портит поток протокола).
func FuzzStubInput(f *testing.F) {
	f.Add([]byte(initLine + "\n{\"id\":2,\"method\":\"tools/list\"}\n"))
	f.Add([]byte("{\"id\":1e999,\"method\":\"ping\"}\n[[[[\n\x00"))
	f.Add([]byte("{\"id\":\"\\ud800\",\"method\":\"tools/call\",\"params\":[]}"))
	f.Fuzz(func(t *testing.T, in []byte) {
		var out bytes.Buffer
		cfg := testCfg()
		cfg.maxIn = 512
		if code := run(cfg, bytes.NewReader(in), &out, io.Discard); code != 0 {
			t.Fatalf("код %d", code)
		}
		for _, l := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
			if len(l) == 0 {
				continue
			}
			var m map[string]any
			dec := json.NewDecoder(bytes.NewReader(l))
			dec.UseNumber() // id вроде 1e999 — годный JSON, но не float64
			if err := dec.Decode(&m); err != nil || m["jsonrpc"] != "2.0" {
				t.Fatalf("в stdout не JSON-RPC: %.100q", l)
			}
		}
	})
}
