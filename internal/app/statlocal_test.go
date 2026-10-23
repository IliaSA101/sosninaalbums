package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Проверки фото чужого альбома не идут по ссылке на сетевой путь (аудит 0.23-sec, 0.22-sec-4):
// ссылка на \\host\share раньше открывала шару при os.Stat.

// netLink — ссылка на настоящий файл target; при net сырая цель, как её видит код, — сетевой путь.
// Без подмены цель локальна и ссылка работает как обычный файл (контроль).
func netLink(t *testing.T, dir, target string, net bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("н/п: ссылки на Windows требуют прав; логика StatLocal общая")
	}
	link := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if net {
		t.Cleanup(library.SetReadLink(func(p string) (string, error) {
			if p == link {
				return "//nosuchhost/share/a.jpg", nil
			}
			return "", os.ErrInvalid
		}))
	}
	return link
}

func TestFileExistsNetLink(t *testing.T) {
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	if !fileExists(netLink(t, dir, file, false)) {
		t.Fatal("контроль: ссылка на локальный файл не найдена")
	}
	dir2 := testkit.TempDir(t)
	file2 := testkit.JPEG(t, dir2, "a.jpg", 10, 10)
	if fileExists(netLink(t, dir2, file2, true)) {
		t.Error("ссылка на сетевой путь принята за файл")
	}
}

func TestAboutOfNetLink(t *testing.T) {
	a := newApp(t)
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	if _, err := a.aboutOf(netLink(t, dir, file, false)); err != nil {
		t.Fatalf("контроль: %v", err)
	}
	dir2 := testkit.TempDir(t)
	file2 := testkit.JPEG(t, dir2, "a.jpg", 10, 10)
	if _, err := a.aboutOf(netLink(t, dir2, file2, true)); !errors.Is(err, library.ErrLinkNotLocal) {
		t.Fatalf("ссылка на сетевой путь: err %v, ждали ErrLinkNotLocal", err)
	}
}

// ShowPhotoInExplorer: файл по сетевой ссылке не «найден» — открывается папка, не выделение.
func TestShowPhotoInExplorerNetLink(t *testing.T) {
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	link := netLink(t, dir, file, true)
	a, calls := revealApp(t, link)
	if err := a.ShowPhotoInExplorer("p"); err != nil || len(*calls) != 1 || (*calls)[0] != "dir:"+dir {
		t.Fatalf("err %v, вызовы %v, ждали открытие папки", err, *calls)
	}
}
