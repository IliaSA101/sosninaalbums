package prefs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// resetMem очищает настройки в памяти (dir == "") до и после теста: переменная общая.
func resetMem(t *testing.T) {
	t.Helper()
	reset := func() { cfgMu.Lock(); memCfg = nil; cfgMu.Unlock() }
	reset()
	t.Cleanup(reset)
}

func assertNoFiles(t *testing.T, dir string) {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil || len(list) != 0 {
		t.Fatalf("в текущей папке появились файлы: %v %v", list, err)
	}
}

// Папки данных нет (""): настройки живут в памяти, круг запись → чтение работает, на диск ничего
// не пишется, пустые значения не ломают память (140 Н4).
func TestConfigNoDirMemory(t *testing.T) {
	resetMem(t)
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	if ConfigPath("") != "" || TemplatesPath("") != "" {
		t.Fatalf("пути без папки: %q %q", ConfigPath(""), TemplatesPath(""))
	}
	if c := LoadConfig(""); c.Level != "" || c.LastProject != "" {
		t.Fatalf("пустая память: %+v", c)
	}
	UpdateConfig("", func(c *Config) { c.Level = "pro"; c.Recent = []string{"/a", "/b"} })
	UpdateConfig("", func(c *Config) { c.LastProject = "/a" }) // вторая правка не затирает первую
	c := LoadConfig("")
	if c.Level != "pro" || c.LastProject != "/a" || len(c.Recent) != 2 {
		t.Fatalf("круг через память: %+v", c)
	}
	UpdateConfig("", func(c *Config) { c.Recent = nil }) // пустое значение исчезает
	if got := LoadConfig("").Recent; got != nil {
		t.Fatalf("пустой список: %v", got)
	}
	RememberDir("", DirProject, cwd)
	RememberDecor("", "bg", "sa:sage")
	if LoadConfig("").LastDirs[DirProject] != cwd {
		t.Error("RememberDir не в памяти")
	}
	assertNoFiles(t, cwd)
}

// Шаблоны без папки данных: Load пуст без ошибки, Store держит несколько шаблонов в памяти,
// файла нет.
func TestTemplatesNoDirMemory(t *testing.T) {
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	tp := LoadTemplates("")
	if tp.unread || len(tp.List()) != 0 {
		t.Fatalf("Load без папки: %+v", tp)
	}
	if err := tp.Store("", []model.Template{testTpl("один"), testTpl("два")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	if l := tp.List(); len(l) != 2 || l[1].Name != "два" {
		t.Fatalf("в памяти: %+v", l)
	}
	if err := tp.Store("", nil, os.Rename); err != nil || len(tp.List()) != 0 { // пустой список
		t.Fatalf("пустой список: %v %v", err, tp.List())
	}
	if err := tp.Ensure(""); err != nil {
		t.Fatal(err)
	}
	assertNoFiles(t, cwd)
}

// DialogDir без домашней папки не отдаёт относительное «Documents», даже если такая папка есть в
// текущей.
func TestDialogDirNoHomeNotRelative(t *testing.T) {
	resetMem(t)
	for _, k := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"} {
		t.Setenv(k, "")
	}
	cwd := testkit.TempDir(t)
	if err := os.Mkdir(filepath.Join(cwd, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	if got := DialogDir("", DirPhotos); got != "" {
		t.Fatalf("DialogDir = %q, ждали пусто", got)
	}
}
