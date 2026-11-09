// Package secret — токен сервиса листалок (146) в хранилище ключей ОС; запасной путь —
// память до выхода. Токен строго по origin (схема и хост): токен одного сервиса другому
// не отдаётся ни из ОС, ни из памяти (разбор С2). В файл, журнал и тексты ошибок токен не
// попадает никогда: ошибки этого пакета его не содержат.
package secret

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Place — где лежит токен: решает окно («Токен не сохранится — в системе нет хранилища»).
type Place string

const (
	InSystem  Place = "system"  // хранилище ключей ОС
	InSession Place = "session" // память: пропадёт при выходе
)

// Store — токены сервисов по origin. Реализации безопасны для параллельных вызовов.
type Store interface {
	// Get — токен origin; пусто — токена нет. Place — где он лежит.
	Get(origin string) (token string, place Place)
	// Set кладёт токен в ОС, а при её отказе или зависании — в память; Place говорит, куда
	// он лёг. Ошибка — только негодные origin или токен (хранилище не тронуто).
	Set(origin, token string) (Place, error)
	// Delete убирает токен из памяти и из ОС. Ошибка — токен из ОС убрать не удалось.
	Delete(origin string) error
}

// maxToken — предел токена: он идёт в заголовок Authorization, Windows держит секрет до 2560 байт.
const maxToken = 2048

var (
	// ErrEmpty — токена в вводе нет.
	ErrEmpty = errors.New("токен пуст")
	// ErrBadToken — токен с пробелом или управляющим знаком, не ASCII или слишком длинный.
	ErrBadToken = errors.New("токен содержит недопустимые знаки или слишком длинный")
	// ErrBadOrigin — адрес сервиса не годится в ключ хранилища.
	ErrBadOrigin = errors.New("адрес сервиса не годится")
)

// Origin приводит адрес сервиса к ключу хранилища; канон тот же, что у share.ServiceURL (тест
// связи — в internal/app): «схема://хост[:порт]», нижний регистр, IP в краткой записи (netip,
// IPv4 в IPv6 — как IPv4), без порта по умолчанию (:443 у https, :80 у http), порт 1…65535 без
// ведущего нуля. Логин, путь, запрос, якорь и «числовой» хост (2130706433, 127.1) — ошибка: ключ
// не должен зависеть от того, как записан адрес.
func Origin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 {
		return "", ErrBadOrigin
	}
	for i := 0; i < len(raw); i++ {
		if c := raw[i]; c <= ' ' || c >= 0x7f || c == '\\' || c == '%' {
			return "", ErrBadOrigin
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Opaque != "" || strings.ContainsAny(raw, "?#") ||
		u.Path != "" && u.Path != "/" {
		return "", ErrBadOrigin
	}
	scheme, host := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname())
	if scheme != "https" && scheme != "http" || !goodHost(host) {
		return "", ErrBadOrigin
	}
	if a, err := netip.ParseAddr(host); err == nil {
		a = a.Unmap()
		host = a.String()
		if a.Is6() { // IPv6 — в скобках, как в адресе
			host = "[" + host + "]"
		}
	}
	port := u.Port()
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 || port[0] == '0' {
			return "", ErrBadOrigin
		}
	}
	if port == "" || port == "443" && scheme == "https" || port == "80" && scheme == "http" {
		return scheme + "://" + host, nil
	}
	return scheme + "://" + host + ":" + port, nil
}

// goodHost — IP-адрес (без зоны) или DNS-имя из a-z, 0-9, «-» с непустыми метками, последняя
// метка не число: «127.1» и «2130706433» браузер и система читают как IP, а не как имя.
func goodHost(host string) bool {
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Zone() == ""
	}
	if host == "" || len(host) > 253 {
		return false
	}
	labels := strings.Split(host, ".")
	if _, err := strconv.Atoi(labels[len(labels)-1]); err == nil {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			switch c := label[i]; {
			case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			default:
				return false
			}
		}
	}
	return true
}

// checkToken — токен годен для заголовка и хранилища.
func checkToken(token string) error {
	if token == "" {
		return ErrEmpty
	}
	if len(token) > maxToken {
		return ErrBadToken
	}
	for i := 0; i < len(token); i++ {
		if c := token[i]; c <= ' ' || c >= 0x7f {
			return ErrBadToken
		}
	}
	return nil
}

// check — ключ и токен для записи: оба годны, иначе хранилище не трогаем.
func check(origin, token string) (string, error) {
	o, err := Origin(origin)
	if err != nil {
		return "", err
	}
	return o, checkToken(token)
}

// Memory — токены в памяти: запасной путь системного хранилища, а также режим стенда и
// тестов. Ключ — origin, поэтому токен одного адреса другому не отдаётся.
type Memory struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemory — пустое хранилище в памяти.
func NewMemory() *Memory { return &Memory{m: map[string]string{}} }

// Get — токен origin; негодный origin — как отсутствующий.
func (s *Memory) Get(origin string) (string, Place) {
	o, err := Origin(origin)
	if err != nil {
		return "", InSession
	}
	return s.get(o), InSession
}

// Set кладёт токен origin в память.
func (s *Memory) Set(origin, token string) (Place, error) {
	o, err := check(origin, token)
	if err != nil {
		return InSession, err
	}
	s.put(o, token)
	return InSession, nil
}

// Delete убирает токен origin; негодный origin — ошибка.
func (s *Memory) Delete(origin string) error {
	o, err := Origin(origin)
	if err != nil {
		return err
	}
	s.put(o, "")
	return nil
}

// get, put — по уже приведённому origin. Пустой токен в put — удаление.
func (s *Memory) get(o string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[o]
}

func (s *Memory) put(o, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if token == "" {
		delete(s.m, o)
		return
	}
	s.m[o] = token
}
