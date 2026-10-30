package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// testPair — тестовая пара ed25519 из зерна тестовой пары ed25519: пара одна на все тесты и фикстуру testdata/ (закрытый
// ключ выпуска сюда не попадает никогда — это ключ только для тестов).
func testPair() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := sha256.Sum256([]byte("sosninaalbums-update-test-key"))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return priv.Public().(ed25519.PublicKey), priv
}

// feed — TLS-сервер «GitHub» с манифестом и ассетами и источник, который ему доверяет; адреса
// те же, что у выпуска, но на 127.0.0.1.
type feed struct {
	srv  *httptest.Server
	mux  *http.ServeMux
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	hits atomic.Int32
	raw  atomic.Pointer[[]byte] // конверт по адресу манифеста; nil — 404
	fail atomic.Bool            // манифест отвечает 502
}

func newFeed(t *testing.T) *feed {
	t.Helper()
	pub, priv := testPair()
	f := &feed{mux: http.NewServeMux(), pub: pub, priv: priv}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.mux.ServeHTTP(w, r)
	}))
	f.mux.HandleFunc("/latest/"+ManifestName, func(w http.ResponseWriter, _ *http.Request) {
		env := f.raw.Load()
		switch {
		case f.fail.Load():
			http.Error(w, "сбой", http.StatusBadGateway)
		case env == nil:
			http.NotFound(w, nil)
		default:
			_, _ = w.Write(*env)
		}
	})
	t.Cleanup(f.srv.Close)
	return f
}

func (f *feed) source() fetch.Source {
	pool := f.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	return fetch.Source{CatalogURL: f.srv.URL + "/latest/" + ManifestName, Prefix: f.srv.URL + "/releases/download/",
		Hosts: []string{"127.0.0.1"}, Keys: []ed25519.PublicKey{f.pub}, Roots: pool}
}

func (f *feed) client() *Client { return New(f.source()) }

// envelope — SosninaAlbums-update.json: {"sig": подпись байтов body, "body": base64 body}.
func envelope(priv ed25519.PrivateKey, body []byte) []byte {
	data, err := json.Marshal(map[string]string{
		"sig":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, body)),
		"body": base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		panic(err)
	}
	return data
}

// serveManifest — манифест по адресу выпуска, подписанный ключом сервера.
func (f *feed) serveManifest(body []byte) { f.serveRaw(envelope(f.priv, body)) }

// serveRaw — готовый конверт как есть.
func (f *feed) serveRaw(env []byte) { f.raw.Store(&env) }

// setup — установщик заданного размера: содержимое детерминировано, запись манифеста к нему.
func setup(ver string, size int) ([]byte, File) {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*7 + len(ver))
	}
	sum := sha256.Sum256(data)
	return data, File{Name: SetupName(ver), Size: int64(size), SHA256: hex.EncodeToString(sum[:])}
}

// manifestBody — body манифеста версии ver; f nil — без записи Windows. Лишние ключи из rest.
func manifestBody(t *testing.T, ver string, f *File, rest map[string]any) []byte {
	t.Helper()
	m := map[string]any{"format": 1, "app": "sosninaalbums", "version": ver, "date": "2026-11-02",
		"notes": []string{"Первое", "Второе"}, "files": map[string]any{}}
	if f != nil {
		m["files"] = map[string]any{FileKey: f}
	}
	for k, v := range rest {
		m[k] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// swapBody — подпись из sigFrom, body из bodyFrom: подделка «подпись на одних байтах, текст другой».
func swapBody(t *testing.T, sigFrom, bodyFrom []byte) []byte {
	t.Helper()
	var a, b map[string]string
	if err := json.Unmarshal(sigFrom, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bodyFrom, &b); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{"sig": a["sig"], "body": b["body"]})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// serveSetup — ассет выпуска vX.Y.Z.
func (f *feed) serveSetup(ver string, data []byte) {
	f.mux.HandleFunc("/releases/download/v"+ver+"/"+SetupName(ver), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	})
}
