package albums

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testToken = "tok-0123456789abcdef-SECRET"
	testID    = "Abcdefghijklmnopqrstuv_-12"
)

// fast ускоряет тайм-ауты клиента на время теста.
func fast(t *testing.T, mut func(tm *timeouts)) {
	t.Helper()
	old := defaultTimeouts
	defaultTimeouts.header, defaultTimeouts.idle = 300*time.Millisecond, 300*time.Millisecond
	if mut != nil {
		mut(&defaultTimeouts)
	}
	t.Cleanup(func() { defaultTimeouts = old })
}

// tlsClient — клиент к httptest-серверу с TLS; корни CA — через крючок _test.go.
func tlsClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	testRootCAs = pool
	t.Cleanup(func() { testRootCAs = nil })
	c, err := New(srv.URL, testToken, Options{Version: "0.32.0"})
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

// plainClient — клиент к loopback по http.
func plainClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, ErrorBody{Error: Error{Code: code, Message: msg}})
}

func TestClientBase(t *testing.T) {
	good := map[string]string{
		"https://albums.example.org":      "https://albums.example.org",
		"https://Albums.Example.ORG/":     "https://albums.example.org",
		"https://albums.example.org:8443": "https://albums.example.org:8443",
		"https://albums.example.org/svc":  "https://albums.example.org/svc",
		"https://albums.example.org/svc/": "https://albums.example.org/svc",
		"HTTPS://x.example.org":           "https://x.example.org",
		"http://127.0.0.1:34190":          "http://127.0.0.1:34190",
		"http://localhost":                "http://localhost",
		"http://LOCALHOST:80":             "http://localhost:80",
		"http://[::1]:34190":              "http://[::1]:34190",
		"https://127.0.0.1:8443":          "https://127.0.0.1:8443",
		"https://xn--80aswg.xn--p1ai":     "https://xn--80aswg.xn--p1ai",
	}
	for in, want := range good {
		c, err := New(in, testToken, Options{})
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if c.Base() != want {
			t.Errorf("%q: Base = %q, нужно %q", in, c.Base(), want)
		}
	}
	bad := map[string]string{
		"пусто": "", "http чужой": "http://albums.example.org", "http IP": "http://192.168.1.5", "http 0.0.0.0": "http://0.0.0.0:80",
		"ftp": "ftp://albums.example.org", "без схемы": "albums.example.org", "логин": "https://user@albums.example.org",
		"логин с паролем": "https://user:pw@albums.example.org", "запрос": "https://albums.example.org/?a=1", "пустой запрос": "https://albums.example.org?",
		"якорь": "https://albums.example.org/#x", "без хоста": "https://", "порт 0": "https://albums.example.org:0",
		"порт 70000": "https://albums.example.org:70000", "порт не число": "https://albums.example.org:ab", "пробел": "https://albums .example.org",
		"перевод строки": "https://albums.example.org\n", "табуляция": "https://albums.example.org/\t", "обратный слэш": "https://albums.example.org\\x",
		"процент": "https://albums.example.org/%2e%2e", "точки в пути": "https://albums.example.org/a/../b", "точка": "https://albums.example.org/./b",
		"opaque": "https:albums.example.org", "длиннее 200": "https://" + strings.Repeat("a", 190) + ".org",
		"http-домен как localhost.evil": "http://localhost.evil.org",
	}
	for name, in := range bad {
		if _, err := New(in, testToken, Options{}); err == nil {
			t.Errorf("%s: %q принят", name, in)
		} else if strings.Contains(err.Error(), in) && in != "" {
			t.Errorf("%s: ошибка повторяет адрес: %v", name, err)
		}
	}
	// хост не ASCII — отказ с подсказкой про xn--, до сети
	for _, in := range []string{"https://альбомы.рф", "https://xn--80aswg.рф", "https://albums.example.org/путь", "https://аlbums.example.org"} {
		if _, err := New(in, testToken, Options{}); !errors.Is(err, ErrBaseNonASCII) {
			t.Errorf("%q: %v", in, err)
		}
	}
	// токен проверяется тоже, ошибка не повторяет токен
	if _, err := New("https://albums.example.org", "short", Options{}); err == nil || strings.Contains(err.Error(), "short") {
		t.Errorf("короткий токен: %v", err)
	}
}

func TestClientHeaders(t *testing.T) {
	type seen struct{ method, path, auth, ua, ctype string }
	var got []seen
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, seen{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("User-Agent"), r.Header.Get("Content-Type")})
		switch {
		case r.URL.Path == "/v1/health":
			writeJSON(w, 200, Health{API: "v1", Limits: Limits{Pages: 100, PageBytes: 1 << 20}})
		case strings.HasSuffix(r.URL.Path, "/pages/3"):
			w.WriteHeader(204)
		default:
			writeJSON(w, 200, AlbumList{})
		}
	}))
	ctx := context.Background()
	if _, err := c.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.PutPage(ctx, testID, 3, TypeJPEG, strings.NewReader("abc"), 3); err != nil {
		t.Fatal(err)
	}
	if got[0].auth != "" {
		t.Errorf("health получил токен: %q", got[0].auth)
	}
	if got[1].auth != "Bearer "+testToken || got[2].auth != "Bearer "+testToken {
		t.Errorf("токен: %+v", got)
	}
	for _, s := range got {
		if s.ua != "SosninaAlbums/dev" {
			t.Errorf("User-Agent %q", s.ua)
		}
	}
	if got[2].ctype != TypeJPEG || got[2].path != "/v1/albums/"+testID+"/pages/3" {
		t.Errorf("PUT: %+v", got[2])
	}
	if c.Limits().Pages != 100 || c.Limits().PageBytes != 1<<20 || c.Limits().MaxDays != 365 {
		t.Errorf("пределы из health: %+v", c.Limits())
	}
}

func TestClientCreateFlow(t *testing.T) {
	m := goodManifest(4, false) // страница 2 белая
	var body CreateRequest
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/v1/albums" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("запрос: %s %s %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Error(err)
		}
		writeJSON(w, 201, Created{ID: testID, URL: "https://albums.example.org/a/" + testID + "/", ExpiresAt: body.ExpiresAt, Missing: []int{1, 3, 4}})
	}))
	exp := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	cr, err := c.Create(context.Background(), &CreateRequest{Manifest: *m, ExpiresAt: exp, Password: "лето-4827"})
	if err != nil || cr.ID != testID || len(cr.Missing) != 3 {
		t.Fatalf("Create: %+v, %v", cr, err)
	}
	if body.Password != "лето-4827" || !body.ExpiresAt.Equal(exp) || len(body.Manifest.Pages) != 4 || !body.Manifest.Pages[1].Blank {
		t.Errorf("сервер получил %+v", body)
	}
}

// TestClientChecksBeforeNet: негодные аргументы не доходят до сети.
func TestClientChecksBeforeNet(t *testing.T) {
	var hits atomic.Int32
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	ctx := context.Background()
	m := goodManifest(4, false)
	exp := time.Now().Add(time.Hour)
	badM := goodManifest(4, false)
	badM.Pages[0].File = "../x"
	pw := "abc"
	for name, err := range map[string]error{
		"Update id":       func() error { _, e := c.Update(ctx, "../etc", m); return e }(),
		"Update манифест": func() error { _, e := c.Update(ctx, testID, badM); return e }(),
		"Create манифест": func() error { _, e := c.Create(ctx, &CreateRequest{Manifest: *badM, ExpiresAt: exp}); return e }(),
		"Create пароль": func() error {
			_, e := c.Create(ctx, &CreateRequest{Manifest: *m, ExpiresAt: exp, Password: pw})
			return e
		}(),
		"Create срок": func() error {
			_, e := c.Create(ctx, &CreateRequest{Manifest: *m, ExpiresAt: time.Now().Add(-time.Hour)})
			return e
		}(),
		"Create nil":       func() error { _, e := c.Create(ctx, nil); return e }(),
		"PutPage id":       c.PutPage(ctx, "-"+strings.Repeat("a", 21), 1, TypeJPEG, strings.NewReader("x"), 1),
		"PutPage n 0":      c.PutPage(ctx, testID, 0, TypeJPEG, strings.NewReader("x"), 1),
		"PutPage n велик":  c.PutPage(ctx, testID, 301, TypeJPEG, strings.NewReader("x"), 1),
		"PutPage тип":      c.PutPage(ctx, testID, 1, "text/html", strings.NewReader("x"), 1),
		"PutPage размер 0": c.PutPage(ctx, testID, 1, TypeJPEG, strings.NewReader(""), 0),
		"PutPage размер +": c.PutPage(ctx, testID, 1, TypeJPEG, strings.NewReader("x"), 16<<20),
		"Publish id":       func() error { _, e := c.Publish(ctx, "x"); return e }(),
		"Patch id":         func() error { _, e := c.Patch(ctx, "x/y", &Patch{}); return e }(),
		"Patch пароль":     func() error { _, e := c.Patch(ctx, testID, &Patch{Password: &pw}); return e }(),
		"Delete id":        c.Delete(ctx, strings.Repeat("a", 65)),
	} {
		wantInvalid(t, name, err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("до сервера дошло %d запросов", n)
	}
}

func TestClientNoRedirect(t *testing.T) {
	var leaked atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
		writeJSON(w, 200, AlbumList{})
	}))
	defer other.Close()
	for _, code := range []int{301, 302, 303, 307, 308} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/v1/albums", code)
		}))
		_, err := c.List(context.Background())
		if !errors.Is(err, ErrRedirect) {
			t.Errorf("%d: %v", code, err)
		}
		if err != nil && strings.Contains(err.Error(), other.URL) {
			t.Errorf("%d: ошибка несёт адрес перехода: %v", code, err)
		}
	}
	if leaked.Load() != 0 {
		t.Error("Authorization ушёл на другой хост")
	}
}

func TestClientErrors(t *testing.T) {
	cases := []struct {
		status int
		code   string
		is     error
		retry  string
	}{
		{400, CodeBadManifest, nil, ""}, {401, CodeUnauthorized, ErrToken, ""}, {404, CodeNotFound, ErrGone, ""},
		{409, CodeHashMismatch, nil, ""}, {413, CodeTooLarge, ErrLimit, ""}, {429, CodeRateLimited, ErrBusy, "7"},
		{500, CodeInternal, nil, ""}, {503, "", nil, "100000"},
	}
	for _, tc := range cases {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.retry != "" {
				w.Header().Set("Retry-After", tc.retry)
			}
			apiErr(w, tc.status, tc.code, "сообщение")
		}))
		_, err := c.List(context.Background())
		var ae *APIError
		if !errors.As(err, &ae) || ae.Status != tc.status || ae.Code != tc.code || ae.Message != "сообщение" {
			t.Fatalf("%d: %v", tc.status, err)
		}
		for _, sentinel := range []error{ErrToken, ErrGone, ErrLimit, ErrBusy} {
			if errors.Is(err, sentinel) != (sentinel == tc.is) {
				t.Errorf("%d: Is(%v) = %v", tc.status, sentinel, errors.Is(err, sentinel))
			}
		}
		if want := "albums: " + strings.TrimSpace(strings.Join([]string{itoaT(tc.status), tc.code}, " ")); err.Error() != want {
			t.Errorf("Error() = %q, нужно %q", err.Error(), want)
		}
		if tc.retry == "7" && ae.RetryAfter != 7*time.Second {
			t.Errorf("Retry-After: %v", ae.RetryAfter)
		}
		if tc.retry == "100000" && ae.RetryAfter != time.Hour {
			t.Errorf("Retry-After не зажат: %v", ae.RetryAfter)
		}
	}
}

func itoaT(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestClientErrorBodyOdd(t *testing.T) {
	// чужие формы тела ошибки: статус остаётся, код и сообщение пусты или очищены
	for name, body := range map[string]string{
		"html":          "<html><body>502 Bad Gateway</body></html>",
		"пусто":         "",
		"код с мусором": `{"error":{"code":"Bad Code\n<script>","message":"x"}}`,
		"массив":        `[1,2,3]`,
		"код числом":    `{"error":{"code":5}}`,
	} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(502); _, _ = io.WriteString(w, body) }))
		_, err := c.List(context.Background())
		var ae *APIError
		if !errors.As(err, &ae) || ae.Status != 502 || ae.Code != "" || err.Error() != "albums: 502" {
			t.Errorf("%s: %v", name, err)
		}
	}
	// сообщение: обрезано до 200 знаков, без управляющих
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiErr(w, 400, CodeBadManifest, "строка\x00с\nуправляющими"+strings.Repeat("я", 500))
	}))
	_, err := c.List(context.Background())
	var ae *APIError
	if !errors.As(err, &ae) || len([]rune(ae.Message)) != maxMessage || strings.ContainsAny(ae.Message, "\x00\n") {
		t.Errorf("сообщение: %q", ae)
	}
}

func TestClientBadResponses(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		call func(c *Client) error
	}{
		"health чужой api": {`{"api":"v2","limits":{}}`, func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		"health не JSON":   {`<html>`, func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		"health хвост":     {`{"api":"v1","limits":{}} {"x":1}`, func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		"health пустой":    {``, func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		"list плохой id":   {`{"albums":[{"id":"../x","url":"u"}]}`, func(c *Client) error { _, e := c.List(context.Background()); return e }},
		"list url длинный": {`{"albums":[{"id":"` + testID + `","url":"` + strings.Repeat("a", MaxURL+1) + `"}]}`, func(c *Client) error { _, e := c.List(context.Background()); return e }},
		"publish без url":  {`{"expires_at":"2030-01-01T00:00:00Z"}`, func(c *Client) error { _, e := c.Publish(context.Background(), testID); return e }},
		"patch плохой id":  {`{"id":"x"}`, func(c *Client) error { _, e := c.Patch(context.Background(), testID, &Patch{}); return e }},
	} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) }))
		if err := tc.call(c); !errors.Is(err, ErrResponse) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// create: id плохой, missing с белой страницей или повтором
	m := goodManifest(4, false)
	exp := time.Now().Add(time.Hour)
	for name, body := range map[string]string{
		"id плохой":      `{"id":"-abc","url":"u","missing":[]}`,
		"missing белая":  `{"id":"` + testID + `","url":"u","missing":[2]}`,
		"missing повтор": `{"id":"` + testID + `","url":"u","missing":[1,1]}`,
		"missing вне":    `{"id":"` + testID + `","url":"u","missing":[9]}`,
		"url пуст":       `{"id":"` + testID + `","missing":[]}`,
	} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201); _, _ = io.WriteString(w, body) }))
		if _, err := c.Create(context.Background(), &CreateRequest{Manifest: *m, ExpiresAt: exp}); !errors.Is(err, ErrResponse) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// лишние поля в ответе пропускаются
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"api":"v1","new_field":[1,2],"limits":{"pages":5,"future":true}}`)
	}))
	if h, err := c.Health(context.Background()); err != nil || h.Limits.Pages != 5 {
		t.Errorf("чужие поля: %+v, %v", h, err)
	}
}

func TestClientBodyLimit(t *testing.T) {
	pad := func(n int) string { return strings.Repeat(" ", n) }
	// ровно MaxBody — принят; MaxBody+1 — явная ошибка, не обрезанный JSON
	for _, tc := range []struct {
		size   int
		tooBig bool
	}{{MaxBody, false}, {MaxBody + 1, true}, {3 * MaxBody, true}} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := `{"albums":[]}`
			_, _ = io.WriteString(w, body+pad(tc.size-len(body)))
		}))
		_, err := c.List(context.Background())
		if tc.tooBig != errors.Is(err, ErrTooBig) || (!tc.tooBig && err != nil) {
			t.Errorf("размер %d: %v", tc.size, err)
		}
	}
	// тела 4xx и 5xx: HTML-страница прокси больше предела — тоже ошибка предела, а не 401/404
	for _, status := range []int{401, 404, 429, 500, 502} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, strings.Repeat("<p>x</p>", MaxBody/8+10))
		}))
		_, err := c.List(context.Background())
		if !errors.Is(err, ErrTooBig) || errors.Is(err, ErrToken) || errors.Is(err, ErrGone) || errors.Is(err, ErrBusy) {
			t.Errorf("%d: %v", status, err)
		}
	}
	// тело ошибки ровно в пределе читается
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"error":{"code":"internal","message":"m"}}`
		w.WriteHeader(500)
		_, _ = io.WriteString(w, body+pad(MaxBody-len(body)))
	}))
	var ae *APIError
	if _, err := c.List(context.Background()); !errors.As(err, &ae) || ae.Code != "internal" {
		t.Errorf("ошибка в пределе: %v", err)
	}
}

func TestClientBodyStall(t *testing.T) {
	fast(t, nil)
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"albums":`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	start := time.Now()
	_, err := c.List(context.Background())
	var ne *NetError
	if !errors.As(err, &ne) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("застой тела: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("застой тела ждали %v", time.Since(start))
	}
}

func TestClientHeaderStall(t *testing.T) {
	fast(t, nil)
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	start := time.Now()
	_, err := c.List(context.Background())
	var ne *NetError
	if !errors.As(err, &ne) || time.Since(start) > 3*time.Second {
		t.Fatalf("застой заголовков: %v (%v)", err, time.Since(start))
	}
}

// TestClientPutStall: сервер принял соединение и не читает тело — PUT не висит без предела (№12).
func TestClientPutStall(t *testing.T) {
	fast(t, func(tm *timeouts) { tm.putBase, tm.putRate = 400*time.Millisecond, 1<<40 })
	done := make(chan struct{})
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	t.Cleanup(func() { close(done) })
	c.mu.Lock()
	c.lim.PageBytes = 64 << 20
	c.mu.Unlock()
	size := int64(64 << 20) // больше буферов сокета: без чтения сервера запись встанет
	start := time.Now()
	err := c.PutPage(context.Background(), testID, 1, TypeJPEG, zeroReader{}, size)
	var ne *NetError
	if !errors.As(err, &ne) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("застой PUT: %v", err)
	}
	if d := time.Since(start); d < 300*time.Millisecond || d > 5*time.Second {
		t.Errorf("PUT оборван через %v", d)
	}
	// срок растёт с размером: 60 с и 32 КБ/с
	tm := timeouts{putBase: 60 * time.Second, putRate: 32 << 10}
	if tm.putTimeout(0) != 60*time.Second || tm.putTimeout(32<<10*100) != 160*time.Second {
		t.Errorf("putTimeout: %v %v", tm.putTimeout(0), tm.putTimeout(32<<10*100))
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestClientCancel(t *testing.T) {
	started := make(chan struct{})
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-started; cancel() }()
	start := time.Now()
	_, err := c.List(ctx)
	if !errors.Is(err, context.Canceled) || time.Since(start) > 3*time.Second {
		t.Errorf("отмена: %v", err)
	}
	// уже отменённый контекст — без запроса
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := c.List(ctx2); !errors.Is(err, context.Canceled) {
		t.Errorf("отменён заранее: %v", err)
	}
}

// TestClientErrorsNoSecrets (С4): ни в одной ошибке нет токена, id, пароля и пути запроса.
func TestClientErrorsNoSecrets(t *testing.T) {
	const password = "пароль-клиента-5521"
	secrets := []string{testToken, testID, password, "/v1/albums"}
	check := func(what string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: ошибки нет", what)
			return
		}
		text := err.Error()
		for u := errors.Unwrap(err); u != nil; u = errors.Unwrap(u) {
			text += "|" + u.Error()
		}
		for _, s := range secrets {
			if strings.Contains(text, s) {
				t.Errorf("%s: в ошибке есть %q: %s", what, s, text)
			}
		}
	}
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	create := func(c *Client) error {
		_, err := c.Create(ctx, &CreateRequest{Manifest: *goodManifest(4, false), ExpiresAt: exp, Password: password})
		return err
	}
	patch := func(c *Client) error {
		pw := password
		_, err := c.Patch(ctx, testID, &Patch{Password: &pw})
		return err
	}
	put := func(c *Client) error { return c.PutPage(ctx, testID, 1, TypeJPEG, strings.NewReader("x"), 1) }

	// dial: порт закрыт
	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()
	c, err := New(url, testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]func(*Client) error{"create": create, "patch": patch, "put": put,
		"publish": func(c *Client) error { _, e := c.Publish(ctx, testID); return e }, "delete": func(c *Client) error { return c.Delete(ctx, testID) }} {
		check("dial "+name, f(c))
	}

	// TLS: сертификат сервера не доверенный
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	ct, err := New(srv.URL, testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	check("tls create", create(ct))
	check("tls patch", patch(ct))

	// тайм-аут
	fast(t, nil)
	slow, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // без чтения тела обрыв клиента сервер не заметит
		<-r.Context().Done()
	}))
	check("timeout create", create(slow))
	check("timeout put", put(slow))

	checks := map[string]func(*Client) error{"create": create, "patch": patch, "put": put}
	echoNoSecrets(t, check, checks)
}

// echoNoSecrets: сервер повторяет в теле ошибки Authorization, путь и тело запроса — в ошибке клиента их нет.
func echoNoSecrets(t *testing.T, check func(string, error), calls map[string]func(*Client) error) {
	t.Helper()
	for _, status := range []int{401, 404, 500} {
		echo, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			msg := "Authorization: " + r.Header.Get("Authorization") + " path " + r.URL.Path + " body " + string(raw)
			apiErr(w, status, CodeInternal, msg)
		}))
		for name, f := range calls {
			err := f(echo)
			check("echo "+itoaT(status)+" "+name, err)
			var ae *APIError
			if errors.As(err, &ae) {
				for _, s := range []string{testToken, testID} {
					if strings.Contains(ae.Message, s) {
						t.Errorf("echo %d %s: в Message есть %q", status, name, s)
					}
				}
				if !strings.Contains(ae.Message, "***") {
					t.Errorf("echo %d %s: токен в Message не заменён: %q", status, name, ae.Message)
				}
			}
		}
	}
}

func TestClientDelete(t *testing.T) {
	var path, method string
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { path, method = r.URL.Path, r.Method; w.WriteHeader(204) }))
	if err := c.Delete(context.Background(), testID); err != nil || path != "/v1/albums/"+testID || method != "DELETE" {
		t.Errorf("Delete: %v %s %s", err, method, path)
	}
	// путь сервиса сохраняется
	c2, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { path = r.URL.Path; w.WriteHeader(204) }))
	c3, err := New(c2.Base()+"/svc/", testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := c3.Delete(context.Background(), testID); err != nil || path != "/svc/v1/albums/"+testID {
		t.Errorf("путь сервиса: %v %s", err, path)
	}
}

func TestClientPublishMissing(t *testing.T) {
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 409, ErrorBody{Error: Error{Code: CodePagesMissing, Message: "m", Missing: []int{3, 5}}})
	}))
	_, err := c.Publish(context.Background(), testID)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 409 || ae.Code != CodePagesMissing || len(ae.Missing) != 2 {
		t.Errorf("publish 409: %+v %v", ae, err)
	}
	// список missing длиннее числа страниц отбрасывается (не копим мегабайты)
	c2, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		many := make([]int, 5000)
		writeJSON(w, 409, ErrorBody{Error: Error{Code: CodePagesMissing, Missing: many}})
	}))
	_, err = c2.Publish(context.Background(), testID)
	if !errors.As(err, &ae) || ae.Missing != nil {
		t.Errorf("огромный missing: %+v", ae)
	}
}

// TestClientNoUnsafeTLS: в коде клиента нет путей, ослабляющих проверку (design §2.4).
func TestClientNoUnsafeTLS(t *testing.T) {
	c, err := New("https://albums.example.org", testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := c.hc.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil {
		t.Fatal("нет настроек TLS")
	}
	if tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.MinVersion < 0x0303 {
		t.Errorf("TLS: %+v", tr.TLSClientConfig)
	}
	if tr.TLSClientConfig.RootCAs != nil {
		t.Error("корни CA заданы вне теста")
	}
	if c.hc.CheckRedirect == nil || c.hc.Timeout != 0 {
		t.Error("переходы не запрещены")
	}
}

// TestClientTLS: по https с доверенным сертификатом клиент работает; с чужим — нет.
func TestClientTLS(t *testing.T) {
	c, _ := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, Health{API: "v1"})
	}))
	if h, err := c.Health(context.Background()); err != nil || h.API != "v1" || h.Limits != DefaultLimits() {
		t.Fatalf("https: %+v, %v", h, err)
	}
}
