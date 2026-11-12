package peercred

import (
	"errors"
	"runtime"
	"testing"
)

// Решение «ОС не умеет» принимает проба на своей паре, один раз; чужая ошибка запроса само по
// себе не открывает путь (0.32-148-11).
func TestVerdictClassify(t *testing.T) {
	boom := errors.New("WSAEINVAL")
	t.Run("проба: не умеет — ErrUnsupported, проба одна", func(t *testing.T) {
		var v verdict
		calls := 0
		probe := func() bool { calls++; return true }
		for range 3 {
			if err := v.classify(boom, true, probe); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("err = %v, ждали ErrUnsupported", err)
			}
		}
		if calls != 1 {
			t.Errorf("проб: %d, ждали 1", calls)
		}
	})
	t.Run("проба: умеет — любая ошибка запроса отказ, не ErrUnsupported", func(t *testing.T) {
		var v verdict
		calls := 0
		probe := func() bool { calls++; return false }
		for range 3 {
			err := v.classify(boom, true, probe)
			if err == nil || errors.Is(err, ErrUnsupported) || !errors.Is(err, boom) {
				t.Fatalf("err = %v, ждали обычную ошибку с причиной", err)
			}
		}
		if calls != 1 {
			t.Errorf("проб: %d, ждали 1", calls)
		}
	})
	t.Run("не похоже на «не умею» и nil — как есть, проба не зовётся", func(t *testing.T) {
		var v verdict
		probe := func() bool { t.Error("проба вызвана"); return true }
		if err := v.classify(boom, false, probe); !errors.Is(err, boom) || errors.Is(err, ErrUnsupported) {
			t.Errorf("err = %v", err)
		}
		if err := v.classify(nil, true, probe); err != nil {
			t.Errorf("nil: %v", err)
		}
	})
	t.Run("паника пробы — отказ, а не ErrUnsupported", func(t *testing.T) {
		var v verdict
		func() {
			defer func() { _ = recover() }()
			_ = v.classify(boom, true, func() bool { panic("проба") })
		}()
		if err := v.classify(boom, true, func() bool { return true }); errors.Is(err, ErrUnsupported) {
			t.Error("после паники пробы решено «не умеет»")
		}
	})
	t.Run("решение не меняется после первой пробы", func(t *testing.T) {
		var v verdict
		if !v.decide(func() bool { return true }) || !v.decide(func() bool { return false }) {
			t.Error("второй вызов изменил решение")
		}
	})
}

// Linux и macOS умеют называть собеседника: Probe не отказывает; Windows — только nil или ErrUnsupported.
func TestProbe(t *testing.T) {
	err := Probe()
	if err != nil && (runtime.GOOS != "windows" || !errors.Is(err, ErrUnsupported)) {
		t.Errorf("Probe = %v", err)
	}
}
