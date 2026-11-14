package osx

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// FreeBytes (069): у существующей папки место больше нуля, у несуществующей — ошибка, а не 0
// (иначе сбор сказал бы «не хватает места» вместо настоящей причины).
func TestFreeBytes(t *testing.T) {
	dir := testkit.TempDir(t)
	n, err := FreeBytes(dir)
	if err != nil || n == 0 {
		t.Fatalf("FreeBytes(%s) = %d, %v", dir, n, err)
	}
	if n, err := FreeBytes(filepath.Join(dir, "нет", "такой")); err == nil {
		t.Fatalf("FreeBytes несуществующей папки = %d без ошибки", n)
	}
	if _, err := FreeBytes("a\x00b"); err == nil {
		t.Fatal("FreeBytes пути с NUL без ошибки")
	}
}
