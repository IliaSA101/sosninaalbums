package library

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// StatLocal: обычный файл — его сведения; ссылка на локальный файл — сведения цели; ссылка,
// звено которой нелокально, — ErrLinkNotLocal без Stat цели (настоящая цель на диске есть —
// os.Stat прошёл бы); нет файла — ошибка диска.
func TestStatLocal(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	link := filepath.Join(dir, "ссылка.jpg")
	if err := os.Symlink(a, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if st, err := StatLocal(a); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("обычный файл: %v %v", st, err)
	}
	if st, err := StatLocal(link); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		t.Fatalf("локальная ссылка: %v %v", st, err)
	}
	unc := `\\127.0.0.1\nosuchshare\x.jpg`
	if runtime.GOOS != "windows" {
		unc = "//nosuchhost/x.jpg"
	}
	t.Cleanup(SetReadLink(func(p string) (string, error) {
		if p == link {
			return unc, nil
		}
		return "", os.ErrInvalid
	}))
	if _, err := StatLocal(link); !errors.Is(err, ErrLinkNotLocal) {
		t.Fatalf("нелокальная ссылка: %v, ждали ErrLinkNotLocal", err)
	}
	if _, err := StatLocal(filepath.Join(dir, "нет.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("нет файла: %v", err)
	}
}
