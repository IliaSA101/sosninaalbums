package testkit

import (
	"testing"
	"time"
)

// RaceSlowdown не бывает меньше 1: иначе пороги времени в тестах сжались бы.
func TestRaceSlowdown(t *testing.T) {
	if RaceSlowdown < 1 {
		t.Fatalf("RaceSlowdown = %d, нужно >= 1", RaceSlowdown)
	}
}

// Race согласован с RaceSlowdown: под -race запас больше 1, без него — ровно 1.
func TestRaceConst(t *testing.T) {
	if Race != (RaceSlowdown > 1) {
		t.Fatalf("Race = %t при RaceSlowdown = %d", Race, RaceSlowdown)
	}
}

// Limit масштабируется по SA_TEST_SLOW и не сжимается от значения меньше 1 или мусора.
func TestLimit(t *testing.T) {
	base := 100 * time.Millisecond
	cases := map[string]time.Duration{
		"":    base * RaceSlowdown,
		"4":   base * RaceSlowdown * 4,
		"0.1": base * RaceSlowdown,
		"abc": base * RaceSlowdown,
	}
	for env, want := range cases {
		t.Setenv("SA_TEST_SLOW", env)
		if got := Limit(base); got != want {
			t.Errorf("SA_TEST_SLOW=%q: %v, want %v", env, got, want)
		}
	}
}

// Fastest берёт минимум: один быстрый прогон из трёх сбивает шум, а если
// медленны все — результат медленный.
func TestFastest(t *testing.T) {
	n := 0
	d := Fastest(3, func() {
		n++
		if n != 2 {
			time.Sleep(30 * time.Millisecond)
		}
	})
	if n != 3 || d >= 20*time.Millisecond {
		t.Fatalf("прогонов %d, минимум %v", n, d)
	}
	if d := Fastest(2, func() { time.Sleep(30 * time.Millisecond) }); d < 30*time.Millisecond {
		t.Fatalf("медленные прогоны дали %v", d)
	}
	n = 0
	if Fastest(0, func() { n++ }); n != 1 {
		t.Fatalf("n=0: прогонов %d, ждали 1", n)
	}
}
