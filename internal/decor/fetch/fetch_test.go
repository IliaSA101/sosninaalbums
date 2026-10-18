package fetch

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// testLib — TLS-сервер с подписанным каталогом и клиент, доверяющий его сертификату; белый
// список — только 127.0.0.1 (сеть наружу тесты не трогают).
type testLib struct {
	srv  *httptest.Server
	mux  *http.ServeMux
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	hits atomic.Int32

	mu   sync.Mutex
	reqs []*http.Request
}

func newTestLib(t *testing.T) *testLib {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	l := &testLib{mux: http.NewServeMux(), pub: pub, priv: priv}
	l.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.hits.Add(1)
		l.mu.Lock()
		l.reqs = append(l.reqs, r)
		l.mu.Unlock()
		l.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(l.srv.Close)
	return l
}

func (l *testLib) source() Source {
	pool := l.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	return Source{CatalogURL: l.srv.URL + "/main/catalog.json", Prefix: l.srv.URL + "/releases/download/",
		Hosts: []string{"127.0.0.1"}, Keys: []ed25519.PublicKey{l.pub}, Roots: pool}
}

func (l *testLib) client() *Client { return New(l.source()) }

// sig — подпись байтов ключом сервера в base64.
func (l *testLib) sig(data []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(l.priv, data))
}

// envelope — catalog.json как пишет library-pack.mjs: {"sig": подпись байтов body, "body": base64 body}.
func (l *testLib) envelope(body []byte) []byte {
	data, err := json.Marshal(map[string]string{"sig": l.sig(body), "body": base64.StdEncoding.EncodeToString(body)})
	if err != nil {
		panic(err)
	}
	return data
}

func (l *testLib) serve(path string, body []byte) {
	l.mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
}

// Ключи выпуска — два годных ed25519, источник GitHub — https и белый список хостов.
func TestKeysValid(t *testing.T) {
	if got := PublicKeys(Keys[:]); len(got) != 2 {
		t.Fatalf("ключей %d", len(got))
	}
	if PublicKeys([]string{"", "!!", base64.StdEncoding.EncodeToString(make([]byte, 31))}) == nil {
		t.Fatal("nil вместо пустого")
	}
	g := GitHub()
	if !strings.HasPrefix(g.CatalogURL, "https://raw.githubusercontent.com/") || !strings.HasPrefix(g.Prefix, "https://github.com/") ||
		g.LocalHTTP || g.Roots != nil || len(g.Keys) != 2 {
		t.Fatalf("GitHub: %+v", g)
	}
	// Без тега dev Default — всегда GitHub, даже с переменной стенда; с тегом — стенд.
	t.Setenv("SA_LIBRARY_URL", "http://127.0.0.1:34190/")
	if d := Default(); standLibrary != (d.CatalogURL == "http://127.0.0.1:34190/catalog.json" && d.LocalHTTP) ||
		!standLibrary && d.CatalogURL != CatalogURL {
		t.Fatalf("Default (dev %v): %+v", standLibrary, d)
	}
	t.Setenv("SA_LIBRARY_URL", "http://10.0.0.1/") // не loopback — и в dev GitHub
	if d := Default(); d.CatalogURL != CatalogURL || d.LocalHTTP {
		t.Fatalf("Default с чужим адресом: %+v", d)
	}
}

// Белый список: схема, хост, имя пользователя, параметры — на каждом адресе.
func TestFetchAllowlist(t *testing.T) {
	g := GitHub()
	for raw, want := range map[string]bool{
		"https://raw.githubusercontent.com/IliaSA101/x/main/catalog.json": true,
		"https://GitHub.com/IliaSA101/x/releases/download/a-v1/a-1.zip":   true,
		"https://release-assets.githubusercontent.com/x":                  true,
		"https://objects.githubusercontent.com/x":                         true,
		"http://github.com/x":               false,
		"ftp://github.com/x":                false,
		"https://evil.example/x":            false,
		"https://github.com.evil.example/x": false,
		"https://user@github.com/x":         false,
		"https://github.com/x?q=1":          false,
		"https://github.com/x?":             false,
		"https://github.com/x#a":            false,
		"http://127.0.0.1/x":                false,
		"https://api.github.com/repos/IliaSA101/sosninaalbums-library/x":     false,
		"https://raw.githubusercontent.com.:443/IliaSA101/x/main/catalog.js": false,
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := g.allowed(u); got != want {
			t.Errorf("%s: %v, ждали %v", raw, got, want)
		}
	}
	// Стенд: http только к 127.0.0.1.
	st, ok := Stand("http://127.0.0.1:34190/", "")
	if !ok || !st.allowed(mustURL(t, "http://127.0.0.1:34190/a.zip")) || st.allowed(mustURL(t, "http://github.com/a")) {
		t.Fatalf("стенд: %v %+v", ok, st)
	}
	for _, bad := range []string{"", "http://localhost:1/", "http://10.0.0.1/", "http://127.0.0.1:1/x", "file:///c:/x/", "http://127.0.0.1/?a=1"} {
		if _, ok := Stand(bad, ""); ok {
			t.Errorf("стенд %q принят", bad)
		}
	}

	// Хост сервера не в списке — ни одного запроса.
	l := newTestLib(t)
	src := l.source()
	src.Hosts = []string{"github.com"}
	if _, err := New(src).Catalog(context.Background(), 0); !errors.Is(err, ErrAddress) || l.hits.Load() != 0 {
		t.Fatalf("чужой хост: %v, запросов %d", err, l.hits.Load())
	}
	// Переход на хост не из списка (localhost — тот же сервер) — отказ до запроса туда.
	port := l.srv.Listener.Addr().(*net.TCPAddr).Port
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://localhost:"+itoa(port)+"/other", http.StatusFound)
	})
	if _, err := l.client().Catalog(context.Background(), 0); !errors.Is(err, ErrAddress) || l.hits.Load() != 1 {
		t.Fatalf("переход: %v, запросов %d", err, l.hits.Load())
	}
}

// Переходы: пять — можно, шестой — отказ; на переходе нет Referer, cookie и версии в UA.
func TestFetchRedirects(t *testing.T) {
	l := newTestLib(t)
	for i := range 6 { // /r0 → … → /r5 → /ok: шесть переходов
		next := "/r" + itoa(i+1)
		if i == 5 {
			next = "/ok"
		}
		l.mux.HandleFunc("/r"+itoa(i), func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, &http.Cookie{Name: "c", Value: "1"})
			http.Redirect(w, r, next, http.StatusFound)
		})
	}
	l.serve("/ok", []byte("ok"))
	c := l.client()
	if data, err := c.small(context.Background(), l.srv.URL+"/r1", 10); err != nil || string(data) != "ok" {
		t.Fatalf("пять переходов: %v %q", err, data)
	}
	l.mu.Lock()
	if len(l.reqs) != 6 {
		t.Errorf("запросов %d", len(l.reqs))
	}
	for _, r := range l.reqs {
		if r.Header.Get("Referer") != "" || r.Header.Get("Cookie") != "" || r.UserAgent() != UserAgent || r.URL.RawQuery != "" {
			t.Errorf("%s: заголовки %v", r.URL.Path, r.Header)
		}
	}
	l.mu.Unlock()
	if _, err := c.small(context.Background(), l.srv.URL+"/r0", 10); !errors.Is(err, ErrAddress) {
		t.Fatalf("шесть переходов: %v", err)
	}
}

// Переход с https на http — отказ, http-сервер не получает запроса.
func TestFetchRedirectHTTP(t *testing.T) {
	var plain atomic.Int32
	ps := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plain.Add(1)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ps.Close()
	l := newTestLib(t)
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ps.URL+"/catalog.json", http.StatusMovedPermanently)
	})
	if _, err := l.client().Catalog(context.Background(), 0); !errors.Is(err, ErrAddress) || plain.Load() != 0 {
		t.Fatalf("на http: %v, запросов http %d", err, plain.Load())
	}
	// И прямой адрес http — тоже без запроса.
	if _, err := l.client().small(context.Background(), ps.URL+"/x", 10); !errors.Is(err, ErrAddress) || plain.Load() != 0 {
		t.Fatalf("http прямо: %v", err)
	}
}

// Ошибки сети словами: 404, другой код, закрытый порт, отмена, чужой сертификат.
func TestFetchErrors(t *testing.T) {
	l := newTestLib(t)
	l.mux.HandleFunc("/500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	c := l.client()
	_, err := c.small(context.Background(), l.srv.URL+"/none", 10)
	var se *StatusError
	if !errors.Is(err, ErrNotFound) || !errors.As(err, &se) || se.Code != 404 || err.Error() != ErrNotFound.Error() {
		t.Fatalf("404: %v", err)
	}
	if _, err := c.small(context.Background(), l.srv.URL+"/500", 10); errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "500") {
		t.Fatalf("500: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	if _, err := c.small(context.Background(), "https://"+addr+"/x", 10); !errors.Is(err, ErrOffline) || strings.Contains(err.Error(), addr) {
		t.Fatalf("закрытый порт: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.small(ctx, l.srv.URL+"/none", 10); !errors.Is(err, ErrCancelled) {
		t.Fatalf("отмена: %v", err)
	}
	src := l.source()
	src.Roots = nil // системные корни сертификат httptest не знают
	if _, err := New(src).small(context.Background(), l.srv.URL+"/none", 10); !errors.Is(err, ErrTLS) {
		t.Fatalf("сертификат: %v", err)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func itoa(i int) string { return strconv.Itoa(i) }

// ghAsset — переход GitHub с адреса ассета на CDN: подпись доступа в параметрах (форма ответа 302).
const ghAsset = "https://release-assets.githubusercontent.com/github-production-release-asset/870001/5f0c-ab12" +
	"?sp=r&sv=2018-11-09&sr=b&spr=https&se=2026-10-18T10%3A05%3A00Z&rscd=attachment%3B+filename%3Divory-1.zip" +
	"&rsct=application%2Foctet-stream&skoid=96c2d410&sktid=398a6654&skt=2026-10-18T09%3A04%3A00Z&ske=2026-10-18T10%3A04%3A00Z" +
	"&sks=b&skv=2018-11-09&sig=Qm9ndXMlMkJzaWc%3D&jwt=eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjF9.c2ln&filename=ivory-1.zip" +
	"&response-content-disposition=attachment%3B%20filename%3Divory-1.zip&response-content-type=application%2Foctet-stream"

// Переход на CDN ассетов: параметры можно только на переходе, только https и только хостам CDN;
// первый запрос — без параметров и туда.
func TestFetchRedirectCDN(t *testing.T) {
	g := GitHub()
	for raw, want := range map[string]bool{
		ghAsset: true,
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/1/2?X-Amz-Signature=ab&X-Amz-Expires=300": true,
		"https://release-assets.githubusercontent.com/x":                                                                        true,
		"https://release-assets.githubusercontent.com/x?":                                                                       true,
		"https://github.com/x?q=1":                                          false, // хост из списка, но не CDN
		"https://raw.githubusercontent.com/x/main/catalog.json?v=2":         false,
		"http://release-assets.githubusercontent.com/x?sig=1":               false,
		"https://u@release-assets.githubusercontent.com/x?sig=1":            false,
		"https://release-assets.githubusercontent.com/x?sig=1#a":            false,
		"https://release-assets.githubusercontent.com.evil.example/x?sig=1": false,
	} {
		req := &http.Request{URL: mustURL(t, raw), Header: http.Header{"Referer": {"https://github.com/"}}}
		err := (&Client{src: g}).redirect(req, []*http.Request{{}})
		if (err == nil) != want {
			t.Errorf("переход %s: %v, ждали %v", raw, err, want)
		}
		if err == nil && req.Header.Get("Referer") != "" {
			t.Errorf("переход %s: Referer остался", raw)
		}
	}
	if g.allowed(mustURL(t, ghAsset)) {
		t.Error("первый запрос с параметрами разрешён")
	}

	// Через сервер: адрес набора → 302 с параметрами → тело. Хост сервера в QueryHosts — архив
	// скачан, параметры дошли; не в QueryHosts (хост из белого списка) — отказ без запроса туда.
	l := newTestLib(t)
	body := []byte("архив набора")
	const query = "sp=r&sv=2018-11-09&sig=Qm9n%3D&jwt=eyJ.eyJ.c2ln&filename=ivory-1.zip"
	l.mux.HandleFunc("/releases/download/ivory-v1/ivory-1.zip", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/cdn/5f0c?"+query, http.StatusFound)
	})
	var cdn atomic.Int32
	l.mux.HandleFunc("/cdn/5f0c", func(w http.ResponseWriter, r *http.Request) {
		cdn.Add(1)
		if r.URL.RawQuery != query {
			t.Errorf("параметры перехода: %q", r.URL.RawQuery)
		}
		_, _ = w.Write(body)
	})
	e := entryFor(l, "/releases/download/ivory-v1/ivory-1.zip", body)
	src := l.source()
	src.QueryHosts = []string{"127.0.0.1"}
	if err := New(src).Download(context.Background(), e, dlPath(t), nil); err != nil || cdn.Load() != 1 {
		t.Fatalf("переход на CDN: %v, запросов CDN %d", err, cdn.Load())
	}
	p := dlPath(t)
	if err := l.client().Download(context.Background(), e, p, nil); !errors.Is(err, ErrAddress) || cdn.Load() != 1 {
		t.Fatalf("параметры не CDN: %v, запросов CDN %d", err, cdn.Load())
	}
	assertGone(t, p)
	// Первый запрос с параметрами и к хосту из QueryHosts — отказ до запроса.
	hits := l.hits.Load()
	if _, err := New(src).small(context.Background(), l.srv.URL+"/cdn/5f0c?"+query, 100); !errors.Is(err, ErrAddress) || l.hits.Load() != hits {
		t.Fatalf("первый запрос с параметрами: %v", err)
	}
}
