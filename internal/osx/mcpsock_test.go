package osx

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestMCPSockNoConfigDir: нет папки настроек (ошибка, пусто, относительный путь) — путь "", а не
// «mcp/sock» от текущей папки процесса (ловушка 0.31-144-29).
func TestMCPSockNoConfigDir(t *testing.T) {
	oldR, oldC := roamingDirFn, userConfigDir
	t.Cleanup(func() { roamingDirFn, userConfigDir = oldR, oldC })
	for name, f := range map[string]func() (string, error){
		"ошибка":        func() (string, error) { return "/x", errors.New("нет HOME") },
		"пусто":         func() (string, error) { return "", nil },
		"относительный": func() (string, error) { return filepath.Join("a", "b"), nil },
	} {
		userConfigDir = f
		if runtime.GOOS == "windows" { // там известная папка настоящая и от userConfigDir не зависит
			roamingDirFn = func() string { return absDir(f) }
		}
		if got := MCPSock(DataName("")); got != "" {
			t.Errorf("%s: MCPSock = %q, ждали пусто", name, got)
		}
	}
}

// TestMCPSock: путь — <настройки>/<имя данных>/mcp/sock; у стенда свой; пустое имя — "".
func TestMCPSock(t *testing.T) {
	base := testkit.TempDir(t)
	oldR := roamingDirFn
	t.Cleanup(func() { roamingDirFn = oldR })
	roamingDirFn = func() string { return base }
	if got, want := MCPSock(DataName("")), filepath.Join(base, "SosninaAlbums", "mcp", "sock"); got != want {
		t.Errorf("MCPSock = %q, ждали %q", got, want)
	}
	if got, want := MCPSock(DataName("b")), filepath.Join(base, "SosninaAlbums-stand-b", "mcp", "sock"); got != want {
		t.Errorf("стенд: MCPSock = %q, ждали %q", got, want)
	}
	if got := MCPSock(""); got != "" {
		t.Errorf("пустое имя: %q", got)
	}
}
