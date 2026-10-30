//go:build !windows

package osx

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Запущенный процесс подбирается в фоне (нет зомби); неудачный запуск — ошибка без ожидания.
func TestStartCmdReaps(t *testing.T) {
	exe, err := exec.LookPath("true")
	if err != nil {
		t.Skip("нет команды true")
	}
	cmd := exec.Command(exe)
	if err := startCmd(cmd); err != nil {
		t.Fatal(err)
	}
	// Signal возвращает ErrProcessDone только после Wait: у зомби процесс ещё «жив» для ОС.
	deadline := time.Now().Add(5 * time.Second)
	for !errors.Is(cmd.Process.Signal(syscall.Signal(0)), os.ErrProcessDone) {
		if time.Now().After(deadline) {
			t.Fatal("процесс не подобран: Wait не вызван")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := startCmd(exec.Command("/нет/такой/команды")); err == nil {
		t.Fatal("ждали ошибку запуска")
	}
}
