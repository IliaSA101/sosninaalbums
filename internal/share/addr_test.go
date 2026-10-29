package share

import (
	"net/url"
	"strings"
	"testing"
)

func TestShareServiceURL(t *testing.T) {
	// Обычные адреса: канонический origin, одинаковый и со стендовым флагом, и без него.
	ok := map[string]string{
		"https://albums.example.org":      "https://albums.example.org",
		"  https://Albums.Example.ORG/ ":  "https://albums.example.org",
		"https://albums.example.org\n":    "https://albums.example.org", // вставка с переводом строки на конце
		"https://albums.example.org:8443": "https://albums.example.org:8443",
		"https://192.168.1.5":             "https://192.168.1.5",
		"HTTPS://Example.com":             "https://example.com",
		// Канон: пустой порт и порт по умолчанию сняты, IP в краткой записи.
		"https://example.com:":       "https://example.com",
		"https://example.com:443":    "https://example.com",
		"https://example.com:443/":   "https://example.com",
		"https://example.com:8080":   "https://example.com:8080",
		"https://[2001:DB8:0:0::1]":  "https://[2001:db8::1]",
		"https://[::ffff:192.0.2.1]": "https://192.0.2.1",
		"https://[2001:db8::1]:443":  "https://[2001:db8::1]",
		"https://[2001:db8::1]:8443": "https://[2001:db8::1]:8443",
	}
	for in, want := range ok {
		for _, loop := range []bool{false, true} {
			if got, err := ServiceURL(in, loop); err != nil || got != want {
				t.Errorf("%q (loopback %v): %q, %v (ждали %q)", in, loop, got, err, want)
			}
		}
	}
	// Loopback по http — только со стендовым флагом (0.32-146-25).
	loop := map[string]string{
		"http://localhost:8080":     "http://localhost:8080",
		"http://127.0.0.1:34116/":   "http://127.0.0.1:34116",
		"http://127.1.2.3":          "http://127.1.2.3",
		"http://127.0.0.1:80":       "http://127.0.0.1",
		"http://127.0.0.1:":         "http://127.0.0.1",
		"http://[::1]:9000":         "http://[::1]:9000",
		"http://[0:0:0:0:0:0:0:1]":  "http://[::1]",
		"http://[::ffff:127.0.0.1]": "http://127.0.0.1",
		"https://localhost:8443":    "https://localhost:8443",
		"https://127.0.0.1:8443":    "https://127.0.0.1:8443",
		"https://[::1]":             "https://[::1]",
		"https://[0:0:0:0:0:0:0:1]": "https://[::1]",
		"http://LOCALHOST:8080/":    "http://localhost:8080",
		// *.localhost (RFC 6761) системы сводят к 127.0.0.1: в выпуске отказ, как для localhost.
		"https://app.localhost":      "https://app.localhost",
		"https://A.B.Localhost:8443": "https://a.b.localhost:8443",
		// Неопределённый адрес: на Linux и macOS соединение с ним уходит на этот компьютер.
		"https://0.0.0.0":      "https://0.0.0.0",
		"https://0.0.0.0:8443": "https://0.0.0.0:8443",
		"https://[::]":         "https://[::]",
		"https://[::]:8443":    "https://[::]:8443",
	}
	for in, want := range loop {
		if got, err := ServiceURL(in, true); err != nil || got != want {
			t.Errorf("%q со стендом: %q, %v (ждали %q)", in, got, err, want)
		}
		// В выпуске этот компьютер — отказ при любой схеме: подставной локальный сервер не нужен.
		if got, err := ServiceURL(in, false); err == nil {
			t.Errorf("%q в выпуске принят: %q", in, got)
		}
	}
	bad := []string{
		"", "   ", "albums.example.org", "ftp://albums.example.org", "javascript:alert(1)",
		"http://albums.example.org",          // http не для loopback
		"http://app.localhost",               // поддомен localhost — не этот компьютер (единое правило с albums)
		"http://10.0.0.5",                    // частная сеть — не loopback
		"https://user@albums.example.org",    // логин
		"https://user:pw@albums.example.org", // логин и пароль
		"https://albums.example.org?x=1",
		"https://albums.example.org#top",
		"https://albums.example.org/api", // путь
		"https://albums.example.org\\x",
		"https://albums .example.org",
		"https://albums.example.org/\nx",
		"https://alb\x00ums.example.org",
		"https://альбомы.рф", // не-ASCII: отказ, не punycode (0.32-146-26)
		"https://",
		"https://:443",
		"https://-bad.example.org",
		"https://a..b",
		"https://albums.example.org.",
		"https://0127.0.0.1", // не имя и не IP
		"https://example.123",
		// Шестнадцатеричные и сокращённые записи IPv4 libc-резолвер сводит к адресу (0x7f000001 = 127.0.0.1).
		"https://0x7f000001", "https://0x7f.0.0.0x1", "https://0x7f.0.0.1", "https://0177.0.0.1", "https://127.1",
		"https://2130706433", "https://localhost.0x1", "https://a.0xff",
		"https://albums.example.org:0",
		"https://albums.example.org:65536",
		"https://albums.example.org:08",
		"https://albums.example.org:abc",
		"https://[fe80::1%25eth0]",
		"https://" + strings.Repeat("a", 64) + ".org",
		"https://" + strings.Repeat("a.", 150) + "org",
		"https://" + strings.Repeat("a", MaxServiceURL),
		"//albums.example.org",
		"mailto:a@b.c",
	}
	for _, in := range bad {
		for _, loop := range []bool{false, true} {
			if got, err := ServiceURL(in, loop); err == nil {
				t.Errorf("%q (loopback %v) принят: %q", in, loop, got)
			}
		}
	}
}

// Ссылка поддельного сервера: только на этот компьютер, остальные проверяет CheckURL службы.
func TestShareCheckLoopbackLink(t *testing.T) {
	for _, in := range []string{"http://127.0.0.1:34116/a/abc/", "http://localhost:8080/a/x", "https://[::1]:9/a/x/"} {
		if err := CheckLoopbackLink(in, true); err != nil {
			t.Errorf("%q: %v", in, err)
		}
		if CheckLoopbackLink(in, false) == nil { // не стенд — всегда отказ
			t.Errorf("%q принят без стендового флага", in)
		}
	}
	for _, in := range []string{
		"", "https://albums.example.org/a/x/", "http://10.0.0.5/a/", "javascript:alert(1)", "file:///etc/passwd",
		"http://user@127.0.0.1/a/", "http://127.0.0.1/a/#x", "http://127.0.0.1/a/?t=1",
		"http://127.0.0.1/" + strings.Repeat("a", MaxLinkURL), "http://127.0.0.1/a b",
	} {
		for _, allow := range []bool{false, true} {
			if CheckLoopbackLink(in, allow) == nil {
				t.Errorf("%q принят (стенд %v)", in, allow)
			}
		}
	}
}

// Любой вход: ответ или ошибка, без паники; принятый адрес — канонический origin: повторный разбор
// даёт его же, без «:» и «/» на конце.
func FuzzShareServiceURL(f *testing.F) {
	for _, s := range []string{"https://a.org", "http://[::1]:1", "https://u@a.org", "", "http://x", "https://a.org/%zz", "//a", "https://a.org:99999", "https://a.org:", "https://[0::1]:443"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, loop := range []bool{false, true} {
			o, err := ServiceURL(s, loop)
			if err != nil {
				continue
			}
			again, err := ServiceURL(o, loop)
			if err != nil || again != o {
				t.Fatalf("%q → %q → %q, %v", s, o, again, err)
			}
			if strings.ContainsAny(o, "@?#\\ ") || len(o) > MaxServiceURL || strings.HasSuffix(o, ":") || strings.HasSuffix(o, "/") {
				t.Fatalf("%q → %q", s, o)
			}
			if !loop && (!strings.HasPrefix(o, "https://") || mayBeLoopback(outHost(o))) {
				t.Fatalf("%q → %q: не https или этот компьютер без стенда", s, o)
			}
		}
	})
}

// outHost — имя хоста канонического адреса (без порта и скобок IPv6).
func outHost(o string) string {
	u, err := url.Parse(o)
	if err != nil {
		return o
	}
	return u.Hostname()
}
