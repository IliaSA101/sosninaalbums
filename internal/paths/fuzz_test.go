package paths

import (
	"runtime"
	"strings"
	"testing"
)

// FuzzCheckLocalPath: принятый на Windows путь — только `X:` + разделитель, без `:`
// дальше второй позиции и без NUL.
func FuzzCheckLocalPath(f *testing.F) {
	for _, s := range []string{
		`C:\a`, `D:/a/b`, `\\h\s`, `\\?\C:\x`, `C:x`, `C:\a:s`, "C:\\a\x00", "", `é:\я`,
		`C:\Съёмки\1.jpg`, `\??\UNC\h\s`, "//h/s", `Ｃ:\x`, `C:\a::$DATA`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if CheckLocalPath(p) != nil {
			return
		}
		if strings.ContainsRune(p, 0) || len(p) > MaxPathLen {
			t.Fatalf("принят %q: NUL или слишком длинный", p)
		}
		if runtime.GOOS != "windows" {
			return
		}
		ok := len(p) >= 3 && p[1] == ':' && IsSep(p[2]) &&
			(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') &&
			!strings.ContainsRune(p[2:], ':')
		if !ok {
			t.Fatalf("принят %q: не вид X:<разделитель> или есть ':' дальше", p)
		}
	})
}
