package update

import (
	"net/url"
	"os"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Где лежит выпуск: подписанный манифест — ассет последнего выпуска (releases/latest — только
// опубликованный, не черновик), установщик — ассет выпуска vX.Y.Z (design §5.1).
const (
	ManifestName = "SosninaAlbums-update.json"
	ManifestURL  = "https://github.com/IliaSA101/sosninaalbums/releases/latest/download/" + ManifestName
	AssetPrefix  = "https://github.com/IliaSA101/sosninaalbums/releases/download/"
)

// Hosts — белый список: github.com и CDN ассетов после 302 (параметры подписи — только им).
var Hosts = []string{"github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"}

// GitHub — источник выпуска.
func GitHub() fetch.Source {
	return fetch.Source{CatalogURL: ManifestURL, Prefix: AssetPrefix, Hosts: Hosts, QueryHosts: fetch.CDNHosts,
		Keys: fetch.PublicKeys(Keys[:])}
}

// Stand — источник стенда QA: base — http(s)://127.0.0.1:<порт>/…/ (только loopback, путь с «/»
// в конце, без параметров): манифест лежит прямо в нём (ленту отдаёт python -m http.server),
// установщик — в «vX.Y.Z/»; key — тестовый открытый ключ (base64; пусто — ключи выпуска, и
// подпись тестовым ключом не сойдётся). Иначе ok = false.
func Stand(base, key string) (fetch.Source, bool) {
	u, err := url.Parse(base)
	if base == "" || err != nil || u.Hostname() != "127.0.0.1" || !strings.HasSuffix(u.Path, "/") ||
		u.Scheme != "http" && u.Scheme != "https" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return fetch.Source{}, false
	}
	keys := fetch.PublicKeys([]string{key})
	if len(keys) == 0 {
		keys = fetch.PublicKeys(Keys[:])
	}
	return fetch.Source{CatalogURL: base + ManifestName, Prefix: base, Hosts: append([]string{"127.0.0.1"}, Hosts...),
		QueryHosts: fetch.CDNHosts, Keys: keys, LocalHTTP: true}, true
}

// Default — откуда проверять и разрешена ли проверка при запуске: выпуск — GitHub и да; сборка
// dev — стенд из SA_UPDATE_URL/SA_UPDATE_KEY и да, а без стенда — GitHub (кнопка «Проверить»
// работает) и нет: сборка разработки сама в сеть не ходит.
func Default() (src fetch.Source, auto bool) { return defaultFrom(standEnabled, os.Getenv) }

// defaultFrom — тело Default с подставляемыми enabled и getenv (проверяется без тега dev).
func defaultFrom(enabled bool, getenv func(string) string) (fetch.Source, bool) {
	if !enabled {
		return GitHub(), true
	}
	if s, ok := Stand(getenv("SA_UPDATE_URL"), getenv("SA_UPDATE_KEY")); ok {
		return s, true
	}
	return GitHub(), false
}

// Client — клиент обновления: fetch.Client и источник, из которого строятся адреса.
type Client struct {
	fc  *fetch.Client
	src fetch.Source
}

// New — клиент для src.
func New(src fetch.Source) *Client { return &Client{fc: fetch.New(src), src: src} }

// Stand — источник стенда QA (SA_UPDATE_URL): его выпуски выдуманы, поэтому Seen и Checked из
// такой проверки в config.json не пишутся — иначе тестовая лента испортит проверку настоящих.
func (c *Client) Stand() bool { return c.src.LocalHTTP }

// setupURL — адрес установщика версии ver: префикс источника, метка выпуска и имя из SetupName.
// "" — ver негодна.
func (c *Client) setupURL(ver string) string {
	name := SetupName(ver)
	if name == "" {
		return ""
	}
	return c.src.Prefix + "v" + ver + "/" + name
}
