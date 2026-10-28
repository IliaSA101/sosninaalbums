package prefs

import (
	"os"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// config.json и templates.json на Linux и Mac — только для владельца (аудит 0.23-sec); круг
// «запись → чтение» при этом цел, а повторная запись права не расширяет.
func TestConfigFilesPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("н/п: на Windows биты прав не действуют")
	}
	dir := testkit.TempDir(t)
	for range 2 { // вторая запись заменяет файл: права те же
		UpdateConfig(dir, func(c *Config) { c.Level = LevelFull })
		var tp Templates
		if err := tp.Store(dir, []model.Template{testTpl("а"), testTpl("б")}, os.Rename); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{ConfigPath(dir), TemplatesPath(dir)} {
			st, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if st.Mode().Perm() != 0o600 {
				t.Errorf("%s: права %v, ждали 0600", p, st.Mode().Perm())
			}
		}
	}
	if c := loadConfig(dir); c.Level != LevelFull {
		t.Errorf("конфиг после круга: %+v", c)
	}
	re := LoadTemplates(dir)
	if l := re.List(); len(l) != 2 {
		t.Errorf("шаблоны после круга: %d", len(l))
	}
}
