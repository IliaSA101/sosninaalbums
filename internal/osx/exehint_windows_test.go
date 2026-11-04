package osx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestExeHintShortTemp: %TMP% в 8.3-записи, а os.Executable отдаёт длинный путь — exe под Temp
// узнаётся как временный (аудит 143 Г5 п. 4). shortPath — из launch_windows_test.go.
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
	short := shortPath(long)
	if short == "" || short == long {
		t.Skip("н/п: на томе нет коротких имён 8.3")
	}
	yes := func(string) bool { return true }
	if ExeHint(exe, "windows", short, yes) == "" {
		t.Errorf("exe %q под Temp %q (8.3) не признан временным", exe, short)
	}
	if ExeHint(exe, "windows", filepath.Join(filepath.Dir(long), "Other"), yes) != "" {
		t.Error("exe вне Temp признан временным")
	}
}
