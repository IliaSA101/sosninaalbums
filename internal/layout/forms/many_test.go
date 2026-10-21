package forms

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Формы многих фото (спека 072, design §6): ряды DP, «сетка + главное», «две крупных».

// manySets — наборы 17–40 фото: одинаковые обеих ориентаций, смесь, серия, stock с 0 и 30.
func manySets(t testing.TB) map[string][]float64 {
	st := testkit.StockAspects(t)
	out := map[string][]float64{}
	for _, n := range []int{17, 20, 25, 28, 30, 35, 40} {
		out[fmt.Sprintf("гориз%d", n)] = same(n, aspL)
		out[fmt.Sprintf("верт%d", n)] = same(n, aspP)
		out[fmt.Sprintf("смесь%d", n)] = mixed(n, uint64(n))
		out[fmt.Sprintf("серия%d", n)] = series(n, uint64(n))
		out[fmt.Sprintf("stock%d", n)] = st[:n]
		out[fmt.Sprintf("stock30+%d", n)] = st[30 : 30+n]
	}
	return out
}

// manyOut — выдача одного генератора многих фото на наборе (пол не строже relax мм, как GenMin).
func manyOut(as []float64, p layout.Params, relax float64, run func(*gen)) []Cand {
	g := newGen(as, p, relax)
	run(g)
	return g.out
}

// pageOf — 0 у фото левой страницы, 1 — правой (по центру рамки).
func pageOf(x layout.Rect, p layout.Params) int {
	if x.X+x.W/2 < p.PageW {
		return 0
	}
	return 1
}

// pageSpan — первое и последнее фото страницы pg (у P/Q фото страницы идут подряд).
func pageSpan(rs []layout.Rect, p layout.Params, pg int) (lo, hi int) {
	lo, hi = -1, -1
	for i, x := range rs {
		if pageOf(x, p) == pg {
			if lo < 0 {
				lo = i
			}
			hi = i
		}
	}
	return lo, hi
}

// pageRowsOf — ряды G страницы ключа Q[G(…)|G(…)]: у каждого ряда — число фото.
func pageRowsOf(t *testing.T, key string) [2][]int {
	t.Helper()
	l, r, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(key, "Q["), "]"), "|")
	if !ok || !strings.HasPrefix(l, "G(") || !strings.HasPrefix(r, "G(") {
		t.Fatalf("%q — не Q[G|G]", key)
	}
	count := func(g string) []int {
		var rows []int
		for _, row := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(g, "G("), ")"), "),") {
			rows = append(rows, len(strings.Split(row, ",")))
		}
		return rows
	}
	return [2][]int{count(l), count(r)}
}

// minSideOf — мельчайшая сторона всех фото.
func minSideOf(rs []layout.Rect) float64 {
	m := math.Inf(1)
	for _, x := range rs {
		m = math.Min(m, math.Min(x.W, x.H))
	}
	return m
}

// TestManyRows: «Ряды» на 17–40 фото — Q[G|G], Build принимает, вид rows/grid, в ряду ≥ 2 фото,
// рядов на странице 2–6 и поровну или на один больше, в порядке ленты; варианты не мельче 0,97
// самого крупного; на stock 25 и 30 (30×30, пол 0,9 мозаики — как у ряда «Авто») ряды есть и их
// мельчайшая сторона ≥ 0,9 мозаики; на листе — ничего; 6 рядов где-нибудь встречается (DP вышел
// за прежние 4).
func TestManyRows(t *testing.T) {
	six, total := false, 0
	for name, as := range manySets(t) {
		for _, g := range geoms {
			out := manyOut(as, g.p, 0, (*gen).manyRows)
			if g.p.Sheet && len(out) != 0 {
				t.Errorf("%s/%s: на листе %d рядов", g.name, name, len(out))
			}
			total += len(out)
			best := bestSide(out)
			for _, c := range out {
				n := g.name + "/" + name
				checkCand(t, n, as, g.p, c)
				rows := checkRowsCand(t, n, c, best)
				six = six || rows == manyRowsMax
			}
		}
	}
	if !six || total == 0 {
		t.Errorf("рядов %d, 6 на странице %t: разрез DP не выходит за прежние 4", total, six)
	}
	st := testkit.StockAspects(t)
	for _, n := range []int{25, 30} {
		as := st[:n]
		mos := minSideOf(layout.Cached(as, geomSq, layout.ManyBatch)[0].Rects)
		out := manyOut(as, geomSq, 0.9*mos, (*gen).manyRows)
		if len(out) == 0 {
			t.Errorf("stock %d: рядов нет при полу 0,9 мозаики %.1f мм", n, mos)
		}
		for _, c := range out {
			if s := sideOf(c); s < 0.9*mos {
				t.Errorf("stock %d: %q — мельчайшая сторона %.1f мм < 0,9 мозаики %.1f", n, c.Key, s, mos)
			}
		}
	}
}

// bestSide — наибольшая мельчайшая сторона (без главных) среди вариантов.
func bestSide(out []Cand) float64 {
	best := 0.0
	for _, c := range out {
		best = math.Max(best, sideOf(c))
	}
	return best
}

// checkRowsCand — вариант «Рядов»: вид rows/grid, в ряду ≥ 2 фото, рядов на странице 2–6 и поровну
// или на один больше, не мельче keepShare самого крупного best; отдаёт рядов на большей странице.
func checkRowsCand(t *testing.T, name string, c Cand, best float64) int {
	t.Helper()
	if c.Form != Rows && c.Form != Grid {
		t.Errorf("%s: %q — вид %s", name, c.Key, c.Form)
	}
	if singleRow(c.Key) {
		t.Errorf("%s: %q — ряд из одного фото", name, c.Key)
	}
	rows := pageRowsOf(t, c.Key)
	nl, nr := len(rows[0]), len(rows[1])
	if nl < 2 || nl > manyRowsMax || nr < 2 || nr > manyRowsMax || nl-nr > 1 || nr-nl > 1 {
		t.Errorf("%s: %q — рядов %d | %d", name, c.Key, nl, nr)
	}
	if quanta(sideOf(c)) < quanta(keepShare*best) {
		t.Errorf("%s: %q мельче %.2f самого крупного", name, c.Key, keepShare)
	}
	return max(nl, nr)
}

// cornerHero — "" если фото h раскладки c — первое или последнее своей страницы и рядом с ним (по
// высоте) ровно два ряда, вместе той же высоты (±10 %); иначе — что не так.
func cornerHero(c Cand, p layout.Params, h int) string {
	rs := c.Rects
	x := rs[h]
	lo, hi := pageSpan(rs, p, pageOf(x, p))
	if h != lo && h != hi {
		return fmt.Sprintf("главное %d не с края страницы %d…%d", h, lo, hi)
	}
	tops := map[int64]bool{}
	y0, y1 := math.Inf(1), math.Inf(-1)
	for i := lo; i <= hi; i++ {
		y := rs[i]
		if i == h || y.Y >= x.Y+x.H-1e-6 || y.Y+y.H <= x.Y+1e-6 {
			continue
		}
		tops[int64(math.Round(y.Y*1000))] = true
		y0, y1 = math.Min(y0, y.Y), math.Max(y1, y.Y+y.H)
	}
	if len(tops) != 2 || math.Abs((y1-y0)-x.H) > 0.1*x.H {
		return fmt.Sprintf("рядом с главным %d %d рядов высотой %.1f при главном %.1f", h, len(tops), y1-y0, x.H)
	}
	return ""
}

// medianOthers — медиана площадей фото без главных.
func medianOthers(rs []layout.Rect, h HeroInfo) float64 {
	var as []float64
	for i, x := range rs {
		if !h.isHero(i) {
			as = append(as, x.W*x.H)
		}
	}
	slices.Sort(as)
	return as[len(as)/2]
}

// heroOut — общие проверки выдачи gridHero / gridPair: Build принимает, вид hero или pair (пара —
// главное и сосед по блоку, оба крупные), главное (у пары — хоть одно) в углу своей страницы на два
// ряда, последнее из главных ≥ 3 медиан прочих, варианты не мельче 0,97 самого крупного; на листе —
// ничего. visit — на каждую раскладку с её главными.
func heroOut(t *testing.T, run func(*gen), visit func(c Cand, p layout.Params, h HeroInfo)) {
	t.Helper()
	for name, as := range manySets(t) {
		for _, g := range geoms {
			out := manyOut(as, g.p, 0, run)
			if g.p.Sheet && len(out) != 0 {
				t.Errorf("%s/%s: на листе %d", g.name, name, len(out))
			}
			best := bestSide(out)
			for _, c := range out {
				n := g.name + "/" + name
				checkCand(t, n, as, g.p, c)
				if h, ok := checkHeroCand(t, n, c, g.p, best); ok {
					visit(c, g.p, h)
				}
			}
		}
	}
}

// checkHeroCand — проверки heroOut одной раскладки; false — вид не hero/pair (дальше не смотреть).
func checkHeroCand(t *testing.T, name string, c Cand, p layout.Params, best float64) (HeroInfo, bool) {
	t.Helper()
	h := Heroes(c.Rects)
	if c.Form != Hero && c.Form != Pair || h.N == 0 {
		t.Errorf("%s: %q — вид %s, главных %d", name, c.Key, c.Form, h.N)
		return h, false
	}
	bad := cornerHero(c, p, h.Idx[0])
	if h.N == 2 && bad != "" {
		bad = cornerHero(c, p, h.Idx[1])
	}
	if bad != "" {
		t.Errorf("%s: %q — %s", name, c.Key, bad)
	}
	if quanta(h.Top[h.N-1]) < quanta(heroManyRatio*medianOthers(c.Rects, h)) {
		t.Errorf("%s: %q — главное меньше %g медиан", name, c.Key, heroManyRatio)
	}
	if quanta(sideOf(c)) < quanta(keepShare*best) {
		t.Errorf("%s: %q мельче %.2f самого крупного", name, c.Key, keepShare)
	}
	return h, true
}

// TestGridHero: «сетка + главное» на 17–40 фото — heroOut; вид hero бывает, в т. ч. главное справа и
// последним фото страницы.
func TestGridHero(t *testing.T) {
	var right, last bool
	total := 0
	heroOut(t, (*gen).gridHero, func(c Cand, p layout.Params, h HeroInfo) {
		if c.Form != Hero {
			return
		}
		x := h.Idx[0]
		right = right || pageOf(c.Rects[x], p) == 1
		_, hi := pageSpan(c.Rects, p, pageOf(c.Rects[x], p))
		last = last || x == hi
		total++
	})
	if total == 0 || !right || !last {
		t.Errorf("«сетка + главное»: %d раскладок, справа %t, последним %t", total, right, last)
	}
}

// TestGridPair: «две крупных» на 17–40 фото — heroOut; пара с главными на разных страницах бывает, и
// тогда оба — в углу своей страницы на два ряда.
func TestGridPair(t *testing.T) {
	total := 0
	heroOut(t, (*gen).gridPair, func(c Cand, p layout.Params, h HeroInfo) {
		if c.Form != Pair || pageOf(c.Rects[h.Idx[0]], p) == pageOf(c.Rects[h.Idx[1]], p) {
			return
		}
		for _, x := range h.Idx {
			if bad := cornerHero(c, p, x); bad != "" {
				t.Errorf("%q — %s", c.Key, bad)
			}
		}
		total++
	})
	if total == 0 {
		t.Error("«две крупных» по странице ни разу")
	}
}

// TestManyEdges: панорама 3:1 главным (с края страницы), все вертикальные, квадраты — без паники,
// ключи проходят инварианты; генераторы многих фото — ровно на 17–40 фото со страницами.
func TestManyEdges(t *testing.T) {
	pano := same(25, aspL)
	pano[0], pano[11], pano[12], pano[24] = 3, 3, 3, 3
	edges := map[string][]float64{"панорама": pano, "верт30": same(30, aspP), "квадраты28": same(28, 1), "полоски20": same(20, 8)}
	// панорама длиннее суммы прочих первой или последней: разрез у середины — у края ленты (ревью 072)
	for _, n := range []int{17, 40} {
		for _, at := range []int{0, n - 1} {
			as := same(n, aspP)
			as[at] = 20
			edges[fmt.Sprintf("панорама20:1-%d-в%d", n, at)] = as
		}
	}
	for name, as := range edges {
		for _, g := range geoms {
			for _, run := range []func(*gen){(*gen).manyRows, (*gen).gridHero, (*gen).gridPair} {
				for _, c := range manyOut(as, g.p, 0, run) {
					checkCand(t, g.name+"/"+name, as, g.p, c)
				}
			}
		}
	}
	for _, c := range []struct {
		k    int
		p    layout.Params
		want bool
	}{{16, geomSq, false}, {17, geomSq, true}, {40, geomFlat, true}, {41, geomSq, false}, {25, geomSh, false}} {
		if got := newGen(same(c.k, aspL), c.p, 0).many(); got != c.want {
			t.Errorf("%d фото, лист %t: many %t", c.k, c.p.Sheet, got)
		}
	}
	for name, as := range edges {
		for _, g := range geoms[:3] {
			for _, c := range Gen(as, g.p) {
				checkCand(t, g.name+"/"+name+"/Gen", as, g.p, c)
			}
		}
	}
	// с 41 фото ряды и главные многих фото не строятся (Gen — прежние генераторы)
	for _, c := range Gen(same(41, aspL), geomFlat) {
		if c.Form == Hero || c.Form == Pair || c.Form == Rows {
			t.Errorf("41 фото: %q — вид %s", c.Key, c.Form)
		}
	}
}

// TestHeroManyRule (072-4): на 17+ фото главное с долей < 0,30, но ≥ 3 медиан прочих проходит пол
// «С главным фото»; до 16 фото — нет; 2,9 медианы — нет; доля ≥ 0,30 проходит и до 16 фото. У пары —
// по второму крупному (доля < 0,15; до 16 фото 3 медианы пары — уже доля ≥ 0,15, граница — у главного).
func TestHeroManyRule(t *testing.T) {
	const side = 40.0 // прочие — квадраты 40 мм: выше пола 0,12 страницы
	others := func(n int) []layout.Rect {
		var rs []layout.Rect
		for i := range n {
			rs = append(rs, layout.Rect{X: 12 + float64(i%6)*43, Y: 12 + float64(i/6)*43, W: side, H: side})
		}
		return rs
	}
	big := func(x, ratio float64) layout.Rect {
		s := side * math.Sqrt(ratio)
		return layout.Rect{X: x, Y: 12, W: s, H: s}
	}
	ok := func(rs []layout.Rect, f Form) bool {
		_, o := Quality(layout.Result{Rects: rs, Key: "H(0,1)"}, f, geomSq)
		return o
	}
	for _, c := range []struct {
		name  string
		n     int // прочих
		ratio float64
		want  bool
		f     Form          // "" — Hero
		p     layout.Params // нулевая — geomSq
	}{
		{"17 фото, 3 медианы", 16, 3, true, "", layout.Params{}},
		{"30 фото, 3 медианы", 29, 3, true, "", layout.Params{}},
		{"40 фото, 3 медианы", 39, 3, true, "", layout.Params{}},
		{"17 фото, 2,9 медианы", 16, 2.9, false, "", layout.Params{}},
		{"16 фото, 3 медианы", 15, 3, false, "", layout.Params{}},
		{"16 фото, доля ≥ 0,30", 15, 7, true, "", layout.Params{}},
		// правило медиан — только у Hero и Pair на 17–40 фото со страницами (design §2.5, ревью 072)
		{"41 фото, 3 медианы", 40, 3, false, "", layout.Params{}},
		{"лист, 17 фото, 3 медианы", 16, 3, false, "", geomSh},
		{"через корешок, 17 фото, 3 медианы", 16, 3, false, Spine, layout.Params{}},
		{"через корешок, 16 фото, доля ≥ 0,30", 15, 7, true, Spine, layout.Params{}},
	} {
		f, p := cmp.Or(c.f, Hero), geomSq
		if c.p != (layout.Params{}) {
			p = c.p
		}
		rs := append([]layout.Rect{big(312, c.ratio)}, others(c.n)...)
		if _, got := Quality(layout.Result{Rects: rs, Key: "H(0,1)"}, f, p); got != c.want {
			t.Errorf("главное, %s: допустимо %t, ждали %t", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		name  string
		n     int
		ratio float64
		want  bool
	}{
		{"18 фото, 3 медианы", 16, 3, true},
		{"18 фото, 2,9 медианы", 16, 2.9, false},
	} {
		rs := append([]layout.Rect{big(312, c.ratio), big(450, c.ratio)}, others(c.n)...)
		if got := ok(rs, Pair); got != c.want {
			t.Errorf("пара, %s: допустимо %t, ждали %t", c.name, got, c.want)
		}
	}
}

// TestPartRows: разрез DP — ровно r рядов, в ряду ≥ minN фото, наибольшая сумма не больше, чем у
// любого другого разреза (перебор на 8 фото); не делится — nil; ничья — разрез левее.
func TestPartRows(t *testing.T) {
	as := []float64{1.5, 0.67, 1.5, 1.5, 1, 0.67, 1.5, 2}
	pre := prefixSums(as)
	for _, minN := range []int{1, 2} {
		for lo := 0; lo <= 2; lo++ {
			for r := 1; r <= 5; r++ {
				checkPart(t, pre, lo, len(as), r, minN)
			}
		}
	}
	g := newGen(as, geomSq, 0)
	// отрезок вне ленты (разрез страниц у края, ревью 072) — nil, без паники; r > manyRowsMax — мимо таблицы
	for _, c := range []struct{ lo, hi, r, minN int }{{0, 8, 0, 1}, {0, 8, 5, 2}, {0, 8, 9, 1}, {3, 3, 1, 1}, {0, 8, 2, 0},
		{-1, 8, 2, 1}, {0, 9, 2, 1}, {-2, -1, 1, 1}, {9, 8, 1, 1}, {6, 3, 1, 1}, {-1, 9, 7, 1}} {
		if b := partMin(pre, c.lo, c.hi, c.r, c.minN); b != nil {
			t.Errorf("%+v: ждали nil, есть %v", c, b)
		}
		if b := g.part(c.lo, c.hi, c.r, c.minN); b != nil {
			t.Errorf("part %+v: ждали nil, есть %v", c, b)
		}
	}
	g = newGen(as, geomSq, 0)
	// одинаковые фото: 5 в 2 ряда — 2 | 3 и 3 | 2 равны, берётся разрез левее (первый в переборе)
	if b := partMin(prefixSums(same(5, aspL)), 0, 5, 2, 1); !slices.Equal(b, []int{0, 2, 5}) {
		t.Errorf("5 одинаковых в 2 ряда: %v, ждали [0 2 5]", b)
	}
	if !slices.Equal(g.part(0, 8, 3, 2), partMin(pre, 0, 8, 3, 2)) || len(g.parts) != 1 {
		t.Errorf("part: %v, кэш %d", g.part(0, 8, 3, 2), len(g.parts))
	}
}

// allCuts — все разрезы lo…hi−1 на r рядов по ≥ minN фото (перебор).
func allCuts(lo, hi, r, minN int) [][]int {
	if r == 1 {
		if hi-lo >= minN {
			return [][]int{{lo, hi}}
		}
		return nil
	}
	var out [][]int
	for c := lo + minN; c <= hi-minN; c++ {
		for _, rest := range allCuts(c, hi, r-1, minN) {
			out = append(out, append([]int{lo}, rest...))
		}
	}
	return out
}

// checkPart — partMin против перебора: nil ровно когда разрезов нет; иначе ровно r рядов от lo до
// hi по ≥ minN фото, наибольшая сумма ряда не больше, чем у любого разреза.
func checkPart(t *testing.T, pre []float64, lo, hi, r, minN int) {
	t.Helper()
	maxSum := func(b []int) float64 {
		m := 0.0
		for i := 1; i < len(b); i++ {
			m = math.Max(m, pre[b[i]]-pre[b[i-1]])
		}
		return m
	}
	b := partMin(pre, lo, hi, r, minN)
	cands := allCuts(lo, hi, r, minN)
	if len(cands) == 0 || b == nil {
		if (len(cands) == 0) != (b == nil) {
			t.Errorf("lo %d r %d minN %d: разрезов %d, partMin %v", lo, r, minN, len(cands), b)
		}
		return
	}
	if len(b) != r+1 || b[0] != lo || b[r] != hi {
		t.Fatalf("lo %d r %d minN %d: границы %v", lo, r, minN, b)
	}
	for i := 1; i < len(b); i++ {
		if b[i]-b[i-1] < minN {
			t.Errorf("lo %d r %d minN %d: ряд %v короче %d", lo, r, minN, b, minN)
		}
	}
	for _, c := range cands {
		if quanta(maxSum(c)) < quanta(maxSum(b)) {
			t.Errorf("lo %d r %d minN %d: %v лучше %v", lo, r, minN, c, b)
		}
	}
}

// TestKeepLargest: остаются варианты не мельче keepShare самого крупного, в прежнем порядке; до
// from — не трогается; у «С главным» главное в сторону не входит; пустой хвост — без паники.
func TestKeepLargest(t *testing.T) {
	sq := func(s float64) layout.Rect { return layout.Rect{W: s, H: s} }
	cand := func(key string, f Form, rs ...layout.Rect) Cand {
		return Cand{Result: layout.Result{Key: key, Rects: rs}, Form: f}
	}
	g := &gen{}
	g.out = []Cand{cand("до", Rows, sq(1), sq(1))}
	g.keepLargest(1)
	if len(g.out) != 1 {
		t.Fatalf("пустой хвост: %d", len(g.out))
	}
	g.out = append(g.out,
		cand("a", Rows, sq(40), sq(50)),
		cand("b", Rows, sq(38.8), sq(50)), // 0,97 — на пороге
		cand("c", Rows, sq(38.7), sq(50)),
		cand("hero", Hero, sq(200), sq(39), sq(39), sq(39)), // главное 200 мм не в счёт
		cand("мелкий", Rows, sq(30), sq(30)),
	)
	tail := slices.Clone(g.out[1:])
	g.keepLargest(1)
	if want := []string{"до", "a", "b", "hero"}; !slices.Equal(keysOf(g.out), want) {
		t.Errorf("остались %v, ждали %v", keysOf(g.out), want)
	}
	// только вид Hero: прочие отбрасываются и в «самый крупный» не входят (мозаика 50 мм не
	// срезает главное с прочими 39 мм)
	g.out = append(g.out[:1], tail...)
	g.out = append(g.out, cand("мозаика", Mosaic, sq(50), sq(50)), cand("hero2", Hero, sq(200), sq(30), sq(30), sq(30)))
	g.keepLargest(1, Hero)
	if want := []string{"до", "hero"}; !slices.Equal(keysOf(g.out), want) {
		t.Errorf("вид hero: остались %v, ждали %v", keysOf(g.out), want)
	}
	g.out = append(g.out[:1], cand("мозаика", Mosaic, sq(50), sq(50)))
	if g.keepLargest(1, Pair); len(g.out) != 1 {
		t.Errorf("ни одного вида pair: остались %v", keysOf(g.out))
	}
}

func keysOf(cs []Cand) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Key
	}
	return out
}

// BenchmarkForms30 — холодный Gen на stock 30 (design §5: ≤ 4 мс).
func BenchmarkForms30(b *testing.B) {
	as := testkit.StockAspects(b)[:30]
	for b.Loop() {
		Gen(as, geomSq)
	}
}

// TestGenParallel: Gen на многих фото параллельно (ряды «Авто» разных разворотов) — те же ключи, что
// по одному; кэши разреза и страниц — в gen, не общие (под -race общий кэш прототипа ронял variety).
func TestGenParallel(t *testing.T) {
	sets := manySets(t)
	names := sortedNames(sets)
	want := make([][]string, len(names))
	for i, n := range names {
		want[i] = keysOf(Gen(sets[n], geomFlat))
	}
	got := make([][]string, len(names))
	done := make(chan struct{})
	for i, n := range names {
		go func() {
			defer func() { done <- struct{}{} }()
			got[i] = keysOf(Gen(sets[n], geomFlat))
		}()
	}
	for range names {
		<-done
	}
	for i, n := range names {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("%s: параллельно другие ключи", n)
		}
	}
}

// TestManyBuilds (ревью 072): бюджет Build на разворот (buildLimit) и на наборах многих фото —
// TestFormsInvariants смотрит только sets(), а худший найденный (stock 29 на 30×30) — не там: stock
// и stock с 30 — на каждом числе фото 17–40.
func TestManyBuilds(t *testing.T) {
	all := manySets(t)
	st := testkit.StockAspects(t)
	for n := layout.ManyFrom; n <= manyMax; n++ {
		all[fmt.Sprintf("stock%d", n)] = st[:n]
		all[fmt.Sprintf("stock30+%d", n)] = st[30 : 30+n]
	}
	worst, at := 0, ""
	for name, as := range all {
		for _, g := range geoms {
			b, lim := builds(as, g.p), buildLimit(len(as))
			if b > lim {
				t.Errorf("%s/%s: Build %d > %d", g.name, name, b, lim)
			}
			if b > worst {
				worst, at = b, g.name+"/"+name
			}
		}
	}
	t.Logf("больше всего Build: %d (%s)", worst, at)
}

// TestManyPanoramas (ревью 072): панорама 11:1…100:1 у краёв ленты (первые и последние три позиции) и
// в середине, 17–40 фото, 3 геометрии со страницами — Gen без паники, ключи проходят checkCand,
// бюджет Build — buildLimit. Панорама длиннее остальных у края ставит разрез страниц у края ленты,
// и разрезы генераторов выходили за неё. Все позиции подряд — ×7 дольше (≈ 80 с) без новых случаев:
// разрез у края дают только крайние.
func TestManyPanoramas(t *testing.T) {
	ratios := []float64{11, 14, 20, 25, 30, 50, 100}
	ks := make([]int, 0, manyMax-layout.ManyFrom+1)
	for k := layout.ManyFrom; k <= manyMax; k++ {
		ks = append(ks, k)
	}
	lite := testkit.Lite()
	for _, k := range ks {
		t.Run(fmt.Sprint(k), func(t *testing.T) {
			t.Parallel()
			// Под Lite набор подтестов тот же (test-baseline сверяет имена), урезается объём внутри:
			// вне {17, 25, 40} — одно соотношение и только края ленты.
			rs, ats := ratios, []int{0, 1, 2, k / 2, k - 3, k - 2, k - 1}
			if lite {
				rs = []float64{11, 20, 100}
				if k != 17 && k != 25 && k != 40 {
					rs, ats = []float64{20}, []int{0, k - 1}
				}
			}
			for _, at := range ats {
				for _, ratio := range rs {
					as := same(k, aspP)
					if k%2 == 1 {
						as = mixed(k, uint64(k))
					}
					as[at] = ratio
					for _, g := range geoms[:3] {
						checkGen(t, fmt.Sprintf("%s/%v:1 в %d", g.name, ratio, at), as, g.p)
					}
				}
			}
		})
	}
}

// checkGen — все раскладки Gen проходят checkCand, Build — не больше buildLimit.
func checkGen(t *testing.T, name string, as []float64, p layout.Params) {
	t.Helper()
	g := newGen(as, p, 0)
	g.run()
	for _, c := range g.out {
		checkCand(t, name, as, p, c)
	}
	if lim := buildLimit(len(as)); g.builds > lim {
		t.Errorf("%s: Build %d > %d", name, g.builds, lim)
	}
}
