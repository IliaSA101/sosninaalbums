package model

import (
	"slices"
	"testing"
)

// limitProject — книга w×h мм из n разворотов с обложкой пресета cover ("" — без обложки).
func limitProject(w, h float64, n int, cover string) *Project {
	p := New()
	p.Settings.PageW, p.Settings.PageH = w, h
	for range n {
		p.Spreads = append(p.Spreads, &Spread{ID: NewID(), Photos: []string{}})
	}
	if cover != "" {
		p.Cover = &Cover{Sheet: &Spread{ID: CoverID}, Spec: CoverSpec{Preset: cover}}
	}
	return p
}

// TestPrintLimitResolve — типография альбома (085 сц. 1): обложка с пресетом типографии
// называет изделие и важнее страницы; без неё и у «Своих размеров» — кнопка страницы и изделие
// по умолчанию; общий ряд, лист 031, мягкая обложка Нетпринта — предела нет.
func TestPrintLimitResolve(t *testing.T) {
	sheet := NewSheet(281, 284) // размер страницы Фотосферы, но лист — не фотокнига
	for _, c := range []struct {
		name string
		p    *Project
		want *PageLimitView
	}{
		{"Картон на Нетпринте", limitProject(200, 203, 20, "fsfera-card"),
			&PageLimitView{Who: "Фотосфера «Картон»", Range: "до 30 разворотов", Count: 20, Max: 30}},
		{"Нетпринт 20×20 без обложки", limitProject(200, 203, 20, ""),
			&PageLimitView{Who: "Нетпринт «Премиум»", Range: "от 10 до 60 разворотов", Count: 20, Min: 10, Max: 60}},
		{"Свои размеры на Фотосфере", limitProject(281, 284, 3, CoverCustom),
			&PageLimitView{Who: "Фотосфера", Range: "до 57 разворотов", Count: 3, Max: 57}},
		{"чужой пресет обложки", limitProject(281, 284, 58, "future-x"),
			&PageLimitView{Who: "Фотосфера", Range: "до 57 разворотов", Count: 58, Max: 57, Warn: "Разворотов 58 — у Фотосферы не больше 57"}},
		{"Премиум, 62", limitProject(300, 305, 62, "netprint-premium"),
			&PageLimitView{Who: "Нетпринт «Премиум»", Range: "от 10 до 60 разворотов", Count: 62, Min: 10, Max: 60,
				Warn: "Разворотов 62 — у Нетпринта («Премиум») не больше 60"}},
		{"Нетпринт, 6", limitProject(200, 203, 6, ""),
			&PageLimitView{Who: "Нетпринт «Премиум»", Range: "от 10 до 60 разворотов", Count: 6, Min: 10, Max: 60,
				Warn: "Разворотов 6 — у Нетпринта («Премиум») не меньше 10"}},
		{"Нетпринт, 61, без обложки", limitProject(200, 203, 61, ""),
			&PageLimitView{Who: "Нетпринт «Премиум»", Range: "от 10 до 60 разворотов", Count: 61, Min: 10, Max: 60,
				Warn: "Разворотов 61 — у Нетпринта («Премиум») не больше 60"}},
		{"пустой Нетпринт", limitProject(200, 203, 0, ""),
			&PageLimitView{Who: "Нетпринт «Премиум»", Range: "от 10 до 60 разворотов", Min: 10, Max: 60}},
		{"общий 30×30", limitProject(300, 300, 20, ""), nil},
		{"общий 20×20 у Нетпринта 200×203", limitProject(200, 200, 20, ""), nil},
		{"лист 031", sheet, nil},
		{"мягкая Нетпринта", limitProject(200, 203, 20, "netprint-soft"), nil},
		{"Свои размеры, общий формат", limitProject(300, 300, 20, CoverCustom), nil},
	} {
		got, ok := LimitOf(c.p)
		switch {
		case c.want == nil && ok:
			t.Errorf("%s: предел %+v, ждали нет", c.name, got)
		case c.want != nil && (!ok || got != *c.want):
			t.Errorf("%s: %+v (%v),\nждали %+v", c.name, got, ok, *c.want)
		}
	}
}

// TestPrintLimitFormatRow — строка формата точнее общей: Royal 20×20 — от 10, Royal 30×30 —
// от 8; Royal на странице Фотосферы — формат не Нетпринта, общая строка.
func TestPrintLimitFormatRow(t *testing.T) {
	for _, c := range []struct {
		name   string
		w, h   float64
		min    int
		rangeS string
	}{
		{"20×20", 200, 203, 10, "от 10 до 30 разворотов"},
		{"30×30", 300, 305, 8, "от 8 до 30 разворотов"},
		{"Фотосфера 20×20", 201, 203, 8, "от 8 до 30 разворотов"},
	} {
		l, ok := LimitOf(limitProject(c.w, c.h, 9, "netprint-royal"))
		wantWarn := ""
		if c.min > 9 {
			wantWarn = "Разворотов 9 — у Нетпринта («Royal») не меньше 10"
		}
		if !ok || l.Min != c.min || l.Max != 30 || l.Range != c.rangeS || l.Who != "Нетпринт «Royal»" || l.Warn != wantWarn {
			t.Errorf("Royal %s: %+v (%v)", c.name, l, ok)
		}
	}
}

// TestFsferaLimitsMatchSpine — предел Фотосферы = последняя строка таблицы корешка (один
// источник): на пределе корешок из таблицы, за ним — «вне таблицы». Без обложки — тонкие листы.
func TestFsferaLimitsMatchSpine(t *testing.T) {
	want := map[string]int{"fsfera-thin": 57, "fsfera-card": 30, "fsfera-double": 20}
	for _, pr := range CoverPresets() {
		if pr.Group != CoverGroupFsfera {
			continue
		}
		l, ok := LimitFor(pr.ID, "")
		st := pr.Spec.Steps
		if !ok || l.Max != st[len(st)-1].Upto || l.Max != want[pr.ID] || l.Min != 0 {
			t.Errorf("%s: %+v (%v), таблица до %d", pr.ID, l, ok, st[len(st)-1].Upto)
		}
		if _, over := pr.Spec.Spine(l.Max); over {
			t.Errorf("%s: на пределе %d корешок вне таблицы", pr.ID, l.Max)
		}
		if _, over := pr.Spec.Spine(l.Max + 1); !over {
			t.Errorf("%s: за пределом %d корешок в таблице", pr.ID, l.Max+1)
		}
		delete(want, pr.ID)
	}
	if len(want) != 0 {
		t.Errorf("нет пресетов %v", want)
	}
	for _, f := range FsferaFormats() {
		if l, ok := LimitOf(limitProject(f.PageW, f.PageH, 1, "")); !ok || l.Max != 57 || l.Who != CoverGroupFsfera {
			t.Errorf("Фотосфера %s без обложки: %+v (%v)", f.Name, l, ok)
		}
	}
}

// TestPrintGroupsCoverGroups — страж для новой типографии (081): у каждой группы пресетов
// обложки и страницы, кроме общей, есть PrintGroup; изделие по умолчанию — пресет обложки этой
// группы со строкой предела; строки таблицы — про известные изделия и форматы своей группы.
func TestPrintGroupsCoverGroups(t *testing.T) {
	groups := map[string]bool{}
	for _, c := range CoverPresets() {
		groups[c.Group] = true
	}
	for _, pp := range PagePresets() {
		groups[pp.Group] = true
	}
	delete(groups, "")
	for g := range groups {
		pg, ok := PrintGroupOf(g)
		if !ok || pg.Gen == "" {
			t.Errorf("у группы %q нет PrintGroup", g)
			continue
		}
		if pr, ok := CoverPresetOf(pg.Default); !ok || pr.Group != g {
			t.Errorf("%s: изделие по умолчанию %q не из группы", g, pg.Default)
		}
		if _, ok := LimitFor(pg.Default, ""); !ok {
			t.Errorf("%s: у изделия по умолчанию нет предела", g)
		}
	}
	if _, ok := PrintGroupOf(""); ok {
		t.Error("у общего ряда предела нет")
	}
	for _, l := range PrintLimits() {
		pr, ok := CoverPresetOf(l.Product)
		if !ok || !groups[pr.Group] {
			t.Errorf("строка %+v: изделие не из группы типографии", l)
			continue
		}
		if l.Format != "" && !slices.ContainsFunc(PagePresets(), func(pp PagePreset) bool { return pp.Group == pr.Group && pp.Label == l.Format }) {
			t.Errorf("строка %+v: формата нет в ряду %s", l, pr.Group)
		}
		if l.Max > 0 && l.Min > l.Max {
			t.Errorf("строка %+v: от больше до", l)
		}
	}
}

// TestLimitText — текст предупреждения и диапазона: больше, меньше, на границах, пустой альбом,
// изделие из обложки и по умолчанию, «страниц» при Pages, склонение по последнему числу.
func TestLimitText(t *testing.T) {
	np := PrintLimit{Min: 10, Max: 60}
	for _, c := range []struct {
		n             int
		l             PrintLimit
		product, want string
	}{
		{62, np, "Премиум", "Разворотов 62 — у Нетпринта («Премиум») не больше 60"},
		{61, np, "", "Разворотов 61 — у Нетпринта не больше 60"},
		{60, np, "Премиум", ""},
		{10, np, "Премиум", ""},
		{9, np, "Премиум", "Разворотов 9 — у Нетпринта («Премиум») не меньше 10"},
		{6, np, "", "Разворотов 6 — у Нетпринта не меньше 10"},
		{0, np, "", ""}, // пустой альбом: нижний предел не напоминает
		{1000, PrintLimit{Min: 10}, "", ""},
		{124, PrintLimit{Min: 20, Max: 120, Pages: true}, "Классик", "Страниц 124 — у Нетпринта («Классик») не больше 120"},
		{18, PrintLimit{Min: 20, Max: 120, Pages: true}, "", "Страниц 18 — у Нетпринта не меньше 20"},
	} {
		if got := LimitText(c.n, c.l, "Нетпринта", c.product); got != c.want {
			t.Errorf("LimitText(%d, %+v, %q) = %q, ждали %q", c.n, c.l, c.product, got, c.want)
		}
	}
	for l, want := range map[PrintLimit]string{
		{Max: 57}: "до 57 разворотов", {Max: 1}: "до 1 разворота", {Max: 21}: "до 21 разворота", {Max: 11}: "до 11 разворотов",
		{Min: 10, Max: 60}: "от 10 до 60 разворотов", {Min: 10}: "от 10 разворотов", {Min: 1}: "от 1 разворота",
		{Min: 20, Max: 120, Pages: true}: "от 20 до 120 страниц", {Max: 21, Pages: true}: "до 21 страницы", {}: "",
	} {
		if got := rangeText(l); got != want {
			t.Errorf("rangeText(%+v) = %q, ждали %q", l, got, want)
		}
	}
	// Pages: счёт в страницах (разворот = 2), числа предела — как в таблице.
	g := PrintGroup{Name: "Типография", Gen: "Типографии"}
	v := limitView(62, PrintLimit{Min: 20, Max: 120, Pages: true}, g, "Классик")
	want := PageLimitView{Who: "Типография «Классик»", Range: "от 20 до 120 страниц", Count: 124, Min: 20, Max: 120,
		Warn: "Страниц 124 — у Типографии («Классик») не больше 120"}
	if v != want {
		t.Errorf("limitView в страницах: %+v,\nждали %+v", v, want)
	}
}

// PagePresetOf: страница Фотосферы узнаётся по размеру, лист 031 того же размера — нет (кнопки
// ряда страниц у листа нет).
func TestPagePresetOfSheet(t *testing.T) {
	f := FsferaFormats()[0]
	st := New().Settings
	st.PageW, st.PageH = f.PageW, f.PageH
	if pr, ok := PagePresetOf(st); !ok || pr.Group != CoverGroupFsfera {
		t.Fatalf("книга %v×%v: %+v %v", f.PageW, f.PageH, pr, ok)
	}
	st.Sheet = true
	if pr, ok := PagePresetOf(st); ok {
		t.Fatalf("лист: %+v", pr)
	}
}
