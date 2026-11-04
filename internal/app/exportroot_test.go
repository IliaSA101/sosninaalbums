package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Меню «Экспорт…» помнит и папку окна ОС, и корень выгрузок (152-15): обычная папка — корень сама,
// «рядом» (121) корень не меняет, папка окна — как раньше.
func TestExportRememberRootMenu(t *testing.T) {
	e := newExportEnv(t, 2)
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := e.dir
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	c := prefs.LoadConfig(e.a.cfgDir)
	if c.LastDirs[prefs.DirExport] != first || c.LastDirs[prefs.DirExportRoot] != first || prefs.ExportRoot(c) != first {
		t.Fatalf("после выгрузки в обычную папку: %+v", c.LastDirs)
	}

	// «Рядом»: корень остаётся прежним, папка окна — соседняя.
	album := inAlbum(t, e)
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	next := filepath.Join(filepath.Dir(album), "ФА25 — выгрузка")
	c = prefs.LoadConfig(e.a.cfgDir)
	if c.LastDirs[prefs.DirExport] != next || prefs.ExportRoot(c) != first {
		t.Fatalf("после «рядом»: %+v", c.LastDirs)
	}
}
