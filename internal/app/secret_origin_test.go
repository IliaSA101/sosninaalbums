package app

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/secret"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// Канон адреса сервиса один: токен лежит по secret.Origin, служба ходит по share.ServiceURL.
// Если они разойдутся, токен окажется не под тем ключом (Т3, ревью п. 3).
func checkOriginCanon(t *testing.T, raw string) {
	t.Helper()
	for _, loop := range []bool{false, true} {
		o, err := share.ServiceURL(raw, loop)
		if err != nil {
			continue
		}
		got, err := secret.Origin(o)
		if err != nil || got != o {
			t.Fatalf("share.ServiceURL(%q, %v) = %q, secret.Origin = %q, %v", raw, loop, o, got, err)
		}
		// И сырой ввод даёт тот же ключ, что канон share.
		if raw, err := secret.Origin(raw); err != nil || raw != o {
			t.Fatalf("secret.Origin(сырой) = %q, %v; share дал %q", raw, err, o)
		}
	}
}

func TestSecretOriginCanon(t *testing.T) {
	for _, raw := range []string{
		"https://albums.sosnina-films.ru", " HTTPS://Albums.Sosnina-Films.RU:443/ ", "https://host.example:8443",
		"http://127.0.0.1:34190", "http://[::1]:8080", "http://[0:0:0:0:0:0:0:1]", "http://[::ffff:127.0.0.1]:80",
		"http://localhost", "https://203.0.113.5:65535", "https://2130706433", "https://127.1", "https://host.example:0",
		"https://host.example:070", "https://u:p@host.example", "https://host.example/path", "ftp://host.example",
		"https://xn--80aswg.xn--p1ai", "https://хост.рф", "https://host.example:", "",
	} {
		checkOriginCanon(t, raw)
	}
}

// FuzzSecretOriginCanon: на любой строке, которую принял share.ServiceURL, secret.Origin даёт то же.
func FuzzSecretOriginCanon(f *testing.F) {
	for _, s := range []string{
		"https://albums.sosnina-films.ru", "HTTP://[::FFFF:7F00:1]:80", "https://a.b:443", "http://127.0.0.1:34190/",
		"https://2130706433", "https://host.example:00080", "http://[::1", "https://[v1.x]", "https://a..b",
	} {
		f.Add(s)
	}
	f.Fuzz(checkOriginCanon)
}
