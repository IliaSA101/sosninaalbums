//go:build !windows

package mcpbridge

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
)

// platformPackaged — пакетной идентичности MSIX вне Windows нет.
func platformPackaged() bool { return false }

// platformInstanceAlive — замок экземпляра Wails на Linux — имя в D-Bus, на Mac — уведомление;
// без своего клиента D-Bus посредник их не видит. Живую программу ловят сокет (b.live) и отметка
// запуска; второй экземпляр, если всё же стартует, сам уступит замку.
func platformInstanceAlive(string) bool { return false }

// fallbackFDs — сколько дескрипторов перебрать, если каталог со списком не открылся.
const fallbackFDs = 1024

// closeOnExecInherited ставит CLOEXEC всем дескрипторам выше 2 этого процесса. Go закрывает на exec
// только свои дескрипторы: унаследованные от клиента без CLOEXEC (каналы, cgroup раннера) дошли бы
// до программы, и она держала бы чужие концы каналов клиента. Сам посредник от этого не страдает:
// CLOEXEC действует только при exec, свои дескрипторы он продолжает использовать.
func closeOnExecInherited() {
	dir := "/proc/self/fd"
	if runtime.GOOS == "darwin" {
		dir = "/dev/fd"
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		for fd := 3; fd < fallbackFDs; fd++ {
			syscall.CloseOnExec(fd)
		}
		return
	}
	for _, e := range ents {
		if fd, err := strconv.Atoi(e.Name()); err == nil && fd > 2 {
			syscall.CloseOnExec(fd) // в том числе дескриптор самого каталога: он уже прочитан
		}
	}
}

// runDetached запускает name с args отдельным сеансом: stdin, stdout и stderr пусты (/dev/null),
// чужие дескрипторы выше 2 не наследуются (closeOnExecInherited), окружение — ровно env (непустой
// срез: nil означал бы «унаследовать»). Wait — в своей горутине, чтобы не оставлять зомби, пока
// жив посредник.
func runDetached(name string, args []string, dir string, env []string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // name — собственная программа (os.Executable) или /usr/bin/open, args — наш пакет .app
	cmd.Dir = dir
	cmd.Env = append([]string{}, env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	closeOnExecInherited()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
