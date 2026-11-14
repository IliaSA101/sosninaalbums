package pack

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Темы наборов (спека 170, design §2–§3): объявления `themes` шапки манифеста и `topic` предмета.
// Манифест 0.29–0.39 такие поля пропускает, поэтому здесь они не должны ронять разбор: плохая
// запись пропускается с причиной, набор ставится, предметы лежат в базе (`theme`).

// Пределы тем (блок limits файла testdata/pack-manifest-cases.json; TestLimits).
const (
	MaxThemes     = 16   // объявлений в наборе: принимаются первые
	MaxThemesRaw  = 4096 // сырой отрезок `themes` в манифесте, байт: больше — поле пропущено целиком
	MaxThemeName  = 20   // знаков имени (колонка тем — 150 px, 170-13)
	MaxPackThemes = 64   // тем наборов в программе: решает таблица decor; здесь — для сверки с файлом случаев
	ThemeOrderMin = 1
	ThemeOrderMax = 99
	ThemeOrderDef = 50 // нет поля order
)

// Причины пропуска — постоянный словарь: значения из манифеста в журнал не попадают (170-30).
const (
	WhyID     = "id"
	WhyName   = "имя"
	WhyBase   = "база"
	WhyOrder  = "order"
	WhyDup    = "повтор"
	WhyLimit  = "предел"
	WhySize   = "размер"
	WhyType   = "тип"
	maxReason = 16 // причин в журнале; остальное — числом
)

// Theme — принятое объявление темы набора.
type Theme struct {
	ID    string
	Name  string
	Base  string // одна из семи встроенных: её видят 0.29–0.39
	Order int    // 1…99, место среди тем одной базы
}

// ThemeSkips — итог пропусков: причины словами словаря и число пропущенных записей.
type ThemeSkips struct {
	Why []string
	N   int
}

// Note — одна строка журнала на набор; "" — пропусков нет. id набора — %q: перевод строки
// в значении не подделает строку (аудит п. 7).
func (s ThemeSkips) Note(setID string) string {
	if s.N == 0 && len(s.Why) == 0 {
		return ""
	}
	return fmt.Sprintf("набор %q: темы пропущены: %d (%s)", setID, s.N, strings.Join(s.Why, ", "))
}

// builtinThemes — встроенные темы программы (база запасного пути, 170-15); id объявить нельзя.
var builtinThemes = [...]string{"basic", "wedding", "kids", "kindergarten", "school", "grad", "holiday"}

// serviceThemes — служебные id: выборки и разделы списка. `constructor` — потому что фронт берёт
// подсказку строки из обычного объекта, и SIDE_TIPS['constructor'] — функция Object (аудит п. 3).
var serviceThemes = [...]string{"recent", "all", "new", "get", "mine", "brand", "legacy", "constructor"}

// reservedNames — имена встроенных и служебных строк списка (регистр — по themeFold).
var reservedNames = [...]string{
	"Недавние", "Все", "Новые", "Можно скачать", "Мои", "Фирменные", "Прежние макеты",
	"Универсальные", "Свадьба", "Дети", "Детский сад", "Школа", "Выпускной", "Праздник",
}

// IsBuiltinTheme — id встроенной темы.
func IsBuiltinTheme(id string) bool {
	for _, b := range builtinThemes {
		if b == id {
			return true
		}
	}
	return false
}

// ValidThemeID — id темы набора: ^[a-z]{1,16}$ (та же грамматика, что у `theme`), не встроенный
// и не служебный.
func ValidThemeID(id string) bool {
	if !validKind(id) || IsBuiltinTheme(id) {
		return false
	}
	for _, s := range serviceThemes {
		if s == id {
			return false
		}
	}
	return true
}

type script uint8

const (
	scriptNone script = iota
	scriptLatin
	scriptCyrillic
)

// ValidThemeName — имя темы по белому списку (170-28): кириллица А–Я, а–я, Ё, ё; латиница; цифры;
// пробел; дефис. Хотя бы одна буква, в слове (между пробелами и дефисами) буквы одной письменности,
// без пробела по краям и двух подряд, 1…20 знаков. Не чистится, а отвергается: так отпадают
// Mn/Me, заполнители хангыля, пробел Брайля, гомоглифы и управляющие знаки.
func ValidThemeName(name string) bool {
	n, letters := 0, 0
	cur := scriptNone // письменность текущего слова
	prev := ' '       // перед началом имени пробела нет, но и «пробел в начале» — отказ
	for i, r := range name {
		n++
		if n > MaxThemeName {
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
			var sc script
			switch {
			case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
				sc = scriptLatin
			case r >= 'А' && r <= 'я' || r == 'Ё' || r == 'ё':
				sc = scriptCyrillic
			default:
				return false // в том числе U+FFFD от битого UTF-8
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

// themeFold — регистр по белому списку, одинаково в Go и JS: A–Z → a–z, А–Я → а–я, Ё → ё.
// strings.ToLower не годится: «İ» и конечная «Σ» у Go и JS расходятся (аудит п. 5).
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

// themeRec — запись `themes` для разбора. Order — указатель: нет поля — 50.
type themeRec struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Base  string `json:"base"`
	Order *int   `json:"order"`
}

// ParseThemes — объявления из сырого поля `themes` (design §3). Больше 4 КиБ — поле пропущено
// целиком; не массив — пусто; принимаются первые 16 записей, каждая отдельно: плохая — пропуск
// с причиной, остальные принимаются. Повтор id и имя-двойник (без учёта регистра) пропускаются.
// raw == nil или null — поля нет, пропусков нет.
func ParseThemes(raw []byte) ([]Theme, ThemeSkips) {
	var sk ThemeSkips
	if len(raw) > MaxThemesRaw {
		return nil, ThemeSkips{Why: []string{WhySize}, N: 1}
	}
	var recs []json.RawMessage
	if err := json.Unmarshal(raw, &recs); err != nil {
		if len(raw) == 0 {
			return nil, sk
		}
		return nil, ThemeSkips{Why: []string{WhyType}, N: 1}
	}
	skip := func(why string) {
		sk.N++
		if len(sk.Why) < maxReason {
			sk.Why = append(sk.Why, why)
		}
	}
	var out []Theme
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, n := range reservedNames {
		names[themeFold(n)] = true
	}
	for i, r := range recs {
		if i >= MaxThemes {
			// 17-я и дальше — числом; слово «предел» — если в журнале ещё есть место
			sk.N += len(recs) - i
			if len(sk.Why) < maxReason {
				sk.Why = append(sk.Why, WhyLimit)
			}
			break
		}
		t, why := themeDecl(r, ids, names)
		if why != "" {
			skip(why)
			continue
		}
		ids[t.ID], names[themeFold(t.Name)] = true, true
		out = append(out, t)
	}
	return out, sk
}

// themeDecl — одна запись: тема или причина пропуска. Порядок проверок общий с JS и decor.
func themeDecl(raw json.RawMessage, ids, names map[string]bool) (Theme, string) {
	var d themeRec
	if err := json.Unmarshal(raw, &d); err != nil {
		return Theme{}, WhyType
	}
	switch {
	case !ValidThemeID(d.ID):
		return Theme{}, WhyID
	case ids[d.ID]:
		return Theme{}, WhyDup
	case !ValidThemeName(d.Name) || names[themeFold(d.Name)]:
		return Theme{}, WhyName
	}
	base := d.Base
	if base == "" {
		base = "basic"
	}
	if !IsBuiltinTheme(base) {
		return Theme{}, WhyBase
	}
	order := ThemeOrderDef
	if d.Order != nil {
		order = *d.Order
	}
	if order < ThemeOrderMin || order > ThemeOrderMax {
		return Theme{}, WhyOrder
	}
	return Theme{ID: d.ID, Name: d.Name, Base: base, Order: order}, ""
}

// topicOf — тема предмета из сырого поля `topic`: строка — id из объявлений этого манифеста или
// встроенная тема (170-23), иначе "": предмет остаётся в базе, манифест цел (будущая версия может
// сделать поле массивом, а 0.29–0.39 такой набор ставят). Служебные id темой предмета не бывают.
func topicOf(raw json.RawMessage, themes []Theme) string {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return ""
	}
	if IsBuiltinTheme(s) {
		return s
	}
	for _, t := range themes {
		if t.ID == s {
			return s
		}
	}
	return ""
}
