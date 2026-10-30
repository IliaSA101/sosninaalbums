package agents

// Имя клиента, похожее на программу (М2, 149): «Sosnina Albums просит…» от агента — обман, поэтому
// такое имя не показывается ни в вопросе, ни в окне ОС. Сравнение идёт по скелету имени: подмена
// буквы двойником, ударением, пробелом или шириной знака слово не прячет.

import (
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// lookDouble — знаки, которые в строчном начертании читаются как латинские буквы слова «sosnin»:
// греческие, кириллические, армянские, цифры 0 и 1.
var lookDouble = map[rune]rune{
	'ο': 'o', 'α': 'a', 'ε': 'e', 'ι': 'i', 'ν': 'n', // греческие
	'о': 'o', 'а': 'a', 'е': 'e', 'і': 'i', 'ѕ': 's', 'ј': 'j', 'ո': 'n', 'ı': 'i', // кириллица и прочие
	'0': 'o', '1': 'i',
}

// lookKey — скелет имени: NFKD, строчные, без комбинирующих знаков, пробелов, невидимых и
// управляющих знаков. low — как есть (для кириллического «сосн»), latin — ещё и двойники латиницей.
func lookKey(name string) (low, latin string) {
	var lo, la strings.Builder
	for _, r := range printprof.Decompose(name) {
		if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Cc, unicode.Z) {
			continue
		}
		r = unicode.ToLower(r)
		lo.WriteRune(r)
		if c, ok := lookDouble[r]; ok {
			r = c
		}
		la.WriteRune(r)
	}
	return lo.String(), la.String()
}

// Lookalike — имя похоже на «Sosnina»/«Сосн…» с заменой двойников латиницей: греческие
// ο α ε ι ν, кириллица, 0 и 1, полноширинные знаки, ударения и пробелы внутри слова.
func Lookalike(name string) bool {
	low, latin := lookKey(name)
	return strings.Contains(latin, "sosnin") || strings.Contains(low, "сосн")
}
