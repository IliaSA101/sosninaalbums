package decor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Правило объявления темы набора (спека 170, design §3): копия internal/decor/pack/themes.go —
// decor не импортирует pack (depguard, 170-26), как и validPackID. Обе копии и зеркало JS проходят
// одни случаи testdata/pack-manifest-cases.json (TestThemeRulesCases): правишь здесь — правь там.

// Пределы объявлений (блок limits файла случаев; TestThemeRulesLimits сверяет с pack).
const (
	maxThemeDecls   = 16   // объявлений в наборе: принимаются первые
	maxThemeRaw     = 4096 // сырое поле манифеста; в реестре предела байт нет (170-29)
	maxThemeName    = 20   // знаков имени
	maxPackThemes   = 64   // тем наборов в таблице
	themeOrderMin   = 1
	themeOrderMax   = 99
	themeOrderDef   = 50
	maxThemeReasons = 16
)

// Причины пропуска — постоянный словарь: значения из файла в журнал не попадают (170-30).
const (
	whyID    = "id"
	whyName  = "имя"
	whyBase  = "база"
	whyOrder = "order"
	whyDup   = "повтор"
	whyLimit = "предел"
	whySize  = "размер"
	whyType  = "тип"
)

// themeDecl — принятое объявление темы набора (так же хранится в реестре: ключи id, name, base, order).
type themeDecl struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Base  string `json:"base"` // одна из семи встроенных: её видят 0.29–0.39
	Order int    `json:"order"`
}

// themeSkips — итог пропусков: причины словами словаря и число пропущенных записей.
type themeSkips struct {
	why []string
	n   int
}

func (k *themeSkips) add(why string) {
	k.n++
	if len(k.why) < maxThemeReasons {
		k.why = append(k.why, why)
	}
}

// note — одна строка журнала на набор; "" — пропусков нет. id — %q: перевод строки в значении не
// подделает строку (аудит п. 7).
func (k themeSkips) note(setID string) string {
	if k.n == 0 && len(k.why) == 0 {
		return ""
	}
	return fmt.Sprintf("набор %q: темы пропущены: %d (%s)", setID, k.n, strings.Join(k.why, ", "))
}

// baseThemes — семь встроенных тем: база объявления; id объявить нельзя.
var baseThemes = [...]string{"basic", "wedding", "kids", "kindergarten", "school", "grad", "holiday"}

// serviceThemes — служебные id: выборки и разделы списка; `constructor` — потому что фронт берёт
// подсказку строки из обычного объекта (аудит п. 3).
var serviceThemes = [...]string{"recent", "all", "new", "get", "mine", "brand", "legacy", "constructor"}

// reservedNames — имена встроенных и служебных строк списка (регистр — по themeFold).
var reservedNames = [...]string{
	"Недавние", "Все", "Новые", "Можно скачать", "Мои", "Фирменные", "Прежние макеты",
	"Универсальные", "Свадьба", "Дети", "Детский сад", "Школа", "Выпускной", "Праздник",
}

func isBaseTheme(id string) bool {
	for _, b := range baseThemes {
		if b == id {
			return true
		}
	}
	return false
}

// validThemeID — [a-z]{1,16}, не встроенный и не служебный.
func validThemeID(id string) bool {
	if !validKind(id) || isBaseTheme(id) {
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

// validThemeName — имя по белому списку (170-28): кириллица А–Я, а–я, Ё, ё; латиница; цифры; пробел;
// дефис; хотя бы одна буква; в слове буквы одной письменности; без пробела по краям и двух подряд;
// 1…20 знаков. Не чистится, а отвергается.
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

// themeFold — регистр по белому списку, как в Go (pack) и JS: strings.ToLower не годится — «İ» и
// конечная «Σ» у Go и JS расходятся (аудит п. 5).
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

// reservedFolded — занятые имена для проверки двойников; новая карта на каждый разбор.
func reservedFolded() map[string]bool {
	names := make(map[string]bool, len(reservedNames)+maxThemeDecls)
	for _, n := range reservedNames {
		names[themeFold(n)] = true
	}
	return names
}

// declCheck — одно объявление: приведённое или причина пропуска. Порядок проверок общий с pack и
// JS: id → повтор → имя → база → order. order 0 — «не задан» (50): тип записи не знает «нет поля».
func declCheck(d themeDecl, order *int, ids, names map[string]bool) (themeDecl, string) {
	switch {
	case !validThemeID(d.ID):
		return themeDecl{}, whyID
	case ids[d.ID]:
		return themeDecl{}, whyDup
	case !validThemeName(d.Name) || names[themeFold(d.Name)]:
		return themeDecl{}, whyName
	}
	if d.Base == "" {
		d.Base = "basic"
	}
	if !isBaseTheme(d.Base) {
		return themeDecl{}, whyBase
	}
	d.Order = themeOrderDef
	if order != nil {
		d.Order = *order
	}
	if d.Order < themeOrderMin || d.Order > themeOrderMax {
		return themeDecl{}, whyOrder
	}
	return d, ""
}

// themeRec — запись `themes` для разбора: order — указатель (нет поля — 50).
type themeRec struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Base  string `json:"base"`
	Order *int   `json:"order"`
}

// parseThemeDecl — объявления из сырого поля `themes`: не массив — пусто; первые 16 записей, каждая
// отдельно. maxRaw > 0 — предел сырых байт (манифест, 4 КиБ: больше — поле пропущено целиком); 0 —
// без предела (реестр: его размер держит packRegMax, 170-29). nil и null — поля нет, пропусков нет.
func parseThemeDecl(raw []byte, maxRaw int) ([]themeDecl, themeSkips) {
	var sk themeSkips
	if maxRaw > 0 && len(raw) > maxRaw {
		return nil, themeSkips{why: []string{whySize}, n: 1}
	}
	var recs []json.RawMessage
	if err := json.Unmarshal(raw, &recs); err != nil {
		if len(raw) == 0 {
			return nil, sk
		}
		return nil, themeSkips{why: []string{whyType}, n: 1}
	}
	var out []themeDecl
	ids, names := map[string]bool{}, reservedFolded()
	for i, r := range recs {
		if i >= maxThemeDecls {
			// 17-я и дальше — числом; слово «предел» — если в журнале ещё есть место
			sk.n += len(recs) - i
			if len(sk.why) < maxThemeReasons {
				sk.why = append(sk.why, whyLimit)
			}
			break
		}
		var rec themeRec
		if err := json.Unmarshal(r, &rec); err != nil {
			sk.add(whyType)
			continue
		}
		d, why := declCheck(themeDecl{ID: rec.ID, Name: rec.Name, Base: rec.Base}, rec.Order, ids, names)
		if why != "" {
			sk.add(why)
			continue
		}
		ids[d.ID], names[themeFold(d.Name)] = true, true
		out = append(out, d)
	}
	return out, sk
}

// checkThemeDecls — объявления манифеста, пришедшего от службы наборов, ещё раз по правилу: правило
// живёт в двух копиях, и реестр пишется только из проверенного. Order 0 — не задан.
func checkThemeDecls(in []PackTheme) []themeDecl {
	var out []themeDecl
	ids, names := map[string]bool{}, reservedFolded()
	for i, t := range in {
		if i >= maxThemeDecls {
			break
		}
		var order *int
		if t.Order != 0 {
			order = &t.Order
		}
		d, why := declCheck(themeDecl{ID: t.ID, Name: t.Name, Base: t.Base}, order, ids, names)
		if why != "" {
			continue
		}
		ids[d.ID], names[themeFold(d.Name)] = true, true
		out = append(out, d)
	}
	return out
}
