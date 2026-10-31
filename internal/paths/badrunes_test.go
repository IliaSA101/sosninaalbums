package paths

import (
	"strings"
	"testing"
)

// Фильтр знаков 149: каждый вид плохого знака — отдельная строка; хорошие слова проходят.
func TestBadRunes(t *testing.T) {
	cases := []struct {
		name string
		s    string
		bad  bool
	}{
		{"пусто", "", false},
		{"латиница и пробел", "Wedding 2026", false},
		{"кириллица", "Свадьба Петровых", false},
		{"ё как е + Mn", "Мёд", false},
		{"два Mn", "e\u0301\u0302", false},
		{"флаг", "🇷🇺 Съёмка", false},
		{"иероглифы", "結婚式", false},
		{"U+2028", "a\u2028b", true},
		{"U+2029", "a\u2029b", true},
		{"U+3164", "a\u3164b", true},
		{"U+115F", "a\u115Fb", true},
		{"U+1160", "a\u1160b", true},
		{"U+FFA0", "a\uFFA0b", true},
		{"U+202E", "a\u202Eb", true},
		{"U+200B", "a\u200Bb", true},
		{"ZWJ", "👨\u200D👩", true},
		{"NBSP", "a\u00A0b", true},
		{"U+3000", "a\u3000b", true},
		{"табуляция", "a\tb", true},
		{"перевод строки", "a\nb", true},
		{"NUL", "a\x00b", true},
		{"DEL", "a\x7Fb", true},
		{"частный U+E000", "a\uE000b", true},
		{"не назначенный U+0378", "a\u0378b", true},
		{"не символ U+FFFF", "a\uFFFFb", true},
		{"тег U+E0041", "a\U000E0041b", true},
		{"три Mn подряд", "e\u0301\u0302\u0303", true},
		{"Mn прерваны буквой", "e\u0301\u0302ae\u0301\u0302", false},
		{"битый UTF-8", "a\xffb", true},
		{"обрезанный UTF-8", "\xd0", true},
		{"суррогат", "\xed\xa0\x80", true},
		{"стопка из 300 Mn", "e" + strings.Repeat("\u0301", 300), true},
	}
	for _, c := range cases {
		if got := BadRunes(c.s); got != c.bad {
			t.Errorf("%s: BadRunes(%q) = %v, ждали %v", c.name, c.s, got, c.bad)
		}
	}
}

// CheckLocalFor — те же правила, что у CheckLocalPath, но ОС выбирает вызывающий.
func TestCheckLocalFor(t *testing.T) {
	cases := []struct {
		windows bool
		path    string
		ok      bool
	}{
		{true, `D:\Фото\Петровы`, true}, {true, `\\srv\s`, false}, {true, `C:x`, false},
		{true, `C:\a:b`, false}, {true, `\\?\C:\x`, false}, {true, `/x`, false},
		{false, `/media/u/disk/2026`, true}, {false, `//srv/s`, false}, {false, `rel`, false},
		{false, `D:\Фото`, false}, {false, "", false}, {true, "", false},
	}
	for _, c := range cases {
		if err := CheckLocalFor(c.windows, c.path); (err == nil) != c.ok {
			t.Errorf("CheckLocalFor(%v, %q) = %v, ok должно быть %v", c.windows, c.path, err, c.ok)
		}
	}
}
