//go:build windows

package osx

import "syscall"

// explorerSelectAttr — командная строка «explorer /select,"путь"» целиком: Go экранировал бы
// аргумент `/select,` и путь по правилам CommandLineToArgv, а проводник их не понимает
// (путь с запятой тогда открывает не тот файл).
func explorerSelectAttr(path string) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CmdLine: `explorer /select,"` + path + `"`}
}

// explorerOpenAttr — «explorer "папка"» целиком, как explorerSelectAttr: без пробелов Go путь
// не кавычит, и проводник режет «Иванов,Петрова» по запятой.
func explorerOpenAttr(path string) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CmdLine: `explorer "` + path + `"`}
}
