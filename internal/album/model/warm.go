package model

import (
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Раскладки разворотов независимы, а холодный перебор на 8–16 фото — десятки мс:
// открытие альбома и смена настроек считают их параллельно (design 020 §4.2).
// Воркеры живут только внутри вызова под e.mu и проект лишь читают; кэш layout —
// под своим замком.

// parallelLayouts — выключатель для теста: без него View считается как прежде, по порядку.
var parallelLayouts = true

// ParallelLayouts — считать ли раскладки параллельно (WarmLayouts, view).
func ParallelLayouts() bool { return parallelLayouts }

// SetParallelLayouts — выключатель в тесте; рабочему коду линтер вызов запрещает (023).
func SetParallelLayouts(on bool) { parallelLayouts = on }

// maxWorkers — больше воркеров раскладке почти не помогает, а пик памяти растёт.
const maxWorkers = 8

// WarmLayouts прогревает ряд «в порядке ленты» (layout.Cached) у разворотов параллельно:
// следующий за ним последовательный код попадает в кэш. Паника — только в лог:
// тот же счёт повторится последовательно и поведёт себя как раньше.
func (p *Project) WarmLayouts(spreads []*Spread) {
	if !parallelLayouts {
		return
	}
	prm := p.Params()
	var pl PanicLog
	Each(len(spreads), func(i int) {
		s := spreads[i]
		pl.Safely(s.ID, func() { layout.Cached(p.Aspects(s.Photos), prm, VariantLimit) })
	})
	pl.Summary()
}

// Each зовёт f(0…n-1) на min(NumCPU, maxWorkers, n) горутинах; одно задание — без горутин.
// Паники f не ловит: это дело вызывающего (safely), иначе упадёт процесс.
func Each(n int, f func(i int)) {
	w := min(runtime.NumCPU(), maxWorkers, n)
	if w <= 1 {
		for i := range n {
			f(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range w {
		wg.Go(func() {
			for i := int(next.Add(1)) - 1; i < n; i = int(next.Add(1)) - 1 {
				f(i)
			}
		})
	}
	wg.Wait()
}

// PanicLog — паники воркеров одного вызова layAll/warmLayouts. Полный стек пишется только
// для первой: при детерминированном сбое такой же стек на каждый разворот — сотни КБ в лог.
type PanicLog struct{ n atomic.Int64 }

// Safely — счёт раскладки разворота spreadID под recover: паника уходит в лог.
func (l *PanicLog) Safely(spreadID string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			if l.n.Add(1) == 1 {
				log.Printf("ПАНИКА в раскладке разворота %s: %v\n%s", spreadID, r, debug.Stack())
			}
		}
	}()
	f()
}

// Summary дописывает одной строкой число паник после первой (их стек не пишется).
func (l *PanicLog) Summary() {
	if k := l.n.Load() - 1; k > 0 {
		log.Printf("ПАНИКА в раскладке: ещё %d разворотов", k)
	}
}
