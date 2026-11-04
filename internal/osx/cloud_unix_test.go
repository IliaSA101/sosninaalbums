//go:build !windows

package osx

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCloudRootsBadFilesUnix: вместо info.json FIFO или ссылка на настоящий файл — облака нет, чтение не виснет.
func TestCloudRootsBadFilesUnix(t *testing.T) {
	good := testkit.AbsPath("Данные/Dropbox")
	target := filepath.Join(testkit.TempDir(t), "info.json")
	writeCloudFile(t, target, dropboxJSON(t, good, ""))

	for name, mk := range map[string]func(dst string) error{
		"FIFO":   func(dst string) error { return syscall.Mkfifo(dst, 0o600) },
		"ссылка": func(dst string) error { return os.Symlink(target, dst) },
	} {
		home := testkit.TempDir(t)
		dst := dropboxFile(t, home)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := mk(dst); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		done := make(chan []string, 1)
		go func() { done <- CloudRoots(home, allowAll) }()
		select {
		case got := <-done:
			if len(got) != 0 {
				t.Errorf("%s: корни %q", name, got)
			}
		case <-time.After(testkit.Limit(5 * time.Second)):
			t.Fatalf("%s: чтение повисло", name)
		}
	}
}

// TestCloudRootsOpenFIFO: подмена на FIFO уже после Lstat — открытие не ждёт писателя (O_NONBLOCK), а Stat отсекает не-файл.
func TestCloudRootsOpenFIFO(t *testing.T) {
	fifo := filepath.Join(testkit.TempDir(t), "info.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	type res struct {
		f   *os.File
		err error
	}
	done := make(chan res, 1)
	go func() {
		f, err := openCloudFile(fifo)
		done <- res{f, err}
	}()
	select {
	case r := <-done:
		if r.err == nil {
			st, err := r.f.Stat()
			_ = r.f.Close()
			if err != nil || st.Mode().IsRegular() {
				t.Errorf("FIFO открыт как файл: %v %v", st, err)
			}
		}
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("открытие FIFO повисло")
	}
}
