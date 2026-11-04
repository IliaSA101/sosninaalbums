package albumstest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// «Зубы» Conformance: сервер с одной сознательной дырой обязан провалить набор в нужном блоке.
// Мутанты — обёртки над поддельным сервером; набор идёт дочерним процессом (провал t.Run нельзя
// проверить внутри того же теста), блок называется в строке «--- FAIL: …/<блок>».

const teethEnv = "ALBUMSTEST_MUTANT"

// rewriteWriter подменяет статус и заголовки ответа до отправки.
type rewriteWriter struct {
	http.ResponseWriter
	fn   func(h http.Header, status int) int
	done bool
}

func (w *rewriteWriter) WriteHeader(code int) {
	if !w.done {
		w.done = true
		code = w.fn(w.Header(), code)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *rewriteWriter) Write(b []byte) (int, error) {
	if !w.done {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// bodyWriter буферизует ответ и отдаёт его, переписав fn.
type bodyWriter struct {
	http.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (w *bodyWriter) WriteHeader(code int)        { w.status = code }
func (w *bodyWriter) Write(b []byte) (int, error) { return w.buf.Write(b) }

func rewriteBody(r *http.Request, fn func(map[string]any)) {
	raw, _ := io.ReadAll(r.Body)
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		fn(m)
		raw, _ = json.Marshal(m)
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.ContentLength = int64(len(raw))
}

type mutant struct {
	wrap  func(http.Handler) http.Handler
	block string
}

// onRequest — обёртка, меняющая запрос до сервера, если match.
func onRequest(match func(r *http.Request) bool, fn func(r *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if match(r) {
				fn(r)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// onStatus — обёртка, меняющая статус и заголовки ответа, если match.
func onStatus(match func(r *http.Request) bool, fn func(h http.Header, status int) int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if match(r) {
				w = &rewriteWriter{ResponseWriter: w, fn: fn}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isPut(r *http.Request) bool { return r.Method == "PUT" && strings.Contains(r.URL.Path, "/pages/") }

func isCreate(r *http.Request) bool { return r.Method == "POST" && r.URL.Path == "/v1/albums" }

// leakPassword — пароль из запроса создания попадает в ответ списка.
func leakPassword(next http.Handler) http.Handler {
	var pw string
	re := regexp.MustCompile(`"password":"([^"]+)"`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isCreate(r) {
			raw, _ := io.ReadAll(r.Body)
			if m := re.FindSubmatch(raw); m != nil {
				pw = string(m[1])
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
		}
		if r.Method == "GET" && r.URL.Path == "/v1/albums" && pw != "" {
			bw := &bodyWriter{ResponseWriter: w}
			next.ServeHTTP(bw, r)
			out := strings.TrimSuffix(strings.TrimSpace(bw.buf.String()), "}") + `,"debug":"` + pw + `"}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(bw.status)
			_, _ = io.WriteString(w, out)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// onBody — обёртка, переписывающая JSON-объект в ответе на запрос, если match; fn получает и тело запроса.
func onBody(match func(r *http.Request) bool, fn func(req []byte, resp map[string]any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !match(r) {
				next.ServeHTTP(w, r)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(raw))
			bw := &bodyWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(bw, r)
			out := bw.buf.Bytes()
			var m map[string]any
			if json.Unmarshal(out, &m) == nil {
				fn(raw, m)
				out, _ = json.Marshal(m)
			}
			w.WriteHeader(bw.status)
			_, _ = w.Write(out)
		})
	}
}

// echoPassword — пароль из запроса попадает в ответ тем же полем.
func echoPassword(req []byte, resp map[string]any) {
	var in struct {
		Password string `json:"password"`
	}
	if json.Unmarshal(req, &in) == nil && in.Password != "" {
		resp["password"] = in.Password
	}
}

func isManifest(r *http.Request) bool {
	return r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/a/") && strings.HasSuffix(r.URL.Path, "/manifest.json")
}

// cookieOnly — сервис ставит cookie на любую попытку входа и открывает страницы и файлы по одному
// наличию cookie; закрыт только manifest.json. Старая проверка denyLogin (один manifest.json) его пропускала.
func cookieOnly(next http.Handler) http.Handler {
	return cookieSwap(next, true, func(c string) bool { return strings.Contains(c, "sa_session=0.bad") })
}

// anyCookie — вход честный, manifest.json честный, но страница альбома и файлы открыты на любую cookie
// (поддельную, искажённую, отозванную): сервис проверяет подпись только у манифеста.
func anyCookie(next http.Handler) http.Handler {
	return cookieSwap(next, false, func(c string) bool { return c != "" })
}

// cookieSwap — прокси: на запросы страниц и файлов (не manifest.json), у которых cookie подходит под
// fake, подставляется настоящая cookie последнего верного входа. badLogin — отказ входа превращается в
// 200 с негодной cookie (мутант cookieonly).
func cookieSwap(next http.Handler, badLogin bool, fake func(cookie string) bool) http.Handler {
	var mu sync.Mutex
	good := map[string]string{} // id → настоящая cookie последнего верного входа
	albumID := func(p string) string { return strings.SplitN(strings.TrimPrefix(p, "/a/"), "/", 2)[0] }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/login"):
			id := albumID(r.URL.Path)
			w = &rewriteWriter{ResponseWriter: w, fn: func(h http.Header, s int) int {
				if s >= 400 && badLogin {
					h.Set("Set-Cookie", "sa_session=0.bad; Path=/")
					return 200
				}
				for _, ck := range (&http.Response{Header: h}).Cookies() {
					mu.Lock()
					good[id] = ck.Name + "=" + ck.Value
					mu.Unlock()
				}
				return s
			}}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/a/") && !isManifest(r) && fake(r.Header.Get("Cookie")):
			mu.Lock()
			ck := good[albumID(r.URL.Path)]
			mu.Unlock()
			r.Header.Set("Cookie", ck)
		}
		next.ServeHTTP(w, r)
	})
}

func mutants(token string) map[string]mutant {
	return map[string]mutant{
		// список без токена отдаётся: токен подставляет «прокси»
		"noauth": {onRequest(func(r *http.Request) bool {
			return r.URL.Path == "/v1/albums" && r.Method == "GET" && r.Header.Get("Authorization") == ""
		}, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }), "auth"},
		// страницы клиента без nosniff
		"nosniff": {onStatus(func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/a/") },
			func(h http.Header, s int) int { h.Del("X-Content-Type-Options"); return s }), "publish"},
		// несуществующий id отвечает 403, а не 404
		"forbidden": {onStatus(func(r *http.Request) bool { return r.Method == "DELETE" },
			func(h http.Header, s int) int {
				if s == 404 {
					return 403
				}
				return s
			}), "foreign"},
		// хэш и сигнатуру страницы сервер не проверяет (отвечает 204)
		"loosepages": {onStatus(isPut, func(h http.Header, s int) int {
			if s == 409 || s == 400 {
				return 204
			}
			return s
		}), "pages"},
		// пароль альбома теряется при создании
		"nopassword": {onRequest(isCreate, func(r *http.Request) { rewriteBody(r, func(m map[string]any) { delete(m, "password") }) }), "password"},
		// сброс входов молча игнорируется
		"norevoke": {onRequest(func(r *http.Request) bool { return r.Method == "PATCH" },
			func(r *http.Request) { rewriteBody(r, func(m map[string]any) { delete(m, "revoke_sessions") }) }), "revoke"},
		// пароль в ответе списка
		"pwleak": {leakPassword, "password"},
		// пароль в ответе 201 и в ответе PATCH (клиент albums такие поля отбрасывает, набор обязан видеть)
		"pw201":   {onBody(isCreate, echoPassword), "password"},
		"pwpatch": {onBody(func(r *http.Request) bool { return r.Method == "PATCH" }, echoPassword), "password"},
		// сервер принимает file с обходом пути (и любой file) при создании
		"badfile": {onRequest(isCreate, func(r *http.Request) {
			rewriteBody(r, func(m map[string]any) {
				mf, _ := m["manifest"].(map[string]any)
				pages, _ := mf["pages"].([]any)
				for _, p := range pages {
					if pm, ok := p.(map[string]any); ok {
						delete(pm, "file")
					}
				}
			})
		}), "input"},
		// слишком большое тело получает 400, а не 413
		"no413": {onStatus(isCreate, func(h http.Header, s int) int {
			if s == 413 {
				return 400
			}
			return s
		}), "input"},
		// id альбома короче 22 знаков
		"shortid": {onBody(isCreate, func(_ []byte, m map[string]any) {
			if id, ok := m["id"].(string); ok && len(id) > 10 {
				m["id"] = id[:10]
			}
		}), "ids"},
		// в manifest.json попадает срок
		"manifestleak": {onBody(isManifest, func(_ []byte, m map[string]any) { m["expires_at"] = "2030-01-01T00:00:00Z" }), "publish"},
		// неопубликованный альбом отдаёт manifest.json (404 → 200)
		"unpublished": {onStatus(isManifest, func(h http.Header, s int) int {
			if s == 404 {
				return 200
			}
			return s
		}), "publish"},
		// cookie на любой пароль, страницы и файлы открыты по наличию cookie (закрыт только manifest.json)
		"cookieonly": {cookieOnly, "password"},
		// страница и файлы открыты на любую cookie (поддельную, отозванную), manifest.json честный
		"anycookie": {anyCookie, "password"},
		// страница альбома без Referrer-Policy
		"noreferrer": {onStatus(func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/a/") },
			func(h http.Header, s int) int { h.Del("Referrer-Policy"); return s }), "publish"},
	}
}

// TestBrokenChild — тело дочернего прогона: набор против сервера-мутанта из ALBUMSTEST_MUTANT.
func TestBrokenChild(t *testing.T) {
	name := os.Getenv(teethEnv)
	if name == "" {
		t.Skip("дочерний прогон: запускается из TestConformanceTeeth")
	}
	f, err := NewFake(Options{OtherToken: otherTok})
	if err != nil {
		t.Fatal(err)
	}
	var h http.Handler = f
	if name != "none" {
		m, ok := mutants(f.opt.Token)[name]
		if !ok {
			t.Fatalf("нет мутанта %q", name)
		}
		h = m.wrap(f)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	f.SetURL(srv.URL)
	Conformance(t, srv.URL, f.opt.Token, WithOtherToken(otherTok))
}

func TestConformanceTeeth(t *testing.T) {
	if testing.Short() {
		t.Skip("прогоны набора дочерними процессами: не в быстром уровне")
	}
	if os.Getenv(teethEnv) != "" {
		t.Skip("дочерний прогон")
	}
	run := func(name string) (string, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestBrokenChild$", "-test.v", "-test.timeout=60s")
		cmd.Env = append(os.Environ(), teethEnv+"="+name)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("none"); err != nil {
		t.Fatalf("честный сервер провалил набор в дочернем прогоне: %v\n%s", err, out)
	}
	for name, m := range mutants("") {
		out, err := run(name)
		if err == nil {
			t.Errorf("мутант %q прошёл набор целиком", name)
			continue
		}
		if want := "--- FAIL: TestBrokenChild/" + m.block; !strings.Contains(out, want) {
			t.Errorf("мутант %q: нет провала блока %q\n%s", name, m.block, out)
		}
	}
}

// TestCookieFlags: на https cookie входа без Secure или HttpOnly — замечание, на http флаги не требуются.
func TestCookieFlags(t *testing.T) {
	for name, tc := range map[string]struct {
		set   []*http.Cookie
		https bool
		bad   int
	}{
		"https оба флага":    {[]*http.Cookie{{Name: "s", Secure: true, HttpOnly: true}}, true, 0},
		"https без Secure":   {[]*http.Cookie{{Name: "s", HttpOnly: true}}, true, 1},
		"https без HttpOnly": {[]*http.Cookie{{Name: "s", Secure: true}}, true, 1},
		"https две плохие":   {[]*http.Cookie{{Name: "a"}, {Name: "b"}}, true, 2},
		"http без флагов":    {[]*http.Cookie{{Name: "s"}}, false, 0},
		"https нет cookie":   {nil, true, 0},
	} {
		if got := cookieProblems(tc.set, tc.https); len(got) != tc.bad {
			t.Errorf("%s: замечаний %d (%v), ожидалось %d", name, len(got), got, tc.bad)
		}
	}
}
