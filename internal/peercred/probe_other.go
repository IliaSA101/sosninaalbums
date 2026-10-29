//go:build !windows

package peercred

// osUnsupported: ядро Linux и macOS всегда называет учётную запись собеседника.
func osUnsupported() bool { return false }
