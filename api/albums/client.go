package albums

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Options — настройки клиента. Подменить транспорт или HTTP-клиент нельзя: иначе обходится запрет
// переходов и проверки TLS (design §2.4).
type Options struct {
	Version string // версия программы для User-Agent: SosninaAlbums/<версия>; пусто — dev
}

// Client — клиент /v1: проверки адреса и токена до сети, предел ответа, без переходов, ошибки без
// секретов. Безопасен для одновременных вызовов (загрузка страниц в 3 потока). Повторы — снаружи.
type Client struct {
	base  *url.URL
	token string
	ua    string
	hc    *http.Client
	tm    timeouts
	now   func() time.Time

	mu  sync.Mutex
	lim Limits // последние пределы из Health; до него — DefaultLimits
}

// New — клиент сервиса по адресу base (ParseBase) и токену (CheckToken). Сети не касается.
func New(base, token string, opt Options) (*Client, error) {
	u, err := ParseBase(base)
	if err != nil {
		return nil, err
	}
	if err := CheckToken(token); err != nil {
		return nil, err
	}
	return &Client{
		base:  u,
		token: token,
		ua:    "SosninaAlbums/" + cleanVersion(opt.Version),
		hc:    newHTTPClient(defaultTimeouts),
		tm:    defaultTimeouts,
		now:   time.Now,
		lim:   DefaultLimits(),
	}, nil
}

// cleanVersion — версия для заголовка: печатный ASCII без пробелов, до 32 знаков.
func cleanVersion(v string) string {
	if len(v) > 32 {
		v = v[:32]
	}
	for i := 0; i < len(v); i++ {
		if v[i] <= ' ' || v[i] >= 0x7f {
			return "dev"
		}
	}
	if v == "" {
		return "dev"
	}
	return v
}

// Base — адрес сервиса в каноническом виде (без хвостового «/»).
func (c *Client) Base() string { return c.base.String() }

// Limits — пределы сервиса: из последнего Health, до него DefaultLimits.
func (c *Client) Limits() Limits {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lim
}

// Health — GET /v1/health (без токена): версия API и пределы. Пределы запоминаются для проверок.
func (c *Client) Health(ctx context.Context) (*Health, error) {
	var h Health
	if err := c.call(ctx, rq{method: http.MethodGet, path: "/v1/health", noAuth: true}, &h); err != nil {
		return nil, err
	}
	if h.API != "v1" {
		return nil, ErrResponse
	}
	c.mu.Lock()
	c.lim = h.Limits.OrDefault()
	h.Limits = c.lim
	c.mu.Unlock()
	return &h, nil
}

// List — GET /v1/albums: свои альбомы.
func (c *Client) List(ctx context.Context) ([]Album, error) {
	var l AlbumList
	if err := c.call(ctx, rq{method: http.MethodGet, path: "/v1/albums"}, &l); err != nil {
		return nil, err
	}
	for i := range l.Albums {
		if !goodAlbum(&l.Albums[i]) {
			return nil, ErrResponse
		}
	}
	return l.Albums, nil
}

// Create — POST /v1/albums: запрос проверяется ValidateCreate до сети; ссылка не работает до Publish.
func (c *Client) Create(ctx context.Context, req *CreateRequest) (*Created, error) {
	if err := ValidateCreate(req, c.Limits(), c.now()); err != nil {
		return nil, err
	}
	var out Created
	if err := c.call(ctx, rq{method: http.MethodPost, path: "/v1/albums", in: req}, &out); err != nil {
		return nil, err
	}
	if CheckID(out.ID) != nil || !goodURL(out.URL) || out.ExpiresAt.IsZero() || ValidateMissing(&req.Manifest, out.Missing) != nil {
		return nil, ErrResponse
	}
	return &out, nil
}

// Update — PUT /v1/albums/{id}: новая редакция манифеста; ссылка та же, прежняя редакция видна до Publish.
func (c *Client) Update(ctx context.Context, id string, m *Manifest) (*Updated, error) {
	if err := CheckID(id); err != nil {
		return nil, err
	}
	if err := Validate(m, c.Limits()); err != nil {
		return nil, err
	}
	var out Updated
	if err := c.call(ctx, rq{method: http.MethodPut, path: albumPath(id), id: id, in: UpdateRequest{Manifest: *m}}, &out); err != nil {
		return nil, err
	}
	if ValidateMissing(m, out.Missing) != nil {
		return nil, ErrResponse
	}
	return &out, nil
}

// PutPage — PUT /v1/albums/{id}/pages/{n}: size байт из r, Content-Type = typ. Повтор безопасен.
// Срок запроса — 60 с и size на 32 КБ/с; контекст ctx прерывает раньше.
func (c *Client) PutPage(ctx context.Context, id string, n int, typ string, r io.Reader, size int64) error {
	if err := CheckID(id); err != nil {
		return err
	}
	if n < 1 || n > c.Limits().Pages {
		return bad("n", "номер вне книги")
	}
	if typ != TypeJPEG && typ != TypeWebP {
		return bad("type", "image/jpeg или image/webp")
	}
	if size < 1 || size > c.Limits().PageBytes {
		return bad("size", "вне 1…page_bytes")
	}
	return c.call(ctx, rq{
		method: http.MethodPut, path: albumPath(id) + "/pages/" + strconv.Itoa(n), id: id,
		raw: io.LimitReader(r, size), size: size, ctype: typ, timeout: c.tm.putTimeout(size),
	}, nil)
}

// Publish — POST /v1/albums/{id}/publish. Не все страницы — APIError 409 pages_missing с Missing.
func (c *Client) Publish(ctx context.Context, id string) (*Published, error) {
	if err := CheckID(id); err != nil {
		return nil, err
	}
	var out Published
	if err := c.call(ctx, rq{method: http.MethodPost, path: albumPath(id) + "/publish", id: id}, &out); err != nil {
		return nil, err
	}
	if !goodURL(out.URL) || out.ExpiresAt.IsZero() {
		return nil, ErrResponse
	}
	return &out, nil
}

// Patch — PATCH /v1/albums/{id}: срок, пароль, сброс входов. Проверка ValidatePatch — до сети.
func (c *Client) Patch(ctx context.Context, id string, p *Patch) (*Album, error) {
	if err := CheckID(id); err != nil {
		return nil, err
	}
	if err := ValidatePatch(p, c.now()); err != nil {
		return nil, err
	}
	var out Album
	if err := c.call(ctx, rq{method: http.MethodPatch, path: albumPath(id), id: id, in: p}, &out); err != nil {
		return nil, err
	}
	if !goodAlbum(&out) {
		return nil, ErrResponse
	}
	return &out, nil
}

// Delete — DELETE /v1/albums/{id}: альбом удаляется сразу.
func (c *Client) Delete(ctx context.Context, id string) error {
	if err := CheckID(id); err != nil {
		return err
	}
	return c.call(ctx, rq{method: http.MethodDelete, path: albumPath(id), id: id}, nil)
}

func albumPath(id string) string { return "/v1/albums/" + url.PathEscape(id) }

func goodURL(s string) bool { return s != "" && len(s) <= MaxURL }

func goodAlbum(a *Album) bool {
	return CheckID(a.ID) == nil && len(a.URL) <= MaxURL && a.Pages >= 0 && !a.ExpiresAt.IsZero()
}

// rq — один запрос: JSON-тело in либо поток raw известной длины size.
type rq struct {
	method, path string
	id           string // для очистки текстов ошибок
	in           any
	raw          io.Reader
	size         int64
	ctype        string
	noAuth       bool
	timeout      time.Duration
}

// call выполняет запрос и разбирает ответ в out (nil — тела нет). Ошибка: *APIError, *NetError,
// ErrRedirect, ErrTooBig, ErrResponse или ошибка проверки до сети.
func (c *Client) call(ctx context.Context, q rq, out any) error {
	u := *c.base
	u.Path = c.base.Path + q.path
	var body io.Reader
	ctype := q.ctype
	switch {
	case q.in != nil:
		b, err := json.Marshal(q.in)
		if err != nil || len(b) > MaxBody {
			return bad("request", "запрос больше 1 МБ или не кодируется")
		}
		body, ctype = bytes.NewReader(b), "application/json"
	case q.raw != nil:
		body = q.raw
	}
	limit := q.timeout
	if limit <= 0 {
		limit = c.tm.total
	}
	rctx, cancel := context.WithTimeout(ctx, limit)
	rctx, cancelIdle := context.WithCancel(rctx)
	defer cancel()
	defer cancelIdle()

	req, err := http.NewRequestWithContext(rctx, q.method, u.String(), body)
	if err != nil {
		return bad("request", "не удалось собрать запрос")
	}
	if q.raw != nil {
		req.ContentLength = q.size
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if !q.noAuth {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return c.netError(q, u.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return ErrRedirect
	}
	data, err := c.readBody(resp.Body, cancelIdle)
	if err != nil {
		if errors.Is(err, ErrTooBig) {
			return &tooBigError{status: resp.StatusCode}
		}
		return c.netError(q, u.Host, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(q, resp, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return ErrResponse
	}
	return nil
}

// tooBigError — ответ больше MaxBody; статус в тексте, но 401/404 из него не выводятся (прокси
// с огромной страницей не должен стереть токен у вызывающего).
type tooBigError struct{ status int }

func (e *tooBigError) Error() string {
	return "albums: ответ больше 1 МБ (HTTP " + strconv.Itoa(e.status) + ")"
}
func (e *tooBigError) Is(t error) bool { return t == ErrTooBig }

// readBody читает тело не больше MaxBody+1 байт; застой дольше idle обрывает запрос (cancel).
func (c *Client) readBody(r io.Reader, cancel context.CancelFunc) ([]byte, error) {
	var stalled atomic.Bool
	t := time.AfterFunc(c.tm.idle, func() { stalled.Store(true); cancel() })
	defer t.Stop()
	var buf bytes.Buffer
	chunk := make([]byte, 32<<10)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			t.Reset(c.tm.idle)
			if buf.Len()+n > MaxBody {
				return nil, ErrTooBig
			}
			buf.Write(chunk[:n])
		}
		if errors.Is(err, io.EOF) {
			return buf.Bytes(), nil
		}
		if err != nil {
			if stalled.Load() {
				return nil, context.DeadlineExceeded
			}
			return nil, err
		}
	}
}

// netError сводит сбой связи к «<метод> <хост>: <причина>»: путь запроса (в нём id) и токен в текст
// не попадают (разбор С4).
func (c *Client) netError(q rq, host string, err error) error {
	cause := err
	var ue *url.Error
	if errors.As(err, &ue) {
		cause = ue.Err
	}
	return &NetError{Method: q.method, Host: host, msg: redact(cause.Error(), c.token, q.id), err: cause}
}

// apiError разбирает тело ошибки; чужой формат (HTML прокси) даёт ошибку только со статусом.
func (c *Client) apiError(q rq, resp *http.Response, data []byte) error {
	e := &APIError{Status: resp.StatusCode}
	var eb ErrorBody
	if json.Unmarshal(data, &eb) == nil {
		e.Code = cleanCode(eb.Error.Code)
		e.Message = cleanMessage(eb.Error.Message, c.token, q.id)
		e.Missing = cleanMissing(eb.Error.Missing, c.Limits().Pages)
	}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			e.RetryAfter = time.Duration(min(n, 3600)) * time.Second
		}
	}
	return e
}
