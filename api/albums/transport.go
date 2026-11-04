package albums

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"
)

// timeouts — тайм-ауты клиента (design §2.4). Поле defaultTimeouts меняют только тесты.
type timeouts struct {
	dial    time.Duration // соединение
	tls     time.Duration // рукопожатие TLS
	header  time.Duration // заголовки ответа после отправки запроса
	idle    time.Duration // застой тела ответа
	putBase time.Duration // страница: постоянная часть срока PUT
	putRate int64         // и скорость, байт/с, на которую рассчитан остаток
	total   time.Duration // общий срок вызова без своего срока (JSON-запросы): медленный ответ по байту не висит вечно
}

var defaultTimeouts = timeouts{
	dial:    10 * time.Second,
	tls:     10 * time.Second,
	header:  30 * time.Second,
	idle:    30 * time.Second,
	putBase: 60 * time.Second,
	putRate: 32 << 10,
	total:   2 * time.Minute,
}

// testRootCAs — корни CA для httptest-сервера с TLS. Задаётся только из _test.go: через Options
// доверие к чужому сертификату не включить (design §2.4).
var testRootCAs *x509.CertPool

// putTimeout — срок на PUT страницы: 60 с и размер на 32 КБ/с. Застой отправки не висит без предела.
func (t timeouts) putTimeout(size int64) time.Duration {
	if size < 0 {
		size = 0
	}
	return t.putBase + time.Duration(size/t.putRate)*time.Second
}

// newTransport — свой транспорт, не DefaultTransport: TLS не ниже 1.2, свои тайм-ауты.
// Переходов (3xx) запрещает Client.CheckRedirect.
func newTransport(t timeouts) *http.Transport {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: testRootCAs}
	d := &net.Dialer{Timeout: t.dial, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           d.DialContext,
		TLSClientConfig:       cfg,
		TLSHandshakeTimeout:   t.tls,
		ResponseHeaderTimeout: t.header,
		ExpectContinueTimeout: time.Second,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   4,
		DisableCompression:    true, // ответ читается как есть и под предел
		ForceAttemptHTTP2:     true,
	}
}

// newHTTPClient — клиент без переходов: Authorization не уйдёт на другой хост.
func newHTTPClient(t timeouts) *http.Client {
	return &http.Client{
		Transport:     newTransport(t),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
