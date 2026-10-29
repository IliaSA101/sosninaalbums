//go:build !linux || webkit2_41

// Тег: на Linux pkg/assetserver тянет cgo webkit2gtk, которого нет на машине без тега webkit2_41; на Windows и macOS тест идёт всегда.
package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wailsapp/wails/v2/pkg/assetserver"
	wopt "github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// noRuntime — пустые скрипты среды Wails: сервер активов в тесте нужен только ради маршрутизации.
type noRuntime struct{}

func (noRuntime) DesktopIPC() []byte       { return nil }
func (noRuntime) WebsocketIPC() []byte     { return nil }
func (noRuntime) RuntimeDesktopJS() []byte { return nil }

// Адрес /media/... с «/» или «/index.html» в конце через настоящий ассет-сервер Wails: он подменяет 404
// на HTML-страницу (200), поэтому media отвечает 400 — его Wails пропускает как есть. Без ответа 400 тест
// видит 200 и HTML (случай winqa/144-exe: «/media/lp/», «<набор>/bg/0/», «..» → «/media/»).
func TestMediaTrailingSlashThroughAssetServer(t *testing.T) {
	a, _ := tilesApp(t, true)
	srv, err := assetserver.NewAssetServer("", wopt.Options{
		Assets:     fstest.MapFS{"index.html": {Data: []byte("<html>INDEX</html>")}},
		Middleware: a.media,
	}, false, nil, noRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{
		"/media/lp/", "/media/lp/ivory/bg/0/", "/media/", "/media/t/x/", "/media/gen/", "/media/look/x/",
		"/media/lp/x/index.html", "/media/index.html", "/media/t/index.html",
	} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		// Подмена Wails — HTML (своя страница «index.html not found» или index.html приложения), наш отказ — text/plain
		if w.Code != http.StatusBadRequest || strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || w.Body.String() != "неверный адрес\n" {
			t.Errorf("GET %s через ассет-сервер: %d %q %q, ожидалось 400 и тело media", url, w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
	}
	// Другой регистр и «index.html» без «/» перед ним Wails не подменяет: обычный 404 media
	for _, url := range []string{"/media/lp/x/Index.html", "/media/lp/xindex.html"} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, ожидалось 404", url, w.Code)
		}
	}
	// Контроль: вне /media/ корень отдаёт index.html — так Wails и ведёт себя.
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /: %d", w.Code)
	}
}
