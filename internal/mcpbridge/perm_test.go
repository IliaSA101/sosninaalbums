package mcpbridge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestBridgeRefusesOpenDir: папка сокета с чужими правами, ссылка вместо папки, обычный файл
// вместо сокета — dial не делается (программа подключения не видит), заглушка говорит про
// небезопасную настройку. Проверка прав папки — Unix; Windows проверяет владельца и DACL
// (winqa, TestSocketDACL).
func TestBridgeRefusesOpenDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права папки Unix; Windows — владелец и DACL, winqa")
	}
	cases := map[string]func(t *testing.T, dir, sock string) string{
		"0755": func(t *testing.T, dir, sock string) string {
			if err := os.Chmod(filepath.Dir(sock), 0o755); err != nil {
				t.Fatal(err)
			}
			return sock
		},
		"0770": func(t *testing.T, dir, sock string) string {
			if err := os.Chmod(filepath.Dir(sock), 0o770); err != nil {
				t.Fatal(err)
			}
			return sock
		},
		"папка — ссылка": func(t *testing.T, dir, sock string) string {
			link := filepath.Join(dir, "lnk")
			if err := os.Symlink(filepath.Dir(sock), link); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(link, "sock")
		},
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			dir := shortDir(t)
			sock := sockIn(t, dir)
			p := listenProg(t, sock) // слушатель есть: подключись посредник — он бы принял
			bad := mod(t, dir, sock)
			h := startBridge(t, testCfg(bad))
			h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`)
			if got := toolText(t, h.recv()); got != textUnsafe {
				t.Errorf("текст: %q", got)
			}
			p.noConn(150 * time.Millisecond) // и позже по таймеру тоже не подключается
		})
	}
}

// TestBridgeRefusesPlainFile: на месте сокета обычный файл — отказ без dial.
func TestBridgeRefusesPlainFile(t *testing.T) {
	sock := newSock(t)
	if err := os.WriteFile(sock, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := startBridge(t, testCfg(sock))
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"program_status"}}`)
	if got := toolText(t, h.recv()); got != textUnsafe {
		t.Errorf("текст: %q", got)
	}
}

// TestBridgeSecondPath: первый путь списка небезопасен, второй годен — подключается ко второму.
func TestBridgeSecondPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права папки Unix")
	}
	bad := newSock(t)
	if err := os.Chmod(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	good := newSock(t)
	p := listenProg(t, good)
	h := startBridge(t, testCfg(bad, good))
	p.accept()
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	h.none(50 * time.Millisecond) // ушёл в программу
}

func TestWant(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"exe"}, false},
		{[]string{"exe", "--mcp"}, true},
		{[]string{"exe", "--mcp", "x"}, true},
		{[]string{"exe", "mcp"}, false},
		{[]string{"exe", "a.salbum", "--mcp"}, false},
		{[]string{"exe", "--MCP"}, false},
	}
	for _, c := range cases {
		if got := Want(c.args); got != c.want {
			t.Errorf("Want(%q) = %v", c.args, got)
		}
	}
}

// TestSockPaths: список путей — только абсолютные, без повторов; нет дома и папки настроек — пусто.
func TestSockPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("на Windows путь по известной папке, а не по HOME")
	}
	prev := userHome
	t.Cleanup(func() { userHome = prev })
	userHome = func() string { return "" }

	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := SockPaths("SosninaAlbums"); len(got) != 0 {
		t.Errorf("без дома: %v", got)
	}
	t.Setenv("HOME", "relative/home")
	if got := SockPaths("SosninaAlbums"); len(got) != 0 {
		t.Errorf("относительный HOME дал путь: %v", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "rel/cfg")
	if got := SockPaths("SosninaAlbums"); len(got) != 0 {
		t.Errorf("относительный XDG дал путь: %v", got)
	}

	home := testkit.TempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	got := SockPaths("SosninaAlbums")
	if len(got) == 0 || len(got) > 2 {
		t.Fatalf("с домом: %v", got)
	}
	for _, p := range got {
		if !filepath.IsAbs(p) || filepath.Base(p) != "sock" {
			t.Errorf("путь %q", p)
		}
	}
	if runtime.GOOS == "linux" {
		other := testkit.TempDir(t)
		t.Setenv("XDG_CONFIG_HOME", other)
		got := SockPaths("SosninaAlbums")
		want := []string{
			filepath.Join(other, "SosninaAlbums", "mcp", "sock"),
			filepath.Join(home, ".config", "SosninaAlbums", "mcp", "sock"),
		}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("XDG ≠ ~/.config: %v, ждали %v", got, want)
		}
	}
	// Дом из os/user, когда HOME пуст.
	t.Setenv("HOME", "")
	userHome = func() string { return home }
	if runtime.GOOS == "linux" {
		if got := SockPaths("SosninaAlbums"); len(got) == 0 {
			t.Errorf("дом из os/user не использован")
		}
	}
	if got := SockPaths(""); len(got) != 0 {
		t.Errorf("пустое имя данных: %v", got)
	}
}
