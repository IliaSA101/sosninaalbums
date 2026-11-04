package libpacks

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Стенд тестов примера (спека 142): синтетические наборы — пример из трёх стикеров и три донора
// цепочкой needs, которые вместе покрывают его ключи; сеть — httptest с подписанным каталогом.

const (
	demoPackID = "wedding-navy-watercolor-demo"
	docMain    = "wedding-navy-watercolor"
	docFrames  = "wedding-navy-watercolor-frames"
	docLayouts = "wedding-navy-watercolor-layouts"
	demoAsset1 = "wedding-navy-watercolor-demo-v1/wedding-navy-watercolor-demo-1.zip"
)

// pngW — PNG w×20 с прозрачностью; разная ширина — разные байты и разные ключи.
func pngW(t *testing.T, w int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, 20))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// keyOf — ключ предмета библиотеки по байтам (my: и 16 hex sha256).
func keyOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "my:" + hex.EncodeToString(sum[:])[:16]
}

// packZip — архив набора: стикеры ширины widths (по порядку); version/ minApp зафиксированы.
func packZip(t *testing.T, id string, version int, widths ...int) []byte {
	t.Helper()
	files := map[string][]byte{}
	items := []map[string]any{}
	for _, w := range widths {
		name, data := "stickers/s"+itoa(w)+".png", pngW(t, w)
		sum := sha256.Sum256(data)
		files[name] = data
		items = append(items, map[string]any{"file": name, "kind": "sticker", "theme": "wedding", "name": "Стикер " + itoa(w),
			"sha256": hex.EncodeToString(sum[:]), "size": len(data)})
	}
	man, err := json.Marshal(map[string]any{"format": 1, "id": id, "version": version, "minApp": "0.26.0",
		"title": "Набор " + id, "license": "CC0-1.0", "items": items})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	put := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err == nil {
			_, err = w.Write(data)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	put("manifest.json", man)
	for name, data := range files {
		put(name, data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// syncBuf — буфер log.txt, безопасный для фоновых горутин службы.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func catchLog(t *testing.T) *syncBuf {
	t.Helper()
	buf := &syncBuf{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

// demoSrv — библиотека на httptest: подписанный каталог и архивы по /dl/.
type demoSrv struct {
	src  fetch.Source
	priv ed25519.PrivateKey

	catHits, assetHits atomic.Int32

	mu     sync.Mutex
	cat    []byte
	sig    string
	seq    int64
	zips   map[string][]byte // путь после /dl/ → тело
	status map[string]int    // путь → код ответа вместо тела
	got    []string          // запрошенные пути архивов
	hook   func()            // перед отдачей архива
}

func newDemoSrv(t *testing.T) *demoSrv {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	d := &demoSrv{priv: priv, zips: map[string][]byte{}, status: map[string]int{}}
	srv := httptest.NewTLSServer(http.HandlerFunc(d.serve))
	t.Cleanup(srv.Close)
	roots := srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	d.src = fetch.Source{CatalogURL: srv.URL + "/catalog.json", Prefix: srv.URL + "/dl/", Hosts: []string{"127.0.0.1"},
		Keys: []ed25519.PublicKey{pub}, Roots: roots}
	return d
}

func (d *demoSrv) serve(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	cat, sig := d.cat, d.sig
	rel := strings.TrimPrefix(r.URL.Path, "/dl/")
	body, ok := d.zips[rel]
	code := d.status[rel]
	hook := d.hook
	if r.URL.Path != "/catalog.json" {
		d.got = append(d.got, rel)
	}
	d.mu.Unlock()
	switch {
	case r.URL.Path == "/catalog.json":
		d.catHits.Add(1)
		env, _ := json.Marshal(map[string]string{"sig": sig, "body": base64.StdEncoding.EncodeToString(cat)})
		_, _ = w.Write(env)
	case code != 0:
		d.assetHits.Add(1)
		w.WriteHeader(code)
	case ok && hook != nil && len(body) > 1:
		// Заголовки и первый байт уходят, потом хук (клиент уже создал файл), потом остальное.
		d.assetHits.Add(1)
		w.Header().Set("Content-Length", itoa(len(body)))
		_, _ = w.Write(body[:1])
		w.(http.Flusher).Flush()
		hook()
		_, _ = w.Write(body[1:])
	case ok:
		d.assetHits.Add(1)
		_, _ = w.Write(body)
	default:
		http.NotFound(w, r)
	}
}

// row — строка каталога: набор id версии v, архив zipBytes, needs — донор.
type row struct {
	id, needs string
	version   int
	zip       []byte
}

func relOf(id string, v int) string { return id + "-v" + itoa(v) + "/" + id + "-" + itoa(v) + ".zip" }

// publish — каталог из строк rows (подписан ключом сервера) и их архивы.
func (d *demoSrv) publish(rows ...row) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []map[string]any{}
	for _, r := range rows {
		d.zips[relOf(r.id, r.version)] = r.zip
		sum := sha256.Sum256(r.zip)
		e := map[string]any{"id": r.id, "version": r.version, "minApp": "0.26.0", "title": "Набор " + r.id, "size": len(r.zip),
			"sha256": hex.EncodeToString(sum[:]), "url": d.src.Prefix + relOf(r.id, r.version), "counts": map[string]int{"sticker": 1}}
		if r.needs != "" {
			e["needs"] = r.needs
		}
		out = append(out, e)
	}
	d.seq++
	d.cat, _ = json.Marshal(map[string]any{"format": 1, "seq": d.seq, "packs": out})
	d.sig = base64.StdEncoding.EncodeToString(ed25519.Sign(d.priv, d.cat))
}

func (d *demoSrv) setAsset(rel string, body []byte) {
	d.mu.Lock()
	d.zips[rel] = body
	d.mu.Unlock()
}

func (d *demoSrv) setStatus(rel string, code int) {
	d.mu.Lock()
	d.status[rel] = code
	d.mu.Unlock()
}

func (d *demoSrv) asked() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.got...)
}

// demoEnv — служба с примером: файл у «программы» (hasFile/fileZip) и сервер библиотеки.
type demoEnv struct {
	t       *testing.T
	svc     *Service
	ev      *evs
	st      *decor.Store
	dir     string
	srv     *demoSrv
	log     *syncBuf
	version int // версия примера в Demo

	hasFile atomic.Bool
	fileZip atomic.Value // []byte
	reads   atomic.Int32
	relinks atomic.Int32

	// Содержимое наборов: пример — s21…s24; доноры — основной s21, s22; рамки s23; макеты s24: покрывают пример только все три.
	demoZip, mainZip, framesZip, layoutsZip []byte
}

// newDemoEnv — служба с примером и согласием на сеть; file — файл примера есть у программы.
func newDemoEnv(t *testing.T, file bool, mut func(h *Host)) *demoEnv {
	t.Helper()
	e := &demoEnv{t: t, ev: &evs{}, srv: newDemoSrv(t), log: catchLog(t), version: 1}
	e.demoZip = packZip(t, demoPackID, 1, 21, 22, 23, 24)
	e.mainZip = packZip(t, docMain, 1, 21, 22)
	e.framesZip = packZip(t, docFrames, 1, 23)
	e.layoutsZip = packZip(t, docLayouts, 1, 24)
	e.hasFile.Store(file)
	e.fileZip.Store(e.demoZip)
	h := testHost(t, e.ev, func(h *Host) {
		h.Demo = e.demo
		h.Fetcher = func() *fetch.Client { return fetch.New(e.srv.src) }
		h.Relink = func(*decor.Store) { e.relinks.Add(1) }
		if mut != nil {
			mut(h)
		}
	})
	e.svc = New(h)
	e.dir = h.CfgDir()
	e.st, _ = h.Store()
	e.st.SetDemo(demoPackID, []string{docMain, docFrames, docLayouts})
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.Allowed = true })
	return e
}

func (e *demoEnv) demo() Demo {
	sum := sha256.Sum256(e.demoZip)
	return Demo{
		ID: demoPackID, Version: e.version, Related: []string{docMain, docFrames, docLayouts}, Asset: demoAsset1,
		Entry: fetch.Entry{ID: demoPackID, Version: e.version, Title: "Свадьба — синяя акварель (пример)", About: "О примере", License: "CC0-1.0",
			Size: int64(len(e.demoZip)), SHA256: hex.EncodeToString(sum[:]), Counts: fetch.Counts{Bg: 4, Sticker: 7, Spread: 4, Frame: 2}},
		Has: func() bool { return e.hasFile.Load() },
		Read: func() ([]byte, bool) {
			e.reads.Add(1)
			z, _ := e.fileZip.Load().([]byte)
			return z, z != nil
		},
	}
}

// publishDemoAsset — ассет примера на сервере по Asset (настоящие байты).
func (e *demoEnv) publishDemoAsset() { e.srv.setAsset(demoAsset1, e.demoZip) }

// publishChain — каталог из трёх доноров цепочкой needs.
func (e *demoEnv) publishChain() {
	e.srv.publish(row{id: docMain, version: 1, zip: e.mainZip}, row{id: docFrames, needs: docMain, version: 1, zip: e.framesZip},
		row{id: docLayouts, needs: docFrames, version: 1, zip: e.layoutsZip})
}

// stands — набор id стоит; origin — его происхождение.
func (e *demoEnv) stands(id string) (origin string, ok bool) {
	for _, p := range e.st.Packs() {
		if p.ID == id {
			return p.Origin, true
		}
	}
	return "", false
}

func (e *demoEnv) mustStand(id, origin string) {
	e.t.Helper()
	if got, ok := e.stands(id); !ok || got != origin {
		e.t.Fatalf("%s: стоит %v, origin %q, ждали %q\n%s", id, ok, got, origin, e.log.String())
	}
}

// wantZipBytes — размер набора id в списке (PackInfo.Bytes) равен размеру zip, а не сумме на диске.
func (e *demoEnv) wantZipBytes(id string, want int64) {
	e.t.Helper()
	for _, p := range e.st.Packs() {
		if p.ID == id {
			if p.Bytes != want || e.st.PackDiskBytes(id) == want {
				e.t.Fatalf("%s: Bytes %d, ждали размер zip %d (на диске %d)", id, p.Bytes, want, e.st.PackDiskBytes(id))
			}
			return
		}
	}
	e.t.Fatalf("%s не стоит", id)
}

func (e *demoEnv) mustNotStand(id string) {
	e.t.Helper()
	if got, ok := e.stands(id); ok {
		e.t.Fatalf("%s стоит (origin %q), а не должен\n%s", id, got, e.log.String())
	}
}

// addFile — набор из файла (AddPath): так донор или сам пример встаёт «из файла».
func (e *demoEnv) addFile(zipBytes []byte, name string) Add {
	e.t.Helper()
	p := filepath.Join(e.dir, name+".zip")
	if err := os.WriteFile(p, zipBytes, 0o600); err != nil {
		e.t.Fatal(err)
	}
	res, err := e.svc.AddPath(p)
	if err != nil {
		e.t.Fatalf("AddPath %s: %v", name, err)
	}
	return res
}

// placed — пример поставлен из файла службой.
func (e *demoEnv) place() {
	e.t.Helper()
	e.hasFile.Store(true)
	e.svc.PlaceDemo(e.t.Context())
	e.mustStand(demoPackID, decor.OriginDemo)
}

func (e *demoEnv) skip() []string { return prefs.LibraryNetOf(e.dir).Skip }

func (e *demoEnv) rowIDs(list List) []string {
	ids := []string{}
	for _, a := range list.Available {
		ids = append(ids, a.ID)
	}
	return ids
}
