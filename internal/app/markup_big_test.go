package app

import (
	"bytes"
	"context"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Большие JPEG/PNG разметки (086 G2): подложку строит Go, как у PSD.

// TestServeSpreadGuideLargeJPEG — JPEG длиннее 4096 px: ответ image/jpeg ≤ 4096 (nosniff,
// no-cache), второй запрос — из кэша, файл заменён — строится заново; прозрачный PNG — image/png
// из того же слота; слот обложки отдельный.
func TestServeSpreadGuideLargeJPEG(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	built := countPreview(t)
	h := a.media(http.NotFoundHandler())
	get := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		return w
	}
	dir := testkit.TempDir(t)
	path := testkit.JPEG(t, dir, "big.jpg", 5000, 3000)
	if _, err := a.setSpreadGuide(path); err != nil {
		t.Fatal(err)
	}
	w := get("/media/spread-guide?r=1")
	hd := w.Header()
	if w.Code != http.StatusOK || hd.Get("Content-Type") != "image/jpeg" || hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("Cache-Control") != "no-cache" {
		t.Fatalf("ответ %d, заголовки %v", w.Code, hd)
	}
	img, err := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != library.MarkupMaxSide || img.Bounds().Dy() > library.MarkupMaxSide {
		t.Fatalf("JPEG: %v %v", err, img)
	}
	first := w.Body.Bytes()
	if w = get("/media/spread-guide?r=2"); !bytes.Equal(w.Body.Bytes(), first) || built.Load() != 1 {
		t.Fatalf("кэш: построений %d", built.Load())
	}
	// Файл заменён (другой размер и время) — превью заново.
	testkit.JPEG(t, dir, "big.jpg", 6000, 3000)
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	w = get("/media/spread-guide")
	img, err = jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != library.MarkupMaxSide || img.Bounds().Dy() != 2048 || built.Load() != 2 {
		t.Fatalf("после замены: %v %v, построений %d", err, img, built.Load())
	}
	// Прозрачный PNG — PNG; тип берётся из кэша, а не остаётся от прежнего файла.
	pp := testkit.PNG(t, dir, "big.png", 5000, 1000, color.NRGBA{R: 9, A: 255})
	if _, err := a.setSpreadGuide(pp); err != nil {
		t.Fatal(err)
	}
	for range 2 { // второй раз — из кэша, тип прежний
		w = get("/media/spread-guide")
		if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); err != nil || w.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("PNG: %v, тип %q", err, w.Header().Get("Content-Type"))
		}
	}
	if built.Load() != 3 {
		t.Fatalf("построений %d, ждали 3", built.Load())
	}
	// Обложка — свой слот.
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setCoverGuide(path); err != nil {
		t.Fatal(err)
	}
	if w = get("/media/cover-guide"); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/jpeg" || built.Load() != 4 {
		t.Fatalf("обложка: %d, построений %d", w.Code, built.Load())
	}
	if w = get("/media/spread-guide"); w.Header().Get("Content-Type") != "image/png" || built.Load() != 4 {
		t.Fatalf("слот разворотов сбит обложкой: тип %q, построений %d", w.Header().Get("Content-Type"), built.Load())
	}
}

// TestServeCoverGuideSmallFile — JPEG/PNG до 4096 px отдаются байтами файла (превью не
// строится), как и раньше; ровно 4096 — тоже файл.
func TestServeCoverGuideSmallFile(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	built := countPreview(t)
	h := a.media(http.NotFoundHandler())
	dir := testkit.TempDir(t)
	for _, c := range []struct{ path, ct string }{
		{testkit.JPEG(t, dir, "s.jpg", 3000, 2000), "image/jpeg"},
		{testkit.JPEG(t, dir, "e.jpg", library.MarkupMaxSide, 8), "image/jpeg"},
		{testkit.PNG(t, dir, "s.png", 200, 100, color.NRGBA{A: 255}), "image/png"},
	} {
		if _, err := a.setCoverGuide(c.path); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/cover-guide", nil))
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != c.ct || !bytes.Equal(w.Body.Bytes(), want) {
			t.Fatalf("%s: %d, тип %q, %d байт из %d", c.path, w.Code, w.Header().Get("Content-Type"), w.Body.Len(), len(want))
		}
	}
	if built.Load() != 0 {
		t.Fatalf("превью строилось для малых файлов: %d", built.Load())
	}
}

// TestServeGuideBudgetCancel — бюджет декода занят целиком (экспорт): запрос большой разметки с
// отменённым контекстом — сразу 404; запрос, ждущий бюджет, не держит замок кэша — второй запрос
// того же файла уходит по своей отмене, не дожидаясь первого, и не строит заново; отмена не
// запоминается — бюджет свободен, следующий запрос строит превью.
func TestServeGuideBudgetCancel(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	built := countPreview(t)
	var logBuf bytes.Buffer // журнал пишется под своим замком; читаем, когда запросы уже ответили
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	h := a.media(http.NotFoundHandler())
	serve := func(ctx context.Context) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/spread-guide", nil).WithContext(ctx))
		return w.Code
	}
	if _, err := a.setSpreadGuide(testkit.JPEG(t, testkit.TempDir(t), "big.jpg", 5000, 3000)); err != nil {
		t.Fatal(err)
	}
	imgfile.DecodeBudget.Acquire(imgfile.DecodeBudgetPixels)
	held := true
	t.Cleanup(func() {
		if held {
			imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
		}
	})
	limit := testkit.Limit(2 * time.Second)

	// Ответ ждём в своей горутине: зависший запрос — провал теста, а не тайм-аут всего прогона.
	within := func(ctx context.Context) int {
		got := make(chan int, 1)
		go func() { got <- serve(ctx) }()
		select {
		case code := <-got:
			return code
		case <-time.After(limit):
			t.Fatal("запрос висит")
			return 0
		}
	}
	dead, kill := context.WithCancel(context.Background())
	kill()
	if code := within(dead); code != http.StatusNotFound {
		t.Fatalf("отменённый запрос: %d", code)
	}

	// Первый ждёт бюджет с живым контекстом.
	live, stop := context.WithCancel(context.Background())
	defer stop()
	first := make(chan int, 1)
	go func() { first <- serve(live) }()
	for deadline := time.Now().Add(limit); built.Load() < 2 || imgfile.DecodeBudget.Waiting() < 1; {
		if time.Now().After(deadline) {
			t.Fatal("первый запрос не встал в очередь бюджета")
		}
		time.Sleep(time.Millisecond)
	}
	// Второй того же файла — ждёт готовое, но уходит по своей отмене.
	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if code := within(short); code != http.StatusNotFound || built.Load() != 2 {
		t.Fatalf("второй: %d, построений %d", code, built.Load())
	}
	stop()
	select {
	case code := <-first:
		if code != http.StatusNotFound {
			t.Fatalf("первый после отмены: %d", code)
		}
	case <-time.After(limit):
		t.Fatal("первый не ушёл по отмене")
	}

	if s := logWithoutSlow(logBuf.String()); s != "" {
		t.Fatalf("закрытые запросы попали в журнал: %q", s)
	}

	imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
	held = false
	if code := serve(context.Background()); code != http.StatusOK || built.Load() != 3 {
		t.Fatalf("после освобождения бюджета: %d, построений %d", code, built.Load())
	}
	if u := imgfile.DecodeBudget.Used(); u != 0 {
		t.Fatalf("бюджет не возвращён: %d", u)
	}
}
