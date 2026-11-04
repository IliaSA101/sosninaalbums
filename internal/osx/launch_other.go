//go:build !windows

package osx

// longPath — коротких имён 8.3 вне Windows нет.
func longPath(p string) string { return p }
