package names

import (
	"errors"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Пределы вставки (design 030 §10): список больше — ошибка без правки.
const (
	MaxPasteBytes = 64 << 10
	MaxPasteLines = 1000
)

var errTooLong = errors.New("список имён слишком длинный: не больше 1000 строк и 64 КБ")

// Parse разбирает вставленный список: строка — человек. Пустые строки пропускаются,
// нумерация «1.» и «2)» снимается. Строка с Tab или «;» делится по ним (столбцы Excel),
// иначе по пробелам. Слов 1 — Фамилия, 2 — Фамилия Имя, 3+ — остаток в Отчество;
// firstFirst — Имя [Отчество] Фамилия. Имена зажаты ClampPerson.
func Parse(s string, firstFirst bool) ([]model.Person, error) {
	if len(s) > MaxPasteBytes {
		return nil, errTooLong
	}
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	if len(lines) > MaxPasteLines {
		return nil, errTooLong
	}
	var out []model.Person
	for _, line := range lines {
		words := split(unnumber(strings.TrimRight(line, "\r")))
		if len(words) == 0 {
			continue
		}
		if who := model.ClampPerson(person(words, firstFirst)); who != nil {
			out = append(out, *who)
		}
	}
	return out, nil
}

// unnumber снимает нумерацию «  12.» или «3)» в начале строки.
func unnumber(line string) string {
	t := strings.TrimLeftFunc(line, unicode.IsSpace)
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i > 0 && i < len(t) && (t[i] == '.' || t[i] == ')') {
		return t[i+1:]
	}
	return line
}

// split — слова строки: по Tab и «;», если они есть, иначе по пробелам.
func split(line string) []string {
	if !strings.ContainsAny(line, "\t;") {
		return strings.Fields(line)
	}
	var out []string
	for _, w := range strings.FieldsFunc(line, func(r rune) bool { return r == '\t' || r == ';' }) {
		if w = strings.TrimSpace(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// person — слова в имя по порядку «Фамилия Имя Отчество» или, при firstFirst,
// «Имя Отчество Фамилия» (одно слово — тогда Имя).
func person(w []string, firstFirst bool) model.Person {
	if firstFirst {
		switch len(w) {
		case 1:
			return model.Person{First: w[0]}
		case 2:
			return model.Person{First: w[0], Last: w[1]}
		}
		n := len(w) - 1
		return model.Person{First: w[0], Middle: strings.Join(w[1:n], " "), Last: w[n]}
	}
	switch len(w) {
	case 1:
		return model.Person{Last: w[0]}
	case 2:
		return model.Person{Last: w[0], First: w[1]}
	}
	return model.Person{Last: w[0], First: w[1], Middle: strings.Join(w[2:], " ")}
}
