//go:build !windows

package osx

import "os"

// Elevated — процесс запущен от администратора (root): правки агентов такой процесс не принимает
// (149, 0.33-149-12). Unix: действующий uid 0.
func Elevated() bool { return os.Geteuid() == 0 }
