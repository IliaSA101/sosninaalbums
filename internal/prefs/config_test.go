package prefs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestPickDir(t *testing.T) {
	root := filepath.Clean(testkit.TempDir(t))
	docs := filepath.Join(root, "Documents")
	last := filepath.Join(root, "съёмка", "день1")
	for _, d := range []string{docs, last} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gone := filepath.Join(last, "удалена", "глубже")
	file := filepath.Join(root, "f.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, last, fallback, want string
	}{
		{"последняя есть", last, docs, last},
		{"нет папки — ближайший родитель", gone, docs, last},
		{"файл вместо папки — родитель", filepath.Join(file, "x"), docs, root},
		{"нет последней — Документы", "", docs, docs},
		{"относительный путь не берём", "съёмка", docs, docs},
		{"нет и Документов", "", filepath.Join(root, "нет"), ""},
	}
	for _, c := range cases {
		if got := PickDir(c.last, c.fallback); got != c.want {
			t.Errorf("%s: получили %q, ждали %q", c.name, got, c.want)
		}
	}
	for _, bad := range []string{`\server\share\x`, `\?\C:\x`, `\??\C:\x`} {
		if got := PickDir(bad, docs); got != docs {
			t.Errorf("нелокальный %q принят: %q", bad, got)
		}
	}
}

// Настройки не затирают друг друга; битый файл игнорируется; нелокальная папка не пишется.
func TestRememberDir(t *testing.T) {
	dir := testkit.TempDir(t)
	d1, d2 := filepath.Clean(testkit.TempDir(t)), filepath.Clean(testkit.TempDir(t))
	RememberDir(dir, DirProject, d1)
	UpdateConfig(dir, func(c *Config) { c.LastProject = "p.salbum" })
	RememberDir(dir, DirExport, d2)
	RememberDir(dir, DirPhotos, `\server\share`)
	c := LoadConfig(dir)
	if c.LastProject != "p.salbum" || c.LastDirs[DirProject] != d1 || c.LastDirs[DirExport] != d2 || len(c.LastDirs) != 2 {
		t.Errorf("конфиг: %+v", c)
	}
	if got := DialogDir(dir, DirProject); got != d1 {
		t.Errorf("DialogDir=%q, ждали %q", got, d1)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte("{не json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.LastProject != "" || len(c.LastDirs) != 0 {
		t.Errorf("битый конфиг: %+v", c)
	}
	RememberDir(dir, DirExport, d2) // после битого файла запись снова работает
	if LoadConfig(dir).LastDirs[DirExport] != d2 {
		t.Error("запись после битого конфига не прошла")
	}
}
