package model

import (
	"fmt"
	"slices"
)

// Предел разворотов типографии (085): таблица изделий и готовые строки для «Сведений об альбоме»
// и окна «Экспорт». Типография не хранится, а выводится из пресета обложки и формата страницы
// (decisions 085 №1): формат файла не меняется, старые альбомы получают строку сразу.
// Новая типография — профиль printprof (изделия, limits); страж TestPrintGroupsCoverGroups.

// PrintGroup — типография для предела разворотов: имя ряда пресетов, родительный падеж для
// строки предупреждения и изделие по умолчанию (когда обложки этой типографии нет).
type PrintGroup struct {
	Name    string // CoverGroupFsfera, CoverGroupNetprint — как Group у пресетов
	Gen     string // «Фотосферы», «Нетпринта»
	Default string // id пресета обложки: «fsfera-thin», «netprint-premium»
	// NameDefault — называть изделие по умолчанию, когда обложки нет (085 п. 1): у Нетпринта на
	// странице только «Премиум»; у Фотосферы тип листов по странице не узнать — просто «Фотосфера».
	NameDefault bool
}

// PrintLimit — сколько принимает изделие. Строка с Format точнее строки без него.
type PrintLimit struct {
	Product  string // id пресета обложки
	Format   string // подпись кнопки страницы этой группы («20×20»); "" — все форматы
	Min, Max int    // 0 — не задано
	Pages    bool   // типография считает страницы: числа и текст в страницах (разворот = 2)
}

// PrintGroups — типографии с пределом, из профилей по порядку. Функция, а не переменная, как CoverPresets.
func PrintGroups() []PrintGroup { return slices.Clone(tables().pgroups) }

// PrintGroupOf — типография по имени группы пресетов; ok=false — предела у группы нет.
func PrintGroupOf(name string) (PrintGroup, bool) { return tables().printGroupOf(name) }

// printGroupOf — PrintGroupOf по таблицам t.
func (t *printTables) printGroupOf(name string) (PrintGroup, bool) {
	if i := slices.IndexFunc(t.pgroups, func(g PrintGroup) bool { return g.Name == name }); i >= 0 && name != "" {
		return t.pgroups[i], true
	}
	return PrintGroup{}, false
}

// CoverDefaultFor — изделие новой обложки «по умолчанию» для страницы s: изделие по умолчанию
// типографии, чья кнопка страницы подходит (PagePresetOf). Без этого обложка на странице Фабрики
// получала бы «Фотосферу · Тонкие листы» (CoverDefault). Страница вне рядов — CoverDefault.
func CoverDefaultFor(s Settings) string {
	if pg, ok := PagePresetOf(s); ok {
		if g, ok := PrintGroupOf(pg.Group); ok {
			return g.Default
		}
	}
	return CoverDefault
}

// PrintLimits — пределы изделий из профилей (строки limits). Верх, не заданный в профиле, — последняя
// строка таблицы корешка (один источник: за таблицей корешок не посчитать). Нижнего предела Фотосферы
// и пределов мягкой обложки Нетпринта на сайтах нет — строк нет (decisions №6); верх «Премиум»
// 20×20 и 18×13 на сайте не указан — 60, как у остальных (гипотеза, decisions №5). Фабрика — CSV
// шаблонов (081, TestFabrikaSpineMatchesPSD).
func PrintLimits() []PrintLimit { return slices.Clone(tables().limits) }

// LimitFor — предел изделия product в формате format: сначала строка формата, затем общая.
// Нет ни той, ни другой — изделия у формата нет (081 С2: лайфлат Фабрики 25×25).
func LimitFor(product, format string) (PrintLimit, bool) { return tables().limitFor(product, format) }

// limitFor — LimitFor по таблицам t.
func (t *printTables) limitFor(product, format string) (PrintLimit, bool) {
	for _, f := range []string{format, ""} {
		if i := slices.IndexFunc(t.limits, func(l PrintLimit) bool { return l.Product == product && l.Format == f }); i >= 0 {
			return t.limits[i], true
		}
	}
	return PrintLimit{}, false
}

// LimitOf — предел разворотов проекта p и строки для вида; ok=false — типография неизвестна,
// у изделия нет предела или это лист 031. Обложка с пресетом типографии называет изделие и
// типографию; без неё (или «Свои размеры») типографию даёт кнопка страницы, изделие — по умолчанию.
func LimitOf(p *Project) (PageLimitView, bool) {
	if p.Settings.Sheet {
		return PageLimitView{}, false
	}
	t := tables() // одна справка на всю операцию: пресет, группа, кнопка и предел — из одного снимка
	group := ""
	if c := p.Cover; c != nil {
		if pr, ok := t.coverPresetOf(c.Spec.Preset); ok {
			if _, ok := t.printGroupOf(pr.Group); ok {
				group = pr.Group
			}
		}
	}
	if group == "" {
		page, ok := t.pagePresetOf(p.Settings)
		if !ok {
			return PageLimitView{}, false
		}
		group = page.Group
	}
	return t.limitIn(p, group)
}

// LimitIn — предел, если книга p идёт в типографию group (выгрузка «для Фабрики» берёт его
// независимо от того, какую типографию выводит LimitOf). Изделие — пресет обложки этой группы,
// иначе изделие по умолчанию (названо по NameDefault); формат строки — кнопка ряда group с
// размером страницы (PagePresetIn), нет её — общая строка. ok=false — у группы нет предела,
// у изделия нет строки (изделия у формата нет) или это лист 031.
func LimitIn(p *Project, group string) (PageLimitView, bool) { return tables().limitIn(p, group) }

// limitIn — LimitIn по таблицам t.
func (t *printTables) limitIn(p *Project, group string) (PageLimitView, bool) {
	g, ok := t.printGroupOf(group)
	if !ok || p.Settings.Sheet {
		return PageLimitView{}, false
	}
	product, name := g.Default, ""
	if c := p.Cover; c != nil {
		if pr, ok := t.coverPresetOf(c.Spec.Preset); ok && pr.Group == group {
			product, name = pr.ID, pr.Name
		}
	}
	if name == "" && g.NameDefault {
		if pr, ok := t.coverPresetOf(g.Default); ok {
			name = pr.Name
		}
	}
	format := ""
	if page, ok := t.pagePresetIn(p.Settings, group); ok {
		format = page.Label
	}
	l, ok := t.limitFor(product, format)
	if !ok {
		return PageLimitView{}, false
	}
	return limitView(len(p.Spreads), l, g, name), true
}

// limitView — строки вида для spreads разворотов изделия name ("" — не названо) типографии g.
func limitView(spreads int, l PrintLimit, g PrintGroup, name string) PageLimitView {
	n := spreads
	if l.Pages {
		n *= 2
	}
	who := g.Name
	if name != "" {
		who += " «" + name + "»"
	}
	return PageLimitView{Who: who, Range: rangeText(l), Count: n, Min: l.Min, Max: l.Max, Warn: LimitText(n, l, g.Gen, name)}
}

// LimitText — строка предупреждения: «Разворотов 62 — у Нетпринта («Премиум») не больше 60»;
// product "" — изделие не названо («у Нетпринта»). В пределах — "". Нижний предел у пустого
// альбома не напоминает (085 сц. 5).
func LimitText(count int, l PrintLimit, gen, product string) string {
	who := gen
	if product != "" {
		who += " («" + product + "»)"
	}
	noun := "Разворотов"
	if l.Pages {
		noun = "Страниц"
	}
	switch {
	case l.Max > 0 && count > l.Max:
		return fmt.Sprintf("%s %d — у %s не больше %d", noun, count, who, l.Max)
	case count > 0 && count < l.Min:
		return fmt.Sprintf("%s %d — у %s не меньше %d", noun, count, who, l.Min)
	}
	return ""
}

// rangeText — «от 10 до 60 разворотов», «до 57 разворотов», «от 10 разворотов»; склонение —
// по последнему числу (родительный: «до 21 разворота»).
func rangeText(l PrintLimit) string {
	word := func(n int) string {
		one := n%10 == 1 && n%100 != 11
		switch {
		case l.Pages && one:
			return "страницы"
		case l.Pages:
			return "страниц"
		case one:
			return "разворота"
		}
		return "разворотов"
	}
	switch {
	case l.Min > 0 && l.Max > 0:
		return fmt.Sprintf("от %d до %d %s", l.Min, l.Max, word(l.Max))
	case l.Max > 0:
		return fmt.Sprintf("до %d %s", l.Max, word(l.Max))
	case l.Min > 0:
		return fmt.Sprintf("от %d %s", l.Min, word(l.Min))
	}
	return ""
}
