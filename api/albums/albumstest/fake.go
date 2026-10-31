// Package albumstest — поддельный сервер листалок по контракту /v1 (для тестов программы и стенда QA)
// и набор проверок Conformance для любой реализации контракта.
//
// Сервер не для выпуска: пароль хранится простым хэшем, попыток входа не считает, диск — только
// под <dir>/albums-fake/. Он нужен, чтобы служба выгрузки и QA не ждали настоящий сервис.
package albumstest

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// Fault — отказ вместо обычного ответа на запрос /v1 (крючок для тестов повторов и обрывов).
type Fault struct {
	Status     int  // HTTP-статус вместо ответа (0 и не Drop — обычный ответ)
	RetryAfter int  // секунд в заголовке Retry-After (для 429)
	Drop       bool // оборвать соединение без ответа
}

// Options — настройки поддельного сервера.
type Options struct {
	Token      string        // единственный годный токен; пусто — случайный
	OtherToken string        // второй владелец: его альбомы для токена Token «чужие» (пусто — нет)
	Limits     albums.Limits // пределы; нули — DefaultLimits
	Dir        string        // абсолютная папка для файлов страниц (пишет в <Dir>/albums-fake/); пусто — память
	Now        func() time.Time
	// Fault вызывается на каждый запрос /v1 до разбора; nil или (nil) — без отказа.
	Fault func(r *http.Request) *Fault
}

// Fake — http.Handler поддельного сервера. Безопасен для одновременных запросов.
type Fake struct {
	opt    Options
	tokens []string
	lim    albums.Limits
	store  blobs
	key    [32]byte // подпись cookie входа
	mux    *http.ServeMux

	mu     sync.Mutex
	albums map[string]*album
	url    string
	puts   int
}

type album struct {
	id, owner string
	pending   albums.Manifest
	pub       *albums.Manifest // опубликованная редакция, с именами файлов
	expires   time.Time
	salt      [16]byte
	pwHash    [32]byte
	hasPW     bool
	gen       int // поколение входов; revoke_sessions увеличивает
	stored    map[string]bool
}

// NewFake — сервер без httptest (для cmd/albums-fake); адрес ссылок задаёт SetURL.
func NewFake(opt Options) (*Fake, error) {
	if opt.Token == "" {
		opt.Token = "fake-" + randHex(16)
	}
	if err := albums.CheckToken(opt.Token); err != nil {
		return nil, err
	}
	if opt.OtherToken != "" {
		if err := albums.CheckToken(opt.OtherToken); err != nil || opt.OtherToken == opt.Token {
			return nil, errors.New("albumstest: OtherToken негоден или равен Token")
		}
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	f := &Fake{opt: opt, lim: opt.Limits.OrDefault(), albums: map[string]*album{}, tokens: []string{opt.Token}}
	if opt.OtherToken != "" {
		f.tokens = append(f.tokens, opt.OtherToken)
	}
	if _, err := rand.Read(f.key[:]); err != nil {
		return nil, err
	}
	var err error
	if f.store, err = newBlobs(opt.Dir); err != nil {
		return nil, err
	}
	f.routes()
	return f, nil
}

// SetURL задаёт адрес сервера для ссылок в ответах: http://127.0.0.1:порт.
func (f *Fake) SetURL(u string) {
	f.mu.Lock()
	f.url = strings.TrimRight(u, "/")
	f.mu.Unlock()
}

func (f *Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) { f.mux.ServeHTTP(w, r) }

func (f *Fake) routes() {
	m := http.NewServeMux()
	api := func(h func(w http.ResponseWriter, r *http.Request, owner string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if f.fault(w, r) {
				return
			}
			owner, ok := f.auth(r)
			if !ok {
				writeErr(w, http.StatusUnauthorized, albums.CodeUnauthorized, "нужен токен")
				return
			}
			h(w, r, owner)
		}
	}
	m.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		if f.fault(w, r) {
			return
		}
		writeJSON(w, http.StatusOK, albums.Health{API: "v1", Limits: f.lim})
	})
	m.HandleFunc("GET /v1/albums", api(f.list))
	m.HandleFunc("POST /v1/albums", api(f.create))
	m.HandleFunc("PUT /v1/albums/{id}/pages/{n}", api(f.putPage))
	m.HandleFunc("POST /v1/albums/{id}/publish", api(f.publish))
	m.HandleFunc("PUT /v1/albums/{id}", api(f.update))
	m.HandleFunc("PATCH /v1/albums/{id}", api(f.patch))
	m.HandleFunc("DELETE /v1/albums/{id}", api(f.remove))
	m.HandleFunc("GET /a/{id}/{$}", f.viewPage)
	m.HandleFunc("POST /a/{id}/login", f.viewLogin)
	m.HandleFunc("GET /a/{id}/manifest.json", f.viewManifest)
	m.HandleFunc("GET /a/{id}/{file}", f.viewFile)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, albums.CodeNotFound, "нет такого адреса")
	})
	f.mux = m
}

// fault применяет крючок Options.Fault; true — запрос уже обработан.
func (f *Fake) fault(w http.ResponseWriter, r *http.Request) bool {
	if f.opt.Fault == nil {
		return false
	}
	ft := f.opt.Fault(r)
	if ft == nil {
		return false
	}
	if ft.Drop {
		if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
			_ = conn.Close()
		}
		return true
	}
	if ft.Status == 0 {
		return false
	}
	if ft.RetryAfter > 0 {
		w.Header().Set("Retry-After", itoa(ft.RetryAfter))
	}
	writeErr(w, ft.Status, codeFor(ft.Status), "отказ по крючку теста")
	return true
}

func codeFor(status int) string {
	switch status {
	case 400:
		return albums.CodeBadManifest
	case 401:
		return albums.CodeUnauthorized
	case 404:
		return albums.CodeNotFound
	case 413:
		return albums.CodeTooLarge
	case 429:
		return albums.CodeRateLimited
	}
	return albums.CodeInternal
}

// auth — владелец по заголовку Authorization: Bearer; сравнение за постоянное время.
func (f *Fake) auth(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	tok, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return "", false
	}
	owner, found := "", false
	for _, t := range f.tokens {
		if subtle.ConstantTimeCompare([]byte(tok), []byte(t)) == 1 {
			owner, found = t, true
		}
	}
	return owner, found
}

// lookup — альбом по id для владельца. Нет, чужой, истёкший и негодный id неразличимы: nil.
// Вызывать под f.mu. id проверяется CheckID до обращения к таблице и диску.
func (f *Fake) lookup(id, owner string) *album {
	if albums.CheckID(id) != nil {
		return nil
	}
	a := f.albums[id]
	if a == nil || a.owner != owner {
		return nil
	}
	if f.expired(a) {
		f.drop(a)
		return nil
	}
	return a
}

func (f *Fake) expired(a *album) bool { return !f.opt.Now().Before(a.expires) }

func (f *Fake) drop(a *album) {
	delete(f.albums, a.id)
	f.store.drop(a.id)
}

// readJSON читает тело запроса не больше MaxBody; false — ответ уже отправлен.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, albums.MaxBody))
	if err != nil {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			writeErr(w, http.StatusRequestEntityTooLarge, albums.CodeTooLarge, "тело больше 1 МБ")
		} else {
			writeErr(w, http.StatusBadRequest, albums.CodeBadManifest, "тело не читается")
		}
		return false
	}
	if err := json.Unmarshal(raw, v); err != nil {
		writeErr(w, http.StatusBadRequest, albums.CodeBadManifest, "не JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, albums.ErrorBody{Error: albums.Error{Code: code, Message: msg}})
}

func notFound(w http.ResponseWriter) {
	writeErr(w, http.StatusNotFound, albums.CodeNotFound, "альбома нет")
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newID — 22 знака base64url (128 бит), первый не «-».
func newID() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
	for {
		b := make([]byte, 22)
		_, _ = rand.Read(b)
		for i := range b {
			b[i] = alphabet[b[i]&63]
		}
		if b[0] != '-' {
			return string(b)
		}
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// Server — Fake под httptest: адрес в URL, ссылки в ответах настроены.
type Server struct {
	*httptest.Server
	Fake       *Fake
	Token      string
	OtherToken string
}

// New запускает поддельный сервер на loopback. Ошибка настройки (Dir, токен) — паника: вызывают тесты.
func New(opt Options) *Server {
	f, err := NewFake(opt)
	if err != nil {
		panic("albumstest.New: " + err.Error())
	}
	srv := httptest.NewServer(f)
	f.SetURL(srv.URL)
	return &Server{Server: srv, Fake: f, Token: f.opt.Token, OtherToken: f.opt.OtherToken}
}

// Albums — сколько альбомов сейчас на сервере.
func (f *Fake) Albums() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.albums)
}

// Puts — сколько страниц принято за всё время (успешные PUT).
func (f *Fake) Puts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.puts
}

// Stored — хэши страниц альбома, лежащих на сервере.
func (f *Fake) Stored(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	if a := f.albums[id]; a != nil {
		for sha := range a.stored {
			out = append(out, sha)
		}
	}
	return out
}

// Published — манифест опубликованной редакции (с именами файлов); nil — не опубликован или нет.
func (f *Fake) Published(id string) *albums.Manifest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a := f.albums[id]; a != nil && a.pub != nil {
		m := *a.pub
		m.Pages = append([]albums.Page(nil), a.pub.Pages...)
		return &m
	}
	return nil
}

// RootDir — папка файлов страниц (<Dir>/albums-fake) или "" в памяти.
func (f *Fake) RootDir() string {
	if d, ok := f.store.(*diskBlobs); ok {
		return filepath.Clean(d.root)
	}
	return ""
}
