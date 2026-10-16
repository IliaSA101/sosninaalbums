package paths

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// FuzzFolderName: имя папки книги (047) — не пустое, локальное, без разделителей и `:`, без
// невидимых (Cf), управляющих и U+FFFD, без точки и пробела по краям, не длиннее FolderMaxRunes,
// без устройств Windows; повторный вызов его не меняет.
func FuzzFolderName(f *testing.F) {
	for _, s := range []string{"Иванов", "", "con.txt", "a/b\\c", "\u200b", "...", "COM¹", "\xff", strings.Repeat("я ", 40), "a:b", "\u202eфдп.exe", "\ufffd", " .а. ", "x\u0085y"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := FolderName(s, "Альбом")
		if got == "" || !filepath.IsLocal(got) || strings.ContainsAny(got, `\/`) || utf8.RuneCountInString(got) > FolderMaxRunes {
			t.Fatalf("FolderName(%q) = %q", s, got)
		}
		bad := strings.IndexFunc(got, func(r rune) bool {
			return r == utf8.RuneError || r == ':' || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
		})
		if bad >= 0 || !utf8.ValidString(got) || strings.Trim(got, ". ") != got {
			t.Fatalf("FolderName(%q) = %q — запретный знак или край", s, got)
		}
		if got != "Альбом" && reservedName(got) {
			t.Fatalf("FolderName(%q) = %q — устройство", s, got)
		}
		if again := FolderName(got, "Альбом"); again != got {
			t.Fatalf("не идемпотентна: %q → %q → %q", s, got, again)
		}
	})
}

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
