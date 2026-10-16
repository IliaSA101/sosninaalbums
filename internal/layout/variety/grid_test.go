package variety

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Сетки генератора featured (grid.go) и жёсткие запреты allowed — на синтетических входах.

// TestGridTopTies (ревью 041 G3): оценки, равные до кванта (последний бит — FMA на arm64), —
// ничья, остаётся раньше найденное; заметно лучшее вытесняет; keep 1 и 2.
func TestGridTopTies(t *testing.T) {
	ulp := math.Nextafter(0.5, 1)
	for _, c := range []struct {
		name  string
		keep  int
		qs    []float64
		first []int // ожидаемые разрезы best[i].cuts[0] по порядку
	}{
		{"ничья на бит — первое", 1, []float64{0.5, ulp}, []int{0}},
		{"ничья на бит ниже — первое", 1, []float64{ulp, 0.5}, []int{0}},
		{"лучше на квант — второе", 1, []float64{0.5, 0.5 + 2e-9}, []int{1}},
		{"keep 2: ничья — по порядку", 2, []float64{0.5, ulp, 0.4}, []int{0, 1}},
		{"keep 2: лучшее вперёд", 2, []float64{0.4, 0.5, 0.45}, []int{1, 2}},
	} {
		top := gridTop{keep: c.keep}
		for i, q := range c.qs {
			top.add(q, []int{i}, true)
		}
		if top.n != len(c.first) {
			t.Fatalf("%s: n=%d", c.name, top.n)
		}
		for i, want := range c.first {
			if got := top.best[i].cuts[0]; got != want {
				t.Errorf("%s: место %d — разбиение %d, ждали %d", c.name, i, got, want)
			}
		}
	}
}

// TestGridsEmpty: сетки нет — фото нет или область пуста.
func TestGridsEmpty(t *testing.T) {
	g := newGen(sameAspects(8, aspL), geomSq)
	for name, ts := range map[string][]tn{
		"нет фото":       g.grids(3, 3, 100, 100, 2),
		"нулевая ширина": g.grids(0, 4, 0, 100, 2),
		"нулевая высота": g.grids(0, 4, 100, -1, 2),
	} {
		if len(ts) != 0 {
			t.Errorf("%s: %d сеток", name, len(ts))
		}
	}
	if g.grid(0, 4, 100, 0) != nil {
		t.Error("grid в пустой области")
	}
}

// depthOf — глубина дерева ключа (лист — 0), как у штрафа вложенности layout.
func depthOf(t tn) int {
	d := 0
	for _, k := range t.kids {
		d = max(d, depthOf(k)+1)
	}
	return d
}

// TestGridFitIsLayoutScore: оценка сетки (fit) — та же, что score layout (штраф разброса
// spreadPenalty, порог мелких smallWant — копии констант layout), без штрафа вложенности: на листе
// (корешка нет) Build сетки во всю область даёт Score = q − 0.015·глубина. Ряды и столбцы.
func TestGridFitIsLayoutScore(t *testing.T) {
	sheet := geomSq
	sheet.Sheet = true
	n := 0
	for _, as := range [][]float64{sameAspects(12, aspL), seriesAspects(9, 0.65, 3), mixedAspects(14, 7), {aspL, aspP}} {
		g := newGen(as, sheet)
		for _, rows := range []bool{true, false} {
			top := gridTop{keep: 2}
			w, h := g.sw, g.ch
			if !rows {
				w, h = h, w
			}
			g.orient(0, len(as), rows, w, h, &top)
			for _, c := range top.best[:top.n] {
				tr := g.tree(0, len(as), c.cuts[:c.nc], c.rows)
				r, ok := layout.Build(tr.String(), as, sheet)
				if !ok {
					t.Fatalf("%v: Build(%q) отказал", as, tr)
				}
				if want := c.q - 0.015*float64(depthOf(tr)); math.Abs(r.Score-want) > 1e-9 {
					t.Errorf("%q: Score %.9f, оценка сетки без вложенности %.9f", tr, r.Score, want)
				}
				n++
			}
		}
	}
	if n < 8 {
		t.Fatalf("проверено %d сеток", n)
	}
}

// TestAllowed (решение №31) на синтетических Rects 30×30 см: главное одно или пара, его доля,
// не-главные через корешок и у пола, через сгиб — только широкое главное; без layflat сгиба нет.
func TestAllowed(t *testing.T) {
	r := func(x, y, w, h float64) layout.Rect { return layout.Rect{X: x, Y: y, W: w, H: h} }
	small := []layout.Rect{r(320, 12, 60, 40), r(390, 12, 60, 40), r(460, 12, 60, 40)}
	with := func(rs ...layout.Rect) []layout.Rect { return append(rs, small...) }
	for _, c := range []struct {
		name string
		p    layout.Params
		rs   []layout.Rect
		want bool
	}{
		{"главное слева", geomSq, with(r(12, 12, 200, 150)), true},
		{"не-главное через корешок", geomSq, with(r(12, 12, 200, 150), r(280, 100, 60, 40)), false},
		{"то же без layflat", geomFlat, with(r(12, 12, 200, 150), r(280, 100, 60, 40)), true},
		{"не-главное касается корешка", geomSq, with(r(12, 12, 200, 150), r(240.005, 100, 60, 40)), true},
		{"не-главное мельче пола", geomSq, with(r(12, 12, 200, 150), r(320, 100, 60, 35.9)), false},
		{"не-главное ровно на полу", geomSq, with(r(12, 12, 200, 150), r(320, 100, 54, 36)), true},
		{"главное широкое через сгиб", geomSq, with(r(200, 100, 200, 150)), true},
		{"главное-портрет через сгиб", geomSq, with(r(250, 100, 100, 150)), false},
		{"главное-портрет через сгиб, без layflat", geomFlat, with(r(250, 100, 100, 150)), true},
		{"главное мало по доле", geomSq, with(r(12, 12, 80, 50), r(12, 100, 60, 40), r(12, 150, 60, 40), r(12, 200, 60, 40)), false},
		{"главного нет", geomSq, with(r(12, 12, 60, 40)), false},
		{"пара крупных", geomSq, with(r(12, 12, 100, 100), r(150, 12, 100, 100)), true},
		{"пара: портрет через сгиб", geomSq, with(r(12, 12, 100, 100), r(260, 12, 80, 125)), false},
		{"пара: широкое через сгиб", geomSq, with(r(12, 12, 100, 100), r(240, 160, 125, 80)), true},
	} {
		g := newGen(make([]float64, len(c.rs)), c.p)
		if got := g.allowed(c.rs); got != c.want {
			t.Errorf("%s: %v, ждали %v (главных %d)", c.name, got, c.want, forms.Heroes(c.rs).N)
		}
	}
}

// TestFeaturedNoRoom: страница ниже 1 мм внутри полей — ни одной сетки (фото < 1 мм), генератор
// пуст и не падает; панорама шире разворота через корешок не идёт (по бокам места нет).
func TestFeaturedNoRoom(t *testing.T) {
	low := layout.Params{PageW: 300, PageH: 24.5, Margin: 12, Gap: 3, Layflat: true}
	if fs := featured(sameAspects(12, aspL), low); len(fs) != 0 {
		t.Errorf("страница 0.5 мм: %d раскладок", len(fs))
	}
	as := sameAspects(12, aspL)
	as[5] = 3 // 3:1 — на высоте страницы шире разворота
	g := newGen(as, geomSq)
	g.spine()
	if len(g.seen) == 0 {
		t.Fatal("через корешок не пробовано ничего: случай не проверен")
	}
	for key := range g.seen {
		r, ok := layout.Build(key, as, geomSq)
		if x := r.Rects; ok && x[5].X < geomSq.PageW && x[5].X+x[5].W > geomSq.PageW {
			t.Errorf("%q: панорама шире разворота через корешок", key)
		}
	}
}
