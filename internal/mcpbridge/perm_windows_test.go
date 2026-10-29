package mcpbridge

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestSockDirWindows: проверка папки сокета на Windows. Годна — владелец свой SID и в DACL только
// «свой SID + SYSTEM»; отказ (errUnsafe, а на уровне посредника textUnsafe) — «Администраторы»,
// «Все», запрещающая запись, чужой владелец, точка соединения вместо папки.
func TestSockDirWindows(t *testing.T) {
	me := userSID(t).String()
	base := "D:P(A;OICI;FA;;;" + me + ")(A;OICI;FA;;;SY)"
	cases := []struct {
		name   string
		mutate func(t *testing.T, dir string)
	}{
		{"Администраторы в DACL", func(t *testing.T, dir string) { setDACL(t, dir, true, base+"(A;OICI;FA;;;BA)") }},
		{"Все в DACL", func(t *testing.T, dir string) { setDACL(t, dir, true, base+"(A;OICI;FA;;;WD)") }},
		{"запрещающая запись", func(t *testing.T, dir string) { setDACL(t, dir, true, "D:P(D;OICI;FA;;;WD)"+base[3:]) }},
		{"чужой владелец", func(t *testing.T, dir string) {
			admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
			if err != nil {
				t.Fatal(err)
			}
			if err := trySetDACL(dir, admins, false, base); err != nil {
				t.Skipf("процесс не может назначить владельца Администраторы: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sock := newSock(t)
			tc.mutate(t, filepath.Dir(sock))
			if _, err := checkOnly(sock); !errors.Is(err, errUnsafe) {
				t.Fatalf("checkSock: %v, ждали errUnsafe", err)
			}
			h := startBridge(t, testCfg(sock))
			h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`)
			if got := toolText(t, h.recv()); got != textUnsafe {
				t.Errorf("текст: %q", got)
			}
		})
	}

	t.Run("точка соединения вместо папки", func(t *testing.T) {
		good := newSock(t)
		linkParent := shortDir(t)
		link := filepath.Join(linkParent, "lnk")
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Dir(good)).CombinedOutput(); err != nil {
			t.Skipf("mklink /J не удалась: %v %s", err, out)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
		if _, err := checkOnly(filepath.Join(link, "sock")); !errors.Is(err, errUnsafe) {
			t.Fatalf("checkSock через точку соединения: %v, ждали errUnsafe", err)
		}
	})

	t.Run("годная папка: сокет найден", func(t *testing.T) {
		sock := newSock(t)
		if present, err := checkOnly(sock); present || err != nil {
			t.Fatalf("без сокета: present=%v err=%v", present, err)
		}
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ln.Close() })
		if present, err := checkOnly(sock); !present || err != nil {
			t.Fatalf("с сокетом: present=%v err=%v", present, err)
		}
	})

	t.Run("обычный файл на месте сокета", func(t *testing.T) {
		sock := newSock(t)
		if err := os.WriteFile(sock, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := checkOnly(sock); !errors.Is(err, errUnsafe) {
			t.Fatalf("файл вместо сокета: %v", err)
		}
	})
}
