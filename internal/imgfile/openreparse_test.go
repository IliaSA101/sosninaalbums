package imgfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestOpenNoReparseCheckLocal: пустое, относительное и с NUL имя отказывает до обращения к ОС с понятной ошибкой;
// обычный абсолютный путь открывается как прежде.
func TestOpenNoReparseCheckLocal(t *testing.T) {
	dir := testkit.TempDir(t)
	file := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(file, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"", "a.bin", filepath.Join("sub", "a.bin"), file + "\x00"} {
		f, err := OpenNoReparse(p)
		if err == nil {
			_ = f.Close()
			t.Fatalf("%q: ожидалась ошибка", p)
		}
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%q: отказ должен быть до ОС, а не «нет файла»: %v", p, err)
		}
	}
	f, err := OpenNoReparse(file)
	if err != nil {
		t.Fatalf("абсолютный путь: %v", err)
	}
	defer func() { _ = f.Close() }()
	b := make([]byte, 3)
	if n, _ := f.Read(b); string(b[:n]) != "abc" {
		t.Fatalf("прочитано %q", b[:n])
	}
}
