//go:build linux

package fetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Безопасность загрузок (аудит 144 Г2): временный файл не открывается заново по пути — ссылка и
// FIFO, подложенные до или во время сети, не задеваются и не вешают загрузку.

const swapSentinel = "не трогать"

// sentinel — файл-цель ссылки с известным содержимым.
func sentinel(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(p, []byte(swapSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func wantSentinel(t *testing.T, p string) {
	t.Helper()
	if got, err := os.ReadFile(p); err != nil || string(got) != swapSentinel {
		t.Fatalf("цель ссылки тронута: %q, %v", got, err)
	}
}

// withTimeout — загрузка не должна висеть: FIFO без читателя держит open на запись вечно.
func withTimeout(t *testing.T, run func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		return err
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("загрузка зависла")
		return nil
	}
}

// Download по пути: ссылка и FIFO, лежавшие там до запуска, заменяются новым файлом.
func TestDownloadPathPlanted(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte("x"), 5000)
	l.serve("/a.zip", body)
	e := entryFor(l, "/a.zip", body)
	for name, plant := range map[string]func(dst, target string) error{
		"ссылка": func(dst, target string) error { return os.Symlink(target, dst) },
		"FIFO":   func(dst, _ string) error { return syscall.Mkfifo(dst, 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			target := sentinel(t, dir)
			dst := filepath.Join(dir, ".dl-ivory.tmp")
			if err := plant(dst, target); err != nil {
				t.Fatal(err)
			}
			if err := withTimeout(t, func() error { return l.client().Download(context.Background(), e, dst, nil) }); err != nil {
				t.Fatal(err)
			}
			wantSentinel(t, target)
			if got, _ := os.ReadFile(dst); !bytes.Equal(got, body) {
				t.Errorf("файл %d байт", len(got))
			}
		})
	}
}

// swapServe — сервер подменяет временный файл в dir на то, что делает plant, и только затем
// отдаёт ответ: подмена попадает ровно в окно «файл создан, сеть идёт».
func swapServe(l *testLib, path string, body []byte, dir string, plant func(tmp string) error) {
	l.mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		list, _ := filepath.Glob(filepath.Join(dir, "sw-*.tmp"))
		for _, tmp := range list {
			_ = os.Remove(tmp)
			if err := plant(tmp); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	})
}

// DownloadTemp: путь подменён ссылкой или FIFO на время сети — цель не тронута, загрузка не висит,
// отказ ErrDisk, временного файла нет.
func TestDownloadTempSwapped(t *testing.T) {
	body := bytes.Repeat([]byte("y"), 5000)
	for name, plant := range map[string]func(tmp, target string) error{
		"ссылка": func(tmp, target string) error { return os.Symlink(target, tmp) },
		"FIFO":   func(tmp, _ string) error { return syscall.Mkfifo(tmp, 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			l := newTestLib(t)
			dir := testkit.TempDir(t)
			target := sentinel(t, dir)
			swapServe(l, "/s.zip", body, dir, func(tmp string) error { return plant(tmp, target) })
			e := entryFor(l, "/s.zip", body)
			err := withTimeout(t, func() error {
				p, err := l.client().DownloadTemp(context.Background(), e, dir, "sw-*.tmp", nil)
				if err == nil {
					t.Errorf("подмена прошла, путь %q", p)
				}
				return err
			})
			if !errors.Is(err, ErrDisk) {
				t.Fatalf("ошибка: %v", err)
			}
			wantSentinel(t, target)
			if list, _ := filepath.Glob(filepath.Join(dir, "sw-*.tmp")); len(list) != 0 {
				t.Errorf("остался %v", list)
			}
		})
	}
}
