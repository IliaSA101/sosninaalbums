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

// Канал (FIFO) по пути фото отклоняется сразу, а не вешает вызов в ожидании писателя
// (аудит 0.23-sec): так открывала бы папка с подложенным каналом.
func TestOpenRegularFIFODoesNotBlock(t *testing.T) {
	dir := testkit.TempDir(t)
	fifo := filepath.Join(dir, "a.jpg")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	// Если вызов завис — разблокировать его, открыв канал на запись, чтобы горутина не осталась.
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = w.Close()
		}
	})
	calls := map[string]func() error{
		"OpenRegular": func() error {
			f, err := OpenRegular(fifo)
			if err == nil {
				_ = f.Close()
			}
			return err
		},
		"CheckSize": func() error { return CheckSize(fifo) },
		"FileDims": func() error {
			_, err := FileDims(fifo, MaxPixels)
			return err
		},
		"OpenOriented": func() error {
			img, release, err := OpenOriented(fifo)
			if err == nil && img != nil {
				release()
			}
			return err
		},
	}
	for name, call := range calls {
		done := make(chan error, 1)
		go func() { done <- call() }()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("%s: канал принят за обычный файл", name)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: вызов завис на канале", name)
		}
	}
}
