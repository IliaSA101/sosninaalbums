//go:build unix

package app

import (
	"os"
	"syscall"
	"testing"
)

// TestOpenQuietRace: между Lstat и открытием файл заменили FIFO или ссылкой — file, без зависания и без
// смены альбома (У102).
func TestOpenQuietRace(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			r := newOQRig(t)
			orig := quietOpenFile
			t.Cleanup(func() { quietOpenFile = orig })
			quietOpenFile = func(p string) (*os.File, error) {
				if err := os.Remove(p); err != nil {
					t.Error(err)
				}
				switch kind {
				case "fifo":
					if err := syscall.Mkfifo(p, 0o644); err != nil {
						t.Error(err)
					}
				default:
					if err := os.Symlink(r.prev, p); err != nil {
						t.Error(err)
					}
				}
				return orig(p)
			}
			r.asks(t, kind, r.target, whyFile)
		})
	}
}
