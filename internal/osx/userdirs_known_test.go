package osx

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// userDirsIn: файл настроек, запасные имена, повторы, относительные и пустые пути, огромный и
// отсутствующий файл.
func TestUserDirsPersonal(t *testing.T) {
	home := testkit.TempDir(t)
	cfg := testkit.TempDir(t)
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(cfg, "user-dirs.dirs"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := func(names ...string) []string {
		var out []string
		for _, n := range names {
			out = append(out, filepath.Join(home, n))
		}
		return out
	}
	// Нет файла: запасные имена.
	if got := userDirsIn(home, cfg, nil); !slices.Equal(got, want("Documents", "Desktop", "Downloads")) {
		t.Fatalf("без файла: %q", got)
	}
	// Русские имена из файла идут первыми; запасные не пропадают.
	write([]byte("XDG_DOCUMENTS_DIR=\"$HOME/Документы\"\nXDG_DESKTOP_DIR=\"$HOME/Desktop\"\n"))
	got := userDirsIn(home, cfg, []string{"relative", "", filepath.Join(home, "Документы")})
	if !slices.Equal(got, want("Документы", "Desktop", "Documents", "Downloads")) {
		t.Fatalf("с файлом: %q", got)
	}
	// Файл больше предела не читается, остальное работает.
	write(append([]byte("XDG_DOCUMENTS_DIR=\"$HOME/Документы\"\n"), bytes.Repeat([]byte("#"), maxUserDirsFile)...))
	if got := userDirsIn(home, cfg, nil); !slices.Equal(got, want("Documents", "Desktop", "Downloads")) {
		t.Fatalf("огромный файл: %q", got)
	}
	// Файл — папка, домашней нет, домашняя относительная: без паники, только то, что дала ОС.
	if err := os.Remove(filepath.Join(cfg, "user-dirs.dirs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cfg, "user-dirs.dirs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := userDirsIn(home, cfg, nil); !slices.Equal(got, want("Documents", "Desktop", "Downloads")) {
		t.Fatalf("файл — папка: %q", got)
	}
	for _, h := range []string{"", "rel/home"} {
		if got := userDirsIn(h, cfg, []string{home}); !slices.Equal(got, []string{home}) {
			t.Fatalf("home %q: %q", h, got)
		}
	}
	// Настоящий вызов: пути абсолютные и без повторов.
	all := UserDirs()
	for i, p := range all {
		if !filepath.IsAbs(p) || strings.Contains(p, "..") || slices.Contains(all[i+1:], p) {
			t.Errorf("UserDirs: %q", p)
		}
	}
}
