//go:build !windows

package peercred

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const foreignUser = "nobody"

// skipUnlessForeign: чужая учётная запись — только CI (SA_PEER_FOREIGN=sudo) и только с sudo без пароля.
func skipUnlessForeign(t *testing.T) {
	t.Helper()
	switch os.Getenv("SA_PEER_FOREIGN") {
	case "":
		t.Skip("чужая учётная запись — только CI: SA_PEER_FOREIGN=sudo")
	case "sudo":
	default:
		t.Fatalf("SA_PEER_FOREIGN=%q: ждали sudo", os.Getenv("SA_PEER_FOREIGN"))
	}
	if err := exec.Command("sudo", "-n", "-u", foreignUser, "true").Run(); err != nil {
		t.Skipf("нет sudo -n -u %s: %v", foreignUser, err)
	}
}

// foreignDir — свежая папка 0755 с подпапкой s 0777, куда чужой может писать (сокет, «готов»); убирается в конце теста.
func foreignDir(t *testing.T) string {
	t.Helper()
	// Именно /tmp, не os.TempDir: на macOS это /var/folders/…/T с правами 0700, чужой туда не войдёт.
	dir, err := os.MkdirTemp("/tmp", "sa-peer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeDir(t, dir) })
	if err := os.Chmod(dir, 0o755); err != nil { // копия бинарника: читать и запускать всем
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "s")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o777); err != nil { // сокет и «готов» создаёт чужой
		t.Fatal(err)
	}
	return dir
}

// foreignCmd запускает копию бинарника от nobody; переменные идут через env — sudo сбрасывает окружение.
func foreignCmd(_ *testing.T, exe, mode, sock string) *exec.Cmd {
	return exec.Command("sudo", "-n", "-u", foreignUser, "env", helperEnv+"="+mode, sockEnv+"="+sock, exe)
}
