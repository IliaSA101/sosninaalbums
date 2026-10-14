//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// shortPath — имя 8.3 для существующего пути; пусто, если на томе короткие имена выключены.
func shortPath(p string) string {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return ""
	}
	buf := make([]uint16, syscall.MAX_PATH)
	n, err := syscall.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n > uint32(len(buf)) {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// %TEMP% в виде 8.3 при длинном имени exe (и наоборот) — всё равно «во временной папке».
func TestLaunchPlaceShortNames(t *testing.T) {
	// Короткие имена бывают выключены на D: (рабочий диск разработки), поэтому берём
	// настоящий %LOCALAPPDATA%\Temp, а если его нет — t.TempDir.
	tmp := t.TempDir()
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		if d, err := os.MkdirTemp(filepath.Join(base, "Temp"), "launch-test-*-длинное-имя"); err == nil {
			tmp = d
			t.Cleanup(func() { _ = os.RemoveAll(d) })
		}
	}
	sub := filepath.Join(tmp, "Temp1_очень длинное имя архива.zip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(sub, "SosninaAlbums.exe")
	shortTmp, shortSub := shortPath(tmp), shortPath(sub)
	if shortTmp == "" || shortSub == "" || strings.EqualFold(shortSub, sub) {
		t.Skip("н/п: на томе нет коротких имён 8.3")
	}
	if got := LaunchPlace(exe, shortTmp); got != PlaceTemp {
		t.Errorf("temp в 8.3: %q", got)
	}
	if got := LaunchPlace(filepath.Join(shortSub, "SosninaAlbums.exe"), tmp); got != PlaceTemp {
		t.Errorf("exe в 8.3: %q", got)
	}
	if got := LaunchPlace(filepath.Join(shortSub, "x.exe"), shortTmp); got != PlaceTemp {
		t.Errorf("оба в 8.3: %q", got)
	}
	other := t.TempDir()
	if got := LaunchPlace(filepath.Join(shortPath(other), "x.exe"), shortTmp); got != PlaceNormal {
		t.Errorf("чужая папка в 8.3 не должна совпасть: %q", got)
	}
}

// Там, где 8.3 выключены (проверка выше пропущена), хотя бы убеждаемся, что longPath не портит
// путь: существующий остаётся тем же, несуществующий и пустой возвращаются как есть.
func TestLongPathKeepsPath(t *testing.T) {
	dir := t.TempDir()
	if got := longPath(dir); !strings.EqualFold(got, dir) {
		t.Errorf("существующий путь: %q → %q", dir, got)
	}
	missing := filepath.Join(dir, "нет", "такого")
	if got := longPath(missing); got != missing {
		t.Errorf("несуществующий путь: %q → %q", missing, got)
	}
	if got := longPath(""); got != "" {
		t.Errorf("пустой путь: %q", got)
	}
}
