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
		if err := checkDACLPath(dir, tu.User.Sid); err != nil {
			t.Fatalf("checkDACL после Start: %v", err)
		}
		t.Logf("DACL проверен после Start: владелец %s, защита от наследования, 2 записи", tu.User.Sid)
		client(t, r.sock, "dacl")
		r.svc.Close()
		r.start(t) // папка уже наша — снова проходит
		if err := checkDACLPath(dir, tu.User.Sid); err != nil {
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

// TestSocketDACLHandle: права пишутся по дескриптору самой папки (0.32-148-6). Точка соединения
// на месте mcp — errNoPerm, цель не тронута; openDirNoFollow отказывает точке соединения и файлу
// по атрибутам дескриптора (ветка подмены после Lstat); DACL, записанный и прочитанный по
// дескриптору, проходит сверку; пока дескриптор жив, mcp не переименовать, после release — можно.
func TestSocketDACLHandle(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	user := tu.User.Sid
	t.Run("точка соединения", func(t *testing.T) {
		base := shortDir(t)
		dir, target := filepath.Join(base, "mcp"), filepath.Join(base, "чужая")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", dir, target).CombinedOutput(); err != nil {
			t.Skipf("точка соединения не создана: %v %s", err, out)
		}
		// Lstat видит точку соединения как ModeIrregular и отказывает раньше, чем дело доходит до
		// дескриптора, — поэтому ветку дескриптора (подмена после Lstat) проверяем напрямую.
		wantNoPerm(t, dir)
		if _, err := secureDir(dir); !errors.Is(err, errNoPerm) {
			t.Fatalf("secureDir = %v, ждали errNoPerm", err)
		}
		if checkDACLPath(target, user) == nil {
			t.Fatal("права записаны в цель точки соединения")
		}
	})
	t.Run("обычная папка и файл", func(t *testing.T) {
		dir := filepath.Join(shortDir(t), "mcp")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		h, err := openDirNoFollow(dir)
		if err != nil {
			t.Fatalf("обычная папка: %v", err)
		}
		_ = windows.CloseHandle(h)
		file := filepath.Join(shortDir(t), "файл")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		wantNoPerm(t, file)
	})
	t.Run("дескриптор держит папку", func(t *testing.T) {
		dir := filepath.Join(shortDir(t), "mcp")
		release, err := secureDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		held := true
		defer func() {
			if held {
				release()
			}
		}()
		if err := checkDACLPath(dir, user); err != nil {
			t.Fatalf("DACL после secureDir: %v", err)
		}
		moved := dir + "-moved"
		if err := os.Rename(dir, moved); err == nil {
			t.Fatal("папка с открытым дескриптором переименована")
		}
		release()
		held = false
		if err := os.Rename(dir, moved); err != nil {
			t.Fatalf("после release переименование не прошло: %v", err)
		}
	})
	t.Run("Listen внутри удержанной папки", func(t *testing.T) {
		r := newRig(t, Options{}, "")
		r.start(t) // open держит дескриптор до конца Listen: bind в папке проходит
		client(t, r.sock, "handle")
	})
}

// wantNoPerm: openDirNoFollow(p) отказывает с errNoPerm и не оставляет открытый дескриптор.
func wantNoPerm(t *testing.T, p string) {
	t.Helper()
	h, err := openDirNoFollow(p)
	if err == nil {
		_ = windows.CloseHandle(h)
	}
	if !errors.Is(err, errNoPerm) {
		t.Fatalf("openDirNoFollow(%s) = %v, ждали errNoPerm", filepath.Base(p), err)
	}
}

// checkDACLPath — checkDACL по пути папки (в программе дескриптор читается по открытому handle).
func checkDACLPath(dir string, user *windows.SID) error {
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	return checkDACL(sd, user)
}
