package model

import "testing"

// sharedBook — книга w×h из n разворотов с обложкой cover и полями страницы (300 dpi, layflat).
func sharedBook(w, h float64, n int, cover string, safe, bleed float64) *Project {
	p := limitProject(w, h, n, cover)
	p.Settings.DPI, p.Settings.Layflat, p.Settings.Safe, p.Settings.Bleed = 300, true, safe, bleed
	return p
}

// TestLimitSharedPage (081 С3–С4): размеры, общие у нескольких типографий. Обложка Фабрики на
// 300×305 берёт строку 30×30 Фабрики (а не кнопку Нетпринта); без обложки остаётся Нетпринт (сц. QA
// 085); 21×30 без обложки различают зона и обрез: Фабрика 10/3, Фотосфера 8/2; LimitIn не зависит
// от типографии, которую выводит LimitOf.
func TestLimitSharedPage(t *testing.T) {
	for _, c := range []struct {
		name     string
		p        *Project
		who      string
		min, max int
		warn     string
	}{
		{"Фабрика на 300×305", sharedBook(300, 305, 9, "fabrika-photo", 10, 3), "Фабрика Фотокниги «Фотобумага»", 10, 40,
			"Разворотов 9 — у Фабрики Фотокниги («Фотобумага») не меньше 10"},
		{"Нетпринт на 300×305 с теми же полями", sharedBook(300, 305, 20, "netprint-premium", 10, 3), "Нетпринт «Премиум»", 10, 60, ""},
		{"300×305 без обложки — Нетпринт", sharedBook(300, 305, 20, "", 10, 3), "Нетпринт «Премиум»", 10, 60, ""},
		{"20×20 без обложки, поля одинаковы — Нетпринт", sharedBook(200, 203, 20, "", 10, 3), "Нетпринт «Премиум»", 10, 60, ""},
		{"20×20 с обложкой Фабрики", sharedBook(200, 203, 21, "fabrika-layflat-card", 10, 3), "Фабрика Фотокниги «Лайфлат, картон»", 2, 20,
			"Разворотов 21 — у Фабрики Фотокниги («Лайфлат, картон») не больше 20"},
		{"21×30 с полями Фабрики", sharedBook(208, 296, 41, "", 10, 3), "Фабрика Фотокниги", 0, 40,
			"Разворотов 41 — у Фабрики Фотокниги не больше 40"},
		{"21×30 с полями Фотосферы", sharedBook(208, 296, 58, "", 8, 2), "Фотосфера", 0, 57,
			"Разворотов 58 — у Фотосферы не больше 57"},
		{"21×30 с чужими полями — первая по порядку, Фотосфера", sharedBook(208, 296, 3, "", 5, 0), "Фотосфера", 0, 57, ""},
		{"30×20 только у Фабрики", sharedBook(304, 203, 3, "", 5, 0), "Фабрика Фотокниги", 0, 40, ""},
	} {
		got, ok := LimitOf(c.p)
		if !ok || got.Who != c.who || got.Min != c.min || got.Max != c.max || got.Warn != c.warn {
			t.Errorf("%s: %+v (%v), ждали %s %d–%d %q", c.name, got, ok, c.who, c.min, c.max, c.warn)
		}
	}
	// LimitIn: типография выбрана вызывающим; обложка чужой группы изделия не называет.
	np := sharedBook(300, 305, 58, "netprint-premium", 10, 3)
	if got, ok := LimitIn(np, CoverGroupFsfera); !ok || got.Warn != "Разворотов 58 — у Фотосферы не больше 57" || got.Who != CoverGroupFsfera {
		t.Errorf("LimitIn Фотосфера при обложке Нетпринта: %+v (%v)", got, ok)
	}
	if got, ok := LimitIn(np, CoverGroupFabrika); !ok || got.Min != 10 || got.Max != 40 || got.Warn != "Разворотов 58 — у Фабрики Фотокниги не больше 40" {
		t.Errorf("LimitIn Фабрика при обложке Нетпринта: %+v (%v)", got, ok)
	}
	for _, g := range []string{"", "нет такой"} {
		if got, ok := LimitIn(np, g); ok {
			t.Errorf("LimitIn(%q) = %+v", g, got)
		}
	}
	if got, ok := LimitIn(NewSheet(300, 305), CoverGroupFabrika); ok {
		t.Errorf("лист 031: %+v", got)
	}
	// Лайфлат 25×25 — изделия у формата нет: LimitIn предела не даёт.
	if got, ok := LimitIn(sharedBook(250, 254, 12, "fabrika-layflat-paper", 10, 3), CoverGroupFabrika); ok {
		t.Errorf("лайфлат 25×25: %+v", got)
	}
}

// TestPagePresetIn (081 С4): кнопка именно этого ряда; общий ряд, лист 031 и размер вне ряда —
// нет. PagePresetOf: при равных размерах первой выбирается кнопка, совпавшая по всем полям,
// иначе первая по порядку.
func TestPagePresetIn(t *testing.T) {
	st := Settings{PageW: 300, PageH: 305, DPI: 300, Layflat: true, Safe: 10, Bleed: 3}
	for _, group := range []string{CoverGroupNetprint, CoverGroupFabrika} {
		if pr, ok := PagePresetIn(st, group); !ok || pr.Group != group || pr.Label != "30×30" {
			t.Errorf("%s: %+v (%v)", group, pr, ok)
		}
	}
	for _, g := range []string{"", CoverGroupFsfera} {
		if pr, ok := PagePresetIn(st, g); ok {
			t.Errorf("%q: %+v", g, pr)
		}
	}
	sheet := st
	sheet.Sheet = true
	if pr, ok := PagePresetIn(sheet, CoverGroupFabrika); ok {
		t.Errorf("лист: %+v", pr)
	}
	// Поля Фабрики 21×30 (зона 10, обрез 3) выбирают её кнопку; чужие — первую (Фотосфера).
	st = Settings{PageW: 208, PageH: 296, DPI: 300, Layflat: true, Safe: 10, Bleed: 3}
	if pr, ok := PagePresetOf(st); !ok || pr.Group != CoverGroupFabrika {
		t.Errorf("поля Фабрики: %+v", pr)
	}
	st.Safe, st.Bleed = 8, 2
	if pr, ok := PagePresetOf(st); !ok || pr.Group != CoverGroupFsfera {
		t.Errorf("поля Фотосферы: %+v", pr)
	}
	st.DPI = 150 // ни одна не совпала целиком — первая по порядку
	if pr, ok := PagePresetOf(st); !ok || pr.Group != CoverGroupFsfera {
		t.Errorf("чужие поля: %+v", pr)
	}
}

// TestCoverDefaultFor: изделие новой обложки по странице — по умолчанию типографии, чья кнопка
// подошла (зона и обрез различают 21×30 Фабрики и Фотосферы); вне рядов — CoverDefault.
func TestCoverDefaultFor(t *testing.T) {
	for _, c := range []struct {
		name        string
		w, h        float64
		safe, bleed float64
		want        string
	}{
		{"30×20 Фабрики", 304, 203, 10, 3, "fabrika-photo"},
		{"300×300 вне рядов", 300, 300, 10, 3, CoverDefault},
		{"30×20 Фотосферы", 286, 203, 10, 3, "fsfera-thin"},
		{"300×305 общий размер — Нетпринт", 300, 305, 10, 3, "netprint-premium"},
		{"21×30 с полями Фабрики", 208, 296, 10, 3, "fabrika-photo"},
		{"21×30 с полями Фотосферы", 208, 296, 8, 2, "fsfera-thin"},
	} {
		if got := CoverDefaultFor(sharedBook(c.w, c.h, 1, "", c.safe, c.bleed).Settings); got != c.want {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
	if got := CoverDefaultFor(NewSheet(304, 203).Settings); got != CoverDefault {
		t.Errorf("лист 031: %q", got)
	}
}
