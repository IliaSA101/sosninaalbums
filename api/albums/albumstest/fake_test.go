package albumstest

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

const otherTok = "other-token-0123456789"

// TestFakeConformance: поддельный сервер проходит набор проверок контракта (в памяти и на диске).
func TestFakeConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("полный набор проверок: не в быстром уровне")
	}
	t.Run("память", func(t *testing.T) {
		srv := New(Options{OtherToken: otherTok})
		defer srv.Close()
		Conformance(t, srv.URL, srv.Token, WithOtherToken(srv.OtherToken))
		if n := srv.Fake.Albums(); n != 0 {
			t.Errorf("после набора на сервере осталось %d альбомов", n)
		}
	})
	t.Run("диск", func(t *testing.T) {
		dir := t.TempDir()
		srv := New(Options{OtherToken: otherTok, Dir: dir})
		defer srv.Close()
		Conformance(t, srv.URL, srv.Token, WithOtherToken(srv.OtherToken))
		if left, _ := os.ReadDir(filepath.Join(dir, "albums-fake")); len(left) != 0 {
			t.Errorf("после набора на диске остались папки альбомов: %d", len(left))
		}
	})
}

func TestFakeOptions(t *testing.T) {
	for name, opt := range map[string]Options{
		"токен короткий": {Token: "short"}, "токен с пробелом": {Token: "abc def abc def abc def"},
		"Dir относительный": {Dir: "rel/dir"}, "Dir точка": {Dir: "."}, "Dir нет": {Dir: filepath.Join(t.TempDir(), "нет")},
		"Dir файл":       {Dir: writeFile(t)},
		"Other = Token":  {Token: otherTok, OtherToken: otherTok},
		"Other короткий": {OtherToken: "x"},
	} {
		if _, err := NewFake(opt); err == nil {
			t.Errorf("%s: принято", name)
		}
	}
	f, err := NewFake(Options{})
	if err != nil || f.opt.Token == "" || albums.CheckToken(f.opt.Token) != nil {
		t.Errorf("токен по умолчанию: %v", err)
	}
	if f.RootDir() != "" {
		t.Error("в памяти RootDir должен быть пуст")
	}
}

func writeFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestFakeDiskLayout: файлы страниц лежат только в <dir>/albums-fake/<id>/<sha256>.jpg и уходят с альбомом.
func TestFakeDiskLayout(t *testing.T) {
	dir := t.TempDir()
	srv := New(Options{Dir: dir})
	defer srv.Close()
	c, err := albums.New(srv.URL, srv.Token, albums.Options{})
	if err != nil {
		t.Fatal(err)
	}
	bk := newBook(60)
	cr, err := c.Create(context.Background(), &albums.CreateRequest{Manifest: bk.m, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range cr.Missing {
		if err := c.PutPage(context.Background(), cr.ID, n, albums.TypeJPEG, bytes.NewReader(bk.imgs[n]), int64(len(bk.imgs[n]))); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range cr.Missing {
		p := filepath.Join(dir, "albums-fake", cr.ID, bk.m.Pages[n-1].SHA256+".jpg")
		st, err := os.Stat(p)
		if err != nil {
			t.Errorf("файл страницы %d: %v", n, err)
		} else if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 { // на Windows права POSIX не бывают: 0o666
			t.Errorf("права файла страницы %d: %v, нужно 0600", n, st.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "albums-fake" {
		t.Errorf("сервер написал за пределами albums-fake: %v", entries)
	}
	if got := srv.Fake.RootDir(); got != filepath.Join(dir, "albums-fake") {
		t.Errorf("RootDir = %q", got)
	}
	if err := c.Delete(context.Background(), cr.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "albums-fake", cr.ID)); !os.IsNotExist(err) {
		t.Errorf("папка альбома после DELETE: %v", err)
	}
}

// TestFakePathSafety: id из пути — только по таблице; обход пути не удаляет и не читает чужое.
func TestFakePathSafety(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	if err := os.MkdirAll(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := New(Options{Dir: dir})
	defer srv.Close()
	for _, id := range []string{"..%2f..%2fvictim", "%2e%2e%2fvictim", "..", "victim", strings.Repeat("a", 22), "a/../../victim"} {
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", srv.URL+"/v1/albums/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+srv.Token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode >= 500 || res.StatusCode == 204 {
			t.Errorf("DELETE %q: %d", id, res.StatusCode)
		}
	}
	if _, err := os.Stat(filepath.Join(victim, "keep.txt")); err != nil {
		t.Errorf("чужой файл тронут: %v", err)
	}
	// страницы клиента с обходом тоже закрыты
	for _, p := range []string{"/a/..%2fvictim/manifest.json", "/a/" + strings.Repeat("a", 22) + "/..%2f..%2fvictim%2fkeep.txt", "/a/x/keep.txt"} {
		res, err := http.DefaultClient.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode == 200 {
			t.Errorf("GET %s: 200", p)
		}
	}
}

// TestFakeFaults: крючки отказа — для тестов повторов и обрывов службы выгрузки.
func TestFakeFaults(t *testing.T) {
	var puts atomic.Int32
	mode := atomic.Int32{}
	srv := New(Options{Fault: func(r *http.Request) *Fault {
		if r.Method != "PUT" || !strings.Contains(r.URL.Path, "/pages/") {
			return nil
		}
		switch mode.Load() {
		case 1:
			return &Fault{Status: 429, RetryAfter: 3}
		case 2:
			return &Fault{Status: 503}
		case 3:
			if puts.Add(1) > 1 {
				return &Fault{Drop: true}
			}
		}
		return nil
	}})
	defer srv.Close()
	c, err := albums.New(srv.URL, srv.Token, albums.Options{})
	if err != nil {
		t.Fatal(err)
	}
	bk := newBook(70)
	ctx := context.Background()
	cr, err := c.Create(ctx, &albums.CreateRequest{Manifest: bk.m, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	put := func(n int) error {
		return c.PutPage(ctx, cr.ID, n, albums.TypeJPEG, bytes.NewReader(bk.imgs[n]), int64(len(bk.imgs[n])))
	}
	mode.Store(1)
	var ae *albums.APIError
	if err := put(1); !errors.As(err, &ae) || ae.Status != 429 || ae.RetryAfter != 3*time.Second || !errors.Is(err, albums.ErrBusy) {
		t.Errorf("429: %v", err)
	}
	mode.Store(2)
	if err := put(1); !errors.As(err, &ae) || ae.Status != 503 {
		t.Errorf("503: %v", err)
	}
	mode.Store(3) // первая страница проходит, на второй соединение обрывается
	if err := put(1); err != nil {
		t.Errorf("первая страница: %v", err)
	}
	var ne *albums.NetError
	if err := put(3); !errors.As(err, &ne) {
		t.Errorf("обрыв: %v", err)
	}
	mode.Store(0)
	if err := put(3); err != nil {
		t.Errorf("после сбоя повтор: %v", err)
	}
	if srv.Fake.Puts() != 2 || len(srv.Fake.Stored(cr.ID)) != 2 {
		t.Errorf("принято страниц %d, на сервере %d", srv.Fake.Puts(), len(srv.Fake.Stored(cr.ID)))
	}
	// health тоже проходит через крючок, но без Fault — работает
	if _, err := c.Health(ctx); err != nil {
		t.Error(err)
	}
}

// TestFakeLimits: свои пределы и зажим срока к max_days.
func TestFakeLimits(t *testing.T) {
	srv := New(Options{Limits: albums.Limits{Pages: 4, PageBytes: 2000, AlbumBytes: 3000, MaxDays: 2}})
	defer srv.Close()
	c, err := albums.New(srv.URL, srv.Token, albums.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	h, err := c.Health(ctx)
	if err != nil || h.Limits.Pages != 4 || h.Limits.MaxDays != 2 || h.Limits.TitleLen != albums.MaxTitle {
		t.Fatalf("health: %+v %v", h, err)
	}
	bk := newBook(80)
	cr, err := c.Create(ctx, &albums.CreateRequest{Manifest: bk.m, ExpiresAt: time.Now().Add(100 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(cr.ExpiresAt); d > 49*time.Hour || d < 47*time.Hour {
		t.Errorf("срок не зажат к 2 дням: %v", d)
	}
	// больше страниц, чем разрешено: клиент знает пределы после Health и отказывает до сети
	big := newBook(81)
	big.m.Pages = append(big.m.Pages, albums.Page{N: 5, Role: albums.RolePage, Blank: true}, albums.Page{N: 6, Role: albums.RolePage, Blank: true})
	if _, err := c.Create(ctx, &albums.CreateRequest{Manifest: big.m, ExpiresAt: time.Now().Add(time.Hour)}); err == nil {
		t.Error("6 страниц при пределе 4")
	}
	// тот же запрос в обход клиента сервер тоже отвергает
	r := (&conf{t: t, base: srv.URL, token: srv.Token, http: http.DefaultClient})
	if res := r.json(t, "POST", "/v1/albums", srv.Token, albums.CreateRequest{Manifest: big.m, ExpiresAt: time.Now().Add(time.Hour)}); res.status != 400 {
		t.Errorf("сервер принял 6 страниц: %d", res.status)
	}
}
