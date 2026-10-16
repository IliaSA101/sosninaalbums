package variety

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

func build4(t *testing.T, key string, asp []float64, p layout.Params) layout.Result {
	t.Helper()
	r, ok := layout.Build(key, asp, p)
	if !ok {
		t.Fatalf("Build(%q) отказал", key)
	}
	return r
}

// TestLook: одна картинка разными деревьями и номерами — один вид; зеркало и сдвиг больше
// кванта — другой; сдвиг меньше кванта — тот же.
func TestLook(t *testing.T) {
	asp := sameAspects(4, aspL)
	a := Look(build4(t, "H(V(0,1),V(2,3))", asp, geomSq).Rects, geomSq)
	b := Look(build4(t, "V(H(0,1),H(2,3))", asp, geomSq).Rects, geomSq)
	if a != b {
		t.Errorf("2×2 «столбцами» и «рядами» — одна картинка:\n%s\n%s", a, b)
	}
	// те же фото в другом порядке номеров — та же картинка
	c := Look(build4(t, "H(V(3,2),V(1,0))", asp, geomSq).Rects, geomSq)
	if a != c {
		t.Errorf("номера фото не должны менять вид")
	}

	three := sameAspects(3, aspL)
	left := Look(build4(t, "H(0,V(1,2))", three, geomSq).Rects, geomSq)
	right := Look(build4(t, "H(V(0,1),2)", three, geomSq).Rects, geomSq)
	if left == right {
		t.Error("зеркало — другой вид (крупное слева ≠ справа)")
	}

	rects := []layout.Rect{{X: 10, Y: 10, W: 100, H: 80}, {X: 120, Y: 10, W: 100, H: 80}}
	base := Look(rects, geomSq)
	near := slices.Clone(rects)
	near[0].X += 0.2 * lookQuant * geomSq.PageH // меньше кванта
	if Look(near, geomSq) != base {
		t.Error("сдвиг меньше кванта не меняет вид")
	}
	far := slices.Clone(rects)
	far[0].X += 1.5 * lookQuant * geomSq.PageH
	if Look(far, geomSq) == base {
		t.Error("сдвиг больше кванта — другой вид")
	}
}

func TestLookEdges(t *testing.T) {
	if Look(nil, geomSq) != "" {
		t.Error("нет фото — пустой вид")
	}
	if Look([]layout.Rect{{W: 1, H: 1}}, layout.Params{}) != "" {
		t.Error("нулевая высота страницы: вида нет, но и паники нет")
	}
	// рамки не отсортированы: вид от порядка фото не зависит
	r1 := []layout.Rect{{X: 0, Y: 0, W: 50, H: 50}, {X: 100, Y: 100, W: 90, H: 90}}
	r2 := []layout.Rect{r1[1], r1[0]}
	if Look(r1, geomSq) != Look(r2, geomSq) {
		t.Error("порядок фото не должен менять вид")
	}
}

// TestFamily: порядок bleed → air → hero → dense; главное — от 3 фото и ≥ 1.6× второго.
func TestFamily(t *testing.T) {
	p := geomSq // разворот 600×300 = 180000 мм²
	rc := func(w, h float64) layout.Rect { return layout.Rect{W: w, H: h} }
	cases := []struct {
		name  string
		key   string
		rects []layout.Rect
		want  Family
		big   int
	}{
		{"в край по B[", "B[full:0]", []layout.Rect{rc(600, 300)}, Bleed, -1},
		{"в край по E(", "P[E(0)|1]", []layout.Rect{rc(300, 300), rc(100, 100)}, Bleed, -1},
		{"в край важнее воздуха", "P[E(0)|S(1)]", []layout.Rect{rc(50, 50), rc(50, 50)}, Bleed, -1},
		{"воздух по заполнению", "H(0,1)", []layout.Rect{rc(100, 100), rc(100, 100)}, Air, -1},
		{"воздух важнее главного", "H(0,1,2)", []layout.Rect{rc(200, 200), rc(50, 50), rc(50, 50)}, Air, 0},
		{"главное больше предела 1.6×", "H(0,1,2)", []layout.Rect{rc(300, 300), rc(240, 225), rc(250, 200)}, Hero, 0},
		{"главное чуть меньше предела", "H(0,1,2)", []layout.Rect{rc(300, 300), rc(260, 220), rc(250, 225)}, Dense, -1},
		{"у пары главного нет", "H(0,1)", []layout.Rect{rc(400, 300), rc(100, 100)}, Dense, -1},
		{"плотно", "H(0,1,2)", []layout.Rect{rc(300, 200), rc(300, 200), rc(300, 200)}, Dense, -1},
	}
	for _, c := range cases {
		in := Describe(layout.Result{Key: c.key, Rects: c.rects}, p)
		if in.Family != c.want {
			t.Errorf("%s: семейство %s, ждали %s (fill %.2f)", c.name, in.Family, c.want, in.Fill)
		}
		if c.want == Hero && in.Big != c.big {
			t.Errorf("%s: главное %d, ждали %d", c.name, in.Big, c.big)
		}
	}
	// пустая раскладка: ничего не падает
	if in := Describe(layout.Result{}, p); in.Big != -1 || in.MinSide != 0 || in.Fill != 0 {
		t.Errorf("пустая раскладка: %+v", in)
	}
	if in := Describe(layout.Result{Rects: []layout.Rect{rc(1, 1)}}, layout.Params{}); in.Fill != 0 {
		t.Errorf("нулевой разворот: заполнение %v", in.Fill)
	}
}

// TestFamilyOfForms: сегмент фильтра подборки есть у каждой формы; подпись мозаики пуста
// (весь ряд «Плотно»), у остальных форм — название вида.
func TestFamilyOfForms(t *testing.T) {
	want := map[forms.Form]Family{
		forms.Mosaic: Dense, forms.Strip: Strip, forms.Rows: Strip, forms.Grid: Strip,
		forms.Ladder: Figure, forms.Contrast: Figure, forms.Frame: Figure,
		forms.Air: Air, forms.Asym: Air,
		forms.Spine: Hero, forms.Hero: Hero, forms.Pair: Hero,
		forms.Bleed: Bleed,
	}
	for f, fam := range want {
		if got := FamilyOf(f); got != fam {
			t.Errorf("FamilyOf(%s) = %s, ждали %s", f, got, fam)
		}
		name := FormName(f)
		if (f == forms.Mosaic) != (name == "") || (f != forms.Mosaic && name != forms.Name(f)) {
			t.Errorf("FormName(%s) = %q", f, name)
		}
	}
	if FamilyOf("неизвестная") != Dense {
		t.Error("неизвестная форма — не «Плотно»")
	}
}

// TestDescribeHeroes: главное одно или пара крупных (часть 2); Fair — оценка без штрафа разброса
// за главные: max площадь берётся у первого не-главного; без главных Fair = Score.
func TestDescribeHeroes(t *testing.T) {
	rc := func(w, h float64) layout.Rect { return layout.Rect{W: w, H: h} }
	for _, c := range []struct {
		name        string
		rects       []layout.Rect
		heroes, big int
		fam         Family
		fairGain    float64
	}{
		{"одно главное", []layout.Rect{rc(300, 300), rc(100, 100), rc(100, 100), rc(50, 100)}, 1, 0, Hero, 0.05 * 3.1699250014423126},
		{"пара крупных", []layout.Rect{rc(100, 100), rc(300, 300), rc(280, 280), rc(100, 100)}, 2, -1, Hero, 0.05 * 3.1699250014423126},
		{"пара на трёх — нет", []layout.Rect{rc(300, 300), rc(280, 280), rc(100, 100)}, 0, -1, Dense, 0},
		{"ровно 1.6× — главное", []layout.Rect{rc(160, 100), rc(100, 100), rc(100, 100), rc(100, 100)}, 1, 0, Air, 0.05 * 0.6780719051126377},
		{"ровно и без главных", []layout.Rect{rc(300, 200), rc(300, 200), rc(300, 200), rc(300, 200)}, 0, -1, Dense, 0},
	} {
		in := Describe(layout.Result{Key: "H(0,1,2,3)", Rects: c.rects, Score: 0.5}, geomSq)
		if in.Heroes != c.heroes || in.Big != c.big || in.Family != c.fam {
			t.Errorf("%s: главных %d, главное %d, семейство %s; ждали %d, %d, %s", c.name, in.Heroes, in.Big, in.Family, c.heroes, c.big, c.fam)
		}
		if d := in.Fair - 0.5; d < c.fairGain-1e-9 || d > c.fairGain+1e-9 {
			t.Errorf("%s: Fair − Score = %.6f, ждали %.6f", c.name, d, c.fairGain)
		}
	}
}

// TestFamilyBuilt: семейства настоящих раскладок — ключи из layout.Build.
func TestFamilyBuilt(t *testing.T) {
	asp := []float64{aspL, aspL, aspP}
	hero := build4(t, "H(0,V(1,2))", asp, geomSq)
	if in := Describe(hero, geomSq); in.Family == Bleed {
		t.Errorf("обычное дерево не «в край»: %+v", in)
	}
	for _, r := range layout.Cached(sameAspects(3, aspL), geomSq, 24) {
		if in := Describe(r, geomSq); in.Fill <= 0 || in.MinSide <= 0 || in.Look == "" {
			t.Errorf("%s: %+v", r.Key, in)
		}
	}
}

// TestTwins: самый ранний с тем же видом; −1 — первый; пусто — пусто.
func TestTwins(t *testing.T) {
	got := Twins([]string{"a", "b", "a", "c", "b", "a"})
	if want := []int{-1, -1, 0, -1, 1, 0}; !slices.Equal(got, want) {
		t.Errorf("Twins = %v, ждали %v", got, want)
	}
	if len(Twins(nil)) != 0 {
		t.Error("пусто — пусто")
	}
}

// TestTwinsRow (design §10, «Подборка»): 12 одинаковых горизонтальных — в ряду 24 без
// «Похожих» видно ≤ 8; смешанный набор — свёрнуто ≤ 2.
func TestTwinsRow(t *testing.T) {
	visible := func(asp []float64) (vis, row int) {
		pool := Pool(asp, geomSq, 24, Less)
		looks := make([]string, len(pool))
		for i, c := range pool {
			looks[i] = c.Look
		}
		for _, tw := range Twins(looks) {
			if tw < 0 {
				vis++
			}
		}
		return vis, len(pool)
	}
	if vis, n := visible(sameAspects(12, aspL)); n < 20 || vis > 8 {
		t.Errorf("12 одинаковых: видно %d из %d, нужно ≤ 8", vis, n)
	}
	for seed := uint64(1); seed <= 3; seed++ {
		if vis, n := visible(mixedAspects(10, seed)); n-vis > 2 {
			t.Errorf("смесь из 10 (сид %d): свёрнуто %d из %d, нужно ≤ 2", seed, n-vis, n)
		}
	}
}
