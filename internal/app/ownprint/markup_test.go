package ownprint

import (
	"bytes"
	"context"
	"image/color"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// reset во время идущего построения: результат уже начатого построения отдаётся ждущему, но не
// запоминается (путь поменяли — старое превью не должно встать в кэш); следующий get строит заново
// и уже запоминает.
func TestMarkupCacheResetDuringBuild(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "m.bin")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var built atomic.Int32
	started, gate := make(chan struct{}, 4), make(chan struct{})
	old := MarkupPreview
	t.Cleanup(func() { MarkupPreview = old })
	MarkupPreview = func(context.Context, string) ([]byte, string, error) {
		n := built.Add(1)
		started <- struct{}{}
		if n == 1 {
			<-gate // первое построение держим, пока не сделан reset
		}
		return []byte{byte(n)}, "image/png", nil
	}
	var c markupCache
	got := make(chan []byte, 1)
	go func() {
		d, _, _ := c.get(context.Background(), path)
		got <- d
	}()
	<-started
	c.reset()
	close(gate)
	if d := <-got; len(d) != 1 || d[0] != 1 {
		t.Fatalf("ждущий получил %v", d)
	}
	c.mu.Lock()
	cached := c.data != nil || c.key != (markupKey{})
	c.mu.Unlock()
	if cached {
		t.Fatal("результат построения, начатого до reset, запомнен")
	}
	for range 2 { // второй get — уже из кэша, без третьего построения
		if d, _, err := c.get(context.Background(), path); err != nil || len(d) != 1 || d[0] != 2 {
			t.Fatalf("после reset: %v %v", d, err)
		}
	}
	if n := built.Load(); n != 2 {
		t.Fatalf("построений %d, ждали 2", n)
	}
}

// serve — запрос Markups.Serve; код ответа и сам ответ.
func serve(g *Markups, slot int, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	g.Serve(w, httptest.NewRequest(http.MethodGet, "/media/x", nil), slot, path)
	return w
}

// TestMarkupsServe — раздача без App: нулевой Markups готов к работе; нечитаемый путь и PSD без
// сведённого слоя — 404 без строки в журнале; малый PNG — файлом; превью PSD строится один раз на
// слот, Reset одного слота не трогает другой.
func TestMarkupsServe(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := testkit.TempDir(t)
	var g Markups // нулевое значение

	for _, p := range []string{"", filepath.Join(dir, "нет.png")} {
		if w := serve(&g, MarkupCover, p); w.Code != http.StatusNotFound {
			t.Fatalf("путь %q: %d, ждали 404", p, w.Code)
		}
	}
	png := testkit.PNG(t, dir, "a.png", 8, 4, color.NRGBA{R: 200, A: 255})
	w := serve(&g, MarkupSpread, png)
	h := w.Header()
	if w.Code != http.StatusOK || h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "no-cache" || w.Body.Len() == 0 {
		t.Fatalf("малый PNG: %d %v, %d байт", w.Code, h, w.Body.Len())
	}
	flat := testkit.PSD(t, dir, "flat.psd", testkit.PSDSpec{W: 8, H: 8, NoMerged: true})
	for range 2 {
		if w := serve(&g, MarkupCover, flat); w.Code != http.StatusNotFound {
			t.Fatalf("PSD без сведённого: %d", w.Code)
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("404 попал в журнал: %q", buf.String())
	}

	var built atomic.Int32
	old := MarkupPreview
	t.Cleanup(func() { MarkupPreview = old })
	MarkupPreview = func(context.Context, string) ([]byte, string, error) {
		return []byte{byte(built.Add(1))}, "image/png", nil
	}
	full := testkit.PSD(t, dir, "full.psd", testkit.PSDSpec{W: 8, H: 8, C: color.NRGBA{A: 255}})
	body := func(slot int) byte {
		w := serve(&g, slot, full)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || w.Body.Len() != 1 {
			t.Fatalf("слот %d: %d %v", slot, w.Code, w.Header())
		}
		return w.Body.Bytes()[0]
	}
	if a, b, a2 := body(MarkupCover), body(MarkupSpread), body(MarkupCover); a != 1 || b != 2 || a2 != 1 {
		t.Fatalf("кэш по слотам: %d %d %d", a, b, a2)
	}
	g.Reset(MarkupCover)
	if a, b := body(MarkupCover), body(MarkupSpread); a != 3 || b != 2 {
		t.Fatalf("после Reset обложки: %d %d, ждали 3 2", a, b)
	}
	if n := built.Load(); n != 3 {
		t.Fatalf("построений %d, ждали 3", n)
	}
}
