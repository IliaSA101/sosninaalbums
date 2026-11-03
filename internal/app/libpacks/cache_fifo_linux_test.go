//go:build linux

package libpacks

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Именованный канал на месте файла списка: Lstat отсекает его до открытия, чтение не зависает
// (open канала без писателя ждал бы вечно) — безопасность З1.
func TestReadCatalogFIFO(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), catalogFile)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo недоступен: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := readCatalog(path, func([]byte) (fetch.Catalog, error) {
			t.Error("parse вызван для канала")
			return fetch.Catalog{}, nil
		})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, errCacheKind) {
			t.Fatalf("канал: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("чтение канала зависло")
	}
}
