package devtest

import (
	"os"
	"strings"
	"testing"
)

func TestTempDirAndCleanup(t *testing.T) {
	var dir string
	t.Run("создаётся", func(t *testing.T) {
		dir = TempDir(t, "devtest")
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Fatalf("папка не создана: %v", err)
		}
		if !strings.Contains(dir, "devtest-") {
			t.Fatalf("имя без префикса: %s", dir)
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("папка не удалена после теста: %v", err)
	}
}

func TestSelfExeRunnable(t *testing.T) {
	exe := SelfExe(t)
	if exe == "" {
		t.Fatal("пустой путь")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("по пути нет файла: %v", err)
	}
}

// Страж, заменивший исключения testkit.TestNoOwnBinaryLink для этих файлов: без ветки Windows
// (return os.Args[0]) ссылка без .exe на Windows не запускается (так CI уронил sheetcut 24.10).
func TestSelfExeWindowsBranch(t *testing.T) {
	src, err := os.ReadFile("devtest.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `runtime.GOOS == "windows"`) {
		t.Fatal("в SelfExe нет ветки Windows")
	}
}
