package testkit

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// blockedRE — горутина blockedHere уже стоит на приёме из канала.
var blockedRE = regexp.MustCompile(`chan receive[^\n]*\]:\n[^\n]*blockedHere`)

// blockedHere — горутина, по имени которой отчёт находит зависший код.
func blockedHere(stop <-chan struct{}) { <-stop }

// Отчёт содержит состояние и стек зависшей горутины с именем её функции.
func TestStuckReportShowsBlockedGoroutine(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	go blockedHere(stop)
	// Горутина должна дойти до ожидания: опрашиваем отчёт, а не гадаем паузой.
	var r string
	deadline := time.Now().Add(Limit(time.Second))
	for time.Now().Before(deadline) {
		if r = StuckReport("экспорт", "готово 1 из 3"); blockedRE.MatchString(r) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for _, want := range []string{"зависание: экспорт", "состояние: готово 1 из 3", "blockedHere", "chan receive"} {
		if !strings.Contains(r, want) {
			t.Errorf("в отчёте нет %q", want)
		}
	}
	if strings.Contains(StuckReport("x", ""), "состояние:") {
		t.Error("пустое состояние напечатано")
	}
}

// WaitCh: значение приходит — true без отчёта; не приходит — false, функция состояния вызвана.
func TestWaitCh(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "ok"
	if v, ok := WaitCh(t, ch, time.Second, "x", func() string { t.Error("состояние без таймаута"); return "" }); !ok || v != "ok" {
		t.Fatalf("получено %q, %v", v, ok)
	}
	asked := false
	v, ok := WaitCh(t, ch, 30*time.Millisecond, "x", func() string { asked = true; return "s" })
	if ok || v != "" || !asked {
		t.Fatalf("таймаут: %q, %v, состояние запрошено %v", v, ok, asked)
	}
}
