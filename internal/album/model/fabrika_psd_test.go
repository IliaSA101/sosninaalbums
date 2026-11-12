package model

import (
	"slices"
	"testing"
)

// psdRow — столбец «Диапазон корешков» одного CSV шаблона Фабрики: разворотов от from до to,
// корешок — пары (до n разворотов включительно, мм), как строки CSV, склеенные в ступени.
type psdRow struct {
	product  string
	from, to int
	steps    []float64
}

// Числа ниже — из CSV в архивах PSD-шаблонов Фабрики Фотокниги (скачаны 24.10, раздел «PSD-шаблоны»
// в docs/research/fabrika-requirements.md). Файлов архива тест не читает. Источник — имя CSV в
// архиве, столбец «Диапазон корешков для …»: «Photobook 20x20.csv», «Photobook 21x30.csv»,
// «Photobook 25x25.csv», «Photobook 30x20.csv», «Photobook 30x30.csv», «Layflat 20x20.csv», lang-ok
// «Layflat 21x30.csv».
var (
	psdPhoto  = []float64{8, 8, 12, 10, 16, 12, 20, 14, 24, 16, 27, 18, 31, 20, 35, 22, 39, 24, 40, 26}
	psdPhoto1 = []float64{4, 8, 6, 10, 8, 12, 10, 14, 12, 16, 14, 18, 16, 20, 18, 22, 20, 24, 22, 26, 24, 28, 26, 30, 28, 32, 30, 34}
	psdPhoto2 = []float64{2, 8, 3, 10, 4, 12, 5, 14, 6, 16, 7, 18, 8, 20, 9, 22}
	// Layflat 20x20.csv и Layflat 21x30.csv совпадают.
	psdLayflat = []psdRow{
		{"fabrika-layflat-paper", 10, 30, []float64{13, 10, 17, 12, 21, 14, 26, 16, 30, 18}},
		{"fabrika-layflat-card", 2, 20, []float64{7, 8, 10, 10, 13, 12, 16, 14, 20, 16}},
		{"fabrika-layflat-card-1", 2, 12, []float64{4, 8, 5, 10, 7, 12, 9, 14, 11, 16, 12, 18}},
	}
	// Photobook 30x30.csv: от 10 разворотов; столбца «прослойка 2 мм» нет.
	psdPhoto3030 = []psdRow{
		{"fabrika-photo", 10, 40, []float64{12, 10, 16, 12, 20, 14, 24, 16, 27, 18, 31, 20, 35, 22, 39, 24, 40, 26}},
		{"fabrika-photo-1", 10, 30, []float64{10, 14, 12, 16, 14, 18, 16, 20, 18, 22, 20, 24, 22, 26, 24, 28, 26, 30, 28, 32, 30, 34}},
	}
)

// TestFabrikaSpineMatchesPSD: пределы разворотов и корешок каждого изделия и формата равны CSV
// шаблонов Фабрики — по каждому числу разворотов от минимума до максимума, плюс «за максимумом —
// вне таблицы». Изделия, которых у формата в архиве нет, предела не имеют. Таблица корешка других
// форматов не меняется ради 30×20 (обложки не становятся «не по шаблону»).
func TestFabrikaSpineMatchesPSD(t *testing.T) {
	type format struct {
		name string
		rows []psdRow
	}
	photo := func(min1, min2 int) []psdRow {
		return []psdRow{
			{"fabrika-photo", 1, 40, psdPhoto}, {"fabrika-photo-1", min1, 30, psdPhoto1}, {"fabrika-photo-2", min2, 9, psdPhoto2},
		}
	}
	cases := []format{
		{"20×20", append(photo(2, 2), psdLayflat...)},
		{"21×30", append(photo(2, 2), psdLayflat...)},
		{"25×25", photo(2, 2)},
		{"30×20", append(photo(1, 1)[:2:2], psdRow{"fabrika-photo-2", 1, 10, append(slices.Clone(psdPhoto2), 10, 24)})},
		{"30×30", psdPhoto3030},
	}
	all := map[string]bool{}
	for _, c := range fabrikaCoverPresets() {
		all[c.ID] = true
	}
	for _, c := range cases {
		page := FabrikaFormats()[slices.IndexFunc(FabrikaFormats(), func(f PrintFormat) bool { return f.Name == c.name })]
		st := Settings{PageW: page.PageW, PageH: page.PageH}
		have := map[string]bool{}
		for _, r := range c.rows {
			have[r.product] = true
			checkPSDRow(t, c.name, st, r)
		}
		// Нет столбца в CSV — нет и изделия у формата.
		for id := range all {
			if _, ok := LimitFor(id, c.name); ok != have[id] && c.name != "30×30" {
				t.Errorf("%s %s: предел есть = %v, в CSV столбец есть = %v", c.name, id, ok, have[id])
			}
		}
	}
	// 30×30: прослойки 2 мм и лайфлата нет.
	for _, id := range []string{"fabrika-photo-2", "fabrika-layflat-paper", "fabrika-layflat-card", "fabrika-layflat-card-1"} {
		if _, ok := LimitFor(id, "30×30"); ok {
			t.Errorf("30×30 %s: предел есть, в CSV столбца нет", id)
		}
	}
}

// TestFabrikaSpine30x20Only: ступень «10 разворотов — 24 мм» только у 30×20 с прослойкой 2 мм.
// Прочие форматы и изделия берут общую таблицу — готовые обложки не «не по шаблону»; у старой
// обложки 30×20 / 2 мм (8 ступеней) «Исправить» предложат из-за новой строки.
func TestFabrikaSpine30x20Only(t *testing.T) {
	for _, f := range FabrikaFormats() {
		for _, base := range fabrikaCoverPresets() {
			pr, _ := CoverPresetFor(base.ID, Settings{PageW: f.PageW, PageH: f.PageH})
			longer := base.ID == "fabrika-photo-2" && f.Name == "30×20"
			if got := !slices.Equal(pr.Spec.Steps, base.Spec.Steps); got != longer {
				t.Errorf("%s %s: таблица отличается от общей = %v, ждали %v", f.Name, base.ID, got, longer)
			}
			// Обложка по общей таблице: «не по шаблону» только у 30×20 / 2 мм.
			p := fsferaProject(f.PageW, f.PageH, 5, base.Spec)
			p.Cover.Spec.W, p.Cover.Spec.H, p.Cover.Spec.Wrap = pr.Spec.W, pr.Spec.H, pr.Spec.Wrap
			want := ""
			if longer {
				want = "Не по шаблону Фабрики Фотокниги 30×20: корешок или расстав не по таблице"
			}
			if got := CoverFixText(p); got != want {
				t.Errorf("%s %s: старая обложка %q, ждали %q", f.Name, base.ID, got, want)
			}
		}
	}
}

// checkPSDRow сверяет изделие r формата name на странице st с CSV: предел, корешок по каждому
// числу разворотов и «за максимумом — вне таблицы».
func checkPSDRow(t *testing.T, name string, st Settings, r psdRow) {
	t.Helper()
	pr, _ := CoverPresetFor(r.product, st)
	l, ok := LimitFor(r.product, name)
	// «От 1» сайта — Min 0 (нет нижнего предела).
	wantMin := r.from
	if wantMin == 1 {
		wantMin = 0
	}
	if !ok || l.Min != wantMin || l.Max != r.to {
		t.Errorf("%s %s: предел %+v (%v), CSV %d–%d", name, r.product, l, ok, r.from, r.to)
	}
	want := steps(r.steps...)
	for n := r.from; n <= r.to; n++ {
		i := slices.IndexFunc(want, func(s SpineStep) bool { return n <= s.Upto })
		if mm, over := pr.Spec.Spine(n); over || mm != want[i].Mm {
			t.Errorf("%s %s, %d разворотов: корешок %v (вне таблицы %v), CSV %v", name, r.product, n, mm, over, want[i].Mm)
		}
	}
	if _, over := pr.Spec.Spine(r.to + 1); !over {
		t.Errorf("%s %s: %d разворотов в таблице, CSV кончается на %d", name, r.product, r.to+1, r.to)
	}
	// Кроме 30×30 таблица — ровно ступени CSV (у 30×30 она общая с форматами от 1 разворота).
	if name != "30×30" && !slices.Equal(pr.Spec.Steps, want) {
		t.Errorf("%s %s: ступени %+v, CSV %+v", name, r.product, pr.Spec.Steps, want)
	}
}
