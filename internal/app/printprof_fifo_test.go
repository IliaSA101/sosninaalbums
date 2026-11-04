//go:build linux || darwin

package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Канал вместо файла типографии отклоняется сразу, а не вешает импорт (design §6 п. 1);
// ссылка на канал — так же.
func TestImportFifoRejected(t *testing.T) {
	a, ev := printApp(t)
	dir := testkit.TempDir(t)
	fifo := filepath.Join(dir, "p.saprint")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("канал не создать: %v", err)
	}
	link := filepath.Join(dir, "l.saprint")
	if err := os.Symlink(fifo, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	t.Cleanup(func() { // отпустить возможного читателя, если проверка зависла
		if w, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = w.Close()
		}
	})
	for _, path := range []string{fifo, link} {
		done := make(chan error, 1)
		go func() {
			_, err := a.ownp.ImportPath(path)
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("канал %s принят за файл типографии", filepath.Base(path))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("импорт завис на %s", filepath.Base(path))
		}
	}
	if ownCount(a) != 0 || ev.count("print:changed") != 0 {
		t.Error("канал дошёл до реестра")
	}
}
