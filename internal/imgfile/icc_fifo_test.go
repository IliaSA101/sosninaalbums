//go:build linux || darwin

package imgfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ReadColor на канале по пути фото не виснет в ожидании писателя (аудит 0.23-sec2):
// файл могли подменить каналом между проверкой и чтением.
func TestReadColorFIFODoesNotBlock(t *testing.T) {
	dir := testkit.TempDir(t)
	fifo := filepath.Join(dir, "a.jpg")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = w.Close()
		}
	})
	done := make(chan Color, 1)
	go func() {
		c, _ := ReadColor(fifo)
		done <- c
	}()
	select {
	case c := <-done:
		if c != ColorNone {
			t.Errorf("канал дал профиль %q", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadColor завис на канале")
	}
}
