package agents

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestExeHintShortTemp: %TMP% в 8.3-записи, а os.Executable отдаёт длинный путь — exe под Temp
// узнаётся как временный (аудит 143 Г5 п. 4).
func TestExeHintShortTemp(t *testing.T) {
	long := testkit.TempDir(t)
	dir := filepath.Join(long, "Длинное имя папки для 8.3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "SosninaAlbums.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := windows.UTF16PtrFromString(long)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		t.Skipf("8.3-имена недоступны: %v", err)
	}
	short := windows.UTF16ToString(buf[:n])
	if short == long {
		t.Skip("на томе нет 8.3-имён")
	}
	yes := func(string) bool { return true }
	if ExeHint(exe, "windows", short, yes) == "" {
		t.Errorf("exe %q под Temp %q (8.3) не признан временным", exe, short)
	}
	if ExeHint(exe, "windows", filepath.Join(filepath.Dir(long), "Other"), yes) != "" {
		t.Error("exe вне Temp признан временным")
	}
}
