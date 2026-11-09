//go:build !windows

package salbum

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Устройство и канал — не проект: отказ без открытия (Open канала без писателя завис бы).
func TestFileUIDNotRegular(t *testing.T) {
	fifo := filepath.Join(testkit.TempDir(t), "pipe.salbum")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	for name, path := range map[string]string{"устройство": "/dev/null", "канал": fifo} {
		t.Run(name, func(t *testing.T) {
			done := make(chan string, 1)
			go func() { done <- FileUID(path) }()
			select {
			case got := <-done:
				if got != "" {
					t.Errorf("FileUID(%s) = %q, ждали пусто", path, got)
				}
			case <-time.After(testkit.Limit(5 * time.Second)):
				t.Fatalf("FileUID(%s) завис", path)
			}
		})
	}
}
