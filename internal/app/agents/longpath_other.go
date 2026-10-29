//go:build !windows

package agents

// longPathName — вне Windows 8.3-записей нет.
func longPathName(string) string { return "" }
