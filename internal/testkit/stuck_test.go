package testkit

import (
	"regexp"
	"strings"
	"sync/atomic"
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

// WaitProgress: прогресс растёт — ждёт дольше одного окна и берёт значение; прогресса нет —
// падает за одно окно, функция состояния вызвана; готовое значение приходит без отчёта.
func TestWaitProgress(t *testing.T) {
	const stall = 80 * time.Millisecond
	// медленная работа: 6 шагов по 80 мс (480 мс > окна 400 мс), шаг — 1/5 окна. Запас большой:
	// таймер Windows грубый (≈15,6 мс), и под нагрузкой CI шаг при запасе 1,6× выходил за окно.
	const slowStall = 400 * time.Millisecond
	ch := make(chan string, 1)
	var n atomic.Int64
	go func() {
		for range 6 {
			time.Sleep(80 * time.Millisecond)
			n.Add(1)
		}
		ch <- "ok"
	}()
	if v, ok := WaitProgress(t, ch, slowStall, "x", n.Load, func() string { t.Error("состояние без зависания"); return "" }); !ok || v != "ok" {
		t.Fatalf("медленная работа: %q, %v", v, ok)
	}
	// стоит: счётчик не растёт — false не позже чем за пару окон
	asked := false
	start := time.Now()
	v, ok := WaitProgress(t, ch, stall, "x", func() int64 { return 7 }, func() string { asked = true; return "s" })
	if ok || v != "" || !asked {
		t.Fatalf("остановка: %q, %v, состояние запрошено %v", v, ok, asked)
	}
	if d := time.Since(start); d > 10*stall {
		t.Errorf("остановка ловилась %v, ждали около одного окна", d)
	}
	// уже готовое значение — без ожидания
	ch <- "late"
	if v, ok := WaitProgress(t, ch, stall, "x", func() int64 { return 0 }, nil); !ok || v != "late" {
		t.Fatalf("готовое: %q, %v", v, ok)
	}
}
