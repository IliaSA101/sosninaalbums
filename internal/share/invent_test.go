package share

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

var inventRe = regexp.MustCompile(`^[а-яё]+-\d{4}$`)

// «слово-NNNN»; повторы редки (21 бит: на 1000 попыток в среднем меньше одного), слова из списка.
func TestShareInvent(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		p, err := Invent()
		if err != nil {
			t.Fatal(err)
		}
		if !inventRe.MatchString(p) {
			t.Fatalf("пароль %q не вида слово-NNNN", p)
		}
		if err := CheckPassword(p); err != nil {
			t.Fatalf("придуманный %q не проходит проверку: %v", p, err)
		}
		seen[p] = true
	}
	if len(seen) < 995 {
		t.Errorf("разных паролей %d из 1000", len(seen))
	}
	// Список: без повторов и посторонних знаков, достижимы все слова.
	inList := map[string]bool{}
	for _, w := range words {
		if inList[w] || !regexp.MustCompile(`^[а-яё]{3,12}$`).MatchString(w) {
			t.Errorf("слово %q повторено или негодно", w)
		}
		inList[w] = true
	}
	if len(words) < 200 {
		t.Errorf("в списке %d слов, ждали не меньше 200", len(words))
	}
	hit := map[string]bool{}
	for range 40 * len(words) {
		p, _ := Invent()
		w, _, _ := strings.Cut(p, "-")
		hit[w] = true
	}
	if len(hit) != len(words) {
		t.Errorf("выпало %d слов из %d: есть недостижимые", len(hit), len(words))
	}
}

// Источник случайности: крайние байты дают крайние значения, ошибка источника — ошибка.
func TestShareInventSource(t *testing.T) {
	if p, err := inventFrom(bytes.NewReader(make([]byte, 64))); err != nil || p != words[0]+"-0000" {
		t.Errorf("нули: %q, %v", p, err)
	}
	if _, err := inventFrom(bytes.NewReader(nil)); err == nil {
		t.Error("пустой источник принят")
	}
	if _, err := inventFrom(errReader{}); err == nil {
		t.Error("ошибка источника проглочена")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("нет случайности") }

func TestSharePassword(t *testing.T) {
	for _, p := range []string{"", "лето-4827", "abcd", "пароль с пробелами", strings.Repeat("я", MaxPassword)} {
		if err := CheckPassword(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for name, p := range map[string]string{
		"короткий":     "abc",
		"длинный":      strings.Repeat("я", MaxPassword+1),
		"перевод":      "abc\ndef",
		"нуль":         "abcd\x00",
		"невидимый":    "ab\u200bcd",
		"разделитель":  "ab\u2028cd",
		"не UTF-8":     "abc\xff\xfe",
		"только ctrl":  "\t\t\t\t",
		"RTL-override": "ab\u202ecd",
	} {
		if CheckPassword(p) == nil {
			t.Errorf("%s: %q принят", name, p)
		}
	}
}

func TestShareClientText(t *testing.T) {
	exp := time.Date(2026, 11, 29, 0, 0, 0, 0, time.UTC)
	l := Link{Title: "Свадьба И. и А.", URL: "https://albums.example.org/a/k3J/", Password: "лето-4827", ExpiresAt: exp}
	want := "Альбом «Свадьба И. и А.»: https://albums.example.org/a/k3J/\nПароль: лето-4827\nСсылка работает до 29 ноября 2026."
	if got := ClientText(l); got != want {
		t.Errorf("с паролем:\n%s", got)
	}
	l.Password = ""
	want = "Альбом «Свадьба И. и А.»: https://albums.example.org/a/k3J/\nСсылка работает до 29 ноября 2026."
	if got := ClientText(l); got != want {
		t.Errorf("без пароля:\n%s", got)
	}
	l.ExpiresAt = time.Time{}
	if got := ClientText(l); got != "Альбом «Свадьба И. и А.»: https://albums.example.org/a/k3J/" {
		t.Errorf("без срока:\n%s", got)
	}
	// Название чистится: перевод строки не рвёт текст, пустое — «Альбом».
	l.Title = "Лето\n\x00Пароль: взломан"
	if got := ClientText(l); strings.Count(got, "\n") != 0 || strings.Contains(got, "\x00") {
		t.Errorf("название сломало строки: %q", got)
	}
	l.Title = " \t\n "
	if got := ClientText(l); !strings.HasPrefix(got, "Альбом «Альбом»") {
		t.Errorf("пустое название: %q", got)
	}
	// Дата — родительный падеж всех месяцев.
	for m := time.January; m <= time.December; m++ {
		d := DateRU(time.Date(2027, m, 1, 12, 0, 0, 0, time.UTC))
		if !strings.HasPrefix(d, "1 ") || !strings.HasSuffix(d, " 2027") {
			t.Errorf("%v: %q", m, d)
		}
	}
	if DateRU(time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)) != "8 марта 2026" {
		t.Error("8 марта")
	}
}

func TestShareCleanTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Свадьба":               "Свадьба",
		"  Свадьба   И.  и А. ": "Свадьба И. и А.",
		"":                      "Альбом",
		"\x00\u200b\u202e":      "Альбом",
		"а\r\nб":                "а б",
		"a\xffb":                "ab",
	} {
		if got := CleanTitle(in, 0); got != want {
			t.Errorf("%q → %q, ждали %q", in, got, want)
		}
	}
	long := strings.Repeat("я", 500)
	if got := CleanTitle(long, 0); len([]rune(got)) != MaxTitle {
		t.Errorf("длина %d", len([]rune(got)))
	}
	if got := CleanTitle("ab cd", 3); got != "ab" {
		t.Errorf("обрез по пробелу: %q", got)
	}
	if got := CleanTitle("ab cd", 4); got != "ab c" {
		t.Errorf("обрез: %q", got)
	}
}
