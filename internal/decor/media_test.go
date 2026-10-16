package decor

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg" // декод ответа /media/lib
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// get — запрос к обработчику без разбора URL httptest (мусор в строке запроса должен дойти).
func get(h http.Handler, method, path, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, &http.Request{Method: method, URL: &url.URL{Path: path, RawQuery: query}, Header: http.Header{}})
	return rec
}

// TestMediaGen — превью узора: адрес каталога + px → PNG нужного размера, immutable, второй
// раз — из кэша; мусор, чужой путь, лишние параметры, px вне 16..4096 — 404.
func TestMediaGen(t *testing.T) {
	s, _ := openTemp(t)
	h := s.Handler(nil)
	cat, err := s.Catalog(KindBg, nil, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat.Items[:3] {
		path, query, _ := strings.Cut(it.URL, "?")
		rec := get(h, http.MethodGet, path, query+"&px=64")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
			!strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("%s: %d %v", it.URL, rec.Code, rec.Header())
		}
		img, err := png.Decode(rec.Body)
		if err != nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 32 {
			t.Fatalf("%s: %v, %v", it.URL, img.Bounds(), err)
		}
		again := get(h, http.MethodGet, path, query+"&px=64")
		if _, hit := s.media.get(path + "?" + query + "&px=64"); !hit || again.Code != http.StatusOK {
			t.Errorf("%s: нет в кэше", it.URL)
		}
	}
	ok := "k=dots&w=100&h=50&v=1&px=32"
	if rec := get(h, http.MethodHead, "/media/gen/pat", ok); rec.Code != http.StatusOK {
		t.Errorf("HEAD: %d", rec.Code)
	}
	if rec := get(h, http.MethodHead, "/media/gen/pat", "k=dots&w=1&h=1&v=1&px=100"); rec.Code != http.StatusOK {
		t.Errorf("ровно 100 px/мм: %d", rec.Code)
	}
	bad := []struct{ method, path, query string }{
		{http.MethodPost, "/media/gen/pat", ok},
		{http.MethodGet, "/media/gen/art", ok},
		{http.MethodGet, "/media/gen/pat/", ok},
		{http.MethodGet, "/media/gen/../lib/t/0123456789abcdef", ""},
		{http.MethodGet, "/media/gen/pat", "k=dots&w=100&h=50&v=1"},
		{http.MethodGet, "/media/gen/pat", "k=dots&w=100&h=50&v=1&px=8"},
		{http.MethodGet, "/media/gen/pat", "k=dots&w=100&h=50&v=1&px=4097"},
		{http.MethodGet, "/media/gen/pat", "k=dots&w=1&h=2000&v=1&px=4096"},         // выше 4096 px
		{http.MethodGet, "/media/gen/pat", "k=waves&w=0.01&h=0.01&s=4&v=1&px=4096"}, // плотнее 100 px/мм
		{http.MethodGet, "/media/gen/pat", "k=dots&w=20&h=20&v=1&px=2001"},
		{http.MethodGet, "/media/gen/pat", ok + "&path=C:/x"},
		{http.MethodGet, "/media/gen/pat", ok + "&px=32"},
		{http.MethodGet, "/media/gen/pat", "k=nope&w=100&h=50&v=1&px=32"},
		{http.MethodGet, "/media/gen/pat", "k=dots&w=NaN&h=50&v=1&px=32"},
		{http.MethodGet, "/media/gen/pat", "%zz"},
		{http.MethodGet, "/media/other", ok},
	}
	for _, c := range bad {
		if rec := get(h, c.method, c.path, c.query); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s?%s: %d", c.method, c.path, c.query, rec.Code)
		}
	}
}

// TestMediaGenPanicFreesSlot — паника генератора освобождает слот: после стольких паник,
// сколько слотов, обычный запрос — 200, а не вечное ожидание; отменённый запрос не ждёт слот.
func TestMediaGenPanicFreesSlot(t *testing.T) {
	s, _ := openTemp(t)
	h := s.Handler(nil)
	orig := drawGen
	t.Cleanup(func() { drawGen = orig })
	drawGen = func(gen.Request) image.Image { panic("сбой генератора") }
	for i := range cap(genSem) {
		if rec := get(h, http.MethodGet, "/media/gen/pat", "k=dots&w=100&h=50&v=1&px="+strconv.Itoa(16+i)); rec.Code != http.StatusNotFound {
			t.Fatalf("паника %d: %d", i, rec.Code)
		}
	}
	drawGen = orig
	done := make(chan int, 1)
	go func() { done <- get(h, http.MethodGet, "/media/gen/pat", "k=dots&w=100&h=50&v=1&px=48").Code }()
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("после паник: %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("после паник /media/gen висит: слот не освобождён")
	}

	// Все слоты заняты, клиент ушёл — ответа нет, обработчик возвращается.
	for range cap(genSem) {
		genSem <- struct{}{}
	}
	t.Cleanup(func() {
		for range cap(genSem) {
			<-genSem
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := (&http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/media/gen/pat", RawQuery: "k=dots&w=100&h=50&v=1&px=40"}, Header: http.Header{}}).WithContext(ctx)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		done <- rec.Code
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("отменённый запрос ждёт слот")
	}
}

// TestMediaLib — своя плитка и превью по ключу из реестра (в том числе скрытого); ключ не по
// грамматике, не из реестра, файл без записи, лишние части и параметры, без кэша — 404.
func TestMediaLib(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, cache.Close)
	h := s.Handler(cache)
	a := importOK(t, s, "mine", testkit.JPEG(t, src, "a.jpg", 800, 600), testkit.JPEG(t, src, "b.jpg", 30, 20))
	if err := s.Hide(KindBg, a[1].ID); err != nil {
		t.Fatal(err)
	}
	for _, it := range a {
		for _, preview := range []bool{false, true} {
			path, query, _ := strings.Cut(LibURL(it.ID, preview), "?")
			rec := get(h, http.MethodGet, path, query+"&px=256")
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
				t.Fatalf("%s: %d", path, rec.Code)
			}
			img, _, err := image.Decode(rec.Body)
			if err != nil || (!preview && it.W == 800 && img.Bounds().Dx() != library.ThumbSize) {
				t.Fatalf("%s: %v, %v", path, img.Bounds(), err)
			}
		}
	}
	hexA := strings.TrimPrefix(a[0].ID, "my:")
	stray := filepath.Join(cfg, RootName, "Фоны", "0123456789abcdef.jpg")
	if err := os.WriteFile(stray, mustRead(t, testkit.JPEG(t, src, "s.jpg", 20, 20)), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := Open(cfg)
	h2 := s2.Handler(cache)
	bad := []struct{ path, query string }{
		{"/media/lib/t/0123456789abcdef", ""}, // файл без записи
		{"/media/lib/t/ffffffffffffffff", ""},
		{"/media/lib/t/" + strings.ToUpper(hexA), ""},
		{"/media/lib/t/" + hexA + ".jpg", ""},
		{"/media/lib/x/" + hexA, ""},
		{"/media/lib/t/" + hexA + "/x", ""},
		{"/media/lib/t/../../" + hexA, ""},
		{"/media/lib/" + hexA, ""},
		{"/media/lib/t/" + hexA, "path=C:/Windows/win.ini"},
		{"/media/lib/t/" + hexA, "px=1"},
		{"/media/lib/t/" + hexA, "v=1&v=2"},
		{"/media/lib/t/" + hexA, "%zz"},
	}
	for _, c := range bad {
		if rec := get(h2, http.MethodGet, c.path, c.query); rec.Code != http.StatusNotFound {
			t.Errorf("%s?%s: %d", c.path, c.query, rec.Code)
		}
	}
	if rec := get(h2, http.MethodGet, "/media/lib/t/"+hexA, "v=1"); rec.Code != http.StatusOK {
		t.Errorf("после открытия: %d", rec.Code)
	}
	if rec := get(s2.Handler(nil), http.MethodGet, "/media/lib/t/"+hexA, ""); rec.Code != http.StatusNotFound {
		t.Errorf("без кэша: %d", rec.Code)
	}
}

// TestMediaCacheLRU — предел по байтам: старое вытесняется, огромное не кладётся.
func TestMediaCacheLRU(t *testing.T) {
	c := newMediaCache(100)
	c.put("a", make([]byte, 20))
	c.put("b", make([]byte, 20))
	c.get("a") // a свежее b
	for _, k := range []string{"c", "d", "e"} {
		c.put(k, make([]byte, 20))
	}
	c.put("f", make([]byte, 20)) // 120 > 100: уходит b
	if _, ok := c.get("b"); ok {
		t.Error("b не вытеснен")
	}
	if _, ok := c.get("a"); !ok {
		t.Error("a вытеснен, хотя свежее")
	}
	c.put("big", make([]byte, 30))
	if _, ok := c.get("big"); ok || c.size > c.limit {
		t.Errorf("огромное в кэше, размер %d", c.size)
	}
}

// FuzzMediaGenQuery — любая строка запроса: без паники, 200 или 404; 200 — PNG не больше 4096².
func FuzzMediaGenQuery(f *testing.F) {
	for _, q := range []string{
		"k=dots&w=100&h=50&v=1&px=32", "k=confetti&i=ff0000,00ff00&s=2&r=7&w=10&h=10&v=1&px=16",
		"k=dots&w=1e308&h=1e-308&v=1&px=4096", "k=dots&i=zz&w=1&h=1&v=1&px=16", "", "%", "k=&&&",
	} {
		f.Add(false, q)
	}
	f.Add(true, "k=dots&v=1&px=16")
	for _, q := range []string{ // стикеры (Ф2): растяжимые в крайних рамках, случайный, цвета
		"k=wave-band&a=500&v=1&px=4096", "k=plate&a=0.002&i=ff0000&v=1&px=4096", "k=wave-band&a=1e6&v=1&px=64",
		"k=confetti-pile&r=3&i=ff0000,00ff00,0000ff,ffff00&v=1&px=256", "k=star&a=3&s=2&v=1&px=16",
	} {
		f.Add(true, q)
	}
	s := Open(f.TempDir())
	h := s.Handler(nil)
	f.Fuzz(func(t *testing.T, art bool, q string) {
		path := "/media/gen/pat"
		if art {
			path = "/media/gen/art"
		}
		rec := get(h, http.MethodGet, path, q)
		switch rec.Code {
		case http.StatusNotFound:
		case http.StatusOK:
			cfg, err := png.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
			if err != nil || cfg.Width > 4096 || cfg.Height > 4096 {
				t.Fatalf("%q: %+v, %v", q, cfg, err)
			}
		default:
			t.Fatalf("%q: код %d", q, rec.Code)
		}
	})
}
