//go:build !windows

package agents

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// parentRig — служба на сокете <parent>/mcp/sock; parent — короткая папка теста с правами mode.
func parentRig(t *testing.T, mode os.FileMode) (*rig, string) {
	t.Helper()
	parent := shortDir(t)
	if err := os.Chmod(parent, mode); err != nil {
		t.Fatal(err)
	}
	return newRig(t, Options{}, filepath.Join(parent, "mcp", "sock")), parent
}

// TestParentDirPerms: родитель mcp открыт на запись группе или всем — отказ с текстом для окна и
// без папки mcp; 0755, 0750 и 0700 — работает; ссылка на родителя допустима, проверяется её цель.
func TestParentDirPerms(t *testing.T) {
	cases := []struct {
		mode os.FileMode
		open bool
	}{{0o777, true}, {0o775, true}, {0o757, true}, {0o755, false}, {0o750, false}, {0o700, false}}
	for _, c := range cases {
		r, parent := parentRig(t, c.mode)
		err := r.svc.Start()
		if !c.open {
			if err != nil {
				t.Fatalf("%o: Start = %v", c.mode, err)
			}
			client(t, r.sock, "родитель")
			continue
		}
		if !errors.Is(err, errParentOpen) {
			t.Fatalf("%o: Start = %v, ждали errParentOpen", c.mode, err)
		}
		if st, e := r.svc.State(); st != StateError || e != "папка данных открыта на запись другим пользователям компьютера" {
			t.Errorf("%o: State = %q %q", c.mode, st, e)
		}
		if _, err := os.Lstat(filepath.Join(parent, "mcp")); err == nil {
			t.Errorf("%o: папка mcp создана в открытом родителе", c.mode)
		}
	}

	// Ссылка на родителя — перенос папки настроек на другой диск. Имя ссылки короткое: sun_path.
	for _, c := range cases[:1] {
		testLink(t, c.mode, true)
	}
	testLink(t, 0o700, false)
}

// testLink: родитель — ссылка на папку с правами mode; ждём отказ или работу по правам цели.
func testLink(t *testing.T, mode os.FileMode, open bool) {
	t.Helper()
	_, target := parentRig(t, mode)
	link, err := os.MkdirTemp("/tmp", "sa")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	r := newRig(t, Options{}, filepath.Join(link, "mcp", "sock"))
	err = r.svc.Start()
	if open != errors.Is(err, errParentOpen) || (!open && err != nil) {
		t.Fatalf("ссылка на %o: Start = %v", mode, err)
	}
}

// TestParentDirMissing: родителя нет — его создаёт программа без прав другим, отказа нет.
func TestParentDirMissing(t *testing.T) {
	base := shortDir(t)
	r := newRig(t, Options{}, filepath.Join(base, "p", "mcp", "sock"))
	r.start(t)
	if fi, err := os.Stat(filepath.Join(base, "p")); err != nil || fi.Mode().Perm()&0o022 != 0 {
		t.Fatalf("родитель: %v %v", fi, err)
	}
}
