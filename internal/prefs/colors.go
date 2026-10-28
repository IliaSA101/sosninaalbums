package prefs

// «Свои цвета» окна выбора цвета (спека 109, G1): один список в config.json, общий для всех
// альбомов. Правила списка живут здесь, интерфейс только показывает ответ.

import (
	"slices"
	"strings"
)

// MaxMyColors — предел «Своих цветов».
const MaxMyColors = 10

// NormHex приводит "#rrggbb" и "rrggbb" (регистр любой, пробелы по краям) к "#rrggbb" в нижнем
// регистре — так же, как normalizeHex во фронте; иначе "" и false.
func NormHex(s string) (string, bool) {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "#")
	if len(s) != 6 || strings.Trim(s, "0123456789abcdef") != "" {
		return "", false
	}
	return "#" + s, true
}

// PushColor ставит hex наверх без повторов; мусор отбрасывается (файл правили руками), список
// режется до MaxMyColors. Результат — новый срез, не nil.
func PushColor(list []string, hex string) []string {
	out := make([]string, 0, MaxMyColors)
	if h, ok := NormHex(hex); ok {
		out = append(out, h)
	}
	for _, v := range list {
		if len(out) == MaxMyColors {
			break
		}
		if h, ok := NormHex(v); ok && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

// MyColors — список после проверки; никогда не nil.
func MyColors(dir string) []string {
	return PushColor(LoadConfig(dir).MyColors, "")
}

// RememberColor запоминает hex первым. Мусор — список без изменений, файл не пишется.
func RememberColor(dir, hex string) []string {
	if _, ok := NormHex(hex); !ok {
		return MyColors(dir)
	}
	var out []string
	UpdateConfig(dir, func(c *Config) {
		out = PushColor(c.MyColors, hex)
		c.MyColors = out
	})
	return out
}

// ForgetColor убирает hex. Нет такого или мусор — список без изменений, файл не пишется.
func ForgetColor(dir, hex string) []string {
	h, ok := NormHex(hex)
	cur := MyColors(dir)
	if !ok || !slices.Contains(cur, h) {
		return cur
	}
	var out []string
	UpdateConfig(dir, func(c *Config) {
		out = slices.DeleteFunc(PushColor(c.MyColors, ""), func(v string) bool { return v == h })
		c.MyColors = out
	})
	return out
}

// ImportColors — перенос прежних «Последних» из localStorage: пишет, только если в config.json
// цветов ещё нет (второе окно или вкладка не задвоят список); иначе отдаёт что есть.
func ImportColors(dir string, list []string) []string {
	var out []string
	UpdateConfig(dir, func(c *Config) {
		out = PushColor(c.MyColors, "")
		if len(out) > 0 {
			return
		}
		// PushColor кладёт новый наверх: идём с хвоста, чтобы порядок остался прежним.
		for _, v := range slices.Backward(list) {
			out = PushColor(out, v)
		}
		c.MyColors = out
	})
	return out
}
