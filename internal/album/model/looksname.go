package model

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Имя из названия файла при броске (060, сценарий А): правило строже вольного разбора 043 —
// имя пишется само, без предпросмотра, поэтому ложное срабатывание должно быть редким.

// LooksLikeName — имя из названия файла path, если оно похоже на имя человека: вольный разбор
// («_», «.» и пробел — разделители; номер в начале и отметка копии снимаются; регистр
// исправляется) и сверху фильтры — 2–3 слова; слово с дефисом не делится («Санкт-Петербург» —
// одно слово); в каждом слове ≥ 2 букв; все буквы одной письменности (кириллица или латиница);
// ни одного слова из стоп-списка (looksStop). Порядок всегда «Фамилия Имя [Отчество]». Иначе nil:
// «IMG_1234», «Свадьба Ивановых», «Первый танец», «Иванов», «Иванов П».
func LooksLikeName(path string) *Person {
	base := fileBase(path)
	if !utf8.ValidString(base) || utf8.RuneCountInString(base) > maxFileBase {
		return nil
	}
	base = strings.Map(func(r rune) rune {
		if r == '_' || r == '.' {
			return ' '
		}
		return r
	}, base)
	base = strings.TrimSpace(stripCopy(stripLead(strings.TrimSpace(base))))
	words := strings.Fields(base)
	if len(words) < 2 || len(words) > 3 {
		return nil
	}
	script := (*unicode.RangeTable)(nil)
	for i, w := range words {
		w = fixCase(w)
		words[i] = w
		if !nameWord(w) || stopWord(w) {
			return nil
		}
		if utf8.RuneCountInString(w) > MaxPersonRunes {
			return nil // зажим обрезал бы слово: такое не имя
		}
		letters := 0
		for _, r := range w {
			if !unicode.IsLetter(r) {
				continue
			}
			letters++
			if script == nil {
				script = scriptOf(r)
			}
			if script == nil || !unicode.Is(script, r) {
				return nil
			}
		}
		if letters < 2 {
			return nil
		}
	}
	p := Person{Last: words[0], First: words[1]}
	if len(words) == 3 {
		p.Middle = words[2]
	}
	return ClampPerson(p)
}

// scriptOf — письменность буквы r: кириллица или латиница; иная — nil (не имя).
func scriptOf(r rune) *unicode.RangeTable {
	switch {
	case unicode.Is(unicode.Cyrillic, r):
		return unicode.Cyrillic
	case unicode.Is(unicode.Latin, r):
		return unicode.Latin
	}
	return nil
}

// stopWord — слово (или часть через дефис) из стоп-списка, без учёта регистра.
func stopWord(w string) bool {
	w = strings.ToLower(w)
	if looksStop[w] {
		return true
	}
	for part := range strings.SplitSeq(w, "-") {
		if looksStop[part] {
			return true
		}
	}
	return false
}

// looksStop — стоп-список LooksLikeName (060: объединение разборов A §5 и B §4 и немного
// частого): камеры и служебное, свадебное, школьное. Один список на всё правило — тест
// TestLooksStop сверяет его с таблицей «нет».
var looksStop = func() map[string]bool {
	words := []string{
		// камеры и служебное
		"img", "dsc", "dscn", "dscf", "pxl", "mvi", "mvimg", "vid", "pano", "photo", "image", "foto",
		"scan", "copy", "edit", "final", "export", "untitled", "screenshot", "whatsapp",
		"фото", "снимок", "копия", "скан", "ретушь", "обработка", "фотосессия",
		// свадебное и семейное
		"свадьба", "банкет", "торт", "танец", "первый", "роспись", "загс", "сборы", "прогулка",
		"букет", "кольца", "невеста", "жених", "гости", "семья", "мама", "папа", "дети",
		"wedding", "bride", "groom", "love", "story", "party", "family", "day",
		"день", "год", "новый", "вечер", "утро",
		// школьное
		"выпускной", "выпуск", "выпускники", "класс", "группа", "групповое", "общее", "портрет",
		"учитель", "ученик", "ученики", "школа", "school", "class",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}()
