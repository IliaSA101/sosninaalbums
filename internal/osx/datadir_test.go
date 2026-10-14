package osx

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestWebviewDataDir(t *testing.T) {
	base := testkit.TempDir(t)
	t.Setenv("LOCALAPPDATA", base)
	if got, want := WebViewDir(), filepath.Join(base, "SosninaAlbums", "WebView2"); got != want {
		t.Fatalf("WebViewDir() = %q, ждали %q", got, want)
	}
	// Папка не зависит от имени exe: у каждой версии оно своё.
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("HOME", base)
	if got := WebViewDir(); got != "" && filepath.Base(got) != "WebView2" {
		t.Fatalf("запасной путь: %q", got)
	}
}
