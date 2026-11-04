//go:build windows

package update

import (
	"errors"
	"os/exec"
	"syscall"
)

// Флаги CreateProcess: установщик не должен умереть вместе с программой — ни от закрытия
// консоли, ни от завершения задания (job), в котором её запустили.
const (
	detachedProcess    = 0x00000008
	createBreakawayJob = 0x01000000
	flagsDetached      = detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP
)

// Start — запускает установщик отсоединённым процессом и не ждёт его: после этого программа
// закрывается. cmdline — сырая командная строка (CmdLine). Отказ из-за задания без breakaway
// (ERROR_ACCESS_DENIED) — повтор без этого флага.
func Start(setup, cmdline string) error {
	err := start(setup, cmdline, flagsDetached|createBreakawayJob)
	if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		err = start(setup, cmdline, flagsDetached)
	}
	return err
}

func start(setup, cmdline string, flags uint32) error {
	cmd := exec.Command(setup)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdline, CreationFlags: flags}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
