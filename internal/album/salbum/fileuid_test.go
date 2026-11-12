package salbum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// FileUID — отпечаток из шапки файла «Сохранить как» (167-25): чужие данные, любой сбой — "".
func TestFileUID(t *testing.T) {
	dir := testkit.TempDir(t)
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	uid := model.NewUID()
	cases := []struct {
		name string
		path string
		want string
	}{
		{"годный", write("ok.salbum", `{"version":19,"uid":"`+uid+`","spreads":[]}`), uid},
		{"нет поля", write("none.salbum", `{"version":19}`), ""},
		{"верхний регистр", write("up.salbum", `{"uid":"`+strings.ToUpper(uid)+`"}`), ""},
		{"короткий", write("short.salbum", `{"uid":"abc"}`), ""},
		{"не строка", write("num.salbum", `{"uid":7}`), ""},
		{"не JSON", write("junk.salbum", "\x00\x01 не json"), ""},
		{"пустой файл", write("empty.salbum", ""), ""},
		{"усечённый", write("cut.salbum", `{"version":19,"uid":"`+uid), ""},
		{"нет файла", filepath.Join(dir, "нет.salbum"), ""},
		{"папка", dir, ""},
		{"пустой путь", "", ""},
		{"относительный путь", "относительный.salbum", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FileUID(c.path); got != c.want {
				t.Errorf("FileUID(%q) = %q, ждали %q", c.path, got, c.want)
			}
		})
	}
}

// Относительный путь отвергается, даже когда файл по нему есть и годен: отказ — от CheckLocalPath,
// а не от отсутствия файла.
func TestFileUIDRelativeExisting(t *testing.T) {
	dir := testkit.TempDir(t)
	uid := model.NewUID()
	body := `{"version":19,"uid":"` + uid + `"}`
	if err := os.WriteFile(filepath.Join(dir, "rel.salbum"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if got := FileUID("rel.salbum"); got != "" {
		t.Errorf("относительный путь принят: %q", got)
	}
	if got := FileUID(filepath.Join(dir, "rel.salbum")); got != uid {
		t.Errorf("тот же файл по полному пути: %q, ждали %q", got, uid)
	}
}

// Файл больше предела не читается: ReadLimited отказывает до разбора.
func TestFileUIDHugeFile(t *testing.T) {
	if testkit.Lite() {
		t.Skip("50 МБ на диск — не быстрый уровень")
	}
	uid := model.NewUID()
	p := filepath.Join(testkit.TempDir(t), "huge.salbum")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"uid":"` + uid + `","pad":"`); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(model.MaxFileSize + 10); err != nil { // разреженный файл
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := FileUID(p); got != "" {
		t.Errorf("файл больше предела разобран: %q", got)
	}
}
