package albumstest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// pages: PUT страницы — хэш, сигнатура, тип, номер, предел размера; повтор безопасен.
func (c *conf) pages(t *testing.T) {
	bk := c.newBook(10)
	cr := c.create(t, bk, "")
	if want := []int{1, 3, 4}; !equalInts(cr.Missing, want) {
		t.Fatalf("missing при создании: %v, нужно %v", cr.Missing, want)
	}
	put := func(n string, typ string, data []byte) *resp {
		return c.do(t, "PUT", "/v1/albums/"+cr.ID+"/pages/"+n, c.token, bytes.NewReader(data), map[string]string{"Content-Type": typ})
	}
	check := func(what string, r *resp, status int, code string) {
		t.Helper()
		if r.status != status || (code != "" && r.errCode() != code) {
			t.Errorf("%s: %d %s, нужно %d %s", what, r.status, r.errCode(), status, code)
		}
	}
	// Байты, не совпадающие с манифестом ни по сигнатуре, ни по хэшу: что сервис проверит первым, контракт
	// не говорит — годятся и 400 bad_page, и 409 hash_mismatch.
	either := func(what string, r *resp) {
		t.Helper()
		ok := (r.status == 400 && r.errCode() == albums.CodeBadPage) || (r.status == 409 && r.errCode() == albums.CodeHashMismatch)
		if !ok {
			t.Errorf("%s: %d %s, нужно 400 bad_page или 409 hash_mismatch", what, r.status, r.errCode())
		}
	}
	check("хэш не тот", put("1", albums.TypeJPEG, c.sample(99)), 409, albums.CodeHashMismatch)
	either("не JPEG по сигнатуре", put("1", albums.TypeJPEG, []byte("это не картинка, а текст")))
	either("PNG под видом JPEG", put("1", albums.TypeJPEG, []byte("\x89PNG\r\n\x1a\n0000")))
	either("пустое тело", put("1", albums.TypeJPEG, nil))
	check("тип не как в манифесте", put("1", albums.TypeWebP, bk.imgs[1]), 400, albums.CodeBadPage)
	check("тип text/html", put("1", "text/html", bk.imgs[1]), 400, albums.CodeBadPage)
	check("белая страница", put("2", albums.TypeJPEG, bk.imgs[1]), 400, albums.CodeBadPage)
	for _, n := range []string{"0", "5", "01", "abc", "-1", "1e0", "99999999999"} {
		check("номер "+n, put(n, albums.TypeJPEG, bk.imgs[1]), 400, albums.CodeBadPage)
	}
	if lim := c.lim.PageBytes; lim <= 64<<20 {
		big := make([]byte, lim+1)
		copy(big, []byte{0xFF, 0xD8, 0xFF})
		check("больше page_bytes", put("1", albums.TypeJPEG, big), 413, albums.CodeTooLarge)
	} else {
		t.Logf("page_bytes = %d: проверка 413 пропущена (слишком большое тело)", lim)
	}
	check("верная страница", put("1", albums.TypeJPEG, bk.imgs[1]), 204, "")
	check("повтор безопасен", put("1", albums.TypeJPEG, bk.imgs[1]), 204, "")
	// после неверных попыток страница 3 всё ещё нужна
	up, err := c.api.Update(context.Background(), cr.ID, &bk.m)
	if err != nil || !equalInts(up.Missing, []int{3, 4}) {
		t.Errorf("missing после одной страницы: %v, %v", up, err)
	}
}

func equalInts(a, b []int) bool { return slices.Equal(a, b) }

// get — страница клиента; cookie "" — без входа.
func (c *conf) get(t *testing.T, path, cookie string) *resp {
	hdr := map[string]string{}
	if cookie != "" {
		hdr["Cookie"] = cookie
	}
	return c.do(t, "GET", path, "", nil, hdr)
}

// session — итог попытки входа: значение для заголовка Cookie ("" — входа нет), статус, выданные cookie.
type session struct {
	cookie string
	status int
	set    []*http.Cookie
}

// login — вход к альбому с паролем: POST /a/<id>/login, форма с полем password.
func (c *conf) login(t *testing.T, id, pw string) session {
	form := url.Values{"password": {pw}}.Encode()
	req, err := http.NewRequestWithContext(context.Background(), "POST", c.base+"/a/"+id+"/login", strings.NewReader(form))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("вход: %v", netCause(err)) // без адреса: в нём id
	}
	_ = res.Body.Close()
	s := session{status: res.StatusCode, set: res.Cookies()}
	var parts []string
	for _, ck := range s.set {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	s.cookie = strings.Join(parts, "; ")
	return s
}

// denyLogin — вход с негодным паролем. Контракт не диктует вид отказа (401, 403 или страница формы с
// ошибкой и 200); суть безопасности — выданная cookie (или её отсутствие) не открывает ни одного адреса
// /a/<id>/…: страницу альбома, manifest.json и файл страницы (file известен после верного входа; "" — пропуск).
func (c *conf) denyLogin(t *testing.T, what, id, pw, file string) {
	t.Helper()
	c.denyCookie(t, what+": после попытки входа", id, c.login(t, id, pw).cookie, file)
}

// denyCookie — cookie ("" — без неё) не открывает ни страницу альбома, ни manifest.json, ни файл
// страницы (file == "" — файл пропущен): на каждом адресе нужно 401/403.
func (c *conf) denyCookie(t *testing.T, what, id, cookie, file string) {
	t.Helper()
	paths := []string{"/", "/manifest.json"}
	if file != "" {
		paths = append(paths, "/"+file)
	}
	for _, p := range paths {
		if r := c.get(t, "/a/"+id+p, cookie); !denied(r.status) {
			t.Errorf("%s: %s отвечает %d, нужно 401/403", what, p, r.status)
		}
	}
}

// tamper — та же cookie с изменённым последним знаком значения: подпись не сходится.
func tamper(cookie string) string {
	if cookie == "" {
		return "sa_session=0.deadbeef"
	}
	last := cookie[len(cookie)-1]
	repl := byte('0')
	if last == '0' {
		repl = '1'
	}
	return cookie[:len(cookie)-1] + string(repl)
}

// cookieProblems: на https cookie входа — Secure и HttpOnly; на http (стенд на loopback) флаги не требуются.
// Значение cookie в замечания не попадает.
func cookieProblems(set []*http.Cookie, https bool) []string {
	var out []string
	if !https {
		return nil
	}
	for _, ck := range set {
		if !ck.Secure || !ck.HttpOnly {
			out = append(out, fmt.Sprintf("cookie %q входа: Secure=%v HttpOnly=%v, на https нужны оба", ck.Name, ck.Secure, ck.HttpOnly))
		}
	}
	return out
}

func denied(status int) bool { return status == 401 || status == 403 }

// hasKey — ключ key есть в разобранном JSON на любой глубине.
func hasKey(v any, key string) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == key || hasKey(e, key) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasKey(e, key) {
				return true
			}
		}
	}
	return false
}

// noSecret: в теле ответа API нет ни самих secrets, ни поля password (даже с другим значением).
func (c *conf) noSecret(t *testing.T, what string, body []byte, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if bytes.Contains(body, []byte(s)) {
			t.Errorf("пароль в ответе: %s", what)
		}
	}
	var g any
	if json.Unmarshal(body, &g) == nil && hasKey(g, "password") {
		t.Errorf("в ответе есть поле password: %s", what)
	}
}

// createRaw — POST /v1/albums сырым запросом: клиент albums отбросил бы лишние поля ответа, а набор
// должен видеть их все. Альбом, созданный по ошибке (2xx вместо отказа), удаляется в конце блока.
func (c *conf) createRaw(t *testing.T, body any) *resp {
	t.Helper()
	r := c.json(t, "POST", "/v1/albums", c.token, body)
	if r.status >= 200 && r.status < 300 {
		var cr albums.Created
		if json.Unmarshal(r.body, &cr) == nil && albums.CheckID(cr.ID) == nil {
			t.Cleanup(func() { _ = c.api.Delete(context.Background(), cr.ID) })
		}
	}
	return r
}

// patchRaw — PATCH сырым запросом: тело ответа без secrets и поля password, статус 200.
func (c *conf) patchRaw(t *testing.T, id string, p albums.Patch, secrets ...string) *albums.Album {
	t.Helper()
	r := c.json(t, "PATCH", "/v1/albums/"+id, c.token, p)
	c.noSecret(t, "PATCH", r.body, secrets...)
	var a albums.Album
	if r.status != 200 || json.Unmarshal(r.body, &a) != nil {
		t.Errorf("PATCH: %d %s", r.status, r.errCode())
		return nil
	}
	return &a
}

// publishFlow: publish требует всех страниц; клиент видит манифест и картинки с верными заголовками.
func (c *conf) publishFlow(t *testing.T) {
	bk := c.newBook(20)
	cr := c.create(t, bk, "")
	// неопубликованное не раскрывается: 404 на всех адресах просмотра
	for _, p := range []string{"/manifest.json", "/"} {
		if r := c.get(t, "/a/"+cr.ID+p, ""); r.status != 404 {
			t.Errorf("неопубликованный альбом %s: %d, нужно 404", p, r.status)
		}
	}
	// не все страницы — 409 pages_missing со списком
	c.upload(t, cr.ID, bk, []int{1})
	_, err := c.api.Publish(context.Background(), cr.ID)
	var ae *albums.APIError
	if !errors.As(err, &ae) || ae.Status != 409 || ae.Code != albums.CodePagesMissing || !equalInts(ae.Missing, []int{3, 4}) {
		t.Fatalf("publish без страниц: %v %+v", err, ae)
	}
	c.upload(t, cr.ID, bk, []int{3, 4})
	pub, err := c.api.Publish(context.Background(), cr.ID)
	if err != nil || pub.URL == "" || pub.ExpiresAt.IsZero() {
		t.Fatalf("publish: %+v %v", pub, err)
	}
	if _, err := c.api.Publish(context.Background(), cr.ID); err != nil {
		t.Errorf("повторный publish: %v", err)
	}
	c.checkViewer(t, cr.ID, "", bk)

	// неизвестные поля входа не доходят до manifest.json
	raw := map[string]any{"manifest": map[string]any{
		"schema": 1, "generator": "g", "title": "Т", "evil": "x", "book": map[string]any{"cover": false, "page_mm": albums.Size{W: 100, H: 100}, "evil2": 1},
		"pages": bk.m.Pages}, "expires_at": time.Now().Add(48 * time.Hour).UTC(), "evil3": "y"}
	r := c.createRaw(t, raw)
	var cr2 albums.Created
	if r.status != 201 || json.Unmarshal(r.body, &cr2) != nil {
		t.Fatalf("создание с чужими полями: %d %s", r.status, r.body)
	}
	c.upload(t, cr2.ID, bk, cr2.Missing)
	if _, err := c.api.Publish(context.Background(), cr2.ID); err != nil {
		t.Fatal(err)
	}
	m := c.get(t, "/a/"+cr2.ID+"/manifest.json", "")
	for _, evil := range []string{"evil", "evil2", "evil3"} {
		if m.status != 200 || bytes.Contains(m.body, []byte(evil)) {
			t.Errorf("манифест %d несёт чужое поле %s", m.status, evil)
		}
	}
}

// checkViewer: страница альбома, манифест без пароля и секретов, картинки с image/* и nosniff по именам
// из поля file; cookie — вход. Возвращает имя файла первой небелой страницы ("" — не нашлось).
func (c *conf) checkViewer(t *testing.T, id, cookie string, bk *book, secrets ...string) string {
	t.Helper()
	m := c.get(t, "/a/"+id+"/manifest.json", cookie)
	if m.status != 200 {
		t.Errorf("manifest.json: %d", m.status)
		return ""
	}
	var man albums.Manifest
	if err := json.Unmarshal(m.body, &man); err != nil || len(man.Pages) != len(bk.m.Pages) {
		t.Errorf("manifest.json не разбирается или страниц %d: %v", len(man.Pages), err)
		return ""
	}
	var generic map[string]any
	_ = json.Unmarshal(m.body, &generic)
	for _, key := range []string{"password", "expires_at", "token"} {
		if _, ok := generic[key]; ok {
			t.Errorf("в manifest.json есть %q", key)
		}
	}
	for _, s := range secrets {
		if bytes.Contains(m.body, []byte(s)) {
			t.Error("в manifest.json есть пароль")
		}
	}
	first := ""
	for _, p := range man.Pages {
		if p.Blank {
			if p.File != "" {
				t.Errorf("у белой страницы %d есть файл", p.N)
			}
			continue
		}
		if p.File == "" {
			t.Errorf("у страницы %d в манифесте нет file", p.N)
			continue
		}
		if first == "" {
			first = p.File
		}
		img := c.get(t, "/a/"+id+"/"+p.File, cookie)
		if img.status != 200 || !strings.HasPrefix(img.hdr.Get("Content-Type"), "image/") || img.hdr.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("страница %d: %d, Content-Type %q, nosniff %q", p.N, img.status, img.hdr.Get("Content-Type"), img.hdr.Get("X-Content-Type-Options"))
		}
		if !bytes.Equal(img.body, bk.imgs[p.N]) {
			t.Errorf("страница %d отдана не теми байтами", p.N)
		}
	}
	page := c.get(t, "/a/"+id+"/", cookie)
	if page.status != 200 {
		t.Errorf("страница альбома: %d", page.status)
	}
	// id в адресе — ключ доступа: со страницы он не должен уйти в Referer на другие сайты
	if got := page.hdr.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("страница альбома: Referrer-Policy %q, нужно no-referrer", got)
	}
	return first
}

// newBookWith — книга с теми же n, но другими картинками.
func newBookWith(imgs map[int][]byte) *book {
	bk := newBook(0)
	bk.imgs = imgs
	for i := range bk.m.Pages {
		p := &bk.m.Pages[i]
		if data, ok := imgs[p.N]; ok {
			*p = pageOf(p.N, data)
		}
	}
	return bk
}

func pageOf(n int, data []byte) albums.Page {
	sum := sha256.Sum256(data)
	return albums.Page{N: n, Role: albums.RolePage, Type: albums.TypeJPEG, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), W: 16, H: 16}
}
