package packprep

import (
	"fmt"
	"strings"
	"unicode"
)

// Копия правил вида стикера из internal/decor/browse.go (formOf): devtools не импортирует internal
// (depguard), как themeSet в recipe.go — копия тем decor. Расхождение копий ловит общий вектор
// testdata/parity/decorform.json (TestFormParity здесь и TestFormOf в decor).

// dividerRatio — сторона не меньше чем в 4 раза длиннее другой: разделитель (≥, не >: 2800×700 — ровно 4:1).
const (
	dividerRatio = 4
	maxSide      = 1 << 30 // больше — не размер картинки; умножение на 4 не переполняется
)

// formOf — признак вида стикера в программе: "divider" | "corner" | "" (стикер).
func formOf(name string, w, h int) string {
	if w > 0 && h > 0 && w <= maxSide && h <= maxSide && (w >= dividerRatio*h || h >= dividerRatio*w) {
		return "divider"
	}
	for _, word := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if word == "уголок" || strings.HasPrefix(word, "уголк") {
			return "corner"
		}
	}
	return ""
}

// formWord — вид рецепта словами флага.
var formWord = map[string]string{"": "стикер", "corner": "уголок", "divider": "разделитель"}

// markForm ставит предметам-стикерам флаг, если вид рецепта (kind) не совпал с признаком программы:
// имя и пропорции решают, в какое «Вид» окна «Стикеры» попадёт предмет. Не отказ: имя не меняет
// пропорцию, а единственный выход из ложного «разделителя» — отложенное поле form (0.27-129-109 б).
func markForm(outs []*Output) {
	for _, o := range outs {
		var want string
		switch o.Kind {
		case KindSticker:
		case KindCorner:
			want = "corner"
		case KindDivider:
			want = "divider"
		default:
			continue
		}
		if got := formOf(o.Name, o.W, o.H); got != want {
			o.Flags = append(o.Flags, fmt.Sprintf("вид рецепта: %s, признак: %s", formWord[want], formWord[got]))
		}
	}
}
