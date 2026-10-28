package testkit

import (
	"bytes"
	"fmt"
	"runtime/pprof"
	"testing"
	"time"
)

// stuckDumpMax — потолок отчёта о зависании: на занятой машине горутин сотни, а лог CI режет длинное.
const stuckDumpMax = 1 << 20

// StuckReport — отчёт «где всё стоит»: state (что тест знает о ходе работы) и стеки всех горутин
// (pprof goroutine, debug=2: с состоянием и временем ожидания). Нужен там, где тест ждёт фон по
// таймауту: сбой, не повторяющийся без нагрузки, иначе остаётся без следа.
func StuckReport(what, state string) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "зависание: %s\n", what)
	if state != "" {
		fmt.Fprintf(&b, "состояние: %s\n", state)
	}
	b.WriteString("стеки всех горутин:\n")
	if p := pprof.Lookup("goroutine"); p != nil {
		var g bytes.Buffer
		_ = p.WriteTo(&g, 2)
		if g.Len() > stuckDumpMax {
			g.Truncate(stuckDumpMax)
			g.WriteString("\n… (обрезано)")
		}
		b.Write(g.Bytes())
	}
	return b.String()
}

// WaitCh ждёт значение из ch до d (уже с testkit.Limit от вызывающего). По таймауту пишет в t.Log
// StuckReport (state — необязательная функция состояния, вызывается только по таймауту) и
// возвращает false; решает, как валить тест, вызывающий.
func WaitCh[T any](t testing.TB, ch <-chan T, d time.Duration, what string, state func() string) (T, bool) {
	t.Helper()
	select {
	case v := <-ch:
		return v, true
	case <-time.After(d):
		s := ""
		if state != nil {
			s = state()
		}
		t.Log(StuckReport(what, s))
		var zero T
		return zero, false
	}
}

// WaitProgress ждёт значение из ch, пока работа идёт: зависанием считается не общее время, а
// отсутствие прогресса — progress (счётчик сделанного) не вырос за stall (уже с testkit.Limit).
// Нужен длинным конвейерам (рендер десятков больших файлов): под -race на раннере они идут минуты,
// но каждый шаг укладывается в окно, а настоящая остановка ловится за одно окно. Отчёт и
// возвращаемое значение — как у WaitCh.
func WaitProgress[T any](t testing.TB, ch <-chan T, stall time.Duration, what string, progress func() int64, state func() string) (T, bool) {
	t.Helper()
	last := progress()
	timer := time.NewTimer(stall)
	defer timer.Stop()
	for {
		select {
		case v := <-ch:
			return v, true
		case <-timer.C:
			if cur := progress(); cur != last {
				last = cur
				timer.Reset(stall)
				continue
			}
			s := ""
			if state != nil {
				s = state()
			}
			t.Log(StuckReport(what, s))
			var zero T
			return zero, false
		}
	}
}
