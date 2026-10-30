package testkit

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// DirLink — ссылка link на папку target: символьная, без прав на неё (Windows) — junction;
// не вышло — t.Skip с причиной.
func DirLink(t testing.TB, target, link string) string {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("ссылку на папку не создать: %v", err)
		}
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil { //nolint:gosec // пути теста
			t.Skipf("ни ссылки, ни junction (%v: %s)", err, out)
		}
	}
	return link
}

// MissingDrive — корень свободной буквы диска («Q:\»): том, которого нет, — как отключённый
// внешний диск. Не Windows или все буквы заняты — t.Skip.
func MissingDrive(t testing.TB) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("буквы дисков — только Windows")
	}
	for c := 'Z'; c >= 'D'; c-- {
		root := string(c) + `:\`
		if _, err := os.Lstat(root); err != nil {
			return root
		}
	}
	t.Skip("свободной буквы диска нет")
	return ""
}
