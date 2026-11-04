package collect

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// regular не ходит по ссылке, звено которой нелокально (101-22): ссылка на файл рядом читается
// (сведения цели), пока чтение цели не подменено на UNC; после — отказ, хотя os.Stat по настоящей
// ссылке прошёл бы. Ссылка на папку — не файл. Обычный файл — как был.
func TestCollectRegularLinkNonLocal(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	link, dlink := filepath.Join(dir, "ссылка.jpg"), filepath.Join(dir, "папка.jpg")
	if err := os.Symlink(a, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if err := os.Symlink(dir, dlink); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if st, ok := regular(KindPhoto, link, os.Lstat); !ok || !st.Mode().IsRegular() || st.Size() == 0 {
		t.Fatalf("локальная ссылка: %v %v", st, ok)
	}
	if _, ok := regular(KindPhoto, dlink, os.Lstat); ok {
		t.Fatal("ссылка на папку прошла как файл")
	}
	unc := `\\127.0.0.1\nosuchshare\x.jpg`
	if runtime.GOOS != "windows" {
		unc = "//nosuchhost/x.jpg"
	}
	t.Cleanup(library.SetReadLink(func(p string) (string, error) {
		if p == link {
			return unc, nil
		}
		return "", os.ErrInvalid
	}))
	if _, ok := regular(KindPhoto, link, os.Lstat); ok {
		t.Fatal("regular прошёл по нелокальной ссылке")
	}
	if _, ok := regular(KindPhoto, a, os.Lstat); !ok {
		t.Fatal("обычный файл не прошёл")
	}
}
