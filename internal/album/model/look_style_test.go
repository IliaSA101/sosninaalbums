package model

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// style макета (163): зажим длины и знаков при разборе и ClampLook, круг записи, края.
func TestLookStyleClamp(t *testing.T) {
	cases := map[string]string{
		"":                             "",
		"Акварель":                     "Акварель",
		"  Акварель \n":                "Акварель",
		"А\u0000б\u202eв":              "Абв", // управляющие и bidi-знаки уходят
		"А\nБ\tВ":                      "А Б В",
		strings.Repeat("Ж", 41):        strings.Repeat("Ж", MaxLookStyleRunes),
		strings.Repeat("Ж", 5000):      strings.Repeat("Ж", MaxLookStyleRunes),
		"\xff\xfeСтиль\xc0":            "Стиль", // битый UTF-8
		strings.Repeat(" ", 100) + "Я": "Я",
	}
	for in, want := range cases {
		l := seriesLook()
		l.Style = in
		c, ok := ClampLook(l)
		if !ok || c.Style != want {
			t.Errorf("ClampLook(%q): %q, ждали %q", in, c.Style, want)
		}
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		p, err := ParseLook(raw)
		if err != nil || p.Style != want {
			t.Errorf("ParseLook(%q): %q %v, ждали %q", in, p.Style, err, want)
		}
		if again, err := ParseLook(lookJSON(t, p)); err != nil || again.Style != want || CardOf(again).Style != want {
			t.Errorf("круг записи %q: %q %v", in, again.Style, err)
		}
	}
	// Поле не строка — файл не разбирается (ошибка, не паника); поля нет — "" и без ключа в JSON.
	base := lookJSON(t, seriesLook())
	for _, bad := range []string{`5`, `{"a":1}`, `[]`, `true`} {
		data := append(append([]byte{}, base[:len(base)-1]...), []byte(`,"style":`+bad+`}`)...)
		if _, err := ParseLook(data); err == nil {
			t.Errorf("style=%s принят", bad)
		}
	}
	if strings.Contains(string(base), `"style"`) {
		t.Errorf("пустой стиль попал в файл: %s", base)
	}
	if got := CardOf(seriesLook()).Style; got != "" {
		t.Errorf("CardOf без стиля: %q", got)
	}
}

// checkLookStyle — свойство принятого макета для фаззинга: стиль зажат (знаки, длина, края).
func checkLookStyle(t *testing.T, lt AlbumTpl) {
	t.Helper()
	s := lt.Style
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxLookStyleRunes || s != strings.TrimSpace(s) {
		t.Fatalf("стиль %q не зажат", s)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatalf("в стиле %q знак %U", s, r)
		}
	}
}
