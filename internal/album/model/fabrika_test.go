package model

import (
	"math"
	"slices"
	"testing"
)

// fabrikaProject — книга на странице формата f из n разворотов с обложкой пресета id (по шаблону).
func fabrikaProject(f PrintFormat, n int, id string) *Project {
	pr, _ := CoverPresetFor(id, Settings{PageW: f.PageW, PageH: f.PageH})
	p := fsferaProject(f.PageW, f.PageH, n, pr.Spec)
	p.Settings.Safe, p.Settings.Bleed = f.Safe, f.Bleed
	return p
}

// TestFabrikaFormats (081): пять форматов сайта; лист обложки «Фотобумага» при корешке 8 и файл
// разворота в px при 300 dpi равны числам сайта (research); зона 10, обрез 3, ID уникальны и
// формат узнаётся по странице; лист 031 того же размера — не формат.
func TestFabrikaFormats(t *testing.T) {
	px := func(mm float64) int { return int(math.Round(mm / 25.4 * 300)) }
	want := map[string]struct {
		cover [2]float64 // лист обложки при корешке 8, мм
		dev   [2]int     // разворот, px
	}{
		"20×20": {[2]float64{454, 240}, [2]int{4724, 2398}},
		"21×30": {[2]float64{464, 330}, [2]int{4913, 3496}},
		"25×25": {[2]float64{552, 290}, [2]int{5906, 3000}},
		"30×20": {[2]float64{662, 240}, [2]int{7181, 2398}},
		"30×30": {[2]float64{656, 330}, [2]int{7087, 3602}},
	}
	all := FabrikaFormats()
	if len(all) != len(want) {
		t.Fatalf("форматов %d", len(all))
	}
	ids := map[string]bool{}
	for _, f := range all {
		w, ok := want[f.Name]
		if !ok || ids[f.ID] {
			t.Fatalf("лишний формат или повтор ID: %+v", f)
		}
		ids[f.ID] = true
		p := fabrikaProject(f, 1, "fabrika-photo") // 1 разворот — первая строка таблицы, корешок 8
		sg, cg := p.Geo(p.Spreads[0]), p.Geo(p.Cover.Sheet)
		if got := [2]int{px(sg.W), px(sg.H)}; got != w.dev {
			t.Errorf("%s: разворот %v px, ждали %v", f.Name, got, w.dev)
		}
		if math.Abs(cg.W-w.cover[0]) > 0.01 || math.Abs(cg.H-w.cover[1]) > 0.01 {
			t.Errorf("%s: лист обложки %v × %v, ждали %v", f.Name, cg.W, cg.H, w.cover)
		}
		if f.Safe != 10 || f.Bleed != 3 {
			t.Errorf("%s: зона %v, обрез %v", f.Name, f.Safe, f.Bleed)
		}
		if got, ok := FormatOf(CoverGroupFabrika, ClampSettings(Settings{PageW: f.PageW, PageH: f.PageH})); !ok || got != f {
			t.Errorf("%s: формат по странице не найден: %+v", f.Name, got)
		}
		// Сторонка не меньше страницы: заворот не съедает её.
		if f.SideW < f.PageW || f.SideH < f.PageH {
			t.Errorf("%s: сторонка %v × %v меньше страницы", f.Name, f.SideW, f.SideH)
		}
	}
	if _, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 300, PageH: 305, Sheet: true}); ok {
		t.Error("лист 031 принят за фотокнигу")
	}
	if _, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 300.01, PageH: 305}); ok {
		t.Error("300,01 × 305 принят за 30×30")
	}
	if _, ok := FormatOf(CoverGroupNetprint, Settings{PageW: 300, PageH: 305}); ok {
		t.Error("у Нетпринта шаблона по формату нет")
	}
	if f := FormatNear("", Settings{PageW: 300, PageH: 305}); f != (PrintFormat{}) {
		t.Errorf("нешаблонная группа: %+v", f)
	}
}

// TestFabrikaCoverPresets (081): шесть изделий группы по порядку сайта между Нетпринтом и
// «Своими размерами»; таблицы корешка по возрастанию; пример сайта: 30×30 «Фотобумага» на 20
// разворотах — 662 × 330 мм (основа 648 + корешок 14); за таблицей — «вне таблицы».
func TestFabrikaCoverPresets(t *testing.T) {
	var ids []string
	for _, c := range CoverPresets() {
		if c.Group == CoverGroupFabrika {
			ids = append(ids, c.ID)
			st := c.Spec.Steps
			if c.Spec.Preset != c.ID || len(st) == 0 || c.Spec.Safe != 0 || c.Spec.SpineSafe != 0 {
				t.Errorf("%s: %+v", c.ID, c.Spec)
			}
			for i := 1; i < len(st); i++ {
				if st[i].Upto <= st[i-1].Upto || st[i].Mm < st[i-1].Mm {
					t.Errorf("%s: таблица не растёт на строке %d: %+v", c.ID, i, st)
				}
			}
		}
	}
	want := []string{"fabrika-photo", "fabrika-photo-1", "fabrika-photo-2", "fabrika-layflat-paper", "fabrika-layflat-card", "fabrika-layflat-card-1"}
	if !slices.Equal(ids, want) {
		t.Fatalf("изделия %q", ids)
	}
	all := CoverPresets()
	if all[len(all)-1].ID != CoverCustom {
		t.Error("«Свои размеры» не последние")
	}
	for _, c := range []struct {
		id    string
		n     int
		mm    float64
		over  bool
		steps int
	}{
		{"fabrika-photo", 20, 14, false, 10}, {"fabrika-photo", 40, 26, false, 10}, {"fabrika-photo", 41, 26, true, 10},
		{"fabrika-photo-1", 30, 34, false, 14}, {"fabrika-photo-2", 9, 22, false, 8}, {"fabrika-photo-2", 10, 22, true, 8},
		{"fabrika-layflat-paper", 13, 10, false, 5}, {"fabrika-layflat-card", 20, 16, false, 5}, {"fabrika-layflat-card-1", 12, 18, false, 6},
		{"fabrika-layflat-card", 0, 8, false, 5},
	} {
		pr, _ := CoverPresetOf(c.id)
		if mm, over := pr.Spec.Spine(c.n); mm != c.mm || over != c.over || len(pr.Spec.Steps) != c.steps {
			t.Errorf("%s, %d разворотов: корешок %v (вне таблицы %v), строк %d", c.id, c.n, mm, over, len(pr.Spec.Steps))
		}
	}
	f30 := FabrikaFormats()[4]
	p := fabrikaProject(f30, 20, "fabrika-photo")
	if cg := p.Geo(p.Cover.Sheet); cg.W != 662 || cg.H != 330 || p.CoverGeo().Spine != 14 {
		t.Errorf("30×30, 20 разворотов: %v × %v, корешок %v", cg.W, cg.H, p.CoverGeo().Spine)
	}
}

// TestCoverPresetForFabrika (081, 070 Е): сторонка и заворот — шаблон формата страницы; страница
// вне таблицы — ближайший формат, лист 031 — тоже; безопасные линии 0 (берётся Settings.Safe);
// Фотосфера на странице Фабрики по-прежнему считается по своей таблице.
func TestCoverPresetForFabrika(t *testing.T) {
	for _, c := range []struct {
		st         Settings
		w, h, wrap float64
	}{
		{Settings{PageW: 300, PageH: 305}, 312, 306, 12},
		{Settings{PageW: 200, PageH: 203}, 206, 206, 17},
		{Settings{PageW: 208, PageH: 296}, 211, 296, 17},
		{Settings{PageW: 304, PageH: 203}, 310, 206, 17},
		{Settings{PageW: 300, PageH: 300}, 312, 306, 12}, // ближайший 30×30
		{Settings{PageW: 300, PageH: 305, Sheet: true}, 312, 306, 12},
	} {
		for _, pr := range fabrikaCoverPresets() {
			got, ok := CoverPresetFor(pr.ID, c.st)
			if sp := got.Spec; !ok || sp.Preset != pr.ID || sp.W != c.w || sp.H != c.h || sp.Wrap != c.wrap || sp.Safe != 0 || sp.SpineSafe != 0 {
				t.Errorf("%s на %v×%v: %+v", pr.ID, c.st.PageW, c.st.PageH, sp)
			}
		}
	}
	if got, _ := CoverPresetFor("fsfera-thin", Settings{PageW: 300, PageH: 305}); got.Spec.W != 286 || got.Spec.Wrap != 20 {
		t.Errorf("Фотосфера на странице Фабрики: %+v", got.Spec)
	}
}

// TestCoverFixTextFabrika (081, 047): сторонка не та — размеры листа; заворот при том же листе —
// «заворот»; другая таблица корешка — «корешок или расстав»; по шаблону — "".
func TestCoverFixTextFabrika(t *testing.T) {
	f30 := FabrikaFormats()[4]
	good, _ := CoverPresetFor("fabrika-photo", Settings{PageW: f30.PageW, PageH: f30.PageH})
	wide := good.Spec
	wide.W-- // 660 × 330 вместо 662 × 330
	wrap := good.Spec
	wrap.Wrap, wrap.W, wrap.H = 14, wrap.W-2, wrap.H-4
	tab := good.Spec
	tab.Steps = slices.Clone(tab.Steps)
	tab.Steps[5].Mm = 19 // строка не для 20 разворотов: лист тот же
	for _, c := range []struct {
		spec CoverSpec
		want string
	}{
		{good.Spec, ""},
		{wide, "Не по шаблону Фабрики Фотокниги 30×30: 660 × 330 мм, нужно 662 × 330"},
		{wrap, "Не по шаблону Фабрики Фотокниги 30×30: заворот 14 мм, нужно 12"},
		{tab, "Не по шаблону Фабрики Фотокниги 30×30: корешок или расстав не по таблице"},
	} {
		p := fsferaProject(f30.PageW, f30.PageH, 20, c.spec)
		if got := CoverFixText(p); got != c.want {
			t.Errorf("%+v: %q, ждали %q", c.spec, got, c.want)
		}
	}
	// Свои числа и чужая типография на странице Фабрики не «не по шаблону».
	for _, id := range []string{CoverCustom, "netprint-premium"} {
		pr, _ := CoverPresetOf(id)
		if got := CoverFixText(fsferaProject(f30.PageW, f30.PageH, 20, pr.Spec)); got != "" {
			t.Errorf("%s: %q", id, got)
		}
	}
	// Страница вне таблицы Фабрики — формат ближайший.
	pr, _ := CoverPresetFor("fabrika-photo", Settings{PageW: 300, PageH: 305})
	spec := pr.Spec
	spec.W = 310
	if got := CoverFixText(fsferaProject(300, 300, 20, spec)); got != "Не по шаблону Фабрики Фотокниги 30×30: 658 × 330 мм, нужно 662 × 330" {
		t.Errorf("вне таблицы: %q", got)
	}
}

// TestFabrikaLimits (081 С1–С2, С7): 17 строк; Max — последняя строка таблицы корешка формата; «от 1» сайта —
// Min 0; прослойка 1 и 2 мм — с 2 разворотов, у 30×20 с 1; 30×30 фотобумаги — от 10; изделия нет у формата — строки нет; без обложки изделие
// по умолчанию «Фотобумага» без названия.
func TestFabrikaLimits(t *testing.T) {
	var rows []PrintLimit
	for _, l := range PrintLimits() {
		if pr, _ := CoverPresetOf(l.Product); pr.Group == CoverGroupFabrika {
			rows = append(rows, l)
			pf, _ := CoverPresetFor(l.Product, Settings{PageW: 304, PageH: 203}) // 30×20: самая длинная таблица
			if l.Format != "30×20" && l.Format != "" {
				f := FabrikaFormats()[slices.IndexFunc(FabrikaFormats(), func(f PrintFormat) bool { return f.Name == l.Format })]
				pf, _ = CoverPresetFor(l.Product, Settings{PageW: f.PageW, PageH: f.PageH})
			}
			if st := pf.Spec.Steps; l.Max != st[len(st)-1].Upto || l.Pages || l.Min < 0 || l.Min > l.Max {
				t.Errorf("строка %+v: таблица до %d", l, st[len(st)-1].Upto)
			}
		}
	}
	if len(rows) != 17 {
		t.Fatalf("строк %d, ждали 17: %+v", len(rows), rows)
	}
	for _, c := range []struct {
		product, format string
		min, max        int
		ok              bool
	}{
		{"fabrika-photo", "30×30", 10, 40, true}, {"fabrika-photo", "20×20", 0, 40, true}, {"fabrika-photo", "", 0, 40, true},
		{"fabrika-photo-1", "30×30", 10, 30, true}, {"fabrika-photo-1", "25×25", 2, 30, true},
		{"fabrika-photo-1", "30×20", 0, 30, true}, {"fabrika-photo-1", "", 0, 30, true},
		{"fabrika-photo-2", "20×20", 2, 9, true}, {"fabrika-photo-2", "30×20", 0, 10, true},
		{"fabrika-photo-2", "30×30", 0, 0, false}, // прослойки 2 мм у 30×30 нет
		{"fabrika-layflat-paper", "21×30", 10, 30, true}, {"fabrika-layflat-paper", "25×25", 0, 0, false},
		{"fabrika-layflat-card", "20×20", 2, 20, true}, {"fabrika-layflat-card", "30×30", 0, 0, false},
		{"fabrika-layflat-card-1", "21×30", 2, 12, true}, {"fabrika-layflat-card-1", "", 0, 0, false},
		{"fabrika-none", "20×20", 0, 0, false},
	} {
		l, ok := LimitFor(c.product, c.format)
		if ok != c.ok || (ok && (l.Min != c.min || l.Max != c.max)) {
			t.Errorf("LimitFor(%s, %q) = %+v (%v), ждали %d–%d (%v)", c.product, c.format, l, ok, c.min, c.max, c.ok)
		}
	}
	// Тексты: у обложки изделие названо, у книги без обложки Фабрики — нет.
	f30, f20 := FabrikaFormats()[4], FabrikaFormats()[3]
	for _, c := range []struct {
		name string
		p    *Project
		want PageLimitView
	}{
		{"фотобумага 41", fabrikaProject(f30, 41, "fabrika-photo"),
			PageLimitView{Who: "Фабрика Фотокниги «Фотобумага»", Range: "от 10 до 40 разворотов", Count: 41, Min: 10, Max: 40,
				Warn: "Разворотов 41 — у Фабрики Фотокниги («Фотобумага») не больше 40"}},
		{"фотобумага 30×30, 9", fabrikaProject(f30, 9, "fabrika-photo"),
			PageLimitView{Who: "Фабрика Фотокниги «Фотобумага»", Range: "от 10 до 40 разворотов", Count: 9, Min: 10, Max: 40,
				Warn: "Разворотов 9 — у Фабрики Фотокниги («Фотобумага») не меньше 10"}},
		{"лайфлат картон 21", fabrikaProject(FabrikaFormats()[0], 21, "fabrika-layflat-card"),
			PageLimitView{Who: "Фабрика Фотокниги «Лайфлат, картон»", Range: "от 2 до 20 разворотов", Count: 21, Min: 2, Max: 20,
				Warn: "Разворотов 21 — у Фабрики Фотокниги («Лайфлат, картон») не больше 20"}},
		{"30×20 без обложки", fsferaProject(f20.PageW, f20.PageH, 41, CoverSpec{Preset: CoverCustom}),
			PageLimitView{Who: "Фабрика Фотокниги", Range: "до 40 разворотов", Count: 41, Max: 40,
				Warn: "Разворотов 41 — у Фабрики Фотокниги не больше 40"}},
	} {
		got, ok := LimitOf(c.p)
		if !ok || got != c.want {
			t.Errorf("%s: %+v (%v),\nждали %+v", c.name, got, ok, c.want)
		}
	}
	// Лайфлат 25×25 — изделия нет: предела нет, строки не бывает.
	if got, ok := LimitOf(fabrikaProject(FabrikaFormats()[2], 12, "fabrika-layflat-paper")); ok {
		t.Errorf("лайфлат 25×25: %+v", got)
	}
}
