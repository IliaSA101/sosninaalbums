package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// BookDir (047, 069): нет — путь без создания, есть — та же, выбрана сама папка (регистр не
// важен) — она; файл на месте и негодное имя — отказ со словом вызывающего. Ссылки и junction —
// TestBookDir в export/printshop.
func TestBookDir(t *testing.T) {
	parent := testkit.TempDir(t)
	want := filepath.Join(parent, "Иванов")
	if dir, ok, err := BookDir(parent, "Иванов", "альбома"); err != nil || ok || dir != want {
		t.Fatalf("нет папки: %q %v %v", dir, ok, err)
	}
	if _, err := os.Stat(want); err == nil {
		t.Fatal("BookDir создал папку")
	}
	if err := os.Mkdir(want, 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, ok, err := BookDir(parent, "Иванов", "альбома"); err != nil || !ok || dir != want {
		t.Fatalf("есть папка: %q %v %v", dir, ok, err)
	}
	if dir, ok, err := BookDir(want, "ИВАНОВ", "альбома"); err != nil || !ok || dir != want {
		t.Fatalf("выбрана сама папка: %q %v %v", dir, ok, err)
	}
	if err := os.WriteFile(filepath.Join(parent, "Петров"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BookDir(parent, "Петров", "альбома"); err == nil || !strings.Contains(err.Error(), "папки альбома «Петров»") {
		t.Fatalf("файл на месте папки: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "a/b"} {
		if _, _, err := BookDir(parent, bad, "книги"); err == nil || !strings.Contains(err.Error(), "папки книги") {
			t.Errorf("имя %q: %v", bad, err)
		}
	}
}
