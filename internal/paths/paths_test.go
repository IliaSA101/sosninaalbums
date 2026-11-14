package paths

import (
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// LogQ режет строку из файла по знакам (не байтам) и помечает обрезку.
func TestLogQ(t *testing.T) {
	ya := func(n int) string { return strings.Repeat("я", n) }
	for in, want := range map[string]string{
		"":                               `""`,
		"vignette":                       `"vignette"`,
		"a\nb":                           `"a\nb"`,
		ya(LogRunes):                     `"` + ya(LogRunes) + `"`,
		ya(LogRunes + 1):                 `"` + ya(LogRunes) + `"…`,
		ya(1 << 20):                      `"` + ya(LogRunes) + `"…`,
		"\xff" + strings.Repeat("x", 50): `"\xff` + strings.Repeat("x", LogRunes-1) + `"…`,
	} {
		if got := LogQ(in); got != want {
			t.Errorf("LogQ(%.20q) = %.60s, ждали %.60s", in, got, want)
		}
	}
}

func skipNotWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("пути с буквами дисков")
	}
}

func TestCheckLocalPath(t *testing.T) {
	for _, bad := range []string{
		`\\host\share\a`, `//host/share/a`, `\\?\C:\a`, `\\.\PhysicalDrive0`, `\/host/a`,
		`\??\UNC\h\s`, `\??\C:\x`, `\\?\C:\x`, `\\.\x`, `//h/s`, `\x`, `C:x`, `C:`, `1:\a`,
		"rel/a.jpg", "a.jpg", "", "x\x00y", "C:\\a\x00b",
	} {
		if CheckLocalPath(bad) == nil {
			t.Errorf("путь %q должен отклоняться", bad)
		}
	}
	if err := CheckLocalPath(testkit.TempDir(t)); err != nil {
		t.Errorf("локальный путь отклонён: %v", err)
	}
	if runtime.GOOS == "windows" {
		for _, ok := range []string{`C:\Photos`, `D:/a/b`, `d:\x`} {
			if err := CheckLocalPath(ok); err != nil {
				t.Errorf("путь %q должен проходить: %v", ok, err)
			}
		}
	}
}

func TestCheckLocalPathStream(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("потоки NTFS — только Windows")
	}
	for _, bad := range []string{`C:\a\b.jpg:s`, `C:\a\b.jpg::$DATA`, `C:\a:b\c.jpg`, `C:/a/b:`} {
		if CheckLocalPath(bad) == nil {
			t.Errorf("путь %q должен отклоняться", bad)
		}
	}
}

func TestPrefixMap(t *testing.T) {
	skipNotWindows(t)
	cases := []struct{ old, new, from, to string }{
		{`E:\Съёмки\Ивановы\Отбор\1.jpg`, `F:\Ивановы\Отбор\1.jpg`, `E:\Съёмки`, `F:\`},
		{`E:\A\b\1.jpg`, `E:\x\B\1.JPG`, `E:\A`, `E:\x`},
		{`E:\a\1.jpg`, `F:\a\1.jpg`, `E:\`, `F:\`},
		{`E:\a\1.jpg`, `F:\b\2.jpg`, "", ""}, // имя другое: общего хвоста нет
		{`E:\a\1.jpg`, `E:\a\1.jpg`, "", ""}, // не переезжало
	}
	for _, c := range cases {
		from, to, ok := PrefixMap(c.old, c.new)
		if ok != (c.from != "") || from != c.from || to != c.to {
			t.Errorf("PrefixMap(%s, %s) = %q, %q, %v; ждали %q, %q", c.old, c.new, from, to, ok, c.from, c.to)
		}
	}
	if got, ok := ReplacePrefix(`e:\съёмки\Ивановы`, `E:\Съёмки`, `F:\`); !ok || got != `F:\Ивановы` {
		t.Errorf("ReplacePrefix: %q %v", got, ok)
	}
	if _, ok := ReplacePrefix(`E:\Съёмки2\x`, `E:\Съёмки`, `F:\`); ok {
		t.Error("Съёмки2 не внутри Съёмки")
	}
	if l, ok := DriveLetter("e:"); !ok || l != "E" {
		t.Errorf("DriveLetter: %q %v", l, ok)
	}
	if _, ok := DriveLetter("ee"); ok {
		t.Error("две буквы — не диск")
	}
	if got, ok := SwapDrive(`e:\a\1.jpg`, "E", "F"); !ok || got != `F:\a\1.jpg` {
		t.Errorf("SwapDrive: %q %v", got, ok)
	}
}
