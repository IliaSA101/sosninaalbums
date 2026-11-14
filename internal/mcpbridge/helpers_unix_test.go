//go:build !windows

package mcpbridge

import (
	"os"
	"testing"
)

// lockDir ставит папке сокета права 0700 (MkdirAll мог отдать иные из-за umask).
func lockDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}
