//go:build !windows

package osx

// knownDirs — адреса, которые отдаёт сама ОС: вне Windows их нет (XDG читает userDirsIn).
func knownDirs() []string { return nil }
