package prefs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func testTpl(name string) model.Template {
	return model.Template{ID: model.NewID(), Name: name, Scope: model.TplSpread,
		Boxes: []model.Box{{W: 0.5, H: 0.5}, {X: 0.5, Y: 0.5, W: 0.5, H: 0.5}}}
}

// loadLocked — LoadTemplates, пока открытие файла падает (замок антивируса).
func loadLocked(t *testing.T, dir string) Templates {
	t.Helper()
	old := openTplFile
	t.Cleanup(func() { openTplFile = old })
	openTplFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	tp := LoadTemplates(dir)
	openTplFile = old
	if !tp.unread || tp.bad {
		t.Fatalf("ошибка открытия: unread=%v bad=%v", tp.unread, tp.bad)
	}
	return tp
}

func noBad(t *testing.T, dir string) {
	t.Helper()
	if bad, _ := filepath.Glob(filepath.Join(dir, "templates.bad-*.json")); len(bad) != 0 {
		t.Fatalf("исправный файл ушёл в .bad: %v", bad)
	}
}

// Файл не открылся при старте, но исправен: Store перечитывает его и дописывает новый
// шаблон, ничего не уводя в .bad.
func TestTemplatesOpenErrorRereads(t *testing.T) {
	dir := testkit.TempDir(t)
	var first Templates
	if err := first.Store(dir, []model.Template{testTpl("старый")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	if err := tp.Store(dir, []model.Template{testTpl("новый")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	noBad(t, dir)
	re := LoadTemplates(dir)
	if l := re.List(); len(l) != 2 || l[0].Name != "старый" || l[1].Name != "новый" {
		t.Fatalf("файл после записи: %+v", l)
	}
	if l := tp.List(); len(l) != 2 || tp.unread {
		t.Fatalf("память после записи: %+v unread=%v", l, tp.unread)
	}
}

// Файл снова не открылся — ошибка, файл не переписан и не уведён в .bad.
func TestTemplatesOpenErrorTwice(t *testing.T) {
	dir := testkit.TempDir(t)
	content := []byte(`{"version":1,"templates":[]}`)
	if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	openTplFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	if err := tp.Store(dir, []model.Template{testTpl("n")}, os.Rename); err == nil {
		t.Fatal("запись при неоткрытом файле прошла")
	}
	noBad(t, dir)
	if got, _ := os.ReadFile(TemplatesPath(dir)); !bytes.Equal(got, content) {
		t.Fatalf("файл переписан: %s", got)
	}
	if !tp.unread {
		t.Fatal("после повторной ошибки отметка unread снята")
	}
}

// Файл не открылся при старте, а при перечитывании оказался битым — уходит в .bad.
func TestTemplatesOpenErrorThenBroken(t *testing.T) {
	dir := testkit.TempDir(t)
	content := []byte(`{"version":1,"templates":[`)
	if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	if err := tp.Store(dir, []model.Template{testTpl("n")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	bad, err := filepath.Glob(filepath.Join(dir, "templates.bad-*.json"))
	if err != nil || len(bad) != 1 {
		t.Fatalf("битый файл должен уйти в .bad: %v %v", bad, err)
	}
	if old, _ := os.ReadFile(bad[0]); !bytes.Equal(old, content) {
		t.Fatal("в .bad лежит не исходный файл")
	}
	re := LoadTemplates(dir)
	if l := re.List(); len(l) != 1 || l[0].Name != "n" {
		t.Fatalf("новый файл: %+v", l)
	}
}
