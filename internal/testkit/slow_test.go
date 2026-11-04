package testkit

import (
	"testing"
	"time"
)

// CostTest пропускает тест под -race и ничего не делает без него.
func TestCostTest(t *testing.T) {
	ran := false
	t.Run("sub", func(t *testing.T) {
		CostTest(t)
		ran = true
	})
	if ran == Race {
		t.Fatalf("после CostTest тест продолжился = %t при Race = %t", ran, Race)
	}
}

// Limit растёт с RaceSlowdown и SA_TEST_SLOW; Fastest берёт минимум и хотя бы один запуск.
func TestLimitAndFastest(t *testing.T) {
	t.Setenv("SA_TEST_SLOW", "")
	if got, want := Limit(time.Second), time.Second*RaceSlowdown; got != want {
		t.Fatalf("Limit(1с) = %v, нужно %v", got, want)
	}
	t.Setenv("SA_TEST_SLOW", "4")
	if got, want := Limit(time.Second), 4*time.Second*RaceSlowdown; got != want {
		t.Fatalf("Limit(1с) при SA_TEST_SLOW=4 = %v, нужно %v", got, want)
	}
	t.Setenv("SA_TEST_SLOW", "0.5") // меньше 1 игнорируется: порог не сжимаем
	if got, want := Limit(time.Second), time.Second*RaceSlowdown; got != want {
		t.Fatalf("Limit(1с) при SA_TEST_SLOW=0.5 = %v, нужно %v", got, want)
	}
	n := 0
	if d := Fastest(0, func() { n++ }); n != 1 || d < 0 {
		t.Fatalf("Fastest(0): запусков %d, время %v; нужен ровно один запуск", n, d)
	}
	calls := 0
	d := Fastest(3, func() {
		calls++
		if calls == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	})
	if calls != 3 || d >= 20*time.Millisecond {
		t.Fatalf("Fastest(3): запусков %d, лучшее %v; нужны 3 и быстрее 20 мс", calls, d)
	}
}
