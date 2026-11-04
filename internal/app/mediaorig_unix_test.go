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
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ссылка с нелокальной целью ("//"+абсолютный путь — на Windows это был бы UNC) — 404, тело не
// отдаётся: OpenLocal отказывает, OpenRegular по такой ссылке отдал бы файл (200). Ссылка на
// обычную локальную цель — 200, чтобы тест не проходил из-за общего отказа на ссылках.
func TestMediaOriginalNonLocalLink404(t *testing.T) {
	a, dir := origApp(t)
	orig := testkit.JPEG(t, dir, "настоящее.jpg", 60, 40)
	bad := filepath.Join(dir, "нелокальная.jpg")
	if err := os.Symlink("/"+orig, bad); err != nil {
		t.Skipf("symlink: %v", err)
	}
	good := filepath.Join(dir, "локальная.jpg")
	if err := os.Symlink(orig, good); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := os.Stat(bad); err != nil {
		t.Skipf("ОС не разрешает путь с «//»: %v", err)
	}
	captureLog(t)
	idBad := origPhoto(a, bad, 60, 40, imgfile.ColorNone)
	idGood := origPhoto(a, good, 60, 40, imgfile.ColorNone)
	if code := origReq(a, http.MethodGet, "/media/o/"+idBad).Code; code != http.StatusNotFound {
		t.Errorf("нелокальная цель: %d, ждали 404", code)
	}
	if code := origReq(a, http.MethodGet, "/media/o/"+idGood).Code; code != http.StatusOK {
		t.Errorf("локальная цель: %d, ждали 200", code)
	}
}

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
