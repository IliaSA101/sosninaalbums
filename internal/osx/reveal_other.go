//go:build !windows

package osx

import "syscall"

// explorerSelectAttr — вне Windows проводника нет, ветка недостижима.
func explorerSelectAttr(string) *syscall.SysProcAttr { return nil }

// explorerOpenAttr — вне Windows nil: путь идёт аргументом команды.
func explorerOpenAttr(string) *syscall.SysProcAttr { return nil }
