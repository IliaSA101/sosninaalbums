package model

import (
	"slices"
	"strings"
)

// «Фабрика Фотокниги» (081): форматы Лайфлата и «Фотокниги на фотобумаге», шесть изделий для
// обложки и строки предела разворотов. Сайт 19.10 (docs/research/fabrika-requirements.md): лист
// обложки и разворот точны, заворот, сторонка, обрез и безопасная зона — гипотеза до PSD
// (decisions 081 №2–3).

// CoverGroupFabrika — группа пресетов обложки и ряд пресетов страницы Фабрики Фотокниги.
const CoverGroupFabrika = "Фабрика Фотокниги"

// Шаблон Фабрики: безопасная зона и обрез разворота от края файла, мм.
const (
	fabrikaSafe  = 10.0
	fabrikaBleed = 3.0
	fabrikaWrap  = 17.0 // заворот у всех форматов, кроме 30×30 (там 12 — иначе сторонка ниже страницы)
)

// FabrikaFormats — форматы Фабрики в порядке сайта. Страница — половина разворота сайта
// (30×30 → 600×305), сторонка и заворот делят лист обложки «основа + корешок» (20×20 → 446×240
// при корешке 0). Функция, а не переменная, как FsferaFormats.
func FabrikaFormats() []PrintFormat {
	f := func(name string, pw, ph, sw, sh, wrap float64) PrintFormat {
		return PrintFormat{ID: "fabrika-" + strings.ReplaceAll(name, "×", "x"), Name: name,
			PageW: pw, PageH: ph, Safe: fabrikaSafe, Bleed: fabrikaBleed, SideW: sw, SideH: sh, Wrap: wrap}
	}
	return []PrintFormat{
		f("20×20", 200, 203, 206, 206, fabrikaWrap),
		f("21×30", 208, 296, 211, 296, fabrikaWrap),
		f("25×25", 250, 254, 255, 256, fabrikaWrap),
		f("30×20", 304, 203, 310, 206, fabrikaWrap),
		f("30×30", 300, 305, 312, 306, 12),
	}
}

// fabrikaCoverPresets — изделия Фабрики: корешок зависит от бумаги и прослойки, а не от формата
// (таблица сайта на изделие), кроме 30×20 / 2 мм — fabrikaStepsFor. Safe и SpineSafe 0 — безопасная зона из настроек альбома;
// сторонку и заворот по формату страницы подставляет CoverPresetFor.
func fabrikaCoverPresets() []CoverPreset {
	fb := func(id, name string, st []SpineStep) CoverPreset {
		return CoverPreset{ID: id, Name: name, Group: CoverGroupFabrika, Spec: CoverSpec{Preset: id, Wrap: fabrikaWrap, Steps: st}}
	}
	return []CoverPreset{
		fb("fabrika-photo", "Фотобумага", steps(8, 8, 12, 10, 16, 12, 20, 14, 24, 16, 27, 18, 31, 20, 35, 22, 39, 24, 40, 26)),
		fb("fabrika-photo-1", "Фотобумага, прослойка 1 мм", steps(4, 8, 6, 10, 8, 12, 10, 14, 12, 16, 14, 18, 16, 20, 18, 22, 20, 24, 22, 26, 24, 28, 26, 30, 28, 32, 30, 34)),
		fb("fabrika-photo-2", "Фотобумага, прослойка 2 мм", steps(2, 8, 3, 10, 4, 12, 5, 14, 6, 16, 7, 18, 8, 20, 9, 22)),
		fb("fabrika-layflat-paper", "Лайфлат, бумага", steps(13, 10, 17, 12, 21, 14, 26, 16, 30, 18)),
		fb("fabrika-layflat-card", "Лайфлат, картон", steps(7, 8, 10, 10, 13, 12, 16, 14, 20, 16)),
		fb("fabrika-layflat-card-1", "Лайфлат, картон + прослойка 1 мм", steps(4, 8, 5, 10, 7, 12, 9, 14, 11, 16, 12, 18)),
	}
}

// fabrikaPagePresets — ряд кнопок страницы из FabrikaFormats: 300 dpi, layflat, зона и обрез таблицы.
func fabrikaPagePresets() []PagePreset {
	all := FabrikaFormats()
	out := make([]PagePreset, len(all))
	for i, f := range all {
		out[i] = PagePreset{Label: f.Name, Group: CoverGroupFabrika, W: f.PageW, H: f.PageH, DPI: 300, Layflat: true, Safe: f.Safe, Bleed: f.Bleed,
			Hint: "Фабрика Фотокниги: JPEG sRGB 300 dpi, наибольшее качество; 1-00.jpg — обложка, 1-01.jpg… — развороты; важное — не ближе " + MmText(f.Safe) + " мм к краю"}
	}
	return out
}

// fabrikaStepsFor — таблица корешка изделия id для формата page. Таблица общая для форматов, кроме
// одного: у 30×20 прослойка 2 мм идёт до 10 разворотов (корешок 24), у остальных — до 9 (PSD-шаблоны
// Фабрики 24.10, CSV «Photobook 30x20.csv»). Остальным форматам таблица изделия не меняется — обложки
// с ней не становятся «не по шаблону».
func fabrikaStepsFor(id, page string, base []SpineStep) []SpineStep {
	if id == "fabrika-photo-2" && page == "30×20" {
		return append(slices.Clone(base), SpineStep{Upto: 10, Mm: 24})
	}
	return base
}

// fabrikaLimits — строки предела (081 С1–С2) по CSV шаблонов Фабрики (TestFabrikaSpineMatchesPSD).
// Max задан явно и равен последней строке таблицы корешка формата (fabrikaStepsFor); «от 1» сайта —
// Min 0. Прослойка 1 и 2 мм у 20×20, 21×30, 25×25 — с 2 разворотов, у 30×20 — с 1. Нет строки у
// формата — изделия нет (лайфлат только 20×20 и 21×30, прослойка 2 мм — без 30×30): LimitFor
// не находит ничего.
func fabrikaLimits() []PrintLimit {
	row := func(id, format string, from, to int) PrintLimit {
		return PrintLimit{Product: id, Format: format, Min: from, Max: to}
	}
	// 30×30 принимает от 10 разворотов на любой бумаге, где он бывает.
	out := []PrintLimit{
		row("fabrika-photo", "", 0, 40), row("fabrika-photo", "30×30", 10, 40),
		row("fabrika-photo-1", "", 0, 30), row("fabrika-photo-1", "30×30", 10, 30),
	}
	for _, f := range []string{"20×20", "21×30", "25×25"} {
		out = append(out, row("fabrika-photo-1", f, 2, 30), row("fabrika-photo-2", f, 2, 9))
	}
	out = append(out, row("fabrika-photo-2", "30×20", 0, 10))
	for _, f := range []string{"20×20", "21×30"} {
		out = append(out, row("fabrika-layflat-paper", f, 10, 30), row("fabrika-layflat-card", f, 2, 20), row("fabrika-layflat-card-1", f, 2, 12))
	}
	return out
}
