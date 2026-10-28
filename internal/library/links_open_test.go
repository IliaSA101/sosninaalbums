package library

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// OpenLocal (аудит 123 №8): обычный файл открывается; папка — отказ; ссылка — открывается её
// проверенная цель; нелокальное звено — ErrLinkNotLocal; подмена файла между проверкой и
// открытием — ErrFileChanged, а не чтение подменённого.
func TestOpenLocal(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.saprint")
	other := filepath.Join(dir, "other.saprint")
	for p, data := range map[string]string{a: "проверенный", other: "подменённый"} {
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(p string) (string, error) {
		f, fi, err := OpenLocal(p)
		if err != nil {
			return "", err
		}
		defer f.Close() //nolint:errcheck // тест: только чтение
		b, err := io.ReadAll(f)
		if err == nil && int64(len(b)) != fi.Size() {
			t.Errorf("%s: сведения не открытого файла", p)
		}
		return string(b), err
	}
	if got, err := read(a); err != nil || got != "проверенный" {
		t.Fatalf("обычный файл: %q %v", got, err)
	}
	if _, err := read(dir); err == nil {
		t.Error("папка открыта")
	}
	if _, err := read(filepath.Join(dir, "нет.saprint")); err == nil {
		t.Error("несуществующий открыт")
	}
	// Подмена между проверкой и открытием: другой файл встаёт на место проверенного.
	openLocalHook = func(p string) {
		if err := os.Rename(other, p); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { openLocalHook = nil })
	if got, err := read(a); !errors.Is(err, ErrFileChanged) {
		t.Fatalf("подмена: %q %v, ждали ErrFileChanged", got, err)
	}
	openLocalHook = nil

	link := filepath.Join(dir, "link.saprint")
	if err := os.Symlink(a, link); err != nil {
		t.Skipf("ссылку не создать: %v", err) // Windows без прав
	}
	if got, err := read(link); err != nil || got != "подменённый" {
		t.Fatalf("ссылка на локальный файл: %q %v", got, err)
	}
	unc := `\\127.0.0.1\nosuchshare\x.saprint`
	if runtime.GOOS != "windows" {
		unc = "//nosuchhost/x.saprint"
	}
	t.Cleanup(SetReadLink(func(p string) (string, error) {
		if p == link {
			return unc, nil
		}
		return "", os.ErrInvalid
	}))
	if _, err := read(link); !errors.Is(err, ErrLinkNotLocal) {
		t.Errorf("нелокальная ссылка: %v, ждали ErrLinkNotLocal", err)
	}
}
