//go:build linux

package guidevideo

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// Канал на месте ролика не вешает обработчик (Lstat: не обычный файл).
func TestServeFIFO(t *testing.T) {
	l := newLab(t)
	l.ready("cover", 3000)
	p := l.path("cover", 1, "webm")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan int, 1)
	go func() { done <- l.get("GET", "cover", nil).Code }()
	select {
	case code := <-done:
		if code != 404 {
			t.Errorf("канал: %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("обработчик завис на канале")
	}
}
