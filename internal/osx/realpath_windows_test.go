//go:build windows

package osx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// freeLetter — свободная буква диска с конца алфавита.
func freeLetter(t *testing.T) string {
	t.Helper()
	for c := 'Z'; c >= 'M'; c-- {
		d := string(c) + `:\`
		if _, err := os.Stat(d); err != nil {
			return string(c) + ":"
		}
	}
	t.Skip("свободной буквы диска нет")
	return ""
}

// subst X: <папка>: RealPath раскрывает подставной диск и папку на нём до настоящего пути; обычный
// путь остаётся тем же; несуществующий — ошибка (аудит 0.34, М4). Прогон — winqa.
func TestRealPathSubst(t *testing.T) {
	root, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, ".скрытая")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	letter := freeLetter(t)
	if out, err := exec.Command("subst", letter, root).CombinedOutput(); err != nil {
		t.Skipf("subst недоступен: %v %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("subst", letter, "/D").Run() })

	for _, c := range []struct{ in, want string }{
		{letter + `\.скрытая`, sub},
		{letter + `\`, root},
		{root, root},
	} {
		got, err := RealPath(c.in)
		if err != nil || !strings.EqualFold(got, c.want) {
			t.Errorf("RealPath(%q) = %q, %v; ждали %q", c.in, got, err, c.want)
		}
	}
	if _, err := RealPath(letter + `\нет-такой`); err == nil {
		t.Error("несуществующая папка принята")
	}
}
