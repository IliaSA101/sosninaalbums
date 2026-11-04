package paths

import (
	"unicode"
	"unicode/utf8"
)

// maxMarks — сколько знаков Mn подряд допустимо: «ё» из двух рун — один, стопка из десятков
// закрывает соседние строки окна.
const maxMarks = 2

// BadRunes — строка из чужих рук не годится для показа человеку (149: подсказка папки, имя
// клиента агента): не UTF-8; управляющие (Cc), форматные (Cf: U+200B, bidi, U+200D), частные (Co),
// не назначенные (Cn) знаки; разделители строк и абзацев (Zl, Zp) и пробелы (Zs), кроме
// обычного; «пустые» буквы-заполнители (U+115F, U+1160, U+3164, U+FFA0); больше maxMarks
// знаков Mn подряд. Кириллица, «ё» с одним Mn, эмодзи и флаги (пары региональных знаков, So)
// проходят; эмодзи со связкой ZWJ (Cf) и теги — нет: невидимое не нужно.
// Пустая строка проходит — её судит вызывающий.
func BadRunes(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	marks := 0
	for _, r := range s {
		if badRune(r) {
			return true
		}
		if unicode.Is(unicode.Mn, r) {
			if marks++; marks > maxMarks {
				return true
			}
		} else {
			marks = 0
		}
	}
	return false
}

func badRune(r rune) bool {
	switch r {
	case ' ':
		return false
	case 0x115F, 0x1160, 0x3164, 0xFFA0:
		return true
	}
	// unicode.C у Go — все Cc, Cf, Co, Cs и не назначенные (Cn); из Z плохи все, кроме пробела.
	return unicode.In(r, unicode.C, unicode.Zl, unicode.Zp, unicode.Zs)
}
