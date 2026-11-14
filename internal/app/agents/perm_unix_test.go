//go:build !windows

package agents

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

// TestChmodErr: отказ Chmod на ФС без прав — errNoPerm; прочие ошибки остаются как есть.
func TestChmodErr(t *testing.T) {
	for _, e := range []error{syscall.EPERM, syscall.EACCES, syscall.ENOTSUP, syscall.EOPNOTSUPP} {
		wrapped := &fs.PathError{Op: "chmod", Path: "/x", Err: e}
		if got := chmodErr(wrapped); !errors.Is(got, errNoPerm) {
			t.Errorf("%v → %v, ждали errNoPerm", e, got)
		}
	}
	other := &fs.PathError{Op: "chmod", Path: "/x", Err: syscall.EIO}
	if got := chmodErr(other); errors.Is(got, errNoPerm) || !errors.Is(got, syscall.EIO) {
		t.Errorf("EIO → %v", got)
	}
}
