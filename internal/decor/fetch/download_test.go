package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// entryFor — строка каталога для архива body по пути сервера.
func entryFor(l *testLib, path string, body []byte) Entry {
	sum := sha256.Sum256(body)
	return Entry{ID: "ivory", Version: 1, Title: "Айвори", Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), URL: l.srv.URL + path}
}

func dlPath(t *testing.T) string { return filepath.Join(testkit.TempDir(t), ".dl-ivory.tmp") }

func assertGone(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("временный файл остался: %v", err)
	}
}

// Скачанное сходится — файл байт в байт; хэш не тот — ошибка с названием набора, файла нет.
func TestDownloadHash(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte("набор "), 50_000)
	l.serve("/a.zip", body)
	e := entryFor(l, "/a.zip", body)
	dst := dlPath(t)
	var calls int
	var last int64
	if err := l.client().Download(context.Background(), e, dst, func(n int64) { calls++; last = n }); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, body) || calls == 0 || last != e.Size {
		t.Fatalf("файл %d байт, ход %d/%d", len(got), calls, last)
	}
	// Повтор поверх прежнего файла — тот же итог (файл создаётся заново).
	if err := l.client().Download(context.Background(), e, dst, nil); err != nil {
		t.Fatal(err)
	}
	bad := e
	bad.SHA256 = hex.EncodeToString(make([]byte, 32))
	err := l.client().Download(context.Background(), bad, dst, nil)
	if !errors.Is(err, ErrBroken) || err.Error() != "набор «Айвори» скачался с ошибкой — повторите" {
		t.Fatalf("хэш: %v", err)
	}
	assertGone(t, dst)
	// Строка без годного sha256 или размера — без запроса.
	hits := l.hits.Load()
	for _, e2 := range []Entry{{Title: "x", Size: 0, SHA256: e.SHA256, URL: e.URL}, {Title: "x", Size: 1, SHA256: "zz", URL: e.URL}} {
		if err := l.client().Download(context.Background(), e2, dst, nil); !errors.Is(err, ErrBroken) {
			t.Errorf("строка %+v: %v", e2, err)
		}
	}
	if l.hits.Load() != hits {
		t.Error("запрос по негодной строке")
	}
	// 404 — «не найдена», каталог мимо белого списка — отказ.
	if err := l.client().Download(context.Background(), entryFor(l, "/none.zip", body), dst, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	assertGone(t, dst)
}

// Пределы тела: Content-Length не равен размеру — отказ до чтения; тело без длины длиннее
// размера — обрыв на лишнем байте; короче — ошибка; сервер оборвал тело — ошибка.
func TestDownloadLimit(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte{7}, 200_000)
	var read int
	l.mux.HandleFunc("/len.zip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)+1))
		_, _ = w.Write(append(body, 1))
	})
	l.mux.HandleFunc("/chunked.zip", func(w http.ResponseWriter, _ *http.Request) {
		for range 3 { // втрое больше размера, без Content-Length
			n, _ := w.Write(body)
			read += n
			w.(http.Flusher).Flush()
		}
	})
	l.mux.HandleFunc("/short.zip", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body[:100])
		w.(http.Flusher).Flush()
	})
	l.mux.HandleFunc("/lie.zip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body[:1000]) // заголовок врёт: тела меньше, соединение рвётся
	})
	dst := dlPath(t)
	e := entryFor(l, "/len.zip", body)
	var got int64
	if err := l.client().Download(context.Background(), e, dst, func(n int64) { got = n }); !errors.Is(err, ErrBroken) || got != 0 {
		t.Fatalf("длина: %v, прочитано %d", err, got)
	}
	assertGone(t, dst)
	for _, path := range []string{"/chunked.zip", "/short.zip", "/lie.zip"} {
		e.URL = l.srv.URL + path
		got = 0
		err := l.client().Download(context.Background(), e, dst, func(n int64) { got = n })
		if !errors.Is(err, ErrBroken) || got > e.Size {
			t.Errorf("%s: %v, прочитано %d", path, err, got)
		}
		assertGone(t, dst)
	}
}

// Застой: сервер отдал часть и молчит — обрыв по таймеру застоя, а не ожидание вечно.
func TestDownloadStall(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte{1}, 100_000)
	release := make(chan struct{})
	defer close(release)
	l.mux.HandleFunc("/stall.zip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body[:10_000])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	c := l.client()
	c.stall = 150 * time.Millisecond
	dst := dlPath(t)
	start := time.Now()
	err := c.Download(context.Background(), entryFor(l, "/stall.zip", body), dst, nil)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("застой: %v", err)
	}
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Errorf("застой ждал %v", d)
	}
	assertGone(t, dst)
	// Предел на набор целиком — не застой, а своя ошибка: ход «Скачать все» рвётся сразу (137 Д3).
	c.stall, c.limit, c.rate = time.Minute, 150*time.Millisecond, 0 // rate 0: предел только полом
	err = c.Download(context.Background(), entryFor(l, "/stall.zip", body), dst, nil)
	if !errors.Is(err, ErrSlow) || errors.Is(err, ErrTimeout) || !strings.Contains(err.Error(), "«") {
		t.Fatalf("предел набора: %v", err)
	}
	assertGone(t, dst)
}

// Отмена посреди скачивания — ErrCancelled, файла нет.
func TestDownloadCancel(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte{2}, 2<<20)
	release := make(chan struct{})
	defer close(release)
	l.mux.HandleFunc("/big.zip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body[:100_000])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dst := dlPath(t)
	err := l.client().Download(ctx, entryFor(l, "/big.zip", body), dst, func(int64) { cancel() })
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("отмена: %v", err)
	}
	assertGone(t, dst)
	// Папки нет — ошибка диска без пути.
	l.serve("/x.zip", []byte("xyz"))
	err = l.client().Download(context.Background(), entryFor(l, "/x.zip", []byte("xyz")), filepath.Join(dst, "нет", "x.tmp"), nil)
	if !errors.Is(err, ErrDisk) || err.Error() != ErrDisk.Error() {
		t.Fatalf("диск: %v", err)
	}
}

// DownloadTemp без подмены: файл целый, права 0600; ошибка загрузки или папки — файла нет.
func TestDownloadTempOK(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte("z"), 70_000)
	l.serve("/ok.zip", body)
	dir := testkit.TempDir(t)
	p, err := l.client().DownloadTemp(context.Background(), entryFor(l, "/ok.zip", body), dir, "ok-*.tmp", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, body) {
		t.Fatalf("файл %d байт", len(got))
	}
	// На Windows Go всегда отдаёт 0666 для файла без атрибута «только чтение»: биты прав там ничего не значат.
	if st, _ := os.Stat(p); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("права %v", st.Mode())
	}
	bad := entryFor(l, "/ok.zip", body)
	bad.SHA256 = "00" + bad.SHA256[2:]
	if p2, err := l.client().DownloadTemp(context.Background(), bad, dir, "ok-*.tmp", nil); !errors.Is(err, ErrBroken) || p2 != "" {
		t.Fatalf("хэш: %q %v", p2, err)
	}
	if list, _ := filepath.Glob(filepath.Join(dir, "ok-*.tmp")); len(list) != 1 || list[0] != p {
		t.Errorf("в папке %v", list)
	}
	if _, err := l.client().DownloadTemp(context.Background(), entryFor(l, "/ok.zip", body), filepath.Join(dir, "нет"), "ok-*.tmp", nil); !errors.Is(err, ErrDisk) {
		t.Errorf("нет папки: %v", err)
	}
}

// DownloadTo пишет в открытый файл и оставляет его открытым и на месте — и при успехе, и при ошибке.
func TestDownloadToKeepsFile(t *testing.T) {
	l := newTestLib(t)
	body := []byte("0123456789")
	l.serve("/t.zip", body)
	f, err := os.CreateTemp(testkit.TempDir(t), "to-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // тест
	if err := l.client().DownloadTo(context.Background(), entryFor(l, "/t.zip", body), f, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(nil); err != nil { // дескриптор жив
		t.Errorf("файл закрыт: %v", err)
	}
	if got, _ := os.ReadFile(f.Name()); !bytes.Equal(got, body) {
		t.Errorf("записано %q", got)
	}
	bad := entryFor(l, "/t.zip", body)
	bad.SHA256 = "00" + bad.SHA256[2:]
	if err := l.client().DownloadTo(context.Background(), bad, f, nil); !errors.Is(err, ErrBroken) {
		t.Fatalf("хэш: %v", err)
	}
	if _, err := os.Stat(f.Name()); err != nil {
		t.Errorf("файл удалён: %v", err)
	}
}

// На месте dst лежит обычный файл с мусором: Download его заменяет, внутри только новое тело
// (портативный случай остатка прежней загрузки; ссылка и FIFO — в download_swap_linux_test.go).
func TestDownloadReplacesJunk(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte("n"), 3000)
	l.serve("/j.zip", body)
	dst := dlPath(t)
	if err := os.WriteFile(dst, bytes.Repeat([]byte("МУСОР"), 5000), 0o600); err != nil { // мусор длиннее тела
		t.Fatal(err)
	}
	if err := l.client().Download(context.Background(), entryFor(l, "/j.zip", body), dst, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, body) {
		t.Fatalf("в файле %d байт, хвост мусора не убран", len(got))
	}
}
