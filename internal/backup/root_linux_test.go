//go:build linux

package backup

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Linux: копии — в $XDG_DATA_HOME/<name>/backups, без него или с относительным — в
// ~/.local/share; LOCALAPPDATA (если задан) — как на Windows. Ключ альбома различает регистр папки.
func TestDefaultRootXDGLinux(t *testing.T) {
	data, home := testkit.TempDir(t), testkit.TempDir(t)
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	if got, want := DefaultRoot("SA"), filepath.Join(data, "SA", "backups"); got != want {
		t.Errorf("XDG_DATA_HOME: %q, ждали %q", got, want)
	}
	for _, v := range []string{"", "rel/dir"} {
		t.Setenv("XDG_DATA_HOME", v)
		if got, want := DefaultRoot("SA"), filepath.Join(home, ".local", "share", "SA", "backups"); got != want {
			t.Errorf("XDG_DATA_HOME=%q: %q, ждали %q", v, got, want)
		}
	}
	t.Setenv("LOCALAPPDATA", data)
	if got, want := DefaultRoot("SA"), filepath.Join(data, "SA", "backups"); got != want {
		t.Errorf("LOCALAPPDATA: %q, ждали %q", got, want)
	}
	if Key("/x/Весна/a.salbum") == Key("/x/ВЕСНА/a.salbum") {
		t.Error("ключ копий свёл регистр папки")
	}
	s := New("/x/Копии")
	if s.Inside("/x/КОПИИ/a.salbum") || !s.Inside("/x/Копии/a.salbum") {
		t.Error("Inside свёл регистр")
	}
}

// Относительный LOCALAPPDATA (на Linux его задаёт кто угодно) пропускается, как относительный XDG:
// копии не уходят в текущую папку; несколько вариантов подряд.
func TestDefaultRootRelativeLocalAppData(t *testing.T) {
	home := testkit.TempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	want := filepath.Join(home, ".local", "share", "SA", "backups")
	for _, v := range []string{"rel/dir", ".", "x", "~/data"} {
		t.Setenv("LOCALAPPDATA", v)
		if got := DefaultRoot("SA"); got != want {
			t.Errorf("LOCALAPPDATA=%q: %q, ждали %q", v, got, want)
		}
	}
}
