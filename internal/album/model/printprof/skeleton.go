package printprof

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Двойники названий (аудит 123 №2): «Фабрика Фотокниги» с неразрывным пробелом, латинской «a»
// среди кириллицы, «Й» из двух знаков или невидимым заполнителем выглядит как настоящая. Названия
// сравниваются только по скелету (NameKey), а слово из смеси письменностей не проходит разбор.

// hiddenRune — знаки, которые рисуются пустым местом, но не попадают в Cf, Zs и Mn: заполнители
// Хангыль, пустая клетка Брайля, кхмерские гласные-невидимки; плюс вариационные селекторы.
func hiddenRune(r rune) bool {
	switch r {
	case 0x115F, 0x1160, 0x3164, 0xFFA0, // заполнители Хангыль
		0x2800,         // пустая клетка Брайля
		0x034F,         // соединитель графем
		0x17B4, 0x17B5, // кхмерские гласные-невидимки
		0x180E,  // монгольский разделитель гласных
		0x1D159: // пустая нотная головка (рисуется пробелом)
		return true
	}
	return r >= 0x180B && r <= 0x180F || r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF ||
		r >= 0x1D173 && r <= 0x1D17A // нотные знаки начала и конца вязки, лиги, фразы
}

// homoglyph — латинские и греческие буквы (после свёртки регистра), неотличимые от кириллических
// хотя бы в одном регистре: «B» и «В», «H» и «Н», «ο» и «о».
var homoglyph = map[rune]rune{
	'a': 'а', 'b': 'в', 'c': 'с', 'e': 'е', 'h': 'н', 'k': 'к', 'm': 'м', 'o': 'о', 'p': 'р',
	't': 'т', 'x': 'х', 'y': 'у', 'i': 'і', 'j': 'ј', 's': 'ѕ',
	'α': 'а', 'β': 'в', 'ε': 'е', 'η': 'н', 'κ': 'к', 'μ': 'м', 'ο': 'о', 'ρ': 'р', 'τ': 'т', 'χ': 'х',
}

// NameKey — скелет названия для сравнения двойников: NFKC, свёртка регистра, без пробелов,
// невидимых и комбинирующих знаков, латинские и греческие двойники — кириллицей. Пустой скелет —
// в названии нет видимых знаков.
func NameKey(s string) string {
	var b strings.Builder
	for _, r := range text.NFKC(s) {
		if hiddenRune(r) || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Cc, unicode.Z) {
			continue
		}
		r = unicode.ToLower(unicode.ToUpper(r)) // «ſ» → «s», «K» (кельвин) → «k»
		if c, ok := homoglyph[r]; ok {
			r = c
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SameName — названия — двойники (один скелет).
func SameName(a, b string) bool { return NameKey(a) == NameKey(b) }

// ownMark — пометка «(своя)» / «(своя N)» в названии: так же ищет её интерфейс (printwords.ownLabel).
var ownMark = regexp.MustCompile(`\(своя( [0-9]+)?\)`)

// OwnLabel — подпись своей типографии в списках: название как есть, если в нём уже есть пометка
// «(своя)» / «(своя N)», иначе «<название> (своя)». Зеркало printwords.ownLabel: по подписи
// фотограф и список «Типография» различают типографии (0.26-123-57).
func OwnLabel(name string) string {
	if ownMark.MatchString(name) {
		return name
	}
	return name + " (своя)"
}

// SameOwnLabel — подписи двух своих типографий в списках неотличимы (по скелету, как SameName).
func SameOwnLabel(a, b string) bool { return SameName(OwnLabel(a), OwnLabel(b)) }

// mixedWord — в каком-то слове (подряд идущие буквы и комбинирующие знаки) буквы двух
// письменностей из кириллицы, латиницы и греческого: «Фaбрика» с латинской «a».
func mixedWord(s string) bool {
	var seen *unicode.RangeTable
	for _, r := range text.NFKC(s) {
		if !unicode.IsLetter(r) && !unicode.In(r, unicode.Mn, unicode.Me) {
			seen = nil // граница слова
			continue
		}
		for _, sc := range []*unicode.RangeTable{unicode.Cyrillic, unicode.Latin, unicode.Greek} {
			if !unicode.Is(sc, r) {
				continue
			}
			if seen != nil && seen != sc {
				return true
			}
			seen = sc
		}
	}
	return false
}

// foreignLetter — буква вне кириллицы, латиницы и греческого (армянская «օ», коптская «ⲟ»,
// чероки «Ꭰ», математическая «𝐚»): их двойников кириллицы скелет не сводит, а названиям
// типографий хватает трёх алфавитов. Смотрим и NFKC: «㋐» раскрывается в букву катаканы.
func foreignLetter(s string) bool {
	for _, r := range s + text.NFKC(s) {
		if unicode.IsLetter(r) && !unicode.In(r, unicode.Cyrillic, unicode.Latin, unicode.Greek) {
			return true
		}
	}
	return false
}

// symbolInWord — символ (So, Sm) в одном слове с буквами: «Фабрик⍺» с APL-альфой вместо «а»
// рвал бы слово для mixedWord. Символ между цифрами и пробелами («30×30», «№ 2») — можно.
func symbolInWord(s string) bool {
	letter, symbol := false, false
	for _, r := range s + " " {
		switch {
		case unicode.IsLetter(r) || unicode.In(r, unicode.Mn, unicode.Me):
			letter = true
		case unicode.In(r, unicode.So, unicode.Sm):
			symbol = true
		default: // граница слова
			if letter && symbol {
				return true
			}
			letter, symbol = false, false
		}
	}
	return false
}
