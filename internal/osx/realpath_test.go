package osx

import (
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Префикс GetFinalPathNameByHandle снимается: обычный, UNC, без префикса, пустой и усечённый.
func TestTrimFinalPrefix(t *testing.T) {
	for in, want := range map[string]string{
		`\\?\C:\Users\u`:          `C:\Users\u`,
		`\\?\UNC\srv\share\x`:     `\\srv\share\x`,
		`C:\Users\u`:              `C:\Users\u`,
		`\\srv\share`:             `\\srv\share`,
		`\\?\`:                    ``,
		`\\?\UNC\`:                `\\`,
		``:                        ``,
		`\\?`:                     `\\?`,
		`\\?\Volume{1}\x`:         `\\?\Volume{1}\x`,
		"/home/u":                 "/home/u",
		`\\?\C:\Фото\2026-съёмка`: `C:\Фото\2026-съёмка`,
	} {
		if got := trimFinalPrefix(in); got != want {
			t.Errorf("trimFinalPrefix(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// Вне Windows подставных дисков нет: путь возвращается как есть, в том числе несуществующий.
func TestRealPathOther(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows — realpath_windows_test.go")
	}
	dir := testkit.TempDir(t)
	for _, p := range []string{dir, "/нет/такого", ""} {
		if got, err := RealPath(p); err != nil || got != p {
			t.Errorf("RealPath(%q) = %q, %v", p, got, err)
		}
	}
}
