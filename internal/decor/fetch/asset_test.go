package fetch

import (
	"strings"
	"testing"
)

// AssetURL (спека 142): префикс источника плюс путь ассета с той же проверкой, что у строк
// каталога; стенд качает со своего адреса, чужой хост и «..» — отказ.
func TestAssetURL(t *testing.T) {
	const rel = "wedding-navy-watercolor-demo-v1/wedding-navy-watercolor-demo-1.zip"
	gh := New(GitHub())
	if got, ok := gh.AssetURL(rel); !ok || got != AssetPrefix+rel {
		t.Errorf("GitHub: %q %v", got, ok)
	}
	stand, ok := Stand("http://127.0.0.1:34190/lib/", "")
	if !ok {
		t.Fatal("источник стенда не собран")
	}
	if got, ok := New(stand).AssetURL(rel); !ok || got != "http://127.0.0.1:34190/lib/"+rel {
		t.Errorf("стенд: %q %v", got, ok)
	}
	// Единственная метка без файла и два уровня годны; три — нет.
	if _, ok := gh.AssetURL("a/b.zip"); !ok {
		t.Error("метка/файл отвергнуты")
	}
	bad := []string{
		"", "/", "/x.zip", "a/", "a//b.zip", "a/b/c.zip", "../x.zip", "a/../b.zip", "a/..", "..",
		"a b/c.zip", "a/b.zip?x=1", "a/b.zip#f", "%2e%2e/x.zip", "a\\b.zip", "a@evil/b.zip", "a:b/c.zip",
		"http://evil.example/x.zip", "//evil.example/x.zip", strings.Repeat("a", 101) + "/b.zip",
	}
	for _, r := range bad {
		if got, ok := gh.AssetURL(r); ok || got != "" {
			t.Errorf("%q принят: %q", r, got)
		}
	}
	// Чужой хост и пустой префикс источника: ни один адрес не проходит.
	for name, src := range map[string]Source{
		"чужой хост":     {Prefix: "https://evil.example/dl/", Hosts: GitHubHosts},
		"пустой префикс": {},
		"не https":       {Prefix: "http://github.com/dl/", Hosts: GitHubHosts},
	} {
		if got, ok := New(src).AssetURL(rel); ok || got != "" {
			t.Errorf("%s: принят %q", name, got)
		}
	}
}
