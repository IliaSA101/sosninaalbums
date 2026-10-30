//go:build !windows

package osx

import (
	"errors"
	"syscall"
	"time"
)

// WaitExit ждёт выхода процесса pid не дольше limit; true — процесса больше нет (или pid не
// задан), false — предел вышел. «Открыть в 0.X» (112): новая версия до запуска окна ждёт, пока
// старая отпустит замок единственного экземпляра. Вне Windows — проба kill(pid, 0) раз в 100 мс.
func WaitExit(pid int, limit time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(limit)
	for {
		// ESRCH — процесса нет; EPERM — есть, но чужой: ждём как живого.
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		left := time.Until(deadline)
		if left <= 0 {
			return false
		}
		time.Sleep(min(left, 100*time.Millisecond))
	}
}

// AllowForeground — вне Windows окно вперёд выводит оконная система; no-op.
func AllowForeground() {}
