//go:build linux || darwin

package library

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сайдкар-канал отклоняется сразу, а не вешает чтение (аудит 0.23-sec2).
func TestReadSidecarFIFODoesNotBlock(t *testing.T) {
	dir := testkit.TempDir(t)
	fifo := filepath.Join(dir, "a.xmp")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = w.Close()
		}
	})
	done := make(chan error, 1)
	go func() {
		_, _, err := readSidecar(sidecar{path: fifo, size: 10})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("канал принят за сайдкар")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readSidecar завис на канале")
	}
}
