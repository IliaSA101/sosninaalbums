//go:build linux

package backup

import (
	"os"
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

// Относительные HOME и XDG_*: ни одной абсолютной базы — папки копий нет, а запись через такое
// хранилище не создаёт ничего в текущей папке процесса.
func TestDefaultRootRelativeHomeNoCwdWrites(t *testing.T) {
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	src := filepath.Join(cwd, "a.salbum")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_DATA_HOME", "rel/data")
	for _, home := range []string{"foo", "."} {
		t.Setenv("HOME", home)
		// Go сам отвергает относительный XDG_CACHE_HOME, а вот HOME/.cache — нет.
		t.Setenv("XDG_CACHE_HOME", "")
		root := DefaultRoot("SA")
		if root != "" {
			t.Fatalf("HOME=%q: DefaultRoot = %q, ждали пусто", home, root)
		}
		s := New(root)
		if err := s.Force(src, Key(src)); err != nil {
			t.Errorf("HOME=%q: Force: %v", home, err)
		}
		if err := s.Prune(); err != nil {
			t.Errorf("HOME=%q: Prune: %v", home, err)
		}
	}
	ents, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "a.salbum" {
		t.Errorf("в текущей папке появилось лишнее: %v", ents)
	}
}
