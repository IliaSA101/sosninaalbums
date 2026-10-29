package mcpbridge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSockDirHandleWindows: папка читается по дескриптору, а не по пути — точка соединения,
// файл вместо папки и отсутствующая папка отвергаются, годная папка проходит.
func TestSockDirHandleWindows(t *testing.T) {
	good := newSock(t)
	dir := filepath.Dir(good)
	if err := checkDirACL(dir); err != nil {
		t.Fatalf("годная папка: %v", err)
	}
	if err := checkDirACL(filepath.Join(dir, "нет")); err == nil {
		t.Error("отсутствующая папка прошла")
	}
	file := filepath.Join(shortDir(t), "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openDirNoFollow(file); !errors.Is(err, errUnsafe) {
		t.Errorf("файл вместо папки: %v", err)
	}
	link := filepath.Join(shortDir(t), "lnk")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).CombinedOutput(); err != nil {
		t.Skipf("mklink /J не удалась: %v %s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	if _, err := openDirNoFollow(link); !errors.Is(err, errUnsafe) {
		t.Errorf("точка соединения: %v, ждали errUnsafe", err)
	}
	if err := checkDirACL(link); err == nil {
		t.Error("точка соединения прошла проверку прав")
	}
}
