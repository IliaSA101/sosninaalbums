// Package fetch — единственный сетевой клиент программы (спека 067 Ф3, design §5): каталог
// наборов библиотеки с подписью ed25519 и скачивание архивов. Только stdlib (depguard);
// http.Client вне этого пакета запрещён (forbidigo).
//
// Граница безопасности: только https и хосты белого списка на каждом переходе, без cookie,
// Referer и параметров запроса (кроме перехода на CDN ассетов), User-Agent без версии (решение
// 13). Каталог — конверт с подписью, она проверяется до разбора (0.17 решение №2), архив —
// размером и sha256 из подписанного каталога. В ошибках — ни адресов, ни путей: текст уходит в
// тост и log.txt.
package fetch

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Где лежит библиотека (решение 11): каталог в main репозитория, архивы — ассеты релизов.
const (
	CatalogURL  = "https://raw.githubusercontent.com/IliaSA101/sosninaalbums-library/main/catalog.json"
	AssetPrefix = "https://github.com/IliaSA101/sosninaalbums-library/releases/download/"
	// UserAgent — без версии: и она сведения о пользователе (решение 13).
	UserAgent = "SosninaAlbums"
)

// Keys — открытые ключи подписи каталога, сырые 32 байта ed25519 в base64: текущий и запасной
// (решение 12). Закрытые — у владельца вне git; ротация — docs/architecture/library.md.
var Keys = [...]string{
	"cDIG+sgBjRR6LuNOkxp9idUr7W6kJD05RIjovXJQ/dQ=",
	"nU3/2tnXS0USTqQlRxanz+IkHmfKudkFrCxP1PMky+M=",
}

// GitHubHosts — белый список: raw (каталог), github.com (адрес ассета) и CDN ассетов после 302.
var GitHubHosts = []string{"raw.githubusercontent.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"}

// CDNHosts — CDN ассетов: GitHub отвечает 302 сюда с подписью доступа в параметрах (?sp=…&sig=…),
// поэтому параметры разрешены только на переходе и только этим хостам (0.17 решение №2).
var CDNHosts = []string{"release-assets.githubusercontent.com", "objects.githubusercontent.com"}

// Тайм-ауты (design §5): соединение, TLS и заголовки ответа; «застой» тела; набор целиком.
const (
	dialTimeout   = 10 * time.Second
	tlsTimeout    = 10 * time.Second
	headerTimeout = 20 * time.Second
	stallTimeout  = 30 * time.Second
	packTimeout   = 15 * time.Minute // пол предела набора; больше — по размеру (PackLimit)
	minRate       = 64 << 10         // байт/с, ниже которых набор не успевает (≈ 0,5 Мбит/с; решение 0.29-137-6)
	smallTimeout  = time.Minute      // каталог и подпись — килобайты
	maxRedirects  = 5
)

// Ошибки для тоста: по-русски, без адресов и путей.
var (
	ErrOffline   = errors.New("нет связи с GitHub — проверьте интернет и повторите")
	ErrTimeout   = errors.New("GitHub не отвечает — повторите позже")
	ErrNotFound  = errors.New("библиотека на GitHub не найдена")
	ErrSignature = errors.New("список наборов не прошёл проверку — обновление отменено")
	ErrCatalog   = errors.New("список наборов не читается — обновление отменено")
	ErrNewer     = errors.New("список наборов новее программы — нужна новая версия Sosnina Albums")
	ErrAddress   = errors.New("адрес библиотеки не из разрешённых — обновление отменено")
	ErrTLS       = errors.New("соединение с GitHub не прошло проверку подлинности — обновление отменено")
	ErrCancelled = errors.New("обновление библиотеки отменено")
	ErrBroken    = errors.New("набор скачался с ошибкой — повторите")
	ErrDisk      = errors.New("набор не записывается на диск")
	// ErrSlow — набор не уложился в предел времени (PackLimit): связь медленная, и следующий набор
	// упрётся в него так же. В отличие от застоя (ErrTimeout) ход обрывается сразу.
	ErrSlow      = errors.New("связь слишком медленная — набор не скачался за отведённое время")
	errStall     = errors.New("застой скачивания")
	errPackLimit = errors.New("набор скачивается дольше предела")
)

// StatusError — ответ не 200; код — для log.txt.
type StatusError struct{ Code int }

func (e *StatusError) Error() string {
	if e.Code == http.StatusNotFound {
		return ErrNotFound.Error()
	}
	return fmt.Sprintf("GitHub ответил ошибкой (код %d) — повторите позже", e.Code)
}

// Is — 404 сравнивается с ErrNotFound.
func (e *StatusError) Is(target error) bool {
	return target == ErrNotFound && e.Code == http.StatusNotFound
}

// Source — откуда и чем проверять: каталог, префикс архивов, белый список хостов, ключи.
// Выпуск — GitHub(); стенд QA (тег dev) и тесты подставляют свой.
type Source struct {
	CatalogURL string
	Prefix     string // адрес архива в каталоге — только с этим префиксом
	Hosts      []string
	// QueryHosts — хосты, которым на переходе (не в первом запросе) можно параметры: CDN ассетов.
	QueryHosts []string
	Keys       []ed25519.PublicKey
	// LocalHTTP — http только к 127.0.0.1 (стенд QA с SA_LIBRARY_URL); в выпуске false.
	LocalHTTP bool
	// Roots — доверенные корни TLS; nil — системные (тесты — сертификат httptest).
	Roots *x509.CertPool
}

// GitHub — источник выпуска.
func GitHub() Source {
	return Source{CatalogURL: CatalogURL, Prefix: AssetPrefix, Hosts: GitHubHosts, QueryHosts: CDNHosts, Keys: PublicKeys(Keys[:])}
}

// PublicKeys — ключи из base64; не 32 байта — пропуск (сторож — TestKeysValid).
func PublicKeys(list []string) []ed25519.PublicKey {
	out := make([]ed25519.PublicKey, 0, len(list))
	for _, s := range list {
		if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == ed25519.PublicKeySize {
			out = append(out, ed25519.PublicKey(b))
		}
	}
	return out
}

// allowed — адрес можно запрашивать: https (или http к 127.0.0.1 у стенда), хост из списка,
// без имени пользователя, параметров и якоря.
func (s Source) allowed(u *url.URL) bool { return s.check(u, false) }

// hop — адрес перехода: как allowed, но https-адресу хоста из QueryHosts можно параметры.
func (s Source) hop(u *url.URL) bool { return s.check(u, true) }

func (s Source) check(u *url.URL, hop bool) bool {
	if u == nil || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if (u.RawQuery != "" || u.ForceQuery) && (!hop || u.Scheme != "https" || !slices.Contains(s.QueryHosts, host)) {
		return false
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !s.LocalHTTP || host != "127.0.0.1" {
			return false
		}
	default:
		return false
	}
	return slices.Contains(s.Hosts, host)
}

// Client — клиент библиотеки: свой транспорт, не DefaultClient (его настраивает прокси wails dev).
type Client struct {
	src   Source
	hc    *http.Client
	stall time.Duration
	limit time.Duration // пол предела набора; больше — по размеру при rate
	rate  int64         // байт/с для предела по размеру; тесты обнуляют, чтобы сузить предел
}

// PackLimit — сколько можно качать набор размера size: не меньше packTimeout, а большой — по
// скорости minRate (70 МиБ — 1120 с, MaxZip — 4800 с). Застой (stallTimeout) отдельно ловит
// оборванную связь. Размер вне 0…MaxZip не растит предел: Download такие отвергает до сети.
func PackLimit(size int64) time.Duration { return limitFor(packTimeout, minRate, size) }

// limitFor — floor или время на size байт при скорости rate (байт/с), что больше; rate ≤ 0 —
// только floor (тесты). Переполнения нет: size ≤ MaxZip, size·1e9 < 9,2·1e18.
func limitFor(floor time.Duration, rate, size int64) time.Duration {
	if rate <= 0 {
		return floor
	}
	size = min(max(size, 0), MaxZip)
	return max(floor, time.Duration(size)*time.Second/time.Duration(rate))
}

// New — клиент для src.
func New(src Source) *Client {
	d := &net.Dialer{Timeout: dialTimeout}
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           d.DialContext,
		TLSHandshakeTimeout:   tlsTimeout,
		ResponseHeaderTimeout: headerTimeout,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: src.Roots},
	}
	c := &Client{src: src, stall: stallTimeout, limit: packTimeout, rate: minRate}
	c.hc = &http.Client{Transport: tr, CheckRedirect: c.redirect}
	return c
}

// SetTimeouts — сузить застой и пол предела набора: для тестов, где настоящие 30 с и 15 мин
// не дождаться. Вызывать до первого запроса. Нулевое значение не меняет своё.
func (c *Client) SetTimeouts(stall, limit time.Duration) {
	if stall > 0 {
		c.stall = stall
	}
	if limit > 0 {
		c.limit = limit
	}
}

// redirect — каждый переход: не больше maxRedirects, адрес из белого списка (параметры — только
// CDN ассетов); Referer, который net/http ставит на переходе, снимается.
func (c *Client) redirect(req *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects || !c.src.hop(req.URL) {
		return ErrAddress
	}
	req.Header.Del("Referer")
	return nil
}

// get — GET адреса из белого списка; ответ не 200 — StatusError (тело закрыто).
func (c *Client) get(ctx context.Context, raw string) (*http.Response, error) {
	u, err := url.Parse(raw)
	if err != nil || !c.src.allowed(u) {
		return nil, ErrAddress
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrAddress
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, netErr(ctx, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &StatusError{Code: resp.StatusCode}
	}
	return resp, nil
}

// netErr — ошибка сети словами для тоста: отмена, застой и предел времени — по причине
// контекста; переход мимо белого списка, сертификат, тайм-аут; остальное — «нет связи».
func netErr(ctx context.Context, err error) error {
	switch cause := context.Cause(ctx); {
	case errors.Is(cause, errStall), errors.Is(cause, errPackLimit), errors.Is(cause, context.DeadlineExceeded):
		return ErrTimeout
	case errors.Is(cause, context.Canceled):
		return ErrCancelled
	}
	var cert *tls.CertificateVerificationError
	var ne net.Error
	switch {
	case errors.Is(err, ErrAddress):
		return ErrAddress
	case errors.As(err, &cert):
		return ErrTLS
	case errors.As(err, &ne) && ne.Timeout():
		return ErrTimeout
	}
	return ErrOffline
}
