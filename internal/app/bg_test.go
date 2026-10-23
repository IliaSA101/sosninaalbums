package app

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// goBG и closeBackground не пересекаются: ни паники WaitGroup (Add из нуля вместе с Wait), ни
// фона, стартовавшего после возврата closeBackground. Запускатели идут из горутин вне bg —
// как таймер автосохранения.
func TestGoBGAfterCloseBackground(t *testing.T) {
	for range 200 {
		a := NewApp()
		var started atomic.Int32
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for range 50 {
					a.goBG(func() { started.Add(1) })
				}
			})
		}
		a.closeBackground()
		atClose := started.Load() // все начатые задачи дождались, новых быть не должно
		wg.Wait()
		if n := started.Load(); n != atClose {
			t.Fatalf("после closeBackground стартовало фоновых задач: %d", n-atClose)
		}
		if a.goBG(func() { started.Add(1) }) {
			t.Fatal("goBG после closeBackground запустил задачу")
		}
		a.checkMissing(1, map[string]string{"a": "/нет/a.jpg"}, false) // отказ откатывает счётчик
		if n := a.missPending.Load(); n != 0 {
			t.Fatalf("missPending после отказа goBG: %d", n)
		}
		a.closeBackground() // повторное закрытие безопасно
	}
}

// Отказ goBG (фон закрыт) не оставляет флагов: сверка папок не висит «идущей», проверка при
// запуске не тратит свой раз за сеанс.
func TestGoBGRefusalRollsBackFlags(t *testing.T) {
	a := newApp(t)
	a.bgClosed = true
	if a.beginCheck(false) {
		t.Error("сверка папок началась при закрытом фоне")
	}
	if _, ok := a.check.enter(false, time.Now()); !ok {
		t.Error("check.running остался после отказа")
	}
	a.net.started.Store(false)
	a.cfgDir = testkit.TempDir(t)
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Allowed, n.AtStart = true, true })
	a.scheduleLibraryCheck()
	if a.net.started.Load() {
		t.Error("net.started остался после отказа")
	}
}

// monotone пропускает только растущий done: запоздавший воркер не откатывает полоску прогрева.
func TestMonotoneProgress(t *testing.T) {
	var m monotone
	var got []int
	for _, d := range []int{0, 1, 3, 2, 3, 4} { // 0 — не событие; 3 после 3 и 2 после 3 — запоздавшие
		if m.advance(d) {
			got = append(got, d)
		}
	}
	if want := []int{1, 3, 4}; !slices.Equal(got, want) {
		t.Fatalf("пропущено %v, ждали %v", got, want)
	}
}

// Отчёт о зависании экспорта называет прогресс и бюджет декодов: по нему следующий сбой читается без повтора.
func TestExportStuckState(t *testing.T) {
	e := newExportEnv(t, 1)
	e.steps.Store(1)
	s := e.stuckState()
	for _, want := range []string{"готово 1", "exporting=false", "stopped=false", "DecodeBudget: занято"} {
		if !strings.Contains(s, want) {
			t.Errorf("в состоянии нет %q: %s", want, s)
		}
	}
}
