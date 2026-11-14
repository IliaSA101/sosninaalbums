package libpacks

import (
	"bytes"
	"context"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Раздача клеток /media/lp/ (спека 144 Г3, security 9): метод, хвост, заголовки, без 304 и без сети.

// serveFx — стенд из tilesFx, где любой выход в сеть проваливает тест.
func serveFx(t *testing.T) *prevFx {
	t.Helper()
	fx, _ := tilesFx(t, nil)
	fx.s.h.Fetcher = func() *fetch.Client {
		t.Error("раздача клетки ходит в сеть")
		return nil
	}
	return fx
}

func get(fx *prevFx, method, tail string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/", nil)
	r.URL.Path = "/media/lp/" + tail // без разбора адреса: пробелы и прочее приходят как есть
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	fx.s.ServePreview(w, r, tail)
	return w
}

// Годная клетка — JPEG нужного размера; всё неизвестное — 404.
func TestMediaLP(t *testing.T) {
	fx := serveFx(t)
	w := get(fx, http.MethodGet, "pack-b/sticker/3", nil)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if w.Code != http.StatusOK || err != nil || cfg.Width != 32 || cfg.Height != 32 {
		t.Fatalf("годная клетка: %d %v %+v", w.Code, err, cfg)
	}
	for _, tail := range []string{
		"pack-zzz/bg/0",  // чужой id
		"../bg/0",        // «..» (так приходит %2e%2e)
		"a/b/bg/0",       // четыре сегмента (так приходит a%2Fb)
		"pack-a/bg",      // два сегмента
		"pack-a/bg/0/",   // лишний сегмент
		"",               // пусто
		"pack-a/Bg/0",    // регистр вида
		"pack-a/print/0", // вид не из пяти
		"pack-a/bg/-1",   // знак
		"pack-a/bg/+1",   // знак
		"pack-a/bg/01",   // нуль впереди
		"pack-a/bg/3",    // вне ряда
		"pack-a/bg/99",   // вне ряда
		"pack-a/bg/1e0",  // не число
		"pack-a/bg/",     // пустой индекс
		"pack-a/bg/ 1",   // пробел
		"pack-a/bg/999999999999999999999",
		"con/bg/0",         // имя устройства Windows
		"PACK-A/bg/0",      // регистр id
		"pack-a/sticker/4", // вне ряда стикеров
	} {
		if w := get(fx, http.MethodGet, tail, nil); w.Code != http.StatusNotFound {
			t.Errorf("%q: %d, нужен 404", tail, w.Code)
		}
	}
	if fx.s.lpMiss.Load() == 0 {
		t.Error("промахи не посчитаны")
	}
}

// Набор уже стоит в библиотеке — клетки больше не отдаются.
func TestMediaLPStanding(t *testing.T) {
	fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[0].ID = "ivory" })
	if w := get(fx, http.MethodGet, "ivory/bg/0", nil); w.Code != http.StatusOK {
		t.Fatalf("до установки: %d", w.Code)
	}
	if _, err := fx.s.AddPath(d1Zip(t)); err != nil {
		t.Fatal(err)
	}
	if w := get(fx, http.MethodGet, "ivory/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("стоящий набор: %d", w.Code)
	}
}

// Только GET и HEAD: прочее — 405 с Allow; HEAD — те же заголовки без тела.
func TestMediaLPMethod(t *testing.T) {
	fx := serveFx(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
		w := get(fx, m, "pack-a/bg/0", nil)
		if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s: %d, Allow %q", m, w.Code, w.Header().Get("Allow"))
		}
	}
	g, h := get(fx, http.MethodGet, "pack-a/bg/0", nil), get(fx, http.MethodHead, "pack-a/bg/0", nil)
	if h.Code != http.StatusOK || h.Body.Len() != 0 || h.Header().Get("Content-Length") != g.Header().Get("Content-Length") ||
		h.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("HEAD: %d, тело %d, %v", h.Code, h.Body.Len(), h.Header())
	}
	if w := get(fx, http.MethodHead, "pack-zzz/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("HEAD чужого: %d", w.Code)
	}
}

// Заголовки ответа: тип, nosniff, no-cache, длина = телу.
func TestMediaLPHeaders(t *testing.T) {
	fx := serveFx(t)
	w := get(fx, http.MethodGet, "pack-a/bg/2", nil)
	h := w.Header()
	if h.Get("Content-Type") != "image/jpeg" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "no-cache" ||
		h.Get("Content-Length") != strconv.Itoa(w.Body.Len()) || h.Get("Accept-Ranges") != "" || h.Get("ETag") != "" || h.Get("Last-Modified") != "" {
		t.Errorf("заголовки: %v, тело %d", h, w.Body.Len())
	}
}

// 304 не бывает: условные заголовки и Range не читаются, ответ — всегда целый 200.
func TestMediaLPNever304(t *testing.T) {
	fx := serveFx(t)
	base := get(fx, http.MethodGet, "pack-a/bg/1", nil)
	for _, hdr := range []map[string]string{
		{"If-None-Match": "*"},
		{"If-Modified-Since": "Wed, 21 Oct 2037 07:28:00 GMT"},
		{"If-Match": "x", "If-Unmodified-Since": "Wed, 21 Oct 1999 07:28:00 GMT"},
		{"Range": "bytes=0-9"},
		{"If-Range": "x", "Range": "bytes=5-"},
	} {
		w := get(fx, http.MethodGet, "pack-a/bg/1", hdr)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), base.Body.Bytes()) {
			t.Errorf("%v: %d, тело %d из %d", hdr, w.Code, w.Body.Len(), base.Body.Len())
		}
	}
}

// Паника разбора — 404 (не пустой 200), в журнале нет адреса и пути; службе потом можно работать.
func TestMediaLPRecover(t *testing.T) {
	fx := serveFx(t)
	buf := logTo(t)
	fx.s.afterLstat = func() { panic("сбой разбора") }
	w := get(fx, http.MethodGet, "pack-a/bg/0", nil)
	if w.Code != http.StatusNotFound || w.Body.Len() == 0 && w.Header().Get("Content-Type") == "image/jpeg" {
		t.Fatalf("после паники: %d %v", w.Code, w.Header())
	}
	if out := buf.String(); !strings.Contains(out, "сбой") || strings.Contains(out, "pack-a") || strings.Contains(out, fx.dir) {
		t.Errorf("журнал: %q", out)
	}
	fx.s.afterLstat = nil
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusOK {
		t.Errorf("после сбоя: %d", w.Code)
	}
}

// Без папки настроек раздача пуста (образцы выключены), без списка — тоже.
func TestMediaLPOff(t *testing.T) {
	fx := serveFx(t)
	fx.s.h.CfgDir = func() string { return "" }
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("без папки: %d", w.Code)
	}
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	fx.s.h.CfgDir = func() string { return fx.dir }
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("без согласия: %d", w.Code)
	}
	empty := newPrevFx(t, nil)
	if w := get(empty, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("без списка: %d", w.Code)
	}
}

// Паника после строки задания: у prevLog общий счётчик, а строка о панике всё равно в журнале.
func TestMediaLPRecoverAfterJobLog(t *testing.T) {
	fx := serveFx(t)
	buf := logTo(t)
	fx.s.prevLog("задание: строка") // первая строка счётчика — следующая через prevLog не вышла бы
	fx.s.afterLstat = func() { panic("сбой разбора") }
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusNotFound {
		t.Fatalf("после паники: %d", w.Code)
	}
	if out := buf.String(); !strings.Contains(out, "раздача клетки: сбой (string)") || strings.Contains(out, "pack-a") {
		t.Errorf("журнал: %q", out)
	}
}

// Промахи пишутся по своему счётчику, не по счётчику задания: после строки задания первый промах в журнале.
func TestMediaLPMissLog(t *testing.T) {
	fx := serveFx(t)
	buf := logTo(t)
	fx.s.prevLog("задание: строка")
	get(fx, http.MethodGet, "pack-zzz/bg/0", nil)
	if out := buf.String(); !strings.Contains(out, "нет такой (1)") || strings.Contains(out, "pack-zzz") {
		t.Errorf("журнал: %q", out)
	}
}

// Запрос ушёл, пока слот разбора занят: 404 (не пустой 200) и ни одной строки в журнале.
func TestMediaLPCancelled(t *testing.T) {
	fx := serveFx(t)
	buf := logTo(t)
	fx.s.cells.slot <- struct{}{} // слот занят чужим разбором
	defer func() { <-fx.s.cells.slot }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	fx.s.ServePreview(w, r, "pack-a/bg/0")
	if w.Code != http.StatusNotFound || w.Header().Get("Content-Type") == "image/jpeg" {
		t.Errorf("отменённый запрос: %d %v", w.Code, w.Header())
	}
	if buf.Len() != 0 {
		t.Errorf("журнал: %q", buf.String())
	}
}

// Отзыв согласия виден сразу: после SetNet(false) следующая клетка — 404, после SetNet(true) — снова 200.
func TestMediaLPRevoke(t *testing.T) {
	fx := serveFx(t)
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusOK {
		t.Fatalf("с согласием: %d", w.Code)
	}
	if _, err := fx.s.SetNet(false, false); err != nil {
		t.Fatal(err)
	}
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusNotFound {
		t.Errorf("после отзыва: %d", w.Code)
	}
	if _, err := fx.s.SetNet(true, false); err != nil {
		t.Fatal(err)
	}
	if w := get(fx, http.MethodGet, "pack-a/bg/0", nil); w.Code != http.StatusOK {
		t.Errorf("после повторного согласия: %d", w.Code)
	}
}
