package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/guidevideo"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// guideCacheEnv уводит кэш пользователя во временную папку: тест не пишет в настоящий.
func guideCacheEnv(t *testing.T) string {
	t.Helper()
	dir := testkit.TempDir(t)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("HOME", dir)
	return dir
}

// Адрес /media/guide-video/ ведёт в службу роликов (а не в разбор миниатюр): метод — 405,
// неизвестный ролик — 404, остальной /media не затронут.
func TestGuideVideoMediaRoute(t *testing.T) {
	guideCacheEnv(t)
	a := newApp(t)
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("запрос ушёл мимо /media") })
	h := a.media(next)
	for method, want := range map[string]int{"GET": 404, "HEAD": 404, "POST": 405, "PUT": 405} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/media/guide-video/cover", nil))
		if w.Code != want {
			t.Errorf("%s: код %d, ждали %d", method, w.Code, want)
		}
	}
	if list := a.GuideVideos(); list == nil || len(list) != 0 {
		t.Errorf("GuideVideos без каталога: %#v", list)
	}
	if _, err := a.DownloadGuideVideos([]string{"cover"}); !errors.Is(err, guidevideo.ErrNoConsent) {
		t.Errorf("без согласия: %v", err)
	}
	if err := a.RevealGuideVideo("cover"); !errors.Is(err, guidevideo.ErrNoFile) {
		t.Errorf("Reveal: %v", err)
	}
	a.CancelGuideVideos() // без загрузки — тихо
}

// Папка кэша роликов: абсолютная, у стенда своя; без кэша пользователя — "" (служба выключена).
// Кэш пользователя задают разные переменные: linux — XDG_CACHE_HOME (или $HOME/.cache), windows —
// LOCALAPPDATA, darwin — $HOME/Library/Caches; прочие ОС тест не проверяет.
func TestGuideCacheDir(t *testing.T) {
	base := guideCacheEnv(t)
	var env, root string // переменная кэша и каталог, куда она ведёт кэш
	switch runtime.GOOS {
	case "linux":
		env, root = "XDG_CACHE_HOME", base
	case "windows":
		env, root = "LOCALAPPDATA", base
	case "darwin":
		env, root = "HOME", filepath.Join(base, "Library", "Caches")
	default:
		t.Skip("кэш пользователя на " + runtime.GOOS + " не проверяется")
	}
	t.Setenv(env, base)
	if got, want := guideCacheDir("SosninaAlbums"), filepath.Join(root, "SosninaAlbums", "guide-videos"); got != want {
		t.Errorf("основная: %q, ждали %q", got, want)
	}
	if a, b := guideCacheDir("SosninaAlbums"), guideCacheDir("SosninaAlbums-b"); a == b {
		t.Error("у стенда та же папка")
	}
	// Без кэша пользователя: пустая переменная (linux — и HOME) — не путь от текущей папки.
	t.Setenv(env, "")
	t.Setenv("HOME", "")
	if got := guideCacheDir("SosninaAlbums"); got != "" {
		t.Errorf("без кэша пользователя: %q (путь от текущей папки)", got)
	}
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CACHE_HOME", "relative/cache")
		if got := guideCacheDir("SosninaAlbums"); got != "" {
			t.Errorf("относительный кэш: %q", got)
		}
	}
}

// Отзыв согласия через SetLibraryNet(false) отменяет идущую загрузку роликов.
func TestSetLibraryNetCancelsGuideVideo(t *testing.T) {
	guideCacheEnv(t)
	a := newApp(t)
	if _, err := a.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	h := a.guideHost()
	h.CacheDir = func() string { return filepath.Join(testkit.TempDir(t), "guide-videos") }
	h.Fresh = func(ctx context.Context) (fetch.Catalog, error) {
		close(started)
		<-ctx.Done()
		return fetch.Catalog{}, fetch.ErrCancelled
	}
	a.guidev = guidevideo.New(h)
	res := make(chan error, 1)
	go func() { _, err := a.DownloadGuideVideos([]string{"cover"}); res <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("загрузка не началась")
	}
	if _, err := a.SetLibraryNet(false, false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-res:
		if !errors.Is(err, guidevideo.ErrCancelled) {
			t.Errorf("ошибка: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("загрузка не отменилась")
	}
}
