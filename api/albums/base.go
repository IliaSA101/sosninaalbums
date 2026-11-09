package albums

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// ErrBaseNonASCII — хост (или любая часть адреса) не ASCII: модуль без x/net/idna, домен вводят
// в виде xn-- (разбор №6, 0.32-146-26).
var ErrBaseNonASCII = errors.New("albums: введите адрес в виде xn--… (латиница)")

// ParseBase проверяет адрес сервиса и приводит его к виду без хвостового «/»: https://хост[:порт][/путь];
// http — только к 127.0.0.1, [::1] и localhost. Без имени пользователя, запроса и якоря, не длиннее
// MaxBase знаков, только печатный ASCII. Ошибка не содержит самого адреса.
func ParseBase(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > MaxBase {
		return nil, bad("base", "пустой или длиннее "+strconv.Itoa(MaxBase)+" знаков")
	}
	for i := 0; i < len(raw); i++ {
		switch c := raw[i]; {
		case c >= 0x80:
			return nil, ErrBaseNonASCII
		case c <= ' ' || c == 0x7f || c == '%' || c == '?' || c == '#' || c == '\\':
			return nil, bad("base", "недопустимые знаки: пробел, управляющие, % ? # \\")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, bad("base", "не адрес")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, bad("base", "логин, запрос и якорь не допускаются")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, bad("base", "нет хоста")
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
			return nil, bad("base", "порт вне 1…65535")
		}
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !IsLoopbackHost(host) {
			return nil, bad("base", "http — только к 127.0.0.1, [::1] и localhost")
		}
	default:
		return nil, bad("base", "нужна схема https")
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == ".." || seg == "." {
			return nil, bad("base", "в пути нет . и ..")
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u, nil
}

// IsLoopbackHost — этот компьютер: ровно «localhost» (без поддомена: его адрес зависит от системы
// имён и мог бы уйти в сеть) или IP из 127.0.0.0/8, ::1. Единое правило для http в клиенте и в программе.
func IsLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	a, err := netip.ParseAddr(host)
	return err == nil && a.Unmap().IsLoopback()
}

// MayBeLoopbackHost — этот компьютер может оказаться за именем: IsLoopbackHost, любой «*.localhost»
// (RFC 6761: systemd-resolved и Chromium сводят его к 127.0.0.1) и неопределённый адрес 0.0.0.0, «::»
// (на Linux и macOS соединение с ним уходит на этот компьютер). Регистр не важен, завершающая точка
// снимается. Для запрета «этот компьютер» в выпуске; http разрешает только IsLoopbackHost.
func MayBeLoopbackHost(host string) bool {
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if IsLoopbackHost(h) || strings.HasSuffix(h, ".localhost") {
		return true
	}
	a, err := netip.ParseAddr(h)
	return err == nil && a.Unmap().IsUnspecified()
}
