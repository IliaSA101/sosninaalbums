package model

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPxToPageExact (123 §1.1): px → мм на сетке 0,01 → Px даёт те же px при 72…1200 dpi, на
// разворотах 100…2000 мм (и странных числах px), а мм лежат на сетке 0,01.
func TestPxToPageExact(t *testing.T) {
	pageSweep(t)
	// Круглые мм возвращаются круглыми: страница из таблиц всех типографий и общего ряда.
	for _, pr := range PagePresets() {
		pw, ph, err := PageFromPx(Px(2*pr.W, PrintDPI), Px(pr.H, PrintDPI), PrintDPI)
		if err != nil || !SameMm(pw, pr.W) || !SameMm(ph, pr.H) || (pr.W == math.Trunc(pr.W) && pw != pr.W) {
			t.Errorf("%s %s: %v × %v мм из своих px (%v), ждали %v × %v", pr.Group, pr.Label, pw, ph, err, pr.W, pr.H)
		}
	}
	// Примеры спеки: Фабрика 30×20 (шаблон), чужая цель загрузчика на 1 px меньше, 30×20 общего ряда.
	for _, c := range []struct {
		w, h   int
		pw, ph float64
	}{{7181, 2398, 304, 203}, {7180, 2397, 303.95, 202.95}, {7087, 2362, 300, 200}} {
		pw, ph, err := PageFromPx(c.w, c.h, PrintDPI)
		if err != nil || pw != c.pw || ph != c.ph {
			t.Errorf("%d×%d px: %v × %v (%v), ждали %v × %v", c.w, c.h, pw, ph, err, c.pw, c.ph)
		}
	}
}

// pageSweep — развороты 120…2000 мм при 72…1200 dpi: px → мм → px те же, мм на сетке 0,01.
func pageSweep(t *testing.T) {
	t.Helper()
	for _, dpi := range []int{72, 96, 150, 200, 300, 400, 600, 1200} {
		for w := 120.0; w <= 2000; w += 7.31 {
			W, H := Px(w, dpi), Px(w/2+3.37, dpi)
			pw, ph, err := PageFromPx(W, H, dpi)
			if err != nil {
				if w/2 < minPage || w/2+3.37 > maxPage {
					continue // вне пределов страницы — отказ, а не подгонка
				}
				t.Fatalf("%d×%d px при %d dpi: %v", W, H, dpi, err)
			}
			if Px(2*pw, dpi) != W || Px(ph, dpi) != H {
				t.Fatalf("%d×%d px при %d dpi → %v×%v мм = %d×%d px", W, H, dpi, pw, ph, Px(2*pw, dpi), Px(ph, dpi))
			}
			if g := math.Round(pw*100) / 100; g != pw {
				t.Fatalf("%v мм не на сетке 0,01", pw)
			}
		}
	}
}

// TestPageFromPxErrors: ноль, минус, огромные px, предел страницы, плохой dpi — ошибка, а не паника
// или молчаливая подгонка; текст называет предел (123 §1.1).
func TestPageFromPxErrors(t *testing.T) {
	for _, c := range []struct {
		w, h, dpi int
		want      string
	}{
		{90000, 2398, 300, "Разворот 90000 px при 300 dpi — страница больше 1000 мм"},
		{7181, 90000, 300, "Высота 90000 px при 300 dpi — страница больше 1000 мм"},
		{100, 2398, 300, "меньше 50 мм"},
		{7181, 100, 300, "меньше 50 мм"},
		{0, 2398, 300, "больше нуля"},
		{7181, -5, 300, "больше нуля"},
		{math.MaxInt, 2398, 300, "больше 1000 мм"},
		{7181, math.MaxInt, 300, "больше 1000 мм"},
		{math.MinInt, math.MinInt, 300, "больше нуля"},
		{7181, 2398, 0, "dpi"},
		{7181, 2398, 71, "dpi"},
		{7181, 2398, 1201, "dpi"},
		{7181, 2398, -300, "dpi"},
	} {
		pw, ph, err := PageFromPx(c.w, c.h, c.dpi)
		if err == nil || !strings.Contains(err.Error(), c.want) || pw != 0 || ph != 0 {
			t.Errorf("%d×%d @%d: %v × %v, %v; ждали ошибку «%s»", c.w, c.h, c.dpi, pw, ph, err, c.want)
		}
	}
	// Края допустимого: ровно 1000 мм страницы и ровно 50 мм.
	if pw, _, err := PageFromPx(Px(2000, 300), Px(500, 300), 300); err != nil || pw != 1000 {
		t.Errorf("страница 1000 мм: %v %v", pw, err)
	}
	if pw, _, err := PageFromPx(Px(100, 300), Px(500, 300), 300); err != nil || pw != 50 {
		t.Errorf("страница 50 мм: %v %v", pw, err)
	}
}

// coverPx — размер листа обложки в px так же, как выгрузка 00.jpg: Px от X[7] и Y[3].
func coverPx(spec CoverSpec, pw, ph float64, n, dpi int) (int, int) {
	g := fsferaProject(pw, ph, n, spec).CoverGeo()
	return Px(g.X[7], dpi), Px(g.Y[3], dpi)
}

// TestSideFromPx (123 §1.1): сторонка из px даёт лист ровно этих px при заворотах 12/17/20 и
// корешках 6…26, dpi 72…1200; примеры спеки — Фабрика 30×20, корешок 26.
func TestSideFromPx(t *testing.T) {
	sideSweep(t)
	fab, _ := CoverPresetFor("fabrika-photo", Settings{PageW: 304, PageH: 203})
	for _, c := range []struct {
		w, h   int
		sw, sh float64
	}{{8031, 2835, 310, 206}, {8031, 2836, 310, 206.1}} {
		sw, sh, err := SideFromPx(c.w, c.h, PrintDPI, fab.Spec, 26)
		if err != nil || sw != c.sw || sh != c.sh {
			t.Errorf("%d×%d px: %v × %v (%v), ждали %v × %v", c.w, c.h, sw, sh, err, c.sw, c.sh)
		}
	}
}

// sideSweep — заворот 0/12/17/20, расстав 0/9, корешок 6/13,5/26, dpi 72…1200: лист обложки
// из подобранной сторонки выходит ровно в введённые px.
func sideSweep(t *testing.T) {
	t.Helper()
	for _, dpi := range []int{72, 150, 300, 600, 1200} {
		for _, wrap := range []float64{0, 12, 17, 20} {
			for _, hinge := range []float64{0, 9} {
				for _, spine := range []float64{6, 13.5, 26} {
					for W := 1000; W <= 12000; W += 811 {
						spec := CoverSpec{Wrap: wrap, Hinge: hinge, Steps: []SpineStep{{Upto: 40, Mm: spine}}}
						H := W/3 + 17
						sw, sh, err := SideFromPx(W, H, dpi, spec, spine)
						if err != nil {
							continue // сторонка вне 50…1000 при этом dpi
						}
						spec.W, spec.H = sw, sh
						if gw, gh := coverPx(spec, 300, 200, 40, dpi); gw != W || gh != H {
							t.Fatalf("dpi %d заворот %v расстав %v корешок %v: %d×%d px → сторонка %v×%v → %d×%d px",
								dpi, wrap, hinge, spine, W, H, sw, sh, gw, gh)
						}
					}
				}
			}
		}
	}
}

// TestSideFromPxErrors: ноль, огромные px, сторонка вне 50…1000 — ошибка с пределом.
func TestSideFromPxErrors(t *testing.T) {
	spec := CoverSpec{Wrap: 17}
	for _, c := range []struct {
		w, h, dpi int
		want      string
	}{
		{0, 2835, 300, "больше нуля"},
		{8031, -1, 300, "больше нуля"},
		{200000, 2835, 300, "Ширина обложки 200000 px при 300 dpi — сторонка больше 1000 мм"},
		{8031, 200000, 300, "Высота обложки 200000 px при 300 dpi — сторонка больше 1000 мм"},
		{400, 2835, 300, "сторонка меньше 50 мм"},
		{8031, 300, 300, "сторонка меньше 50 мм"},
		{math.MaxInt, math.MaxInt, 300, "больше 1000 мм"},
		{8031, 2835, 5, "dpi"},
	} {
		sw, sh, err := SideFromPx(c.w, c.h, c.dpi, spec, 26)
		if err == nil || !strings.Contains(err.Error(), c.want) || sw != 0 || sh != 0 {
			t.Errorf("%d×%d @%d: %v × %v, %v; ждали «%s»", c.w, c.h, c.dpi, sw, sh, err, c.want)
		}
	}
}

// pageTolCase — вектор testdata/parity/pagetol.json (presets.ts PAGE_TOL, presetActive).
type pageTolCase struct {
	A    float64 `json:"a"`
	B    float64 `json:"b"`
	Same bool    `json:"same"`
}

type pageTolFile struct {
	Tol   float64       `json:"tol"`
	Cases []pageTolCase `json:"cases"`
}

// pageTolCases — пары мм: края допуска, шум float, соседние форматы таблиц (1 мм) и шаги 0,01.
func pageTolCases() []pageTolCase {
	pairs := [][2]float64{
		{304, 304}, {304, 303.95}, {304, 303.75}, {304, 303.74}, {304, 304.25}, {304, 304.26},
		{203, 202.95}, {203, 203.2}, {203, 203.25}, {203, 203.3}, {202.95, 203.2}, {202.95, 203.21},
		{201, 200}, {251, 250}, {286, 285}, {201, 200.76}, {201, 200.75}, {201, 200.74},
		{0, 0.25}, {0, 0.26}, {0, 0}, {300, 300.01}, {100, 99.75}, {1000, 999.76},
	}
	out := make([]pageTolCase, 0, len(pairs)*2)
	for _, pr := range pairs {
		out = append(out, pageTolCase{pr[0], pr[1], SameMm(pr[0], pr[1])}, pageTolCase{pr[1], pr[0], SameMm(pr[1], pr[0])})
	}
	return out
}

// TestSamePageTol (123 §1.2): допуск PageTol симметричен, включает край, не пускает 1 мм между
// форматами; ни у одной шаблонной группы нет двух форматов ближе 2·PageTol (иначе формат
// страницы был бы неоднозначным); вектор паритета pagetol.json совпадает с Go (-update пишет).
func TestSamePageTol(t *testing.T) {
	if PageTol != 0.25 {
		t.Fatalf("PageTol = %v, спека 123 — 0,25 мм", PageTol)
	}
	want := map[[2]float64]bool{
		{304, 303.95}: true, {304, 303.75}: true, {304, 303.74}: false, {304, 304.25}: true, {304, 304.26}: false,
		{201, 200}: false, {251, 250}: false, {286, 285}: false, {0, 0.25}: true, {0, 0.26}: false,
		{202.95, 203.2}: true, {202.95, 203.21}: false,
	}
	for k, w := range want {
		if SameMm(k[0], k[1]) != w || SameMm(k[1], k[0]) != w {
			t.Errorf("SameMm(%v, %v) = %v, ждали %v", k[0], k[1], !w, w)
		}
	}
	formatsApart(t)

	path := filepath.Join("..", "..", "..", "testdata", "parity", "pagetol.json")
	if *update {
		data, _ := json.MarshalIndent(pageTolFile{Tol: PageTol, Cases: pageTolCases()}, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f pageTolFile
	if err := json.Unmarshal(data, &f); err != nil || f.Tol != PageTol || len(f.Cases) != len(pageTolCases()) {
		t.Fatalf("вектор паритета: %v, допуск %v, %d случаев", err, f.Tol, len(f.Cases))
	}
	for i, c := range f.Cases {
		if SameMm(c.A, c.B) != c.Same {
			t.Errorf("вектор %d: SameMm(%v, %v) = %v, в файле %v", i, c.A, c.B, !c.Same, c.Same)
		}
	}
}

// TestFormatOfTol (123 §1.2, сц. 6): 303,95×202,95 — Фабрика 30×20, кнопка подсвечена, предел и
// ближайший формат те же; 201×203, 251×254, 200×200 — не Фабрика; Фотосфера — тем же допуском.
func TestFormatOfTol(t *testing.T) {
	s := Settings{PageW: 303.95, PageH: 202.95, DPI: 300, Layflat: true, Safe: 10, Bleed: 3}
	f, ok := FormatOf(CoverGroupFabrika, s)
	if !ok || f.Name != "30×20" {
		t.Fatalf("303,95 × 202,95: %+v %v", f, ok)
	}
	if n := FormatNear(CoverGroupFabrika, s); n.Name != "30×20" {
		t.Errorf("FormatNear: %+v", n)
	}
	pr, ok := PagePresetIn(s, CoverGroupFabrika)
	if !ok || pr.Label != "30×20" || !presetFits(pr, s) {
		t.Errorf("PagePresetIn: %+v %v, кнопка не совпала", pr, ok)
	}
	if pr, ok := PagePresetOf(s); !ok || pr.Group != CoverGroupFabrika || pr.Label != "30×20" {
		t.Errorf("PagePresetOf: %+v %v", pr, ok)
	}
	// Зона и обрез остаются точными: допуск только у размера.
	bad := s
	bad.Bleed = 2
	if presetFits(pr, bad) {
		t.Error("обрез 2 вместо 3 принят за кнопку Фабрики")
	}
	for _, c := range []struct {
		group string
		w, h  float64
	}{
		{CoverGroupFabrika, 201, 203}, {CoverGroupFabrika, 251, 254}, {CoverGroupFabrika, 200, 200},
		{CoverGroupFabrika, 303.7, 202.95}, {CoverGroupFabrika, 303.95, 202.7},
		{CoverGroupFsfera, 200, 203}, {CoverGroupFsfera, 250, 254}, {CoverGroupFsfera, 285, 203},
	} {
		if f, ok := FormatOf(c.group, Settings{PageW: c.w, PageH: c.h}); ok {
			t.Errorf("%s: %v × %v принят за %s", c.group, c.w, c.h, f.Name)
		}
		if _, ok := PagePresetIn(Settings{PageW: c.w, PageH: c.h}, c.group); ok {
			t.Errorf("%s: %v × %v нашёл кнопку", c.group, c.w, c.h)
		}
	}
	if f, ok := FormatOf(CoverGroupFsfera, Settings{PageW: 285.8, PageH: 203.2}); !ok || f.Name != "30×20" {
		t.Errorf("Фотосфера 285,8 × 203,2: %+v %v", f, ok)
	}
	// Лист 031 того же размера — не фотокнига и с допуском.
	if _, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 303.95, PageH: 202.95, Sheet: true}); ok {
		t.Error("лист 031 принят за фотокнигу")
	}
	// Страница за допуском: ближайший формат по сумме разниц, как раньше.
	if n := FormatNear(CoverGroupFabrika, Settings{PageW: 300, PageH: 200}); n.Name != "20×20" && n.Name != "30×30" && n.Name != "30×20" {
		t.Errorf("ближайший к 300×200: %+v", n)
	}
}

// TestCoverOffTemplateTol (123 §1.2, сц. 6): сторонка на 1 px от шаблона (309,98×206,12) —
// «по шаблону», на 0,3 мм — нет; заворот и таблица корешка по-прежнему точные.
func TestCoverOffTemplateTol(t *testing.T) {
	page := Settings{PageW: 304, PageH: 203}
	fab, _ := CoverPresetFor("fabrika-photo", page)
	mk := func(edit func(*CoverSpec), s Settings) *Project {
		spec := fab.Spec
		edit(&spec)
		p := fsferaProject(s.PageW, s.PageH, 40, spec)
		return p
	}
	for _, c := range []struct {
		name string
		edit func(*CoverSpec)
		page Settings
		off  bool
	}{
		{"шаблон", func(*CoverSpec) {}, page, false},
		{"сторонка на 1 px", func(s *CoverSpec) { s.W, s.H = 309.98, 206.12 }, page, false},
		{"край допуска", func(s *CoverSpec) { s.W, s.H = 309.75, 206.25 }, page, false},
		{"за допуском по ширине", func(s *CoverSpec) { s.W, s.H = 309.7, 206 }, page, true},
		{"за допуском по высоте", func(s *CoverSpec) { s.W, s.H = 310, 206.3 }, page, true},
		{"заворот 18", func(s *CoverSpec) { s.W, s.H, s.Wrap = 310, 206, 18 }, page, true},
		{"страница из px", func(s *CoverSpec) { s.W, s.H = 310, 206 }, Settings{PageW: 303.95, PageH: 202.95}, false},
	} {
		if _, _, _, off := mk(c.edit, c.page).CoverOffTemplate(); off != c.off {
			t.Errorf("%s: off=%v, ждали %v", c.name, off, c.off)
		}
	}
	if CoverFixText(mk(func(s *CoverSpec) { s.W, s.H = 309.98, 206.12 }, page)) != "" {
		t.Error("сторонка на 1 px: «Не по шаблону»")
	}
	if CoverFixText(mk(func(s *CoverSpec) { s.W, s.H = 309.7, 206 }, page)) == "" {
		t.Error("сторонка на 0,3 мм: «Не по шаблону» не появилось")
	}
}

// FuzzPxFit: любые px, dpi и обложка — без паники; принятый ответ в пределах и даёт те же px.
func FuzzPxFit(f *testing.F) {
	f.Add(7181, 2398, 300, 17.0, 0.0, 26.0)
	f.Add(0, -1, 0, -5.0, 1e300, math.NaN())
	f.Add(math.MaxInt, math.MinInt, 1200, 50.0, 20.0, 100.0)
	f.Fuzz(func(t *testing.T, w, h, dpi int, wrap, hinge, spine float64) {
		if pw, ph, err := PageFromPx(w, h, dpi); err == nil {
			if pw < minPage || pw > maxPage || ph < minPage || ph > maxPage || Px(2*pw, dpi) != w || Px(ph, dpi) != h {
				t.Fatalf("%d×%d @%d → %v×%v", w, h, dpi, pw, ph)
			}
		}
		spec := ClampCoverSpec(CoverSpec{Wrap: wrap, Hinge: hinge})
		spine = Finite(spine, 0)
		if spine < 0 || spine > 100 {
			spine = 10
		}
		if sw, sh, err := SideFromPx(w, h, dpi, spec, spine); err == nil {
			x, y := coverAxes(spec.Wrap, sw, spec.Hinge, spine, sh)
			if sw < coverMinSide || sw > coverMaxSide || sh < coverMinSide || sh > coverMaxSide || Px(x[7], dpi) != w || Px(y[3], dpi) != h {
				t.Fatalf("%d×%d @%d → %v×%v", w, h, dpi, sw, sh)
			}
		}
	})
}

// formatsApart — ни у одной шаблонной группы и ни в одном ряду кнопок нет двух форматов в допуске
// друг друга: иначе формат страницы был бы неоднозначным.
func formatsApart(t *testing.T) {
	t.Helper()
	for _, g := range templateGroups() {
		fs := g.Formats()
		for i, a := range fs {
			for _, b := range fs[i+1:] {
				if SameMm(a.PageW, b.PageW) && SameMm(a.PageH, b.PageH) {
					t.Errorf("%s: %s и %s в допуске друг друга", g.Group, a.Name, b.Name)
				}
			}
		}
	}
	for _, pr := range PagePresets() {
		for _, o := range PagePresets() {
			if pr.Group == o.Group && pr.Label != o.Label && SameMm(pr.W, o.W) && SameMm(pr.H, o.H) {
				t.Errorf("ряд %q: кнопки %s и %s в допуске друг друга", pr.Group, pr.Label, o.Label)
			}
		}
	}
}

// TestCoverFixPxTail — хвост с px: пусто, когда px сошлись (сторонка на ≤ 0,01 мм); иначе «файл … нужен …».
func TestCoverFixPxTail(t *testing.T) {
	if got := coverFixPx([2]float64{310, 206}, [2]float64{310.005, 206.004}); got != "" {
		t.Errorf("px сошлись, а хвост %q", got)
	}
	want := CoverFixPxMark + "7795 × 3898 px, нужен 7819 × 3898)"
	if got := coverFixPx([2]float64{660, 330}, [2]float64{662, 330}); got != want {
		t.Errorf("%q, ждали %q", got, want)
	}
}
