package libpacks

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// remember не берёт каталог с меньшим seq: ни память, ни файл (140 Н3). Равный seq и больший заменяют.
func TestRememberKeepsNewerSeq(t *testing.T) {
	fx := newCacheFx(t)
	if got := fx.s.remember(fx.catalog(fx.env(5, "sage"))); got.Seq != 5 {
		t.Fatalf("первый: %+v", got)
	}
	got := fx.s.remember(fx.catalog(fx.env(4, "old")))
	if got.Seq != 5 || len(got.Packs) != 1 || got.Packs[0].ID != "sage" {
		t.Errorf("итог после отката: %+v", got)
	}
	if cat, _, _ := fx.s.cached(); cat == nil || cat.Seq != 5 || cat.Packs[0].ID != "sage" {
		t.Errorf("память: %+v", cat)
	}
	if file, err := os.ReadFile(fx.path()); err != nil || !bytes.Equal(file, fx.env(5, "sage")) {
		t.Errorf("файл откатился: %v", err)
	}
	// Равный и больший — заменяют; параллельные вызовы (под -race) оставляют наибольший.
	fx.s.remember(fx.catalog(fx.env(5, "sage", "ruby")))
	if cat, _, _ := fx.s.cached(); len(cat.Packs) != 2 {
		t.Errorf("равный seq не заменил: %+v", cat)
	}
	var wg sync.WaitGroup
	for seq := int64(1); seq <= 20; seq++ {
		wg.Add(1)
		go func() { defer wg.Done(); fx.s.remember(fx.catalog(fx.env(seq, "x"))) }()
	}
	wg.Wait()
	if cat, _, _ := fx.s.cached(); cat.Seq != 20 {
		t.Errorf("после гонки seq %d, ждали 20", cat.Seq)
	}
	if again := fx.service(); func() int64 { c, _, _ := again.cached(); return c.Seq }() != 20 {
		t.Error("файл после гонки не с наибольшим seq")
	}
}

// Cached: пусто без списка, после записи — каталог (ролики сохраняются); Fresh пишет seq и список.
func TestCachedAndFresh(t *testing.T) {
	fx := newCacheFx(t)
	if _, ok := fx.s.Cached(); ok {
		t.Fatal("каталог до записи")
	}
	env := fx.env(7, "sage")
	var mu sync.Mutex
	serve := env
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(serve)
	}))
	defer srv.Close()
	src := fx.src
	src.CatalogURL, src.Prefix, src.Hosts = srv.URL+"/c.json", cachePrefix, []string{"127.0.0.1", "github.com"}
	src.Roots = srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	fx.s.h.Fetcher = func() *fetch.Client { return fetch.New(src) }
	cat, err := fx.s.Fresh(context.Background())
	if err != nil || cat.Seq != 7 || cat.Env != nil {
		t.Fatalf("Fresh: %+v, %v", cat, err)
	}
	if n := prefs.LibraryNetOf(fx.dir); n.CatalogSeq != 7 {
		t.Errorf("seq в config.json: %d", n.CatalogSeq)
	}
	if got, ok := fx.s.Cached(); !ok || got.Seq != 7 || len(got.Packs) != 1 {
		t.Errorf("Cached: %+v %v", got, ok)
	}
	if file, err := os.ReadFile(fx.path()); err != nil || !bytes.Equal(file, env) {
		t.Errorf("файл списка: %v", err)
	}
	// Откат на сервере (меньший seq) — ErrSignature, прежний каталог цел.
	mu.Lock()
	serve = fx.env(6, "old")
	mu.Unlock()
	if _, err := fx.s.Fresh(context.Background()); !errors.Is(err, fetch.ErrSignature) {
		t.Errorf("откат: %v", err)
	}
	if got, _ := fx.s.Cached(); got.Seq != 7 {
		t.Errorf("после отката: %+v", got)
	}
}

// Fresh убирает устаревшие листы образцов (gcPreviews по действующему списку); задание держит
// prevMu — уборка пропущена, лист цел.
func TestFreshCollectsPreviews(t *testing.T) {
	fx := newCacheFx(t)
	var mu sync.Mutex
	serve := fx.env(1, "sage")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(serve)
	}))
	defer srv.Close()
	src := fx.src
	src.CatalogURL, src.Prefix, src.Hosts = srv.URL+"/c.json", cachePrefix, []string{"127.0.0.1", "github.com"}
	src.Roots = srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	fx.s.h.Fetcher = func() *fetch.Client { return fetch.New(src) }
	dir := filepath.Join(fx.dir, previewDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	sheet := filepath.Join(dir, strings.Repeat("c", 64)+previewExt)
	if err := os.WriteFile(sheet, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.s.prevMu.Lock() // идёт задание образцов
	if _, err := fx.s.Fresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	fx.s.prevMu.Unlock()
	if _, err := os.Lstat(sheet); err != nil {
		t.Fatal("Fresh убрал лист при идущем задании")
	}
	mu.Lock()
	serve = fx.env(2, "sage")
	mu.Unlock()
	if _, err := fx.s.Fresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(sheet); err == nil {
		t.Error("Fresh не убрал лист, которого нет в списке")
	}
}
