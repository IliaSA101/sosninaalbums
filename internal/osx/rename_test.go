package osx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Занятое имя — ошибка fs.ErrExist, оба файла целы; свободное — файл переехал; источника нет —
// ошибка, но не «занято».
func TestRenameNoReplace(t *testing.T) {
	dir := testkit.TempDir(t)
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	for p, s := range map[string]string{a: "a", b: "b"} {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := RenameNoReplace(a, b); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("занятое имя: %v", err)
	}
	for p, want := range map[string]string{a: "a", b: "b"} {
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Fatalf("%s: %q, ждали %q", filepath.Base(p), got, want)
		}
	}
	if err := RenameNoReplace(a, c); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(c); string(got) != "a" {
		t.Fatalf("после переименования: %q", got)
	}
	if _, err := os.Lstat(a); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("старое имя осталось: %v", err)
	}
	if err := RenameNoReplace(a, filepath.Join(dir, "d")); err == nil || errors.Is(err, fs.ErrExist) {
		t.Fatalf("нет источника: %v", err)
	}
}
