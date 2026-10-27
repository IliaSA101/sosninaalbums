//go:build !windows

package app

import (
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// FIFO и ссылка на FIFO — 404 сразу: os.Open канала без писателя повесил бы обработчик навсегда,
// library.OpenLocal его не открывает.
func TestMediaOriginalFIFO404(t *testing.T) {
	a, dir := origApp(t)
	fifo := filepath.Join(dir, "канал.jpg")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	link := filepath.Join(dir, "ссылка.jpg")
	if err := os.Symlink(fifo, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	captureLog(t)
	for _, path := range []string{fifo, link} {
		id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
		done := make(chan int, 1)
		go func() { done <- origReq(a, http.MethodGet, "/media/o/"+id).Code }()
		select {
		case code := <-done:
			if code != http.StatusNotFound {
				t.Errorf("%s: %d, ждали 404", filepath.Base(path), code)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s: обработчик завис на открытии канала", filepath.Base(path))
		}
	}
}
