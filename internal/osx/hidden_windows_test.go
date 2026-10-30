//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ход по элементам ниже корня тома на подменённых атрибутах: корень (сам HIDDEN|SYSTEM у NTFS) не
// проверяется, скрытый элемент в начале, середине и конце, нечитаемый элемент, путь без тома.
func TestHiddenWalk(t *testing.T) {
	dir := uint32(syscall.FILE_ATTRIBUTE_DIRECTORY)
	attrs := map[string]uint32{
		`D:\`:                hiddenAttrs, // как корень NTFS
		`D:\Фото`:            dir,
		`D:\Фото\2026`:       dir,
		`D:\Скрытая`:         syscall.FILE_ATTRIBUTE_HIDDEN,
		`D:\Скрытая\Сканы`:   dir,
		`D:\Фото\Сист`:       syscall.FILE_ATTRIBUTE_SYSTEM,
		`D:\Фото\Сист\Конец`: dir,
	}
	read := func(p string) (uint32, error) {
		if a, ok := attrs[p]; ok {
			return a, nil
		}
		return 0, os.ErrNotExist
	}
	for p, want := range map[string]bool{
		`D:\`:                false, // корень тома не смотрим
		`D:\Фото\2026`:       false,
		`D:/Фото/2026`:       false, // косые черты
		`D:\Скрытая\Сканы`:   true,  // HIDDEN в начале
		`D:\Фото\Сист\Конец`: true,  // SYSTEM в середине
		`D:\Фото\Сист`:       true,  // и в конце
		`D:\Фото\нет`:        true,  // не прочитать
		`Фото\2026`:          true,  // без тома
	} {
		if got := hiddenWith(p, read); got != want {
			t.Errorf("%s: %v, ждали %v", p, got, want)
		}
	}
}

// Winqa: настоящие атрибуты — SetFileAttributes на папке виден через fileAttrs; корень тома Hidden
// не смотрит (временная папка лежит под скрытым AppData, поэтому весь путь тут не проверяем).
func TestFolderHiddenWindows(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "Скрытая")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	u, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		attr uint32
		want bool
	}{
		{syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_HIDDEN, true},
		{syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_SYSTEM, true},
		{syscall.FILE_ATTRIBUTE_DIRECTORY, false},
	} {
		if err := syscall.SetFileAttributes(u, c.attr); err != nil {
			t.Fatal(err)
		}
		a, err := fileAttrs(dir)
		if err != nil || (a&hiddenAttrs != 0) != c.want {
			t.Errorf("атрибуты %#x: прочитано %#x (%v), скрытая = %v", c.attr, a, err, c.want)
		}
	}
	if vol := filepath.VolumeName(dir) + `\`; Hidden(vol) {
		t.Errorf("корень тома %s считается скрытым", vol)
	}
}
