//go:build linux

package guidevideo

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Безопасность загрузок (аудит 144 Г2): временный файл ролика не открывается заново по пути —
// ссылка или FIFO на его месте не задевают цель и не вешают загрузку.
func TestDownloadSwappedTemp(t *testing.T) {
	for name, plant := range map[string]func(path, target string) error{
		"ссылка": func(path, target string) error { return os.Symlink(target, path) },
		"FIFO":   func(path, _ string) error { return syscall.Mkfifo(path, 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			l := newLab(t)
			body := l.webm("cover", 1, 3000)
			target := filepath.Join(testkit.TempDir(t), "target.txt")
			if err := os.WriteFile(target, []byte("не трогать"), 0o600); err != nil {
				t.Fatal(err)
			}
			inner := l.mux
			l.mux = http.NewServeMux()
			l.mux.HandleFunc("/main/catalog.json", inner.ServeHTTP)
			l.mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
				list, _ := filepath.Glob(filepath.Join(l.dir, "gv-*.tmp"))
				for _, tmp := range list {
					_ = os.Remove(tmp)
					if err := plant(tmp, target); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
				_, _ = w.Write(body)
			})
			done := make(chan error, 1)
			go func() { _, err := l.svc.Download([]string{"cover"}); done <- err }()
			select {
			case err := <-done:
				if !errors.Is(err, ErrVideoDisk) {
					t.Fatalf("ошибка: %v", err)
				}
			case <-time.After(testkit.Limit(10 * time.Second)):
				t.Fatal("загрузка зависла")
			}
			if got, err := os.ReadFile(target); err != nil || string(got) != "не трогать" {
				t.Fatalf("цель ссылки тронута: %q, %v", got, err)
			}
			if got := l.listDir(); len(got) != 0 {
				t.Errorf("в папке %v", got)
			}
		})
	}
}
