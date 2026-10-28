//go:build linux

package libpacks

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Именованный канал на месте листа: Lstat отсекает его до открытия, чтение не зависает.
func TestPreviewNotRegularFIFO(t *testing.T) {
	fx := newPrevFx(t, nil)
	p := previewOf("", sheetJPEG(t, 1))
	dir := filepath.Join(fx.dir, previewDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, p.SHA256+previewExt), 0o600); err != nil {
		t.Skipf("mkfifo недоступен: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := fx.s.readPreview(p); done <- err }()
	select {
	case err := <-done:
		if err != errNoSheet {
			t.Fatalf("канал: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("чтение канала зависло")
	}
}
