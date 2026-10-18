package app

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Разметка типографии в App (066): выбор файла и раздача /media/*-guide.

// countPreview подменяет построение превью счётчиком обращений.
func countPreview(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	old := markupPreview
	markupPreview = func(path string) ([]byte, string, error) {
		n.Add(1)
		return library.MarkupPreview(path)
	}
	t.Cleanup(func() { markupPreview = old })
	return &n
}

// TestServeSpreadGuidePSD — /media/spread-guide: PSD отдаётся PNG-превью (nosniff, no-cache),
// путь только из проекта; повтор — из кэша, файл изменился — строится заново; одновременные
// запросы строят один раз; пропал, испорчен, нет картинки или разметки — 404. Слоты обложки и
// разворотов разные.
func TestServeSpreadGuidePSD(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	built := countPreview(t)
	h := a.media(http.NotFoundHandler())
	get := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		return w
	}
	if w := get("/media/spread-guide?r=1"); w.Code != http.StatusNotFound {
		t.Fatalf("без разметки: %d", w.Code)
	}
	dir := testkit.TempDir(t)
	c := color.NRGBA{R: 10, G: 200, B: 30, A: 255}
	path := testkit.PSD(t, dir, "spread.psd", testkit.PSDSpec{W: 40, H: 20, C: c, DPI: 300})
	if _, err := a.setSpreadGuide(path); err != nil {
		t.Fatal(err)
	}
	w := get("/media/spread-guide?r=2")
	hd := w.Header()
	if w.Code != http.StatusOK || hd.Get("Content-Type") != "image/png" || hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("Cache-Control") != "no-cache" {
		t.Fatalf("ответ %d, заголовки %v", w.Code, hd)
	}
	img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 40 || img.Bounds().Dy() != 20 || color.NRGBAModel.Convert(img.At(3, 3)) != c {
		t.Fatalf("PNG: %v %v", err, img)
	}
	first := w.Body.Bytes()
	other := testkit.PSD(t, dir, "other.psd", testkit.PSDSpec{W: 8, H: 8, C: color.NRGBA{A: 255}})
	if w = get("/media/spread-guide?path=" + other + "&r=3"); !bytes.Equal(w.Body.Bytes(), first) || built.Load() != 1 {
		t.Fatalf("путь из запроса или кэш: построений %d", built.Load())
	}
	// Одновременные запросы того же файла: построение одно.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { get("/media/spread-guide") })
	}
	wg.Wait()
	if built.Load() != 1 {
		t.Fatalf("построений %d", built.Load())
	}
	// Файл перезаписан (другой размер и время) — новое превью.
	testkit.PSD(t, dir, "spread.psd", testkit.PSDSpec{W: 30, H: 20, C: c})
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if img, err = png.Decode(bytes.NewReader(get("/media/spread-guide").Body.Bytes())); err != nil || img.Bounds().Dx() != 30 || built.Load() != 2 {
		t.Fatalf("после замены: %v %v, построений %d", err, img, built.Load())
	}
	// Обложка — свой слот: её разметка не вытесняет разворотов.
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setCoverGuide(other); err != nil {
		t.Fatal(err)
	}
	if w = get("/media/cover-guide"); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || built.Load() != 3 {
		t.Fatalf("обложка: %d, построений %d", w.Code, built.Load())
	}
	if get("/media/spread-guide"); built.Load() != 3 {
		t.Fatalf("слот разворотов сбит обложкой: построений %d", built.Load())
	}
	// Без сведённого изображения — 404 (видны только направляющие).
	flat := testkit.PSD(t, dir, "flat.psd", testkit.PSDSpec{W: 8, H: 8, NoMerged: true})
	if v, err := a.setSpreadGuide(flat); err != nil || !v.SpreadMarkup.OK || v.SpreadMarkup.Image || v.SpreadMarkup.Note == "" {
		t.Fatalf("PSD без сведённого: %v %+v", err, v.SpreadMarkup)
	}
	if w = get("/media/spread-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("без картинки: %d", w.Code)
	}
	for _, u := range []string{"/media/spread-guide/x.psd", "/media/spread-guide2", "/media/spread-guide/../cover-guide"} {
		if w := get(u); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", u, w.Code)
		}
	}
	if _, err := a.setSpreadGuide(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("8BPS испорчен"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w = get("/media/spread-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("испорченный файл: %d", w.Code)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if w = get("/media/spread-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("файл пропал: %d", w.Code)
	}
	if a.ClearSpreadGuide(); get("/media/spread-guide").Code != http.StatusNotFound {
		t.Fatal("разметку убрали, а раздача осталась")
	}
}

// TestServeGuideNoImageLog — PSD без сведённого — обычный случай: 404 без строки в журнале, и
// при повторе из кэша тоже; сбой построения превью — 404 и строка в журнале (без пути).
func TestServeGuideNoImageLog(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	h := a.media(http.NotFoundHandler())
	get := func() int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/spread-guide", nil))
		return w.Code
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := testkit.TempDir(t)
	flat := testkit.PSD(t, dir, "flat.psd", testkit.PSDSpec{W: 8, H: 8, NoMerged: true})
	if _, err := a.setSpreadGuide(flat); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if c := get(); c != http.StatusNotFound {
			t.Fatalf("без картинки: %d", c)
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("PSD без сведённого попал в журнал: %q", buf.String())
	}
	old := markupPreview
	markupPreview = func(string) ([]byte, string, error) { return nil, "", errors.New("сбой разбора") }
	t.Cleanup(func() { markupPreview = old })
	full := testkit.PSD(t, dir, "full.psd", testkit.PSDSpec{W: 8, H: 8, C: color.NRGBA{A: 255}})
	if _, err := a.setSpreadGuide(full); err != nil {
		t.Fatal(err)
	}
	if c := get(); c != http.StatusNotFound || !strings.Contains(buf.String(), "превью PSD не построено: сбой разбора") || strings.Contains(buf.String(), dir) {
		t.Fatalf("сбой превью: %d, журнал %q", c, buf.String())
	}
}

// TestSetGuideResetsPreviewError — разовый сбой построения превью запоминается (файл не
// разбирается на каждый запрос), но повторный выбор того же файла сбрасывает запись: превью
// строится заново, а не держит 404 до изменения файла. Так у обеих разметок, слоты независимы.
func TestSetGuideResetsPreviewError(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	h := a.media(http.NotFoundHandler())
	get := func(url string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		return w.Code
	}
	log.SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	fail := true
	var built atomic.Int32
	old := markupPreview
	markupPreview = func(path string) ([]byte, string, error) {
		built.Add(1)
		if fail {
			return nil, "", errors.New("сбой чтения")
		}
		return library.MarkupPreview(path)
	}
	t.Cleanup(func() { markupPreview = old })
	dir := testkit.TempDir(t)
	sp := testkit.PSD(t, dir, "s.psd", testkit.PSDSpec{W: 8, H: 8, C: color.NRGBA{A: 255}})
	cv := testkit.PSD(t, dir, "c.psd", testkit.PSDSpec{W: 8, H: 8, C: color.NRGBA{A: 255}})
	for _, c := range []struct {
		url string
		set func(string) (any, error)
		pth string
	}{
		{"/media/spread-guide", func(p string) (any, error) { return a.setSpreadGuide(p) }, sp},
		{"/media/cover-guide", func(p string) (any, error) { return a.setCoverGuide(p) }, cv},
	} {
		fail = true
		if _, err := c.set(c.pth); err != nil {
			t.Fatal(err)
		}
		before := built.Load()
		if code := get(c.url); code != http.StatusNotFound {
			t.Fatalf("%s при сбое: %d", c.url, code)
		}
		get(c.url) // ошибка закэширована: второго построения нет
		if built.Load() != before+1 {
			t.Fatalf("%s: построений %d, ждали 1", c.url, built.Load()-before)
		}
		fail = false
		if code := get(c.url); code != http.StatusNotFound {
			t.Fatalf("%s: без повторного выбора кэш ошибки должен держаться: %d", c.url, code)
		}
		if _, err := c.set(c.pth); err != nil { // тот же файл, правка холостая
			t.Fatal(err)
		}
		if code := get(c.url); code != http.StatusOK {
			t.Fatalf("%s после повторного выбора: %d", c.url, code)
		}
	}
}

// TestSetGuidePSBRejected —PSB, повреждённый PSD и чужой тип не принимаются ни обложкой, ни
// разворотами: ошибка без правки, папка диалога не запоминается.
func TestSetGuidePSBRejected(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	psb := testkit.PSD(t, dir, "big.psb", testkit.PSDSpec{W: 8, H: 8, Version: 2})
	asPSD := testkit.PSD(t, dir, "big.psd", testkit.PSDSpec{W: 8, H: 8, Version: 2})
	broken := filepath.Join(dir, "broken.psd")
	if err := os.WriteFile(broken, []byte("8BPS"), 0o600); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	for _, set := range []func(string) (any, error){
		func(p string) (any, error) { return a.setSpreadGuide(p) },
		func(p string) (any, error) { return a.setCoverGuide(p) },
	} {
		for _, p := range []string{psb, asPSD} {
			if _, err := set(p); !errors.Is(err, library.ErrPSB) {
				t.Errorf("%s: %v", filepath.Base(p), err)
			}
		}
		if _, err := set(broken); err == nil {
			t.Error("повреждённый PSD принят")
		}
	}
	if v := a.editor().View(); v.Rev != rev || v.SpreadMarkup != nil || v.Cover.Markup != nil {
		t.Fatalf("отказ изменил проект: rev %d → %d", rev, v.Rev)
	}
	if a.dialogDir(dirSpreadGuide) == dir || a.dialogDir(dirCoverGuide) == dir {
		t.Fatal("папка диалога запомнена при отказе")
	}
}

// TestPickSpreadGuide — диалог «Разметка типографии» с фильтром PSD/JPEG/PNG и своей папкой;
// отказ — без правки; выбранный файл — правка и запомненная папка.
func TestPickSpreadGuide(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	var (
		pick string
		opts wr.OpenDialogOptions
	)
	openFileDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) { opts = o; return pick, nil }
	rev := a.editor().View().Rev
	if v, err := a.PickSpreadGuide(); err != nil || v.Rev != rev || v.SpreadMarkup != nil {
		t.Fatalf("отказ: %v", err)
	}
	if opts.Title != "Разметка типографии" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.psd;*.jpg;*.jpeg;*.png" {
		t.Fatalf("диалог %+v", opts)
	}
	dir := testkit.TempDir(t)
	pick = testkit.PSD(t, dir, "spread.psd", testkit.PSDSpec{W: 8, H: 4})
	v, err := a.PickSpreadGuide()
	if err != nil || v.SpreadMarkup == nil || v.SpreadMarkup.Name != "spread.psd" || a.dialogDir(dirSpreadGuide) != dir || a.dialogDir(dirCoverGuide) == dir {
		t.Fatalf("выбор: %v %+v, папки %q %q", err, v.SpreadMarkup, a.dialogDir(dirSpreadGuide), a.dialogDir(dirCoverGuide))
	}
	if v = a.ClearSpreadGuide(); v.SpreadMarkup != nil {
		t.Fatal("разметка не снята")
	}
}
