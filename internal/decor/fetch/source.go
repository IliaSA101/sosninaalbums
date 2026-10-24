package fetch

import (
	"net/url"
	"os"
	"strings"
)

// Стенд QA (design §5): SA_LIBRARY_URL — каталог и архивы с http://127.0.0.1:<порт>/,
// SA_LIBRARY_KEY — открытый тестовый ключ (base64). Читаются только в сборке с тегом dev
// (standLibrary); в выпуске забытая в системе переменная ничего не меняет.

// Default — источник программы: GitHub, у стенда dev — свой, если задан.
func Default() Source {
	if !standLibrary {
		return GitHub()
	}
	if s, ok := Stand(os.Getenv("SA_LIBRARY_URL"), os.Getenv("SA_LIBRARY_KEY")); ok {
		return s
	}
	return GitHub()
}

// Stand — источник стенда: base — http(s)://127.0.0.1:<порт>/…/ (только loopback, путь с «/»
// в конце, без параметров), key — открытый ключ в base64 (пусто — ключи выпуска: подпись
// тестовым ключом тогда не сойдётся). Иначе ok = false.
func Stand(base, key string) (Source, bool) {
	u, err := url.Parse(base)
	if base == "" || err != nil || u.Hostname() != "127.0.0.1" || !strings.HasSuffix(u.Path, "/") ||
		u.Scheme != "http" && u.Scheme != "https" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return Source{}, false
	}
	keys := PublicKeys([]string{key})
	if len(keys) == 0 {
		keys = PublicKeys(Keys[:])
	}
	hosts := append([]string{"127.0.0.1"}, GitHubHosts...)
	return Source{CatalogURL: base + "catalog.json", Prefix: base, Hosts: hosts, QueryHosts: CDNHosts, Keys: keys, LocalHTTP: true}, true
}
