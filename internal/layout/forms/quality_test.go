package forms

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// scoreCopy — layout.score, собранный из членов, которые повторяет Quality: заполнение разворота,
// разброс площадей, «мелко» (в Quality заменено полом), корешок, глубина.
func scoreCopy(rs []layout.Rect, box layout.Rect, spine float64, depth int) float64 {
	sum, minSide := 0.0, math.Inf(1)
	for _, r := range rs {
		sum += r.W * r.H
		minSide = math.Min(minSide, math.Min(r.W, r.H))
	}
	s := sum/(box.W*box.H) - spreadTerm(rs, HeroInfo{})
	if want := 0.22 * box.H; minSide < want && len(rs) > 1 {
		s -= 2 * (want - minSide) / box.H
	}
	p := layout.Params{PageW: spine, Layflat: true}
	if spine == 0 {
		p.Sheet = true
	}
	return s - spineTerm(rs, p) - depthW*float64(depth)
}

// TestQualityMatchesScoreTerms: копии членов score (разброс, корешок, глубина) дают ту же оценку,
// что layout, у разворотных ключей ряда Layouts и у Build новых ключей разворота; лист — без корешка.
func TestQualityMatchesScoreTerms(t *testing.T) {
	n := 0
	for _, p := range []layout.Params{geomSq, geomWide, geomSh} {
		spine := p.PageW
		if p.Sheet {
			spine = 0
		}
		box := layout.Rect{X: p.Margin, Y: p.Margin, W: 2*p.PageW - 2*p.Margin, H: p.PageH - 2*p.Margin}
		for k := 2; k <= 7; k++ {
			for seed := uint64(1); seed <= 3; seed++ {
				as := mixed(k, seed*uint64(k))
				rs := layout.Layouts(as, p, 40)
				for _, c := range Gen(as, p) {
					rs = append(rs, c.Result)
				}
				for _, r := range rs {
					if k, _ := shapeOf(r.Key); k != 'T' || len(r.Key) > 1 && r.Key[:2] == "B[" {
						continue // у P, Q, «в край» — своя поправка pairScore / bleedPenalty
					}
					n++
					if got := scoreCopy(r.Rects, box, spine, depthOf(r.Key)); math.Abs(got-r.Score) > 1e-9 {
						t.Fatalf("%q: копия score %.9f, layout %.9f", r.Key, got, r.Score)
					}
				}
			}
		}
	}
	if n < 100 {
		t.Fatalf("сверено только %d ключей", n)
	}
}

// TestQualityStripNearMosaic: лента на 8 одинаковых фото — Q не ниже мозаики − 0.15 (вертикальные:
// горизонтальные на квадратной странице не проходят пол ленты 0.18 — лента на них не строится).
func TestQualityStripNearMosaic(t *testing.T) {
	for _, g := range geoms[:2] {
		as := same(8, aspP)
		best := layout.Layouts(as, g.p, 1)[0]
		mq, ok := Quality(best, Mosaic, g.p)
		if !ok {
			t.Fatalf("%s: мозаика не допустима", g.name)
		}
		strips := 0
		for _, c := range Gen(as, g.p) {
			if c.Form != Strip {
				continue
			}
			strips++
			if c.Q < mq-0.15 {
				t.Errorf("%s: лента %q Q %.3f < мозаика %.3f − 0.15", g.name, c.Key, c.Q, mq)
			}
		}
		if strips == 0 {
			t.Errorf("%s: на 8 вертикальных нет ни одной ленты", g.name)
		}
	}
	// Горизонтальные 8 на квадрате: лента мельче пола — её нет.
	for _, c := range Gen(same(8, aspL), geomSq) {
		if c.Form == Strip {
			t.Errorf("лента %q на 8 горизонтальных прошла пол", c.Key)
		}
	}
}

// TestQualityFloors: полы допустимости — мельчайшая сторона, корешок, главное; мозаика и «в край»
// допустимы всегда (синтетические Rects на странице 30×30).
func TestQualityFloors(t *testing.T) {
	r := func(x, y, w, h float64) layout.Rect { return layout.Rect{X: x, Y: y, W: w, H: h} }
	row := func(h float64) []layout.Rect { return []layout.Rect{r(12, 12, 100, h), r(120, 12, 100, h)} }
	ok := func(rs []layout.Rect, f Form, p layout.Params, key string) bool {
		_, o := Quality(layout.Result{Rects: rs, Key: key}, f, p)
		return o
	}
	for _, c := range []struct {
		name string
		rs   []layout.Rect
		f    Form
		p    layout.Params
		want bool
	}{
		{"лента: на полу 0.18", row(54), Strip, geomSq, true},
		{"лента: мельче пола", row(53.9), Strip, geomSq, false},
		{"ряды: на полу 0.14", row(42), Rows, geomSq, true},
		{"ряды: мельче пола", row(41.9), Rows, geomSq, false},
		// 055: пол лесенки 0.18 (был 0.25 — 4V на разворот 0.245 не проходили).
		{"лесенка: на полу 0.18", row(54), Ladder, geomSq, true},
		{"лесенка: мельче пола", row(53.9), Ladder, geomSq, false},
		{"мозаика: мелко — всё равно да", row(5), Mosaic, geomSq, true},
		{"в край: да", row(5), Bleed, geomSq, true},
		{"через корешок широкое", []layout.Rect{r(210, 12, 180, 100), r(12, 12, 100, 60)}, Strip, geomSq, true},
		{"через корешок: портрет", []layout.Rect{r(250, 12, 100, 150), r(12, 12, 100, 60)}, Strip, geomSq, false},
		{"через корешок: узкая полоска", []layout.Rect{r(297, 12, 100, 60), r(12, 12, 100, 60)}, Strip, geomSq, false},
		{"то же без layflat — не пересекает", []layout.Rect{r(250, 12, 100, 150), r(12, 12, 100, 60)}, Strip, geomFlat, true},
		{"то же на листе", []layout.Rect{r(250, 12, 100, 150), r(12, 12, 100, 60)}, Strip, geomSh, true},
		{"главное: нет главного", row(100), Hero, geomSq, false},
		{"главное слева", []layout.Rect{r(12, 12, 200, 150), r(320, 12, 60, 40), r(390, 12, 60, 40), r(460, 12, 60, 40)}, Hero, geomSq, true},
		{"главное: не-главное через корешок", []layout.Rect{r(12, 12, 200, 150), r(280, 100, 60, 40), r(320, 12, 60, 40), r(390, 12, 60, 40)}, Hero, geomSq, false},
	} {
		if got := ok(c.rs, c.f, c.p, "H(0,1)"); got != c.want {
			t.Errorf("%s: допустимо %v, ждали %v", c.name, got, c.want)
		}
	}
	if _, o := Quality(layout.Result{}, Strip, geomSq); o {
		t.Error("пустая раскладка допустима")
	}
	if _, o := Quality(layout.Result{Rects: row(100)}, Strip, layout.Params{}); o {
		t.Error("нулевая страница допустима")
	}
	if q, o := Quality(layout.Result{Rects: row(5), Score: 0.42, Key: "B[full:0]"}, Bleed, geomSq); !o || q != 0.42 {
		t.Errorf("в край: %v %v, ждали оценку layout 0.42", q, o)
	}
}

// TestSpineOK (decisions 0.13.1 №23): правило корешка для ключа не из ряда — то же, что у полов Gen:
// лесенка с фото 0.74 на корешке (2,4 мм) — нет, и Quality её не пускает; с широким через корешок
// — да; «воздух» V(0,1) с узким на корешке — нет; мозаика с узким на корешке — да (Gen её не режет);
// без корешка (лист, без layflat) и пустая — да; все кандидаты Gen правило проходят.
func TestSpineOK(t *testing.T) {
	const v, n = 200.0 / 300, 222.0 / 300
	build := func(key string, as []float64, p layout.Params) layout.Result {
		t.Helper()
		r, ok := layout.Build(key, as, p)
		if !ok {
			t.Fatalf("%s на %v не строится", key, as)
		}
		return r
	}
	narrow := build("A(0,1,2,3)", []float64{v, n, v, v}, geomSq)
	if _, adm := Quality(narrow, Of(narrow.Key, narrow.Rects, geomSq), geomSq); adm || SpineOK(narrow, geomSq) {
		t.Errorf("лесенка с узким фото на корешке: Quality %v, SpineOK %v — ждали нет", adm, SpineOK(narrow, geomSq))
	}
	for _, c := range []struct {
		name string
		r    layout.Result
		p    layout.Params
		want bool
	}{
		{"лесенка: широкое через корешок", build("A(0,1,2,3)", []float64{aspL, aspL, aspL, n}, geomSq), geomSq, true},
		{"воздух: узкое на корешке", build("V(0,1)", []float64{n, v}, geomSq), geomSq, false},
		{"воздух на листе", build("V(0,1)", []float64{n, v}, geomSh), geomSh, true},
		{"мозаика: узкое на корешке", build("H(0,1)", []float64{n, v}, geomSq), geomSq, true},
		{"те же рамки без layflat", narrow, geomFlat, true},
		{"пустая", layout.Result{}, geomSq, true},
	} {
		if got := SpineOK(c.r, c.p); got != c.want {
			t.Errorf("%s (%s): %v, ждали %v", c.name, c.r.Key, got, c.want)
		}
	}
	for _, as := range [][]float64{{v, n, v, v}, same(4, aspL), {aspL, v, aspL, n, v}, same(7, v)} {
		for _, c := range Gen(as, geomSq) {
			if !SpineOK(c.Result, geomSq) {
				t.Errorf("%v: кандидат Gen %s не проходит SpineOK", as, c.Key)
			}
		}
	}
}

// TestQualityOrder: чем плотнее блок и чем меньше разброс — тем выше Q; доля рамки вне полосы
// вида режет оценку.
func TestQualityOrder(t *testing.T) {
	as := same(4, aspL)
	dense := layout.Layouts(as, geomSq, 1)[0]
	qd, _ := Quality(dense, Mosaic, geomSq)
	if qd < 0.5 {
		t.Errorf("плотная мозаика: Q %.3f", qd)
	}
	small, ok := layout.Build("S(H(0,1,2,3))", as, geomSq)
	if !ok {
		t.Fatal("S(H) не построена")
	}
	qa, _ := Quality(small, Air, geomSq)
	qm, _ := Quality(small, Mosaic, geomSq) // та же картинка как «мозаика»: доля рамки мала для полосы
	if qm >= qa {
		t.Errorf("блок поменьше: как мозаика %.3f, как воздух %.3f — полоса не работает", qm, qa)
	}
}

// TestDepthOf: глубина ключа — как node.depth у перебора; мусор не роняет.
func TestDepthOf(t *testing.T) {
	for key, want := range map[string]int{
		"0": 0, "H(0,1)": 1, "V(H(0,1),2)": 2, "G(0,1)": 1, "G(H(0,1),2)": 2, "L(0,1,2)": 1,
		"P[H(0,1)|V(H(2,3),4)]": 2, "S(H(0,1))": 1, "Q[0|H(m1,2)]": 1, "P[-|0]": 0, "B[full:0]": 0,
		"H(V(0,1,2),3,V(4,5))": 2, "": 0, ")))": 0, "((": 0,
	} {
		if got := depthOf(key); got != want {
			t.Errorf("%q: глубина %d, ждали %d", key, got, want)
		}
	}
	// У ряда Layouts глубина ключа совпадает с оценкой layout (через TestQualityMatchesScoreTerms);
	// здесь — V → H → V: три уровня.
	if got := depthOf("V(H(V(0,1),2),H(3,V(4,5)))"); got != 3 {
		t.Errorf("глубокое дерево: %d", got)
	}
}

// TestHeroes: главное, пара, равные, меньше трёх и четырёх фото; ничья — раньше.
func TestHeroes(t *testing.T) {
	rc := func(w, h float64) layout.Rect { return layout.Rect{W: w, H: h} }
	for _, c := range []struct {
		name string
		rs   []layout.Rect
		n    int
		idx  [2]int
	}{
		{"одно", []layout.Rect{rc(300, 300), rc(100, 100), rc(100, 100), rc(50, 100)}, 1, [2]int{0, 1}},
		{"пара", []layout.Rect{rc(100, 100), rc(300, 300), rc(280, 280), rc(100, 100)}, 2, [2]int{1, 2}},
		{"пара на трёх — нет", []layout.Rect{rc(300, 300), rc(280, 280), rc(100, 100)}, 0, [2]int{0, 1}},
		{"ровно 1.6×", []layout.Rect{rc(160, 100), rc(100, 100), rc(100, 100), rc(100, 100)}, 1, [2]int{0, 1}},
		{"равные", []layout.Rect{rc(300, 200), rc(300, 200), rc(300, 200), rc(300, 200)}, 0, [2]int{0, 1}},
		{"два фото", []layout.Rect{rc(300, 300), rc(10, 10)}, 0, [2]int{0, 1}},
		{"пусто", nil, 0, [2]int{-1, -1}},
	} {
		h := Heroes(c.rs)
		if h.N != c.n || h.Idx != c.idx {
			t.Errorf("%s: главных %d индексы %v, ждали %d %v", c.name, h.N, h.Idx, c.n, c.idx)
		}
	}
	if h := Heroes([]layout.Rect{rc(300, 300), rc(100, 100), rc(100, 100)}); !h.isHero(0) || h.isHero(1) {
		t.Error("isHero")
	}
}

// TestGenMin (решение 0.13 №40): GenMin с minSide 0, отрицательным и крупнее любого пола — ровно Gen;
// с minSide меньше пола вида — сверх Gen только формы, которые строгий пол не проходят, а мельчайшая
// сторона у них не меньше minSide; на наборах 20–30 фото такие есть.
func TestGenMin(t *testing.T) {
	keys := func(cs []Cand) []string {
		out := make([]string, len(cs))
		for i, c := range cs {
			out[i] = c.Key
		}
		return out
	}
	const relax = 36.0 // мм: 0.12 высоты страницы при полах 0.14–0.25
	extra := 0
	for _, as := range [][]float64{mixed(20, 20), mixed(25, 25), mixed(30, 3), series(25, 25), same(12, aspL)} {
		gen := keys(Gen(as, geomFlat))
		in := make(map[string]bool, len(gen))
		for _, k := range gen {
			in[k] = true
		}
		for _, m := range []float64{0, -1, 1e6} {
			if got := keys(GenMin(as, geomFlat, m)); !slices.Equal(got, gen) {
				t.Errorf("%d фото, minSide %g: %d ключей, у Gen %d", len(as), m, len(got), len(gen))
			}
		}
		for _, c := range GenMin(as, geomFlat, relax) {
			if in[c.Key] {
				continue
			}
			extra++
			side := math.Inf(1)
			for _, r := range c.Rects {
				side = math.Min(side, math.Min(r.W, r.H))
			}
			if _, ok := Quality(c.Result, c.Form, geomFlat); ok || quanta(side) < quanta(relax) {
				t.Errorf("%d фото: %q — строгий пол %t, мельчайшая сторона %.1f мм", len(as), c.Key, ok, side)
			}
		}
	}
	if extra == 0 {
		t.Fatal("minSide не добавил ни одной формы: тест ничего не проверяет")
	}
}

// TestPageRowsFallback (решение 0.13 №40): запасной перебор Q[G|G] (разрез ±1, ряды 2–4 с разницей
// на один) — только когда у середины с равным числом рядов ничего не прошло; в его рядах не меньше
// двух фото. Смеси и серии 12–30 фото без layflat; запасной срабатывает хоть раз.
func TestPageRowsFallback(t *testing.T) {
	fallback := 0
	for n := 12; n <= 30; n++ {
		for seed := uint64(1); seed <= 4; seed++ {
			for _, as := range [][]float64{mixed(n, seed), series(n, seed)} {
				got, spare := pageRowKeys(as)
				if len(spare) > 0 && len(got) > 0 {
					t.Errorf("%d фото: запасные %v при основных %v", n, spare, got)
				}
				for _, k := range spare {
					if singleRow(k) {
						t.Errorf("%d фото: в запасном %q ряд из одного фото", n, k)
					}
				}
				fallback += len(spare)
			}
		}
	}
	if fallback == 0 {
		t.Fatal("запасной перебор ни разу не сработал: тест ничего не проверяет")
	}
}

// pageRowKeys — ключи Q[G|G] вида «ряды» у Gen без layflat: основные (разрез у середины, рядов
// поровну) и запасные (прочие).
func pageRowKeys(as []float64) (main, spare []string) {
	pre := prefixSums(as)
	n := len(as)
	c0 := cutNear(pre, 0, n, 0.5)
	base := map[string]bool{}
	for r := 2; r <= maxGrid; r++ {
		base[pairKey("", gEq(pre, 0, c0, r), gEq(pre, c0, n, r))] = true
	}
	for _, c := range Gen(as, geomFlat) {
		switch {
		case c.Form != Rows || !strings.HasPrefix(c.Key, "Q["):
		case base[c.Key]:
			main = append(main, c.Key)
		default:
			spare = append(spare, c.Key)
		}
	}
	return main, spare
}

// singleRow — у ключа Q[G(…)|G(…)] есть ряд из одного фото (лист прямо в G, а не H(…)).
func singleRow(key string) bool {
	depth := 0
	for i := 0; i < len(key); i++ {
		switch c := key[i]; {
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 1 && c >= '0' && c <= '9' && (key[i-1] == '(' || key[i-1] == ','):
			return true // лист на уровне рядов G
		}
	}
	return false
}
