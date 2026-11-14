// Package share — чистая логика «Отправить по ссылке» (146): манифест из страниц книги, хэш сцены,
// состояние ссылки, пароль «Придумать», текст для клиента, проверка адреса сервиса. Сети и диска
// настроек здесь нет: клиент сервиса, shares.json и токен — в sharer, prefs и secret.
package share

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// Пределы адресов (design §6 п. 2): адрес сервиса и ссылка из ответа сервера.
const (
	MaxServiceURL = 200
	MaxLinkURL    = 500
)

// ServiceURL — адрес сервиса, как его ввёл фотограф, приведённый к каноническому origin
// «схема://хост[:порт]»: схема и имя хоста в нижнем регистре, IP в краткой записи (netip), пустой
// порт и порт по умолчанию (443 у https, 80 у http) сняты. Токен хранится и отправляется строго
// по этому origin (0.32-146-30), поэтому два написания одного адреса дают одну строку.
//
// https; http — только loopback и только при allowLoopback (сборка стенда и тесты, 0.32-146-25:
// флаг приходит из Host). Логина, пути, запроса и якоря быть не может, хост — только ASCII
// (0.32-146-26). Негодный адрес — ошибка до сети.
func ServiceURL(raw string, allowLoopback bool) (string, error) {
	u, err := parseWeb(strings.TrimSpace(raw), MaxServiceURL)
	if err != nil {
		return "", err
	}
	if u.Path != "" && u.Path != "/" {
		return "", errors.New("адрес сервиса без пути: только сайт, например https://albums.example.org")
	}
	host := u.Hostname()
	if a, err := netip.ParseAddr(host); err == nil {
		a = a.Unmap()
		host = a.String()
		if a.Is6() {
			host = "[" + host + "]"
		}
	}
	if !allowLoopback && (u.Scheme == "http" || mayBeLoopback(u.Hostname())) {
		return "", errors.New("адрес сервиса должен быть https://-адресом сайта, не этого компьютера")
	}
	port := u.Port()
	if port == "" || port == "443" && u.Scheme == "https" || port == "80" && u.Scheme == "http" {
		return u.Scheme + "://" + host, nil
	}
	return u.Scheme + "://" + host + ":" + port, nil
}

// CheckLoopbackLink — ссылка из ответа сервиса ведёт на этот компьютер (http или https, не длиннее
// MaxLinkURL, без логина, запроса и якоря). Для ссылок поддельного сервера на стенде:
// printprof.CheckURL loopback отвергает, а служба после него (design §3.3 п. 7) пускает
// такую ссылку отдельно и только при allowLoopback. Без allowLoopback — всегда отказ. Обычные ссылки проверяет CheckURL.
func CheckLoopbackLink(raw string, allowLoopback bool) error {
	if !allowLoopback {
		return errors.New("ссылка на этот компьютер допустима только на стенде")
	}
	u, err := parseWeb(raw, MaxLinkURL)
	if err != nil {
		return err
	}
	if !loopback(u.Hostname()) {
		return errors.New("ссылка ведёт не на этот компьютер")
	}
	return nil
}

// parseWeb разбирает адрес и отбрасывает всё, чем его можно подменить: управляющие и не-ASCII
// знаки, пробелы, \, логин, запрос и якорь. Схема http пропускается — решает вызывающий.
func parseWeb(raw string, limit int) (*url.URL, error) {
	switch {
	case raw == "":
		return nil, errors.New("адрес пуст")
	case len(raw) > limit:
		return nil, fmt.Errorf("адрес длиннее %d знаков", limit)
	}
	for i := 0; i < len(raw); i++ {
		if c := raw[i]; c <= ' ' || c >= 0x7f || c == '\\' {
			return nil, errors.New("в адресе недопустимые знаки")
		}
	}
	if strings.ContainsRune(raw, '#') || strings.ContainsRune(raw, '?') {
		return nil, errors.New("в адресе не бывает запроса и якоря")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("адрес не разобрать")
	}
	if u.User != nil || strings.ContainsRune(u.Host, '@') {
		return nil, errors.New("в адресе не бывает имени и пароля")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	host := u.Hostname()
	switch {
	case u.Scheme != "https" && u.Scheme != "http":
		return nil, errors.New("адрес должен начинаться с https://")
	case u.Opaque != "" || host == "":
		return nil, errors.New("в адресе нет сайта")
	case !goodHost(host):
		return nil, errors.New("имя сайта в адресе не годится")
	case u.Scheme == "http" && !loopback(host):
		return nil, errors.New("http допустим только для этого компьютера, для сайта нужен https://")
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 || p[0] == '0' {
			return nil, errors.New("порт в адресе не годится")
		}
	}
	return u, nil
}

// loopback — точно этот компьютер (разрешение http и ссылки стенда); правило одно с клиентом сервиса.
func loopback(host string) bool { return albums.IsLoopbackHost(host) }

// mayBeLoopback — имя может вести на этот компьютер, включая «*.localhost» (RFC 6761): запрет в выпуске.
func mayBeLoopback(host string) bool { return albums.MayBeLoopbackHost(host) }

// goodHost — IP-адрес (без зоны) или DNS-имя из a-z, 0-9 и «-» с непустыми метками.
func goodHost(host string) bool {
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Zone() == ""
	}
	if len(host) > 253 {
		return false
	}
	labels := strings.Split(host, ".")
	// Последняя метка — буквы или xn--…, как printprof.asciiHost: «0127.0.0.1», «127.1», «0x7f000001»,
	// «0x7f.0.0.0x1» — не имена, а числа: браузер и libc-резолвер (macOS, Ubuntu с mdns) сводят их к IPv4.
	if tld := labels[len(labels)-1]; len(tld) == 0 ||
		(!strings.HasPrefix(tld, "xn--") && strings.Trim(tld, "abcdefghijklmnopqrstuvwxyz") != "") {
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
