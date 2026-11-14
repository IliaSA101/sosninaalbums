//go:build !windows

package osx

func realPath(p string) (string, error) { return p, nil }
