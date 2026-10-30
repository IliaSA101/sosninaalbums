//go:build windows

package osx

import (
	"errors"
	"math"
	"syscall"
	"time"
)

// errInvalidParameter — ERROR_INVALID_PARAMETER (87): в syscall такой константы нет.
const errInvalidParameter = syscall.Errno(87)

var procAllowSetForegroundWindow = syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// WaitExit ждёт выхода процесса pid не дольше limit; true — процесса больше нет (или pid не
// задан), false — предел вышел или процесс не открыть (нет прав): ждать нечем. «Открыть в 0.X»
// (112): новая версия до запуска окна ждёт, пока старая отпустит замок единственного экземпляра.
func WaitExit(pid int, limit time.Duration) bool {
	if pid <= 0 {
		return true
	}
	if pid > math.MaxInt32 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid)) //nolint:gosec // G115: pid проверен выше
	if err != nil {
		// ERROR_INVALID_PARAMETER — такого процесса уже нет.
		return errors.Is(err, errInvalidParameter)
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	ms := min(max(limit.Milliseconds(), 0), math.MaxUint32-1) // INFINITE (MaxUint32) не передаём
	ev, err := syscall.WaitForSingleObject(h, uint32(ms))     //nolint:gosec // G115: зажато выше
	return err == nil && ev == syscall.WAIT_OBJECT_0
}

// AllowForeground разрешает любому процессу вывести своё окно вперёд (ASFW_ANY): второй
// запуск, начатый Проводником, отдаёт это право первому, чтобы окно с открытым альбомом
// не мигало в панели задач. Ошибку игнорируем: это только удобство.
func AllowForeground() {
	_, _, _ = procAllowSetForegroundWindow.Call(^uintptr(0))
}
