package guidevideo

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// get — запрос к Serve; tail — хвост адреса после /media/guide-video/.
func (l *lab) get(method, tail string, hdr map[string]string) *httptest.ResponseRecorder {
	l.t.Helper()
	r := httptest.NewRequest(method, "/media/guide-video/x", nil)
	r.URL.Path = "/media/guide-video/" + tail // уже разобранный адрес: любые знаки
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	l.svc.Serve(w, r, tail)
	return w
}

// ready — скачанный ролик в кэше и каталог в «сохранённом».
func (l *lab) ready(id string, n int) []byte {
	l.t.Helper()
	body := l.webm(id, 1, n)
	if _, err := l.svc.Download([]string{id}); err != nil {
		l.t.Fatal(err)
	}
	return body
}

// Раздача: 200 и весь файл с типом из вида, nosniff, no-store, длиной; HEAD без тела.
func TestServeFullFile(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 5000)
	w := l.get("GET", "cover", nil)
	h := w.Header()
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), body) {
		t.Fatalf("код %d, тело %d байт", w.Code, w.Body.Len())
	}
	if h.Get("Content-Type") != "video/webm" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Cache-Control") != "no-store" || h.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Errorf("заголовки: %v", h)
	}
	if h.Get("Last-Modified") != "" || h.Get("Accept-Ranges") != "" {
		t.Errorf("дата или Accept-Ranges: %v", h)
	}
	hw := l.get("HEAD", "cover", nil)
	if hw.Code != 200 || hw.Body.Len() != 0 || hw.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Errorf("HEAD: %d, тело %d, %v", hw.Code, hw.Body.Len(), hw.Header())
	}
	// mp4 на darwin — свой тип.
	l.host.OS = func() string { return "darwin" }
	l.rebuild()
	l.video("clip", 1, map[string][]byte{"mp4": []byte("mp4-data-1234"), "webm": []byte("webm-data")})
	l.refresh()
	l.put("clip-1.mp4", []byte("mp4-data-1234"))
	if w := l.get("GET", "clip", nil); w.Code != 200 || w.Header().Get("Content-Type") != "video/mp4" || w.Body.String() != "mp4-data-1234" {
		t.Errorf("mp4: %d %v", w.Code, w.Header())
	}
}

// Range и условные запросы игнорируются: всегда 200 и весь файл, 304 и 206 не бывает.
func TestServeIgnoresRangeAndConditions(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 5000)
	for name, hdr := range map[string]map[string]string{
		"Range":         {"Range": "bytes=0-10"},
		"суффикс":       {"Range": "bytes=-5"},
		"много":         {"Range": "bytes=0-1,5-9"},
		"переполнение":  {"Range": "bytes=0-99999999999999999999"},
		"обратный":      {"Range": "bytes=5-3"},
		"мусор":         {"Range": "bytes=abc"},
		"If-None-Match": {"If-None-Match": "*"},
		"If-Modified":   {"If-Modified-Since": time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)},
		"If-Range":      {"Range": "bytes=0-10", "If-Range": "x"},
		"If-Match":      {"If-Match": "nothing"},
		"If-Unmodified": {"If-Unmodified-Since": "Mon, 02 Jan 2006 15:04:05 GMT"},
	} {
		w := l.get("GET", "cover", hdr)
		if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), body) || w.Header().Get("Content-Range") != "" {
			t.Errorf("%s: код %d, тело %d", name, w.Code, w.Body.Len())
		}
	}
}

// Хвост адреса: не id по грамматике, неизвестный id, чужой вид — 404 без обращения к диску.
func TestServeUnknownID404(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	l.put("secret-1.webm", []byte("не из каталога"))
	for _, tail := range []string{
		"", "nope", "../cover", "..", "cover/", "/cover", "cover/x", "Cover", "COVER", "cover.webm", "cover-1.webm",
		"cover%2f..", "%2e%2e", "co", "cover\x00", "cover ", "secret", strings.Repeat("a", 41), `cover\x`, "cover?x=1", "cover#",
	} {
		w := l.get("GET", tail, nil)
		if w.Code != 404 {
			t.Errorf("%q: код %d", tail, w.Code)
		}
	}
	// Без сохранённого каталога ничего не отдаётся, даже при файле на диске.
	l.cached.Store(nil)
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("без каталога: %d", w.Code)
	}
}

// Только GET и HEAD.
func TestServeMethod(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH", "OPTIONS", "TRACE"} {
		w := l.get(m, "cover", nil)
		if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s: код %d, Allow %q", m, w.Code, w.Header().Get("Allow"))
		}
	}
}

// Ссылка на месте ролика, папка, файл другого размера — 404; ссылка не читается.
func TestServeSymlinkRefused(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 3000)
	other := filepath.Join(filepath.Dir(l.dir), "other.bin")
	if err := os.WriteFile(other, body, 0o600); err != nil {
		t.Fatal(err)
	}
	p := l.path("cover", 1, "webm")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, p); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("ссылка: %d", w.Code)
	}
	_ = os.Remove(p)
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("папка: %d", w.Code)
	}
	_ = os.Remove(p)
	l.put("cover-1.webm", body[:100])
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("другой размер: %d", w.Code)
	}
	// Нет файла — 404 без записи в журнал (не сбой).
	_ = os.Remove(p)
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("нет файла: %d", w.Code)
	}
}

// Подмена между Lstat и открытием (файл другой — другой номер): 404, не чужое содержимое.
func TestServeSwapAfterLstat(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 3000)
	p := l.path("cover", 1, "webm")
	open := l.host.OpenRegular
	l.host.OpenRegular = func(path string) (*os.File, error) {
		evil := bytes.Repeat([]byte{'X'}, len(body)) // тот же размер, другой файл
		tmp := path + ".swap"
		if err := os.WriteFile(tmp, evil, 0o600); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, path); err != nil {
			return nil, err
		}
		return open(path)
	}
	l.rebuild()
	w := l.get("GET", "cover", nil)
	if runtime.GOOS == "windows" {
		t.Skip("Rename поверх файла на Windows зависит от прав")
	}
	if w.Code != 404 || bytes.Contains(w.Body.Bytes(), []byte("XXXX")) {
		t.Errorf("подмена: код %d, тело %d", w.Code, w.Body.Len())
	}
	_ = os.Remove(p)
}

// Старая версия не раздаётся: только имя текущей версии из каталога.
func TestServeOldVersion404(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	l.webm("cover", 2, 3100)
	l.seq.Add(1)
	l.refresh()
	if w := l.get("GET", "cover", nil); w.Code != 404 { // v2 в кэше не скачана, v1 лежит рядом
		t.Errorf("прежняя версия отдана: %d", w.Code)
	}
	if _, err := l.svc.Download([]string{"cover"}); err != nil {
		t.Fatal(err)
	}
	if w := l.get("GET", "cover", nil); w.Code != 200 || w.Body.Len() != 3100 {
		t.Errorf("новая: %d, %d", w.Code, w.Body.Len())
	}
}

// Служба выключена (папка не абсолютная) — 404.
func TestServeDisabled(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	l.host.CacheDir = func() string { return "rel" }
	l.rebuild()
	if w := l.get("GET", "cover", nil); w.Code != 404 {
		t.Errorf("код %d", w.Code)
	}
}

// Параллельные запросы укладываются в слоты и не портят друг друга (под -race).
func TestServeSlots(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 100_000)
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			w := l.get("GET", "cover", nil)
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), body) {
				errs <- errors.New("код " + strconv.Itoa(w.Code))
				return
			}
			errs <- nil
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

// Предел слотов: пока два запроса держат файл, третий файл не открывает; освободился слот — идёт.
func TestServeSlotLimit(t *testing.T) {
	l := newLab(t)
	body := l.ready("cover", 5000)
	release := make(chan struct{})
	var opened atomic.Int32
	l.host.OpenRegular = func(path string) (*os.File, error) {
		opened.Add(1)
		<-release
		return imgfile.OpenRegular(path)
	}
	l.rebuild()
	codes := make(chan int, serveSlots+1)
	for range serveSlots + 1 {
		go func() { codes <- l.get("GET", "cover", nil).Code }()
	}
	deadline := time.Now().Add(testkit.Limit(5 * time.Second))
	for opened.Load() < serveSlots && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	// Третьему слота нет: даём время ошибиться, затем смотрим счётчик открытий.
	time.Sleep(50 * time.Millisecond)
	if n := opened.Load(); n != serveSlots {
		t.Fatalf("открыто файлов %d при слотах %d", n, serveSlots)
	}
	close(release)
	for range serveSlots + 1 {
		if code := <-codes; code != 200 {
			t.Errorf("код %d", code)
		}
	}
	if n := opened.Load(); n != serveSlots+1 {
		t.Errorf("после освобождения открыто %d, ждали %d", n, serveSlots+1)
	}
	_ = body
}

// «Показать файл»: только id; путь собирает Go; нет файла, ссылка, чужой id — ошибка без вызова проводника.
func TestRevealUnknownID(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	for _, id := range []string{"", "nope", "../cover", "Cover", l.path("cover", 1, "webm"), "cover-1.webm"} {
		if err := l.svc.Reveal(id); !errors.Is(err, ErrNoFile) || err.Error() != "ролика нет — скачайте снова" {
			t.Errorf("%q: %v", id, err)
		}
	}
	if len(l.revealed) != 0 {
		t.Errorf("проводник вызван: %v", l.revealed)
	}
	if err := l.svc.Reveal("cover"); err != nil || len(l.revealed) != 1 || l.revealed[0] != l.path("cover", 1, "webm") || !filepath.IsAbs(l.revealed[0]) {
		t.Errorf("Reveal: %v, %v", err, l.revealed)
	}
}

// Ролик не скачан или заменён ссылкой/папкой — «ролика нет», проводник не зовётся.
func TestRevealNotReady(t *testing.T) {
	l := newLab(t)
	l.webm("cover", 1, 3000)
	l.refresh()
	if err := l.svc.Reveal("cover"); !errors.Is(err, ErrNoFile) {
		t.Errorf("не скачан: %v", err)
	}
	if err := os.MkdirAll(l.path("cover", 1, "webm"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := l.svc.Reveal("cover"); !errors.Is(err, ErrNoFile) {
		t.Errorf("папка: %v", err)
	}
	if len(l.revealed) != 0 {
		t.Errorf("проводник вызван: %v", l.revealed)
	}
	// Ошибка проводника — без пути.
	_ = os.Remove(l.path("cover", 1, "webm"))
	l.put("cover-1.webm", make([]byte, 3000))
	l.host.Reveal = func(p string) error { return &os.PathError{Op: "start", Path: p, Err: io.ErrClosedPipe} }
	l.rebuild()
	if err := l.svc.Reveal("cover"); !errors.Is(err, ErrReveal) || strings.Contains(err.Error(), l.dir) {
		t.Errorf("ошибка проводника: %v", err)
	}
}
