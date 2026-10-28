package agents

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestSocketDACL: папка mcp получает защищённый DACL «пользователь + SYSTEM» и владельца-пользователя
// (в том числе у процесса «от администратора»); готовая папка с обычными правами исправляется;
// повторный запуск проходит; точка соединения на месте папки — отказ.
func TestSocketDACL(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("готовая папка и повторный запуск", func(t *testing.T) {
		r := newRig(t, Options{}, "")
		dir := filepath.Dir(r.sock)
		if err := os.MkdirAll(dir, 0o755); err != nil { // права по наследованию от родителя
			t.Fatal(err)
		}
		r.start(t)
		if err := checkDACL(dir, tu.User.Sid); err != nil {
			t.Fatalf("checkDACL после Start: %v", err)
		}
		t.Logf("DACL проверен после Start: владелец %s, защита от наследования, 2 записи", tu.User.Sid)
		client(t, r.sock, "dacl")
		r.svc.Close()
		r.start(t) // папка уже наша — снова проходит
		if err := checkDACL(dir, tu.User.Sid); err != nil {
			t.Fatalf("checkDACL после повторного Start: %v", err)
		}
	})
	t.Run("точка соединения", func(t *testing.T) {
		r := newRig(t, Options{}, "")
		dir := filepath.Dir(r.sock)
		target := filepath.Join(filepath.Dir(dir), "чужая")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", dir, target).CombinedOutput(); err != nil {
			t.Skipf("точка соединения не создана: %v %s", err, out)
		}
		if err := r.svc.Start(); !errors.Is(err, errNoPerm) {
			t.Fatalf("Start = %v, ждали errNoPerm", err)
		}
		if _, err := os.Lstat(filepath.Join(target, "sock")); err == nil {
			t.Fatal("сокет создан по точке соединения")
		}
	})
}
