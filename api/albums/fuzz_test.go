package albums

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// FuzzValidateManifest: любой JSON на входе сервиса — ошибка или годный манифест, без паники; годный
// переживает круг JSON → тип → JSON → Validate (сервис пишет manifest.json повторной сериализацией).
func FuzzValidateManifest(f *testing.F) {
	good, _ := json.Marshal(goodManifest(4, true))
	f.Add(good)
	f.Add([]byte(`{"schema":1,"pages":[{"n":1,"role":"page","file":"../x"}]}`))
	f.Add([]byte(`{"schema":1,"generator":"g","title":"\u202e","book":{"page_mm":{"w":1e999}},"pages":null}`))
	f.Add([]byte(``))
	f.Add([]byte(`[`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}
		var m Manifest
		if json.Unmarshal(data, &m) != nil {
			return
		}
		if err := Validate(&m, Limits{}); err != nil {
			return
		}
		out, err := json.Marshal(&m)
		if err != nil {
			t.Fatalf("годный манифест не сериализуется: %v", err)
		}
		var back Manifest
		if err := json.Unmarshal(out, &back); err != nil || Validate(&back, Limits{}) != nil {
			t.Fatalf("круг сломал годный манифест: %v", err)
		}
		// запрос с этим манифестом: проверка не паникует, пароль и срок независимы
		_ = ValidateCreate(&CreateRequest{Manifest: m, ExpiresAt: time.Now().Add(time.Hour)}, Limits{}, time.Now())
	})
}

// FuzzParsePageN: принятое число строго каноническое и в пределах 1…m.
func FuzzParsePageN(f *testing.F) {
	for _, s := range []string{"1", "62", "0", "01", "-1", "+1", "", "99999999999999999999", "1 ", "٣"} {
		f.Add(s, 62)
	}
	f.Fuzz(func(t *testing.T, s string, m int) {
		n, err := ParsePageN(s, m)
		if err != nil {
			return
		}
		if n < 1 || n > m || strconv.Itoa(n) != s {
			t.Fatalf("ParsePageN(%q, %d) = %d", s, m, n)
		}
	})
}

// FuzzCheckInputs: проверки id, токена и пароля не паникуют; принятое годно по правилам длины.
func FuzzCheckInputs(f *testing.F) {
	f.Add("Abcdefghijklmnopqrstuv", "tok-0123456789abcdef", "лето-4827", "https://albums.example.org/svc")
	f.Add("", "", "", "")
	f.Add("a", "b", "c", "http://[::ffff:127.0.0.1]:8080")
	f.Add("a", "b", "c", "http://app.localhost")
	f.Add("a", "b", "c", "https://user:pw@host")
	f.Add("a", "b", "c", "HTTPS://Альбомы.рф")
	f.Fuzz(func(t *testing.T, id, tok, pw, base string) {
		if CheckID(id) == nil && (len(id) < MinID || len(id) > MaxID || id[0] == '-') {
			t.Fatalf("CheckID принял %q", id)
		}
		if CheckToken(tok) == nil && (len(tok) < MinToken || len(tok) > MaxToken) {
			t.Fatalf("CheckToken принял %q", tok)
		}
		if CheckPassword(pw) == nil && len(pw) > MaxPasswordSize {
			t.Fatalf("CheckPassword принял %d байт", len(pw))
		}
		_, _ = ParseBase(id)
		// принятый адрес сервиса: https или http только к этому компьютеру, без логина, ASCII
		u, err := ParseBase(base)
		if err != nil {
			return
		}
		out := u.String()
		for i := 0; i < len(out); i++ {
			if out[i] <= ' ' || out[i] >= 0x7f {
				t.Fatalf("ParseBase принял не ASCII/пробел: %q → %q", base, out)
			}
		}
		if u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(out, "/") {
			t.Fatalf("ParseBase(%q) = %q: логин, пустой хост, запрос или якорь", base, out)
		}
		if u.Scheme != "https" && (u.Scheme != "http" || !IsLoopbackHost(u.Hostname())) {
			t.Fatalf("ParseBase(%q) = %q: не https и не loopback", base, out)
		}
	})
}
