package testkit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Пустой и относительный HOME — ошибка, папка нигде не создаётся (путь "" + "/x" на Windows — корень диска).
func TestUserTempBadHome(t *testing.T) {
	cwd := TempDir(t)
	t.Chdir(cwd)
	for _, home := range []string{"", "rel", "."} {
		var once sync.Once
		if dir, err := userTempIn(home, time.Now(), &once); err == nil || dir != "" {
			t.Errorf("home %q: %q, %v", home, dir, err)
		}
	}
	if list, _ := os.ReadDir(cwd); len(list) != 0 {
		t.Errorf("в текущей папке появилось %d записей", len(list))
	}
}

// Создаёт sa-testtmp-* внутри home; подметает только старые настоящие папки с этим именем и только один раз.
func TestUserTempSweep(t *testing.T) {
	home := TempDir(t)
	now := time.Now()
	old := now.Add(-userTempMaxAge - time.Hour)
	mkdir := func(name string, mod time.Time) string {
		p := filepath.Join(home, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "f"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldA, oldB := mkdir(userTempPrefix+"old1", old), mkdir(userTempPrefix+"old2", old)
	fresh := mkdir(userTempPrefix+"fresh", now.Add(-time.Hour))
	other := mkdir("Фото-старые", old) // чужое имя
	file := filepath.Join(home, userTempPrefix+"file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, userTempPrefix+"link")
	if err := os.Symlink(other, link); err != nil {
		link = "" // Windows без прав на ссылки: остальное проверяем
	}

	var once sync.Once
	dir, err := userTempIn(home, now, &once)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != home || !strings.HasPrefix(filepath.Base(dir), userTempPrefix) {
		t.Errorf("папка %q не в %q или без префикса", dir, home)
	}
	exists := func(p string) bool { _, err := os.Lstat(p); return err == nil }
	for p, want := range map[string]bool{oldA: false, oldB: false, fresh: true, other: true, file: true, dir: true} {
		if exists(p) != want {
			t.Errorf("%s: существует %v, ждали %v", filepath.Base(p), !want, want)
		}
	}
	if link != "" && !exists(link) {
		t.Error("ссылка с нашим именем удалена")
	}
	if !exists(filepath.Join(other, "f")) {
		t.Error("чужая папка тронута (через ссылку)")
	}

	// Второй вызов с тем же once не подметает: новая старая папка остаётся.
	again := mkdir(userTempPrefix+"old3", old)
	if _, err := userTempIn(home, now, &once); err != nil {
		t.Fatal(err)
	}
	if !exists(again) {
		t.Error("подметание прошло второй раз")
	}
}

// fatalTB перехватывает Fatalf: записывает текст и завершает горутину, как настоящий t.Fatalf.
type fatalTB struct {
	testing.TB
	msg string
}

func (f *fatalTB) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// Ветка darwin на любой ОС: пустой HOME — провал теста (Fatalf до создания чего-либо), годный — папка sa-testtmp-* в
// HOME, удаляемая после теста.
func TestUserTempDarwin(t *testing.T) {
	home := TempDir(t)
	t.Setenv("HOME", "")
	f := &fatalTB{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		userTempFor(f, "darwin")
		t.Error("UserTemp вернулся при пустом HOME")
	}()
	<-done
	if !strings.Contains(f.msg, "HOME") {
		t.Errorf("сообщение: %q", f.msg)
	}

	t.Setenv("HOME", home)
	var dir string
	t.Run("sub", func(t *testing.T) {
		dir = userTempFor(t, "darwin")
		if filepath.Dir(dir) != home || !strings.HasPrefix(filepath.Base(dir), userTempPrefix) {
			t.Errorf("%q не sa-testtmp-* в %q", dir, home)
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("папка не удалена: %v", err)
	}
}

// Настоящий UserTemp: папка есть, пуста и на darwin лежит в HOME; удаляется после теста.
func TestUserTemp(t *testing.T) {
	var dir string
	t.Run("sub", func(t *testing.T) {
		dir = UserTemp(t)
		if list, err := os.ReadDir(dir); err != nil || len(list) != 0 {
			t.Fatalf("папка не пуста или не читается: %v, %d", err, len(list))
		}
		if runtime.GOOS == "darwin" {
			if home := os.Getenv("HOME"); filepath.Dir(dir) != home || !strings.HasPrefix(filepath.Base(dir), userTempPrefix) {
				t.Errorf("%q не sa-testtmp-* в %q", dir, home)
			}
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("папка не удалена: %v", err)
	}
}
