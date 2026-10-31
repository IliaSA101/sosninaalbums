//go:build windows

package agentplaces

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
	e := &Env{Rules: New(Opts{Windows: true, Timeout: time.Second, Disk: diskOK}), System: "C:"}
	viaLink := filepath.Join(link, "Фото") // junction в середине
	for name, dir := range map[string]string{"конец": link, "середина": viaLink} {
		if !HasLinkPart(dir) {
			t.Errorf("%s: junction не найден", name)
		}
		if _, err := e.Admit(dir, prefs.PlaceParent); err == nil {
			t.Errorf("%s: место через junction принято", name)
		}
	}
	if HasLinkPart(target) {
		t.Error("обычная папка считается ссылкой")
	}
}
