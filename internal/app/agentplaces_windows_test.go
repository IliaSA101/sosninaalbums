//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Winqa: junction (`mklink /J`) Go не раскрывает (ModeIrregular), поэтому место проверяется по звеньям
// пути: junction в конце и в середине — не место (сканер из корня-ссылки ничего не берёт).
func TestQuietJunction(t *testing.T) {
	base := testkit.TempDir(t)
	target := filepath.Join(base, "цель", "Фото")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "связь")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Dir(target)).CombinedOutput(); err != nil {
		t.Skipf("mklink /J не удалось: %v: %s", err, out)
	}
	e := &placeEnv{rules: permissiveRules(), system: "C:"}
	viaLink := filepath.Join(link, "Фото") // junction в середине
	for name, dir := range map[string]string{"конец": link, "середина": viaLink} {
		if !hasLinkPart(dir) {
			t.Errorf("%s: junction не найден", name)
		}
		if _, err := e.admit(dir, prefs.PlaceParent); err == nil {
			t.Errorf("%s: место через junction принято", name)
		}
	}
	if hasLinkPart(target) {
		t.Error("обычная папка считается ссылкой")
	}
}
