package osx

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestVolumeOfReal — настоящая машина (Linux): том временной папки либо читается и находится
// обратно через VolumePath, либо честно «без номера» (overlay, tmpfs в контейнере).
func TestVolumeOfReal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("таблица монтирования — Linux; Windows — TestVolIDWindows")
	}
	v, err := VolumeOf(testkit.TempDir(t))
	if errors.Is(err, errNoVolID) {
		t.Skipf("у тома временной папки нет UUID: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.ID, "uuid:") || v.Root == "" {
		t.Fatalf("странный том: %+v", v)
	}
	p, _, err := VolumePath(v.ID, v.Rel)
	if err != nil {
		t.Fatalf("VolumePath(%q, %q): %v", v.ID, v.Rel, err)
	}
	if again, err := VolumeOf(p); err != nil || again.ID != v.ID || again.Rel != v.Rel {
		t.Errorf("круг VolumeOf→VolumePath→VolumeOf: %q → %+v, %v", p, again, err)
	}
}
