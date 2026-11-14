//go:build !windows && !darwin

package app

import "io/fs"

// entryCloud: на Linux заглушек облака нет (FUSE-тома и так не локальные, osx.Remote).
func entryCloud(fs.DirEntry) bool { return false }
