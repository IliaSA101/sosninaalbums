//go:build linux || darwin

package export

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// IsOurs на канале в папке экспорта не виснет (аудит 0.23-sec2).
func TestIsOursFIFODoesNotBlock(t *testing.T) {
	dir := testkit.TempDir(t)
	fifo := filepath.Join(dir, "01.jpg")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = w.Close()
		}
	})
	done := make(chan bool, 1)
	go func() { done <- IsOurs(fifo) }()
	select {
	case ok := <-done:
		if ok {
			t.Error("канал принят за наш файл")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("IsOurs завис на канале")
	}
}
