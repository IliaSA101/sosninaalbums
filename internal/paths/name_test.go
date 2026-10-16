package paths

import (
	"strings"
	"testing"
)

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
