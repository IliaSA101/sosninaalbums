package albumstest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// Option — настройка Conformance.
type Option func(*conf)

// WithOtherToken — токен второго владельца: тогда проверяется и «чужой альбом» (404, как несуществующий).
// Без него проверяются только несуществующие и удалённые id.
func WithOtherToken(tok string) Option { return func(c *conf) { c.other = tok } }

// Conformance — набор проверок любой реализации контракта /v1 по адресу base с токеном token. Проверяет
// и безопасность (разбор С8): токены, одинаковые 404, вид id, пароль не в ответах, сигнатуры и хэши
// страниц, вход к альбому с паролем, сброс входов, заголовки страниц.
//
// ТОЛЬКО на тестовом экземпляре: набор создаёт, правит и удаляет альбомы этого токена. Блоки
// независимы (t.Run): провал одного не прерывает остальные, все провалы видны за один прогон.
// Страницы клиента ждутся по соглашению README: /a/<id>/manifest.json, файлы из поля file и
// POST /a/<id>/login (форма, поле password, ответ — Set-Cookie).
func Conformance(t *testing.T, base, token string, opts ...Option) {
	t.Helper()
	c := &conf{t: t, base: strings.TrimRight(base, "/"), token: token,
		http: &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	for _, o := range opts {
		o(c)
	}
	if _, err := rand.Read(c.salt[:]); err != nil {
		t.Fatalf("Conformance: нет случайных байт: %v", err)
	}
	api, err := albums.New(c.base, token, albums.Options{Version: "conformance"})
	if err != nil {
		t.Fatalf("Conformance: адрес или токен не годятся: %v", err)
	}
	c.api = api
	h, err := api.Health(context.Background())
	if err != nil {
		t.Fatalf("Conformance: health недоступен: %v", err)
	}
	c.lim = h.Limits
	for _, b := range []struct {
		name string
		run  func(*conf, *testing.T)
	}{
		{"health", (*conf).health}, {"auth", (*conf).auth}, {"ids", (*conf).ids}, {"foreign", (*conf).foreign},
		{"input", (*conf).input}, {"pages", (*conf).pages}, {"publish", (*conf).publishFlow},
		{"password", (*conf).password}, {"revoke", (*conf).revoke}, {"lifecycle", (*conf).lifecycle},
	} {
		t.Run(b.name, func(t *testing.T) { b.run(c, t) })
	}
}

type conf struct {
	t           *testing.T
	base, token string
	other       string
	api         *albums.Client
	lim         albums.Limits
	http        *http.Client
	salt        [8]byte // на прогон: картинки не совпадают с прошлыми запусками (сервис с хранением по хэшу не ответит пустым missing)
}

// resp — ответ с телом не больше 1 МБ + 1.
type resp struct {
	status int
	hdr    http.Header
	body   []byte
}

func (r *resp) errCode() string {
	var eb albums.ErrorBody
	_ = json.Unmarshal(r.body, &eb)
	return eb.Error.Code
}

// do — сырой запрос; tok пуст — без Authorization.
func (c *conf) do(t *testing.T, method, path, tok string, body io.Reader, hdr map[string]string) *resp {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, c.base+path, body)
	if err != nil {
		t.Fatalf("запрос %s %s: %v", method, path, err)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, shortPath(path), netCause(err))
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(res.Body, albums.MaxBody+1))
	return &resp{status: res.StatusCode, hdr: res.Header, body: data}
}

// netCause — причина сбоя без *url.Error: в его тексте адрес с id (ключ доступа) или токен в запросе.
func netCause(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// shortPath — путь для сообщений без id (id — ключ доступа, в журналы CI его не несём).
func shortPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if i > 0 && len(s) >= albums.MinID {
			parts[i] = "<id>"
		}
	}
	return strings.Join(parts, "/")
}

func (c *conf) json(t *testing.T, method, path, tok string, v any) *resp {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return c.do(t, method, path, tok, bytes.NewReader(b), map[string]string{"Content-Type": "application/json"})
}

// book — образец книги из четырёх страниц: картинка, белая, две картинки.
type book struct {
	m    albums.Manifest
	imgs map[int][]byte // n → JPEG
}

// sample — JPEG 16×16; seed отличает файлы друг от друга, salt — прогоны. Соль лежит в сегменте COM
// сразу за SOI: декодеру она не мешает, а хэш у каждого прогона свой.
func sample(seed uint8, salt [8]byte) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: seed * 20, G: uint8(x * 15), B: uint8(y * 15), A: 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		panic(err)
	}
	data := b.Bytes()
	out := append([]byte{}, data[:2]...) // FF D8
	out = append(out, 0xFF, 0xFE, 0, byte(2+len(salt)))
	out = append(out, salt[:]...)
	return append(out, data[2:]...)
}

// testSalt — соль тестов самого пакета (не прогонов Conformance).
var testSalt = func() (s [8]byte) { _, _ = rand.Read(s[:]); return }()

func newBook(seed uint8) *book { return newBookSalt(seed, testSalt) }

func (c *conf) newBook(seed uint8) *book { return newBookSalt(seed, c.salt) }

func (c *conf) sample(seed uint8) []byte { return sample(seed, c.salt) }

func newBookSalt(seed uint8, salt [8]byte) *book {
	bk := &book{imgs: map[int][]byte{1: sample(seed, salt), 3: sample(seed+1, salt), 4: sample(seed+2, salt)}}
	bk.m = albums.Manifest{Schema: 1, Generator: "albums-conformance", Title: "Conformance", Book: albums.Book{PageMM: albums.Size{W: 100, H: 100}}}
	for n := 1; n <= 4; n++ {
		p := albums.Page{N: n, Role: albums.RolePage, Blank: true}
		if data, ok := bk.imgs[n]; ok {
			p = pageOf(n, data)
		}
		bk.m.Pages = append(bk.m.Pages, p)
	}
	return bk
}

func (c *conf) request(bk *book, password string) *albums.CreateRequest {
	return &albums.CreateRequest{Manifest: bk.m, ExpiresAt: time.Now().Add(48 * time.Hour).UTC(), Password: password}
}

// create создаёт альбом через клиент; удаление — в конце блока.
func (c *conf) create(t *testing.T, bk *book, password string) *albums.Created {
	t.Helper()
	cr, err := c.api.Create(context.Background(), c.request(bk, password))
	if err != nil {
		t.Fatalf("создание альбома: %v", err)
	}
	t.Cleanup(func() { _ = c.api.Delete(context.Background(), cr.ID) })
	return cr
}

// upload загружает все недостающие страницы через клиент.
func (c *conf) upload(t *testing.T, id string, bk *book, missing []int) {
	t.Helper()
	for _, n := range missing {
		data := bk.imgs[n]
		if err := c.api.PutPage(context.Background(), id, n, albums.TypeJPEG, bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("страница %d: %v", n, err)
		}
	}
}

// published — альбом создан, загружен и опубликован.
func (c *conf) published(t *testing.T, bk *book, password string) *albums.Created {
	t.Helper()
	cr := c.create(t, bk, password)
	c.upload(t, cr.ID, bk, cr.Missing)
	if _, err := c.api.Publish(context.Background(), cr.ID); err != nil {
		t.Fatalf("публикация: %v", err)
	}
	return cr
}

func (c *conf) health(t *testing.T) {
	r := c.do(t, "GET", "/v1/health", "", nil, nil)
	var h albums.Health
	if r.status != 200 || json.Unmarshal(r.body, &h) != nil || h.API != "v1" {
		t.Fatalf("health без токена: %d %s", r.status, r.body)
	}
	if h.Limits.PageBytes < 1 || h.Limits.Pages < 2 || h.Limits.AlbumBytes < 1 || h.Limits.TitleLen < 1 || h.Limits.MaxDays < 1 {
		t.Errorf("пределы health: %+v", h.Limits)
	}
}

// auth: без токена и с чужим — 401 везде, кроме health.
func (c *conf) auth(t *testing.T) {
	id := strings.Repeat("a", 22)
	for _, ep := range []struct{ method, path string }{
		{"GET", "/v1/albums"}, {"POST", "/v1/albums"}, {"PUT", "/v1/albums/" + id}, {"PATCH", "/v1/albums/" + id},
		{"DELETE", "/v1/albums/" + id}, {"PUT", "/v1/albums/" + id + "/pages/1"}, {"POST", "/v1/albums/" + id + "/publish"},
	} {
		for name, tok := range map[string]string{"без токена": "", "чужой токен": "x" + c.token + "x", "пустой Bearer": " "} {
			hdr := map[string]string{"Content-Type": "application/json"}
			if name == "пустой Bearer" {
				hdr["Authorization"] = "Bearer "
			}
			r := c.do(t, ep.method, ep.path, tok, strings.NewReader("{}"), hdr)
			if r.status != 401 || r.errCode() != albums.CodeUnauthorized {
				t.Errorf("%s %s (%s): %d %s", ep.method, shortPath(ep.path), name, r.status, r.errCode())
			}
		}
	}
	// другая схема авторизации тоже отказ
	r := c.do(t, "GET", "/v1/albums", "", nil, map[string]string{"Authorization": "Basic " + c.token})
	if r.status != 401 {
		t.Errorf("схема Basic: %d", r.status)
	}
	// токен в строке запроса не принимается
	if r := c.do(t, "GET", "/v1/albums?token="+url.QueryEscape(c.token), "", nil, nil); r.status != 401 {
		t.Errorf("токен в запросе: %d", r.status)
	}
}

// ids: новый id — не короче 22 знаков, из алфавита, первый не «-», разные.
func (c *conf) ids(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		cr := c.create(t, c.newBook(uint8(i)), "")
		if err := albums.CheckID(cr.ID); err != nil {
			t.Errorf("id нового альбома: %v", err)
		}
		if seen[cr.ID] {
			t.Error("id повторился")
		}
		seen[cr.ID] = true
		if cr.URL == "" || len(cr.URL) > albums.MaxURL || cr.ExpiresAt.IsZero() {
			t.Errorf("ответ создания: %+v", cr)
		}
	}
}

// foreign: чужой, удалённый и несуществующий id отвечают одинаково — 404 not_found.
func (c *conf) foreign(t *testing.T) {
	ids := c.foreignIDs(t)
	good := c.newBook(3).m
	for name, id := range ids {
		for _, ep := range []struct {
			method, path string
			body         any
		}{
			{"PUT", "", albums.UpdateRequest{Manifest: good}}, {"PATCH", "", albums.Patch{RevokeSessions: true}}, {"DELETE", "", nil},
			{"PUT", "/pages/1", nil}, {"POST", "/publish", nil},
		} {
			path := "/v1/albums/" + id + ep.path
			var r *resp
			if ep.body != nil {
				r = c.json(t, ep.method, path, c.token, ep.body)
			} else {
				r = c.do(t, ep.method, path, c.token, strings.NewReader("x"), map[string]string{"Content-Type": albums.TypeJPEG})
			}
			if r.status != 404 || r.errCode() != albums.CodeNotFound {
				t.Errorf("%s: %s %s → %d %s, нужно 404 not_found", name, ep.method, shortPath(path), r.status, r.errCode())
			}
		}
	}
	// негодные id в пути — не 2xx и не 5xx
	for _, id := range []string{"..", "%2e%2e", "-" + strings.Repeat("a", 30), strings.Repeat("a", 500), "a%00b"} {
		r := c.do(t, "DELETE", "/v1/albums/"+id, c.token, nil, nil)
		// 3xx — серверы чистят путь с «..» переходом
		if r.status != 404 && r.status != 400 && (r.status < 300 || r.status >= 400) {
			t.Errorf("DELETE %q: %d", shortPath(id), r.status)
		}
	}
}

// foreignIDs — id, на которые сервис обязан отвечать одинаково: несуществующий, удалённый, чужой.
func (c *conf) foreignIDs(t *testing.T) map[string]string {
	gone := c.create(t, c.newBook(1), "")
	if err := c.api.Delete(context.Background(), gone.ID); err != nil {
		t.Fatalf("удаление: %v", err)
	}
	ids := map[string]string{"несуществующий": "Zz" + strings.Repeat("q", 24), "удалённый": gone.ID}
	if c.other == "" {
		return ids
	}
	o, err := albums.New(c.base, c.other, albums.Options{})
	if err != nil {
		t.Fatalf("OtherToken: %v", err)
	}
	cr, err := o.Create(context.Background(), c.request(c.newBook(7), ""))
	if err != nil {
		t.Fatalf("альбом второго владельца: %v", err)
	}
	t.Cleanup(func() { _ = o.Delete(context.Background(), cr.ID) })
	ids["чужой"] = cr.ID
	list, err := c.api.List(context.Background())
	if err != nil {
		t.Errorf("список: %v", err)
	}
	for _, a := range list {
		if a.ID == cr.ID {
			t.Error("чужой альбом виден в списке")
		}
	}
	return ids
}

// input: негодные запросы — 400, слишком большие — 413; чужие поля манифеста отбрасываются.
func (c *conf) input(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC()
	mut := func(f func(m *albums.Manifest)) any {
		m := c.newBook(5).m
		f(&m)
		return albums.CreateRequest{Manifest: m, ExpiresAt: future}
	}
	pw := func(p string) any {
		return albums.CreateRequest{Manifest: c.newBook(5).m, ExpiresAt: future, Password: p}
	}
	cases := map[string]any{
		"file с обходом":    mut(func(m *albums.Manifest) { m.Pages[0].File = "../../x" }),
		"file":              mut(func(m *albums.Manifest) { m.Pages[0].File = "001.jpg" }),
		"title bidi":        mut(func(m *albums.Manifest) { m.Title = "А\u202eБ" }),
		"title пуст":        mut(func(m *albums.Manifest) { m.Title = "   " }),
		"title длинный":     mut(func(m *albums.Manifest) { m.Title = strings.Repeat("я", albums.MaxTitle+1) }),
		"generator длинный": mut(func(m *albums.Manifest) { m.Generator = strings.Repeat("g", albums.MaxGenerator+1) }),
		"page_mm":           mut(func(m *albums.Manifest) { m.Book.PageMM.W = 1 }),
		"пропуск в n":       mut(func(m *albums.Manifest) { m.Pages[2].N = 9 }),
		"sha256 заглавный":  mut(func(m *albums.Manifest) { m.Pages[0].SHA256 = strings.ToUpper(m.Pages[0].SHA256) }),
		"пароль 3 знака":    pw("abc"),
		"пароль 65 знаков":  pw(strings.Repeat("a", 65)),
		"пароль с \\n":      pw("abc\ndef"),
		"срок в прошлом":    albums.CreateRequest{Manifest: c.newBook(5).m, ExpiresAt: time.Now().Add(-time.Hour).UTC()},
		"срок не указан":    map[string]any{"manifest": c.newBook(5).m},
		"манифест пуст":     map[string]any{"expires_at": future},
		"страниц нет":       mut(func(m *albums.Manifest) { m.Pages = nil }),
		"schema 2":          mut(func(m *albums.Manifest) { m.Schema = 2 }),
	}
	for name, body := range cases {
		r := c.createRaw(t, body) // 201 по ошибке не оставит альбом на тестовом сервисе
		if r.status != 400 || r.errCode() != albums.CodeBadManifest {
			t.Errorf("%s: %d %s, нужно 400 bad_manifest", name, r.status, r.errCode())
		}
	}
	for name, raw := range map[string]string{"не JSON": "{", "пустое тело": "", "массив": "[]"} {
		r := c.do(t, "POST", "/v1/albums", c.token, strings.NewReader(raw), map[string]string{"Content-Type": "application/json"})
		if r.status != 400 {
			t.Errorf("%s: %d", name, r.status)
		}
	}
	big := `{"manifest":{"title":"` + strings.Repeat("a", 2<<20) + `"}}`
	if r := c.do(t, "POST", "/v1/albums", c.token, strings.NewReader(big), map[string]string{"Content-Type": "application/json"}); r.status != 413 {
		t.Errorf("тело 2 МБ: %d, нужно 413", r.status)
	}
	for name, body := range map[string]any{"PATCH пароль 3 знака": albums.Patch{Password: ptr("abc")}, "PATCH срок в прошлом": albums.Patch{ExpiresAt: ptr(time.Now().Add(-time.Hour))}} {
		cr := c.create(t, c.newBook(6), "")
		if r := c.json(t, "PATCH", "/v1/albums/"+cr.ID, c.token, body); r.status != 400 {
			t.Errorf("%s: %d", name, r.status)
		}
	}
}

func ptr[T any](v T) *T { return &v }
