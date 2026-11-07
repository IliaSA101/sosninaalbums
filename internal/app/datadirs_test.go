package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// noUserDirs — окружение без папок пользователя: UserConfigDir и UserCacheDir отдают ошибку
// (Linux — без HOME и XDG_*, Windows — без APPDATA и LOCALAPPDATA), текущая папка — пустая
// временная. Возвращает её: после теста в ней не должно появиться ничего.
func noUserDirs(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	return cwd
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range list {
		t.Errorf("в текущей папке появилось %q", de.Name())
	}
}

// dataDirs без папок пользователя (и с относительными) — "" и "", а не путь от текущей папки.
func TestDataDirsNeverRelative(t *testing.T) {
	cwd := noUserDirs(t)
	if cfg, thumbs := dataDirs("SosninaAlbums"); cfg != "" || thumbs != "" {
		t.Fatalf("dataDirs без папок пользователя: %q, %q", cfg, thumbs)
	}
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", "rel/cfg")
		t.Setenv("XDG_CACHE_HOME", "rel/cache")
		if cfg, thumbs := dataDirs("SosninaAlbums"); cfg != "" || thumbs != "" {
			t.Fatalf("dataDirs с относительными XDG: %q, %q", cfg, thumbs)
		}
		// Есть только кэш: настройки выключены, миниатюры — нет.
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("XDG_CACHE_HOME", testkit.TempDir(t))
		if cfg, thumbs := dataDirs("SosninaAlbums"); cfg != "" || !filepath.IsAbs(thumbs) {
			t.Fatalf("только кэш: cfg=%q thumbs=%q", cfg, thumbs)
		}
	}
	assertEmptyDir(t, cwd)
}

// Без папки настроек настройки живут в памяти, черновика нет, миниатюр нет — и в текущей папке
// не появляется ни config.json, ни черновика, ни templates.json.
func TestNoConfigDirWritesNothing(t *testing.T) {
	cwd := noUserDirs(t)
	a := newApp(t)
	a.cfgDir = "" // как после startup без папки настроек
	a.emit = func(string, ...any) {}
	t.Cleanup(func() { prefs.UpdateConfig("", func(c *prefs.Config) { *c = prefs.Config{} }) })

	a.updateConfig(func(c *prefs.Config) { c.Level = "pro" })
	if got := a.loadConfig().Level; got != "pro" {
		t.Errorf("настройки в памяти: уровень %q", got)
	}
	a.rememberRecent(filepath.Join(cwd, "a.salbum"))
	a.rememberDir(prefs.DirProject, cwd)
	if a.draftPath() != "" || a.badDraftPath(time.Now()) != "" {
		t.Errorf("пути черновика: %q, %q", a.draftPath(), a.badDraftPath(time.Now()))
	}
	a.loadTemplates()
	a.changed(a.editor().View())
	a.flushSave()
	a.removeDraft()
	if c := a.openCache(""); c != nil {
		t.Error("кэш без папки открылся")
	}
	if err := a.ShowDataFolder(); err == nil {
		t.Error("ShowDataFolder без папки данных не отказал")
	}
	assertEmptyDir(t, cwd)
}
