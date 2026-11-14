package imgfile

import (
	"io/fs"
	"runtime"
	"testing"
)

// TestMaybeRegular: обычный файл — да; папка, ссылка, канал, сокет, устройство — нет; ModeIrregular (облачная
// заглушка и точка соединения) — только на Windows, где окончательно решает дескриптор.
func TestMaybeRegular(t *testing.T) {
	win := runtime.GOOS == "windows"
	for name, c := range map[string]struct {
		m    fs.FileMode
		want bool
	}{
		"обычный":    {0o644, true},
		"пустой":     {0, true},
		"папка":      {fs.ModeDir | 0o755, false},
		"ссылка":     {fs.ModeSymlink | 0o777, false},
		"канал":      {fs.ModeNamedPipe | 0o644, false},
		"сокет":      {fs.ModeSocket | 0o644, false},
		"устройство": {fs.ModeDevice | fs.ModeCharDevice, false},
		"irregular":  {fs.ModeIrregular | 0o644, win},
	} {
		if got := MaybeRegular(c.m); got != c.want {
			t.Errorf("%s: MaybeRegular(%v) = %v, ожидалось %v", name, c.m, got, c.want)
		}
	}
}
