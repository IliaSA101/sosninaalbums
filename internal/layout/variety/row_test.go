package variety

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ряд «Авто» по формам и выбор форм (051 G6, design §6–7).

var geomSheet = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, Sheet: true}

// rowSets — наборы 1–40 фото: одинаковые, серии, смесь.
func rowSets() map[string][]float64 {
	out := map[string][]float64{}
	for _, n := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12, 16, 17, 20, 25, 32, 40} {
		out[fmt.Sprintf("гориз%d", n)] = sameAspects(n, aspL)
		out[fmt.Sprintf("серия%d", n)] = seriesAspects(n, 0.65, uint64(n))
		out[fmt.Sprintf("смесь%d", n)] = mixedAspects(n, uint64(n))
	}
	return out
}

// TestRowFirstIsBest: Row[0] — лучшая раскладка layout (Layout "" и «Меньше» не меняются), ряд не
// длиннее rowLen, ключи без повторов, каждый строится Build с теми же Rects; формы вставлены с Q не
// хуже Q[0] − rowQSlack и каждая — впервые в ряду; ряд layout не теряет порядка.
func TestRowFirstIsBest(t *testing.T) {
	for _, p := range []layout.Params{geomSq, geomWide, geomFlat, geomSheet} {
		for name, as := range rowSets() {
			n := rowLen(len(as))
			base := layout.Cached(as, p, n)
			e := rowOf(as, p, n)
			if len(base) == 0 {
				if len(e.row) != 0 {
					t.Fatalf("%s: ряд layout пуст, а Row — %d", name, len(e.row))
				}
				continue
			}
			if e.row[0].Key != base[0].Key || len(e.row) > n || len(e.res) != len(e.row) {
				t.Fatalf("%s layflat=%t: Row[0] %q, Cached[0] %q, длина %d/%d", name, p.Layflat, e.row[0].Key, base[0].Key, len(e.row), n)
			}
			checkRow(t, name, as, p, e, base)
		}
	}
}

func checkRow(t *testing.T, name string, as []float64, p layout.Params, e rowEntry, base []layout.Result) {
	t.Helper()
	keys, shown := map[string]bool{}, map[forms.Form]bool{}
	q0 := quanta(e.row[0].Q - rowQSlack)
	last := -1         // позиция последнего взятого из ряда layout: порядок ряда сохраняется
	var ladders []byte // направления лесенок ряда
	for i, c := range e.row {
		if keys[c.Key] {
			t.Fatalf("%s: повтор ключа %q", name, c.Key)
		}
		keys[c.Key] = true
		r, ok := layout.Build(c.Key, as, p)
		if !ok || !slices.Equal(r.Rects, c.Rects) || r.Score != c.Score || e.res[i].Key != c.Key {
			t.Fatalf("%s: %q не строится или Build даёт другое", name, c.Key)
		}
		if f := forms.Of(c.Key, c.Rects, p); f != c.Form {
			t.Fatalf("%s: %q форма %s, Of — %s", name, c.Key, c.Form, f)
		}
		if e.gen[c.Key] {
			again := shown[c.Form]
			if c.Form == forms.Ladder {
				// 055: вторая лесенка — разрешённый повтор вида: ровно одна и другого направления
				d := forms.LadderDir(c.Key)
				if again && (len(ladders) != 1 || ladders[0] == d) {
					t.Fatalf("%s: лесенка %q — третья или того же направления, что %q", name, c.Key, ladders)
				}
				again, ladders = false, append(ladders, d)
			}
			if again || quanta(c.Q) < q0 || c.Form == forms.Mosaic {
				t.Fatalf("%s: форма %s (%q, Q %.3f) повтор или ниже допуска", name, c.Form, c.Key, c.Q)
			}
		} else {
			j := slices.IndexFunc(base, func(r layout.Result) bool { return r.Key == c.Key })
			if j <= last {
				t.Fatalf("%s: %q из ряда layout не по порядку (%d после %d)", name, c.Key, j, last)
			}
			last = j
		}
		shown[c.Form] = true
	}
}

// TestRowPrefix: с ManyFrom фото формы только в первой порции — Row(…, 24)[:12] == Row(…, 12)
// (как префиксное свойство ряда layout 027), и дальше порциями.
func TestRowPrefix(t *testing.T) {
	for _, n := range []int{17, 25, 40, 64} {
		for _, as := range [][]float64{mixedAspects(n, 7), sameAspects(n, aspL)} {
			ResetCache()
			small := Row(as, geomSq, layout.ManyBatch)
			for _, m := range []int{2, 3, layout.ManyCap / layout.ManyBatch} {
				big := Row(as, geomSq, m*layout.ManyBatch)
				if len(big) < len(small) || !slices.EqualFunc(big[:len(small)], small, func(a, b forms.Cand) bool { return a.Key == b.Key }) {
					t.Fatalf("%d фото: Row(%d) не продолжает Row(%d)", n, m*layout.ManyBatch, layout.ManyBatch)
				}
				re := rowOf(as, geomSq, m*layout.ManyBatch)
				for i, c := range big {
					if re.gen[c.Key] && i >= layout.ManyBatch {
						t.Fatalf("%d фото: форма %q на %d — вне первой порции", n, c.Key, i)
					}
				}
				small = big
			}
		}
	}
}

// TestRowForms: в первых 8 плитках ряда «Авто» ≥ 4 форм (спека 051, сценарий 9) на 3, 5, 8, 12,
// 16 и 25 фото — если столько допустимо: форм в первых 8 ряда layout и среди кандидатов с Q в
// допуске ряда (ниже — у набора меньше форм, и тогда в первых 8 — все допустимые).
func TestRowForms(t *testing.T) {
	full := 0
	for _, p := range []layout.Params{geomSq, geomFlat} {
		for _, n := range []int{3, 5, 8, 12, 16, 25} {
			for _, as := range [][]float64{mixedAspects(n, uint64(n)), seriesAspects(n, 0.65, uint64(n)), sameAspects(n, aspL)} {
				row := Row(as, p, rowLen(n))
				seen := map[forms.Form]bool{}
				var names []string
				for _, c := range row[:min(8, len(row))] {
					seen[c.Form] = true
					names = append(names, string(c.Form))
				}
				want := min(4, availableForms(as, p))
				if len(seen) >= 4 {
					full++
				}
				if testing.Verbose() {
					t.Logf("%d фото layflat=%t: %s (допустимо %d)", n, p.Layflat, strings.Join(names, " "), want)
				}
				if len(seen) < want {
					t.Errorf("%d фото layflat=%t: в первых 8 форм %d из %d допустимых (%s)", n, p.Layflat, len(seen), want, strings.Join(names, " "))
				}
			}
		}
	}
	// Из 36 наборов (решение 0.13 №40: было 24): форм меньше только у 3 фото без layflat (лента,
	// мелкое — крупное, главное) и у серии 25 фото (мозаика крупнее любых рядов одной высоты).
	if full < 30 {
		t.Errorf("≥ 4 форм в первых 8 — только у %d наборов из 36", full)
	}
}

// availableForms — сколько разных форм может показать ряд: первые 8 ряда layout и кандидаты форм
// с Q не хуже Q лучшей − rowQSlack.
func availableForms(as []float64, p layout.Params) int {
	base := layout.Cached(as, p, rowLen(len(as)))
	if len(base) == 0 {
		return 0
	}
	seen := map[forms.Form]bool{}
	for _, r := range base[:min(8, len(base))] {
		seen[forms.Of(r.Key, r.Rects, p)] = true
	}
	floor := quanta(candOf(base[0], p).Q - rowQSlack)
	cands, _ := formCands(as, p, featured(as, p), formRelax(as, base[0]))
	for _, c := range cands {
		if quanta(c.Q) >= floor {
			seen[c.Form] = true
		}
	}
	return len(seen)
}

// TestRowEdges: пусто, одно фото, нулевая длина и битая геометрия — без паники, ряд пуст или
// из ряда layout; второй вызов — из кэша; ResetCache его чистит.
func TestRowEdges(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)
	if got := Row(nil, geomSq, 24); len(got) != 0 {
		t.Errorf("без фото — %d", len(got))
	}
	if got := Row(sameAspects(3, aspL), geomSq, 0); len(got) != 0 {
		t.Errorf("rowLen 0 — %d", len(got))
	}
	if got := Row(sameAspects(3, aspL), layout.Params{}, 24); len(got) != 0 {
		t.Errorf("битая геометрия — %d", len(got))
	}
	one := Row([]float64{aspL}, geomSq, 24)
	if len(one) == 0 || one[0].Key != layout.Cached([]float64{aspL}, geomSq, 24)[0].Key {
		t.Fatalf("одно фото: %v", one)
	}
	as := mixedAspects(9, 3)
	if RowCached(as, geomSq, 24) {
		t.Fatal("заготовка: ряд уже в кэше")
	}
	first := Row(as, geomSq, 24)
	if !RowCached(as, geomSq, 24) || &Row(as, geomSq, 24)[0] != &first[0] {
		t.Error("второй вызов не из кэша")
	}
	ResetCache()
	if RowCached(as, geomSq, 24) {
		t.Error("ResetCache не очистил ряд")
	}
	if again := Row(as, geomSq, 24); !slices.EqualFunc(again, first, func(a, b forms.Cand) bool { return a.Key == b.Key }) {
		t.Error("холодный кэш дал другой ряд")
	}
}

// TestRowCacheGen: на extraGen записях кэш ряда начинается заново (память не растёт).
func TestRowCacheGen(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)
	old := sameAspects(4, aspL)
	Row(old, geomSq, 24)
	rowCache.Lock()
	for i := len(rowCache.m); i < extraGen; i++ {
		rowCache.m[fmt.Sprint(i)] = rowEntry{}
	}
	rowCache.Unlock()
	as := sameAspects(3, aspL)
	Row(as, geomSq, 24)
	rowCache.Lock()
	n := len(rowCache.m)
	rowCache.Unlock()
	if n != 1 || !RowCached(as, geomSq, 24) || RowCached(old, geomSq, 24) {
		t.Fatalf("после предела записей %d", n)
	}
}

// formStats — формы выбора по альбому: не мозаика, соседи одной формы, разных форм.
type formStats struct {
	spreads, nonMosaic, sameNeighbors, forms int
	mosaicRun                                int // самая длинная серия мозаик подряд
}

func formsOf(a corpusAlbum, picked []int) formStats {
	st := formStats{spreads: len(picked)}
	seen := map[forms.Form]bool{}
	var prev forms.Form
	run := 0
	for i, k := range picked {
		f := a.spreads[i].Pool[k].Form
		seen[f] = true
		if f == forms.Mosaic {
			run++
			st.mosaicRun = max(st.mosaicRun, run)
		} else {
			st.nonMosaic++
			run = 0
		}
		if i > 0 && f == prev {
			st.sameNeighbors++
		}
		prev = f
	}
	st.forms = len(seen)
	return st
}

// stockAlbum — stock 100 на 11 разворотов, «Авто» (plan.AutoPer), пулы как у программы.
func stockAlbum(t *testing.T, p layout.Params) (corpusAlbum, Ctx) {
	stock := testkit.StockAspects(t)
	lo, hi := plan.AutoPer(len(stock), 11)
	pl := plan.PlanRange(stock, p, plan.PlanOpts{Spreads: 11, MinPer: lo, MaxPer: hi})
	a := corpusAlbum{name: "stock 100/11", p: p}
	for _, g := range pl.Groups {
		as := stock[g[0]:g[1]]
		a.sizes, a.asps = append(a.sizes, len(as)), append(a.asps, as)
		a.spreads = append(a.spreads, Spread{Pool: corpusPool(as, p), Star: -1})
	}
	return a, Ctx{Avg: float64(len(stock)) / float64(len(pl.Groups))}
}

// TestPickFormsMore — критерий спеки 051: stock 100, «Больше» — ≥ 60 % разворотов не мозаика,
// соседей одной формы нет, форм ≥ 5; мозаик подряд не больше mosaicRun.
func TestPickFormsMore(t *testing.T) {
	for _, p := range []layout.Params{geomSq, geomFlat} {
		a, c := stockAlbum(t, p)
		st := formsOf(a, Pick(a.spreads, p, More, c))
		t.Logf("«Больше» layflat=%t, размеры %v: %+v", p.Layflat, a.sizes, st)
		if 10*st.nonMosaic < 6*st.spreads || st.sameNeighbors != 0 || st.forms < 5 || st.mosaicRun > tunings[More].mosaicRun {
			t.Errorf("«Больше» layflat=%t: %+v", p.Layflat, st)
		}
	}
}

// TestPickFormsNormal — «Обычное»: ≥ 35 % не мозаика, мозаик подряд не больше mosaicRun; «Меньше» —
// ровно лучшие (Pool[0]).
func TestPickFormsNormal(t *testing.T) {
	for _, p := range []layout.Params{geomSq, geomFlat} {
		a, c := stockAlbum(t, p)
		st := formsOf(a, Pick(a.spreads, p, Normal, c))
		t.Logf("«Обычное» layflat=%t: %+v", p.Layflat, st)
		if 100*st.nonMosaic < 35*st.spreads || st.mosaicRun > tunings[Normal].mosaicRun {
			t.Errorf("«Обычное» layflat=%t: %+v", p.Layflat, st)
		}
		if less := Pick(a.spreads, p, Less, c); slices.ContainsFunc(less, func(k int) bool { return k != 0 }) {
			t.Errorf("«Меньше» не лучшие: %v", less)
		}
	}
}

// TestPickMosaicCap: мозаик подряд — не больше предела, если в пуле есть допустимая форма; пул из
// одной мозаики выбирает её (разворот не остаётся без раскладки).
func TestPickMosaicCap(t *testing.T) {
	mos := cand("m", "m1", 1.0, true)
	mos.Form, mos.Q = forms.Mosaic, 0.9
	strip := cand("s", "s1", 0.5, true)
	strip.Formed, strip.Form, strip.Q = true, forms.Strip, 0.85
	pool := []Cand{mos, strip}
	for _, lv := range []Level{Normal, More} {
		run := tunings[lv].mosaicRun
		prev := slices.Repeat([]forms.Form{forms.Mosaic}, run)
		// Вид ленты как у обоих соседей (штраф 4 против ≤ 2.5 у мозаики): без предела — мозаика.
		near := Ctx{Prev: []string{"s1"}, Next: "s1"}
		if near.Forms = prev; Pick([]Spread{{Pool: pool, Star: -1}}, geomSq, lv, near)[0] != 1 {
			t.Errorf("уровень %d: после %d мозаик выбрана мозаика", lv, run)
		}
		if near.Forms = prev[1:]; run > 1 && Pick([]Spread{{Pool: pool, Star: -1}}, geomSq, lv, near)[0] != 0 {
			t.Errorf("уровень %d: до предела (%d мозаик) мозаика не взята", lv, run-1)
		}
		if got := Pick([]Spread{{Pool: pool[:1], Star: -1}}, geomSq, lv, Ctx{Forms: prev}); got[0] != 0 {
			t.Errorf("уровень %d: пул из одной мозаики — %d", lv, got[0])
		}
		weak := strip
		weak.Q = mos.Q - tunings[lv].deltaForm - 0.01 // форма вне допуска: мозаика остаётся
		if got := Pick([]Spread{{Pool: []Cand{mos, weak}, Star: -1}}, geomSq, lv, Ctx{Forms: prev}); got[0] != 0 {
			t.Errorf("уровень %d: взята форма вне δ_form", lv)
		}
	}
}

// TestPickFormNeighbors: форма соседа штрафуется — из двух равных форм берётся не та, что у
// предыдущего и у следующего; бонус «под размер» тянет редкой группе ленту, плотной — ряды.
func TestPickFormNeighbors(t *testing.T) {
	mk := func(key string, f forms.Form) Cand {
		c := cand(key, key, 0.5, true)
		c.Formed, c.Form, c.Q = true, f, 0.9
		return c
	}
	best := cand("best", "best", 1.0, true)
	best.Form, best.Q = forms.Mosaic, 0.9
	pool := []Cand{best, mk("a", forms.Strip), mk("b", forms.Rows)}
	prev := []string{"best"} // лучшая повторяет вид соседа: выбор — между формами
	for _, c := range []struct {
		name string
		ctx  Ctx
		want int
	}{
		{"сосед — лента", Ctx{Forms: []forms.Form{forms.Strip}}, 2},
		{"сосед — ряды", Ctx{Forms: []forms.Form{forms.Rows}}, 1},
		{"следующий — лента", Ctx{NextForm: forms.Strip}, 2},
		{"позапрошлый — ряды", Ctx{Forms: []forms.Form{forms.Rows, forms.Mosaic}}, 1},
		{"лента часто", Ctx{Forms: []forms.Form{forms.Strip, forms.Mosaic, forms.Strip, forms.Mosaic, forms.Grid}}, 2},
		{"редкая группа", Ctx{Avg: 20}, 1}, // 4 фото ≤ 0.7·20 — лента
		{"плотная группа", Ctx{Avg: 2}, 2}, // 4 фото ≥ 1.3·2 — ряды
	} {
		c.ctx.Prev = prev
		if got := Pick([]Spread{{Pool: pool, Star: -1}}, geomSq, More, c.ctx); got[0] != c.want {
			t.Errorf("%s: выбран %d (%q), ждали %d", c.name, got[0], pool[got[0]].Key, c.want)
		}
	}
}

// TestVarietyLessLinesSame: строки «Меньше» эталона variety.sha — ровно лучшие раскладки layout
// (Cached[0]), как до 051: смена выбора форм их не трогает (decisions 0.13 №14).
func TestVarietyLessLinesSame(t *testing.T) {
	goldenArch(t)
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for line := range strings.Lines(string(data)) {
		if name, sum, ok := strings.Cut(strings.TrimSpace(line), " "); ok && strings.HasSuffix(name, "/0") {
			want[name] = sum
		}
	}
	if len(want) != len(corpus()) {
		t.Fatalf("строк «Меньше» в эталоне %d, альбомов %d", len(want), len(corpus()))
	}
	for _, a := range corpus() {
		var keys []string
		for _, as := range a.asps {
			keys = append(keys, layout.Cached(as, a.p, rowLen(len(as)))[0].Key)
		}
		name := strings.ReplaceAll(a.name, " ", "_") + "/0"
		if got := keysHash(keys); got != want[name] {
			t.Errorf("%s: %s, эталон %s", name, got, want[name])
		}
	}
}

// BenchmarkRow16 — холодный ряд «Авто» на 16 фото при тёплом layout (design §12: ≤ 15 мс сверх
// Layouts).
func BenchmarkRow16(b *testing.B) {
	as := mixedAspects(16, 3)
	layout.Cached(as, geomSq, 24)
	for b.Loop() {
		ResetCache()
		Row(as, geomSq, 24)
	}
}

// TestRowSpeed — страж BenchmarkRow16: холодный ряд на 16 и 40 фото не взрывается.
func TestRowSpeed(t *testing.T) {
	t.Cleanup(ResetCache)
	for _, n := range []int{16, 40} {
		as := mixedAspects(n, 3)
		layout.Cached(as, geomSq, rowLen(n))
		d := testkit.Fastest(3, func() {
			ResetCache()
			Row(as, geomSq, rowLen(n))
		})
		t.Logf("Row %d фото: %v", n, d)
		if lim := testkit.Limit(60 * time.Millisecond); d > lim {
			t.Errorf("Row %d фото %v > %v", n, d, lim)
		}
	}
}
