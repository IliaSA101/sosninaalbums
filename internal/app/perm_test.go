package app

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Права на Linux и Mac (аудит 0.23-sec): новая папка данных (и её новые родители) — 0700,
// log.txt — 0600; уже существующие не меняются (владелец мог выставить свои).
func TestDataDirAndLogPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("н/п: на Windows биты прав не действуют")
	}
	root := testkit.TempDir(t)
	nested := filepath.Join(root, "a", "b")
	if err := makeDataDir(nested); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(root, "a"), nested} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v, %v; ждали 0700", p, st, err)
		}
	}
	// Существующая папка с 0755 остаётся как была.
	shared := filepath.Join(root, "shared")
	if err := os.Mkdir(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeDataDir(shared); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(shared); err != nil || st.Mode().Perm() != 0o755 {
		t.Errorf("существующая папка: %v, %v; ждали 0755", st, err)
	}

	prev := log.Writer()
	t.Cleanup(func() { log.SetOutput(prev) })
	logPath := filepath.Join(nested, "log.txt")
	setupLog(logPath)
	log.Print("проверка")
	if st, err := os.Stat(logPath); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("log.txt: %v, %v; ждали 0600", st, err)
	}
	// Существующий файл остаётся как был.
	oldLog := filepath.Join(nested, "old.txt")
	if err := os.WriteFile(oldLog, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oldLog, 0o644); err != nil {
		t.Fatal(err)
	}
	setupLog(oldLog)
	if st, err := os.Stat(oldLog); err != nil || st.Mode().Perm() != 0o644 {
		t.Errorf("существующий лог: %v, %v; ждали 0644", st, err)
	}
}
