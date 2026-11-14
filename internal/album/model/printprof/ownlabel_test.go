package printprof

import "testing"

// 0.26-123-57: подпись своей в списках — зеркало printwords.ownLabel; совпадение подписей.
func TestOwnLabel(t *testing.T) {
	for name, want := range map[string]string{
		"Альфа":            "Альфа (своя)",
		"Альфа (своя)":     "Альфа (своя)",
		"Альфа (своя 2)":   "Альфа (своя 2)",
		"Бета (своя) плюс": "Бета (своя) плюс", // пометка в середине интерфейс тоже не дописывает
		"Гамма (своя x)":   "Гамма (своя x) (своя)",
		"Дельта (своя2)":   "Дельта (своя2) (своя)",
		"":                 " (своя)",
	} {
		if got := OwnLabel(name); got != want {
			t.Errorf("OwnLabel(%q) = %q, ждали %q", name, got, want)
		}
	}
}

func TestSameOwnLabel(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"QA тест 1", "QA тест 1 (своя)", true},
		{"qa ТЕСТ 1", "QA тест 1 (своя)", true}, // регистр и пробелы — как в SameName
		{"Тест", "Teст (своя)", true},           // латинские двойники
		{"X (своя)", "X (своя 2)", false},
		{"X", "X (своя 2)", false},
		{"X (своя 2)", "X (своя 3)", false},
		{"X", "Y", false},
		{"", "", true},
	} {
		if got := SameOwnLabel(c.a, c.b); got != c.same {
			t.Errorf("SameOwnLabel(%q, %q) = %v, ждали %v", c.a, c.b, got, c.same)
		}
		if SameOwnLabel(c.b, c.a) != c.same {
			t.Errorf("не симметрично: %q, %q", c.a, c.b)
		}
	}
}
