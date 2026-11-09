//go:build e2ebig && !windows

package app

// e2eRAMGB — объём памяти машины для отчёта e2e: вне Windows не определяем (0).
func e2eRAMGB() int { return 0 }
