//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Winqa: UNC в подсказке выгрузки — не место записи, ещё до обращения к диску (У61).
func TestAgentExportHintUNC(t *testing.T) {
	r := permissiveRules()
	r.disk = diskNever(t)
	for _, p := range []string{`\\server\share\Петровы`, `\\?\C:\Выгрузки`, `//server/share/x`, `\\localhost\c$\x`} {
		if _, err := r.writeDir(p); err == nil {
			t.Errorf("%q принято", p)
		}
	}
}

// Winqa: junction (`mklink /J`) в подсказке — Go его не раскрывает; подмена показанной папки junction между
// вопросом и щелчком ловится повтором (У60).
func TestAgentExportHintJunction(t *testing.T) {
	base := testkit.TempDir(t)
	own := filepath.Join(base, "Выгрузки")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	r := permissiveRules()
	r.disk = diskProbe
	shown := filepath.Join(own, "Новая")
	w, err := r.writeDir(shown)
	if err != nil || !w.create {
		t.Fatalf("показанная: %+v %v", w, err)
	}
	other := filepath.Join(base, "Чужая")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", shown, other).CombinedOutput(); err != nil {
		t.Skipf("mklink /J не удалось: %v: %s", err, out)
	}
	if _, err := r.exportPick(shown); err == nil {
		t.Error("подмена junction между вопросом и щелчком не поймана")
	}
	if !hasLinkPart(shown) {
		t.Error("junction не найден по звеньям пути")
	}
}
