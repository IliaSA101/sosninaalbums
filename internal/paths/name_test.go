package paths

import (
	"strings"
	"testing"
)

// Device — устройство Windows по основе имени в любом регистре; похожие имена — не устройства.
func TestDevice(t *testing.T) {
	for _, n := range []string{"con", "NUL", "Aux.json", "com1", "LPT9.txt", "com²", "conin$", "CONOUT$"} {
		if !Device(n) {
			t.Errorf("%q — не устройство", n)
		}
	}
	for _, n := range []string{"", "conx", "com10", "nul-pack", "lpt", "console.json", "ivory"} {
		if Device(n) {
			t.Errorf("%q — устройство", n)
		}
	}
}

// FolderName (047 §4.2): запретные и управляющие знаки — пробел, невидимые убираются, пробелы
// схлопываются, точки и пробелы по краям срезаются, устройства Windows (и с расширением),
// пустое и «..» — запасное имя; не длиннее 64 знаков; обычное имя с цифрами остаётся.
func TestFolderName(t *testing.T) {
	long := strings.Repeat("я", 70)
	for in, want := range map[string]string{
		"Иванов":                         "Иванов",
		"Свадьба 2026":                   "Свадьба 2026",
		"  Иванов  и   Петрова ":         "Иванов и Петрова",
		`a<b>c:d"e/f\g|h?i*j`:            "a b c d e f g h i j",
		"a\tb\nc\x00d":                   "a b c d",
		"a\u200bb\u202ec\ufeff":          "abc",
		"\xff\xfeИванов":                 "Иванов",
		"...Иванов...":                   "Иванов",
		". . .":                          "Альбом",
		"":                               "Альбом",
		"   ":                            "Альбом",
		"..":                             "Альбом",
		"\u200b":                         "Альбом",
		"CON":                            "Альбом",
		"con.txt":                        "Альбом",
		"Nul ":                           "Альбом",
		"aux.":                           "Альбом",
		"LPT9.jpg":                       "Альбом",
		"COM0":                           "Альбом",
		"com¹":                           "Альбом",
		"CONIN$":                         "Альбом",
		"conout$.log":                    "Альбом",
		"CON .txt":                       "Альбом",
		"CONSOLE":                        "CONSOLE",
		"COM10":                          "COM10",
		"LPTX":                           "LPTX",
		"Конь":                           "Конь",
		long:                             strings.Repeat("я", FolderMaxRunes),
		strings.Repeat("я", 63) + " . б": strings.Repeat("я", 63),
	} {
		if got := FolderName(in, "Альбом"); got != want {
			t.Errorf("FolderName(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// FileName (069): основа чистится, как у FolderName, расширение из 1–8 латинских букв и цифр
// сохраняется (регистр тоже); устройство с расширением, пустая основа — запасная основа с тем же
// расширением; негодное «расширение» — часть основы.
func TestFileName(t *testing.T) {
	long, zw := strings.Repeat("я", 70), string(rune(0x200b)) // zw — пробел нулевой ширины
	for in, want := range map[string]string{
		"IMG_0001.jpg":      "IMG_0001.jpg",
		"IMG_0001.JPG":      "IMG_0001.JPG",
		"Свадьба 2026.jpeg": "Свадьба 2026.jpeg",
		"a.tar.gz":          "a.tar.gz",
		"a<b>:c.png":        "a b c.png",
		"  a   b .jpg":      "a b.jpg",
		"CON.jpg":           "Файл.jpg",
		"con.tar.gz":        "Файл.gz",
		"LPT1.psd":          "Файл.psd",
		".jpg":              "Файл.jpg",
		"":                  "Файл",
		"..":                "Файл",
		"...":               "Файл",
		"a.":                "a",
		"a.jp g":            "a.jp g",
		"a.абв":             "a.абв",
		"a.verylongext":     "a.verylongext",
		"a.12345678":        "a.12345678",
		"a" + zw + ".jpg":   "a.jpg",
		"a.j" + zw + "pg":   "a.jpg",
		"a/b\\c.xmp":        "a b c.xmp",
		"\xff.jpg":          "Файл.jpg",
		long + ".jpg":       strings.Repeat("я", FolderMaxRunes) + ".jpg",
		long:                strings.Repeat("я", FolderMaxRunes),
		"IMG (2).jpg.xmp":   "IMG (2).jpg.xmp",
		"CONSOLE.jpg":       "CONSOLE.jpg",
	} {
		if got := FileName(in, "Файл"); got != want {
			t.Errorf("FileName(%q) = %q, ждали %q", in, got, want)
		}
	}
}
