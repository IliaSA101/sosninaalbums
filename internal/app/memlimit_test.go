package app

import (
	"math"
	"os"
	"runtime/debug"
	"testing"
)

// keepMemLimit возвращает предел процесса и флаг «наш предел» после теста.
func keepMemLimit(t *testing.T) {
	t.Helper()
	old := debug.SetMemoryLimit(-1)
	memMu.Lock()
	ours := memOurs
	memMu.Unlock()
	t.Cleanup(func() {
		debug.SetMemoryLimit(old)
		memMu.Lock()
		memOurs = ours
		memMu.Unlock()
	})
}

// Без GOMEMLIMIT — предел 1,5 ГБ; заданный руками не трогаем. Прежний предел процесса — назад.
func TestApplyMemoryLimit(t *testing.T) {
	keepMemLimit(t)
	const other = 3 << 30

	t.Setenv("GOMEMLIMIT", "") // t.Setenv вернёт прежнее значение переменной после теста
	if err := os.Unsetenv("GOMEMLIMIT"); err != nil {
		t.Fatal(err)
	}
	debug.SetMemoryLimit(other)
	applyMemoryLimit()
	if got := debug.SetMemoryLimit(-1); got != 1536<<20 {
		t.Fatalf("без GOMEMLIMIT предел %d МБ, ждали 1536", got>>20)
	}

	t.Setenv("GOMEMLIMIT", "3GiB")
	debug.SetMemoryLimit(other)
	applyMemoryLimit()
	if got := debug.SetMemoryLimit(-1); got != other {
		t.Fatalf("GOMEMLIMIT задан, а предел сменился на %d МБ", got>>20)
	}

	// Пустое значение — тоже «задан»: рантайм читает его как без предела, мы не вмешиваемся.
	t.Setenv("GOMEMLIMIT", "")
	debug.SetMemoryLimit(other)
	applyMemoryLimit()
	if got := debug.SetMemoryLimit(-1); got != other {
		t.Fatalf("GOMEMLIMIT пустой, а предел сменился на %d МБ", got>>20)
	}
}

// Снятие на время экспорта: вложенные снятия возвращают предел только последним возвратом,
// повторный возврат ничего не делает, предел из GOMEMLIMIT не трогается.
func TestLiftMemoryLimit(t *testing.T) {
	keepMemLimit(t)
	t.Setenv("GOMEMLIMIT", "")
	if err := os.Unsetenv("GOMEMLIMIT"); err != nil {
		t.Fatal(err)
	}
	applyMemoryLimit()
	limit := func() int64 { return debug.SetMemoryLimit(-1) }
	r1 := liftMemoryLimit()
	r2 := liftMemoryLimit()
	if got := limit(); got != math.MaxInt64 {
		t.Fatalf("снят: предел %d", got)
	}
	r1()
	r1() // повтор не должен вернуть предел за второго
	if got := limit(); got != math.MaxInt64 {
		t.Fatalf("второй экспорт ещё идёт, а предел %d МБ", got>>20)
	}
	r2()
	if got := limit(); got != memLimit {
		t.Fatalf("после обоих предел %d, ждали %d", got, int64(memLimit))
	}
	r2()
	if got := limit(); got != memLimit {
		t.Fatalf("холостой возврат сменил предел на %d", got)
	}

	const other = 3 << 30
	t.Setenv("GOMEMLIMIT", "3GiB")
	debug.SetMemoryLimit(other)
	applyMemoryLimit()
	r := liftMemoryLimit()
	if got := limit(); got != other {
		t.Fatalf("GOMEMLIMIT задан, а экспорт сменил предел на %d", got)
	}
	r()
	if got := limit(); got != other {
		t.Fatalf("GOMEMLIMIT задан, а возврат сменил предел на %d", got)
	}
}

// Во время экспорта (и удачного, и с ошибкой) предела нет, к export:done он прежний; два экспорта подряд.
func TestExportLiftsMemoryLimit(t *testing.T) {
	keepMemLimit(t)
	t.Setenv("GOMEMLIMIT", "")
	if err := os.Unsetenv("GOMEMLIMIT"); err != nil {
		t.Fatal(err)
	}
	applyMemoryLimit()
	e := newExportEnv(t, 3)
	var during []int64
	e.onStep = func(int) { during = append(during, debug.SetMemoryLimit(-1)) }
	for i, want := range []int{3, 1} {
		if i == 1 { // второе фото — не картинка: разворот 2 падает после шага 1
			if err := os.WriteFile(e.photos[1], []byte("не jpeg"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		during = nil
		msg := e.export(t, ExportOptions{})
		if (msg != "") != (i == 1) {
			t.Fatalf("экспорт %d: ошибка %q", i+1, msg)
		}
		if len(during) != want {
			t.Fatalf("экспорт %d: шагов %d, ждали %d", i+1, len(during), want)
		}
		for _, v := range during {
			if v != math.MaxInt64 {
				t.Errorf("экспорт %d: во время записи предел %d МБ", i+1, v>>20)
			}
		}
		if got := debug.SetMemoryLimit(-1); got != memLimit {
			t.Errorf("экспорт %d: после предел %d, ждали %d", i+1, got, int64(memLimit))
		}
	}
}
