package appkit

import (
	"testing"
	"time"
)

// TestWithin: результат в срок — ok; зависшая функция — ok = false без ожидания; паника — ok = false
// и нулевое значение, программа не падает.
func TestWithin(t *testing.T) {
	if v, ok := Within(time.Second, func() int { return 7 }); !ok || v != 7 {
		t.Errorf("в срок: %d %v", v, ok)
	}
	if v, ok := Within(time.Second, func() string { return "" }); !ok || v != "" {
		t.Errorf("пустое значение: %q %v", v, ok)
	}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	start := time.Now()
	if v, ok := Within(20*time.Millisecond, func() int { <-release; return 1 }); ok || v != 0 {
		t.Errorf("зависшая: %d %v", v, ok)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("зависшая функция держала вызов %v", d)
	}
	if v, ok := Within(time.Second, func() *int { panic("бум") }); ok || v != nil {
		t.Errorf("паника: %v %v", v, ok)
	}
	if _, ok := Within(0, func() int { time.Sleep(50 * time.Millisecond); return 1 }); ok {
		t.Error("нулевой предел не должен ждать")
	}
}
