package guidevideo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// lab — подставная библиотека: TLS-сервер с подписанным тестовым ключом каталогом (поле videos),
// ассеты-ролики, папка кэша во временной папке и служба с Host поверх них. Настоящий ключ владельца
// и сеть наружу тесты не трогают.
type lab struct {
	t    *testing.T
	srv  *httptest.Server
	mux  *http.ServeMux
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	dir  string
	seq  atomic.Int64

	mu       sync.Mutex
	videos   map[string]map[string]any // "id/версия" → строка каталога
	order    []string
	files    map[string][]byte // путь на сервере → тело
	extra    []map[string]any  // строки videos «как есть» (битые)
	events   []Progress
	revealed []string

	hits    atomic.Int32
	allowed atomic.Bool
	cached  atomic.Pointer[fetch.Catalog]
	host    Host
	svc     *Service
}

func newLab(t *testing.T) *lab {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	l := &lab{t: t, mux: http.NewServeMux(), pub: pub, priv: priv, videos: map[string]map[string]any{}, files: map[string][]byte{}}
	l.seq.Store(1)
	l.dir = filepath.Join(testkit.TempDir(t), "guide-videos")
	l.allowed.Store(true)
	l.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.hits.Add(1)
		l.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(l.srv.Close)
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(l.envelope()) })
	l.mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		body, ok := l.files[r.URL.Path]
		l.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	src := fetch.Source{CatalogURL: l.srv.URL + "/main/catalog.json", Prefix: l.srv.URL + "/releases/download/",
		Hosts: []string{"127.0.0.1"}, Keys: []ed25519.PublicKey{pub}, Roots: l.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
	l.host = Host{
		CacheDir: func() string { return l.dir },
		App:      func() string { return "0.30.0" },
		OS:       func() string { return "linux" },
		Allowed:  l.allowed.Load,
		StopCtx:  context.Background,
		Cached: func() (fetch.Catalog, bool) {
			if c := l.cached.Load(); c != nil {
				return *c, true
			}
			return fetch.Catalog{}, false
		},
		Fresh: func(ctx context.Context) (fetch.Catalog, error) {
			cat, err := fetch.New(src).Catalog(ctx, 0)
			if err == nil {
				cat.Env = nil
				l.cached.Store(&cat)
			}
			return cat, err
		},
		Client: func() *fetch.Client { return fetch.New(src) },
		Emit: func(name string, data ...any) {
			if p, ok := data[0].(Progress); ok && name == "guide:video" {
				l.mu.Lock()
				l.events = append(l.events, p)
				l.mu.Unlock()
			}
		},
		Reveal: func(p string) error {
			l.mu.Lock()
			l.revealed = append(l.revealed, p)
			l.mu.Unlock()
			return nil
		},
		OpenRegular: imgfile.OpenRegular,
		Free:        func(string) (uint64, error) { return 1 << 40, nil },
		Rename:      os.Rename,
	}
	l.svc = New(l.host)
	return l
}

// rebuild пересобирает службу после правки l.host.
func (l *lab) rebuild() { l.svc = New(l.host) }

// video кладёт ролик в каталог и ассеты: bodies — вид → тело файла. Возвращает путь файла в кэше.
func (l *lab) video(id string, ver int, bodies map[string][]byte) {
	l.t.Helper()
	files := map[string]any{}
	for kind, body := range bodies {
		name := id + "-" + strconv.Itoa(ver) + "." + kind
		sum := sha256.Sum256(body)
		path := "/releases/download/videos-1/" + name
		l.mu.Lock()
		l.files[path] = body
		l.mu.Unlock()
		files[kind] = map[string]any{"size": len(body), "sha256": hex.EncodeToString(sum[:]), "url": l.srv.URL + path}
	}
	row := map[string]any{"id": id, "version": ver, "minApp": "0.30.0", "seconds": 20, "files": files}
	l.mu.Lock()
	key := id + "/" + strconv.Itoa(ver)
	if _, ok := l.videos[key]; !ok {
		l.order = append(l.order, key)
	}
	l.videos[key] = row
	l.mu.Unlock()
}

// webm — ролик одного вида webm; тело — n байт, различимых по seed.
func (l *lab) webm(id string, ver, n int) []byte {
	l.t.Helper()
	body := make([]byte, n)
	for i := range body {
		body[i] = byte(i*7 + ver + len(id))
	}
	l.video(id, ver, map[string][]byte{fetch.KindWebM: body})
	return body
}

// envelope — catalog.json: {sig, body}; body — format 1, seq, packs [], videos.
func (l *lab) envelope() []byte {
	l.mu.Lock()
	rows := make([]any, 0, len(l.order)+len(l.extra))
	for _, k := range l.order {
		rows = append(rows, l.videos[k])
	}
	for _, e := range l.extra {
		rows = append(rows, e)
	}
	l.mu.Unlock()
	body, err := json.Marshal(map[string]any{"format": 1, "seq": l.seq.Load(), "packs": []any{}, "videos": rows})
	if err != nil {
		panic(err)
	}
	env, err := json.Marshal(map[string]string{
		"sig":  base64.StdEncoding.EncodeToString(ed25519.Sign(l.priv, body)),
		"body": base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		panic(err)
	}
	return env
}

// refresh кладёт свежий каталог в «сохранённый» (как libpacks после Fresh) без скачивания роликов.
func (l *lab) refresh() {
	l.t.Helper()
	if _, err := l.host.Fresh(context.Background()); err != nil {
		l.t.Fatal(err)
	}
}

// path — файл ролика в кэше.
func (l *lab) path(id string, ver int, kind string) string {
	return filepath.Join(l.dir, id+"-"+strconv.Itoa(ver)+"."+kind)
}

// listDir — имена в папке кэша (нет папки — пусто).
func (l *lab) listDir() []string {
	list, _ := os.ReadDir(l.dir)
	var out []string
	for _, e := range list {
		out = append(out, e.Name())
	}
	return out
}

func (l *lab) put(name string, body []byte) {
	l.t.Helper()
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		l.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.dir, name), body, 0o600); err != nil {
		l.t.Fatal(err)
	}
}
