//go:build windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// diskNever — шаг 1 должен отказать до диска: любое обращение к диску — провал теста.
func diskNever(t *testing.T) func(string) (string, error) {
	return func(p string) (string, error) {
		t.Errorf("обращение к диску для %q: строка должна была отказать раньше", p)
		return "", errors.New("диск")
	}
}

// Winqa: UNC в подсказке выгрузки — не место записи, ещё до обращения к диску (У61).
func TestAgentExportHintUNC(t *testing.T) {
	o := permissiveOpts()
	o.Disk = diskNever(t)
	r := agentplaces.New(o)
	for _, p := range []string{`\\server\share\Петровы`, `\\?\C:\Выгрузки`, `//server/share/x`, `\\localhost\c$\x`} {
		if _, err := r.WriteDir(p); err == nil {
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
	r := permissiveOnDisk()
	shown := filepath.Join(own, "Новая")
	w, err := r.WriteDir(shown)
	if err != nil || !w.Create() {
		t.Fatalf("показанная: %+v %v", w, err)
	}
	other := filepath.Join(base, "Чужая")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", shown, other).CombinedOutput(); err != nil {
		t.Skipf("mklink /J не удалось: %v: %s", err, out)
	}
	if _, err := r.ExportPick(shown); err == nil {
		t.Error("подмена junction между вопросом и щелчком не поймана")
	}
	if !agentplaces.HasLinkPart(shown) {
		t.Error("junction не найден по звеньям пути")
	}
}
