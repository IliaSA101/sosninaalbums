package packprep

import (
	"fmt"
	"strings"
)

// Темы набора в рецепте (спека 170, design §2.1, §6). Правило объявления — копия pack.ParseThemes
// и scripts/pack-check/manifest.mjs: devtools не импортирует internal (depguard), как form.go.
// Расхождение копий ловит общий вектор testdata/pack-manifest-cases.json (TestThemeDeclParity).
// Инструмент строже программы: плохая запись — отказ рецепту, а не пропуск.

// Пределы тем (как pack.MaxThemes, MaxThemeName и т. д.).
const (
	maxThemes     = 16
	maxThemeName  = 20
	themeOrderMin = 1
	themeOrderMax = 99
	themeOrderDef = 50
)

// builtinThemeIDs — встроенные темы программы (копия тем decor) и допустимая база объявления.
var builtinThemeIDs = []string{"basic", "wedding", "kids", "kindergarten", "school", "grad", "holiday"}

// serviceThemeIDs — служебные id: выборки и разделы списка; объявить нельзя.
var serviceThemeIDs = []string{"recent", "all", "new", "get", "mine", "brand", "legacy", "constructor"}

// reservedThemeNames — имена встроенных и служебных строк списка.
var reservedThemeNames = []string{
	"Недавние", "Все", "Новые", "Можно скачать", "Мои", "Фирменные", "Прежние макеты",
	"Универсальные", "Свадьба", "Дети", "Детский сад", "Школа", "Выпускной", "Праздник",
}

// ThemeDecl — объявление темы набора в рецепте и в manifest.src.json. Order — указатель: нет поля — 50.
type ThemeDecl struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Base  string `json:"base,omitempty"` // пусто — basic
	Order *int   `json:"order,omitempty"`
}

// resolved — объявление с подставленными умолчаниями (база basic, порядок 50).
func (d ThemeDecl) resolved() ThemeDecl {
	if d.Base == "" {
		d.Base = "basic"
	}
	if d.Order == nil {
		o := themeOrderDef
		d.Order = &o
	}
	return d
}

func inList(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// validThemeID — ^[a-z]{1,16}$, не встроенный и не служебный (pack.ValidThemeID).
func validThemeID(id string) bool {
	if id == "" || len(id) > 16 || inList(builtinThemeIDs, id) || inList(serviceThemeIDs, id) {
		return false
	}
	for _, r := range id {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

type themeScript uint8

const (
	scriptNone themeScript = iota
	scriptLatin
	scriptCyrillic
)

// validThemeName — белый список имени (pack.ValidThemeName, 170-28): кириллица, латиница, цифры, пробел,
// «-»; есть буква; в слове одна письменность; без пробела по краям и двух подряд; 1…20 знаков.
func validThemeName(name string) bool {
	n, letters := 0, 0
	cur := scriptNone
	prev := ' '
	for i, r := range name {
		n++
		if n > maxThemeName {
			return false
		}
		switch {
		case r == ' ':
			if i == 0 || prev == ' ' {
				return false
			}
			cur = scriptNone
		case r == '-':
			cur = scriptNone
		case r >= '0' && r <= '9':
		default:
			var sc themeScript
			switch {
			case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
				sc = scriptLatin
			case r >= 'А' && r <= 'я' || r == 'Ё' || r == 'ё':
				sc = scriptCyrillic
			default:
				return false
			}
			if cur != scriptNone && cur != sc {
				return false
			}
			cur = sc
			letters++
		}
		prev = r
	}
	return n > 0 && letters > 0 && prev != ' '
}

// themeFold — регистр по белому списку, как pack.themeFold (strings.ToLower расходится с JS).
func themeFold(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'А' && r <= 'Я':
			return r + 0x20
		case r == 'Ё':
			return 'ё'
		}
		return r
	}, s)
}

// checkThemeDecls — все нарушения объявлений разом: число, id, имя, повторы, база, порядок.
func checkThemeDecls(ds []ThemeDecl) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf("packprep: "+format, a...)) }
	if len(ds) > maxThemes {
		add("themes: не больше %d объявлений, а их %d", maxThemes, len(ds))
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, n := range reservedThemeNames {
		names[themeFold(n)] = true
	}
	for i, d := range ds {
		switch {
		case !validThemeID(d.ID):
			add("themes[%d] id %q: нужно ^[a-z]{1,16}$, не встроенная и не служебная тема", i, d.ID)
		case ids[d.ID]:
			add("themes[%d] id %q повторяется", i, d.ID)
		}
		switch {
		case !validThemeName(d.Name):
			add("themes[%d] имя %q: кириллица, латиница, цифры, пробел и «-», есть буква, одна письменность в слове, 1…%d знаков", i, d.Name, maxThemeName)
		case names[themeFold(d.Name)]:
			add("themes[%d] имя %q совпадает с другой темой или встроенной строкой списка", i, d.Name)
		}
		if d.Base != "" && !inList(builtinThemeIDs, d.Base) {
			add("themes[%d] base %q: нужна одна из встроенных тем (%s)", i, d.Base, strings.Join(builtinThemeIDs, ", "))
		}
		if d.Order != nil && (*d.Order < themeOrderMin || *d.Order > themeOrderMax) {
			add("themes[%d] order %d: нужно %d…%d", i, *d.Order, themeOrderMin, themeOrderMax)
		}
		ids[d.ID] = true
		names[themeFold(d.Name)] = true
	}
	return errs
}

// themeBase — база темы набора по id объявления; "" — такой темы нет.
func (r *Recipe) themeBase(id string) string {
	for _, d := range r.Themes {
		if d.ID == id {
			return d.resolved().Base
		}
	}
	return ""
}

// themeKnown — тема рецепта или предмета: встроенная или объявленная в themes.
func (r *Recipe) themeKnown(id string) bool {
	return inList(builtinThemeIDs, id) || r.themeBase(id) != ""
}

// resolveTheme — поля манифеста для темы предмета: в theme идёт база (её видят 0.29–0.39), в topic —
// id объявленной темы (design §2.1); у встроенной темы topic пуст.
func (r *Recipe) resolveTheme(id string) (theme, topic string) {
	if base := r.themeBase(id); base != "" {
		return base, id
	}
	return id, ""
}

// checkThemeUse — объявленная тема без единого предмета рецепта — отказ (design §6); used — id тем у предметов.
func (r *Recipe) checkThemeUse(used map[string]bool) []error {
	var errs []error
	for _, d := range r.Themes {
		if !used[d.ID] {
			errs = append(errs, fmt.Errorf("packprep: тема %q объявлена, но ни один предмет её не использует", d.ID))
		}
	}
	return errs
}

// recipeThemeUse — id тем, на которые ссылаются рецепт (по умолчанию) и его предметы: статическая проверка
// до Run; предмет без своей theme берёт тему набора.
func (r *Recipe) recipeThemeUse() map[string]bool {
	used := map[string]bool{}
	for _, it := range r.Items {
		th := it.Theme
		if th == "" {
			th = r.Theme
		}
		used[th] = true
	}
	return used
}

// unusedThemes — для отчёта Run: объявленные темы, у которых после браков (skip) не осталось предметов.
func (r *Recipe) unusedThemes(outs []*Output) []string {
	used := map[string]bool{}
	for _, o := range outs {
		used[o.Topic] = true
	}
	var bad []string
	for _, d := range r.Themes {
		if !used[d.ID] {
			bad = append(bad, d.ID)
		}
	}
	return bad
}
