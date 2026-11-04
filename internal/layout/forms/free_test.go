package forms

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// freeSets — наборы Free (3–5 фото): одинаковые и смесь.
func freeSets() map[string][]float64 {
	return map[string][]float64{
		"гориз3": same(3, aspL), "гориз4": same(4, aspL), "гориз5": same(5, aspL),
		"верт3": same(3, aspP), "верт4": same(4, aspP), "верт5": same(5, aspP),
		"смесь3": mixed(3, 3), "смесь4": mixed(4, 4), "смесь5": mixed(5, 5),
		"лвл5": {aspL, aspP, aspL, aspP, aspL},
	}
}

func scatterTiles(cs []Cand) []Cand {
	var out []Cand
	for _, c := range cs {
		if c.Form == Scatter {
			out = append(out, c)
		}
	}
	return out
}

func anyFold(p layout.Params) layout.Params { p.FoldAny = true; return p }

// TestFreeTiles: у 3–5 фото в «Любые фото» — центр и по плитке на 7 профилей в порядке таблицы;
// каждая строится, проходит сгиб и пол вида, а геометрия совпадает с Build ключа.
func TestFreeTiles(t *testing.T) {
	for _, n := range []int{3, 4, 5} {
		as := same(n, aspP)
		p := anyFold(geomSq)
		cs := Free(as, p)
		sc := scatterTiles(cs)
		if len(sc) != 7 || len(cs) > 8 {
			t.Fatalf("%d фото: плиток %d, из них «Разброс» %d, ждали 7 и центр", n, len(cs), len(sc))
		}
		for i, c := range sc {
			lv := freeProfiles[i][n-freeMin]
			if !strings.HasPrefix(c.Key, "Z("+lv[:1]+"0") {
				t.Errorf("%d фото: плитка %d — %q, профиль %q", n, i, c.Key, lv)
			}
			r, ok := layout.Build(c.Key, as, p)
			if !ok || len(r.Rects) != n || !layout.FoldOK(r.Rects, nil, p) {
				t.Errorf("%d фото: %q не строится или не проходит сгиб", n, c.Key)
			}
			if _, ok := Quality(r, Scatter, p); !ok {
				t.Errorf("%d фото: %q не проходит пол", n, c.Key)
			}
			if Of(c.Key, c.Rects, p) != Scatter {
				t.Errorf("%d фото: Of(%q) не «Разброс»", n, c.Key)
			}
		}
	}
}

// TestFreeCenter: центр с боками n — первая плитка, крупное фото в середине; у 4 фото — одно из двух
// центральных; пропорции без равенства центру — центра нет.
func TestFreeCenter(t *testing.T) {
	for n, want := range map[int]string{3: "H(n0,1,n2)", 5: "H(n0,n1,2,n3,n4)"} {
		cs := Free(same(n, aspP), anyFold(geomSq))
		if len(cs) == 0 || cs[0].Key != want || cs[0].Form != Contrast || !CenterBig(cs[0].Key) {
			t.Errorf("%d фото: первая плитка %+v, ждали центр %q", n, cs[:min(1, len(cs))], want)
		}
	}
	cs := Free(same(4, aspP), anyFold(geomSq))
	if len(cs) == 0 || cs[0].Form != Contrast || !CenterBig(cs[0].Key) {
		t.Errorf("4 фото: нет центра первой плиткой: %v", cs)
	}
	for _, c := range Free([]float64{aspL, aspP, aspL}, anyFold(geomSq)) {
		if c.Form == Contrast {
			t.Errorf("центр уже боков не бывает: %q", c.Key)
		}
	}
}

// TestFreeFoldWide: «Только широкие» (умолчание) и 3 вертикальных — среднее фото на сгибе, на разворот
// ничего не строится: все 7 профилей идут по страницам (2 + 1), центра нет; скрыто 8 раскладок (7 Z и
// центр). У 4 фото сгиб в промежутке — Z на разворот.
func TestFreeFoldWide(t *testing.T) {
	cs, hidden := FreeFold(same(3, aspP), geomSq)
	sc := scatterTiles(cs)
	if len(sc) != 7 || len(cs) != 7 || hidden != 8 {
		t.Fatalf("плиток %d (разброс %d), скрыто %d; ждали 7, 7 и 8", len(cs), len(sc), hidden)
	}
	for _, c := range sc {
		if !strings.HasPrefix(c.Key, "P[Z(") || !layout.FoldOK(c.Rects, nil, geomSq) {
			t.Errorf("%q — не страницы или фото на сгибе", c.Key)
		}
	}
	// «Любые фото» — на разворот, ничего не скрыто
	cs, hidden = FreeFold(same(3, aspP), anyFold(geomSq))
	if hidden != 0 || !strings.HasPrefix(scatterTiles(cs)[0].Key, "Z(") {
		t.Errorf("любые фото: скрыто %d, первая %q", hidden, scatterTiles(cs)[0].Key)
	}
	// 4 фото: все 7 Z — на разворот; скрыты оба центра (крупное фото центра ложится на сгиб)
	cs, hidden = FreeFold(same(4, aspP), geomSq)
	sc = scatterTiles(cs)
	if hidden != 2 || len(sc) != 7 || len(cs) != 7 {
		t.Errorf("4 фото: плиток %d, скрыто %d; ждали 7 и 2", len(cs), hidden)
	}
	for _, c := range sc {
		if !strings.HasPrefix(c.Key, "Z(") {
			t.Errorf("4 фото: %q не на разворот", c.Key)
		}
	}
}

// TestFreeNoLayflat: без layflat — только по страницам, центра нет; у листа — только на разворот.
func TestFreeNoLayflat(t *testing.T) {
	for n := 3; n <= 5; n++ {
		cs, hidden := FreeFold(same(n, aspP), geomFlat)
		if len(cs) == 0 || hidden != 0 {
			t.Fatalf("%d фото без layflat: плиток %d, скрыто %d", n, len(cs), hidden)
		}
		for _, c := range cs {
			if !strings.HasPrefix(c.Key, "P[Z(") {
				t.Errorf("%d фото без layflat: %q", n, c.Key)
			}
		}
		for _, c := range Free(same(n, aspL), geomSh) {
			if strings.Contains(c.Key, "P[") {
				t.Errorf("%d фото, лист: %q по страницам", n, c.Key)
			}
		}
	}
}

// TestFreeEdges: 0–2 и 6+ фото, нулевая геометрия, мусорные пропорции — пусто.
func TestFreeEdges(t *testing.T) {
	for _, as := range [][]float64{nil, {}, {aspL}, same(2, aspL), same(6, aspL), same(40, aspP),
		{aspL, 0, aspL}, {aspL, math.NaN(), aspL}, {aspL, math.Inf(1), aspL}, {aspL, -1, aspL}} {
		if cs, h := FreeFold(as, geomSq); len(cs) != 0 || h != 0 {
			t.Errorf("%v: ждали пусто, получили %d плиток, скрыто %d", as, len(cs), h)
		}
	}
	if cs := Free(same(3, aspP), layout.Params{}); len(cs) != 0 {
		t.Errorf("нулевая геометрия: %d плиток", len(cs))
	}
}

// TestFreeBudget: Build на разворот не больше 24 (design §4.2) при любом наборе и геометрии.
func TestFreeBudget(t *testing.T) {
	for _, g := range append(append([]namedGeom{}, geoms...), geomsAny()...) {
		for name, as := range freeSets() {
			gn := newGen(as, g.p, 0)
			gn.free()
			if gn.builds > 24 {
				t.Errorf("%s/%s: Build %d > 24", g.name, name, gn.builds)
			}
		}
	}
}

// TestFreeNotInGen: Gen «Разброса» и центра n не знает (эталоны forms*.sha, ряд «Авто»), а Free не
// повторяет ключей и всегда отдаёт Scatter или Contrast.
func TestFreeNotInGen(t *testing.T) {
	for _, g := range append(append([]namedGeom{}, geoms...), geomsAny()...) {
		for name, as := range freeSets() {
			for _, c := range Gen(as, g.p) {
				if strings.Contains(c.Key, "Z(") || strings.Contains(c.Key, "n") {
					t.Errorf("%s/%s: Gen дал %q", g.name, name, c.Key)
				}
			}
			seen := map[string]bool{}
			for _, c := range Free(as, g.p) {
				if seen[c.Key] || c.Form != Scatter && c.Form != Contrast {
					t.Errorf("%s/%s: %q (%s) повторён или чужой вид", g.name, name, c.Key, c.Form)
				}
				seen[c.Key] = true
			}
		}
	}
}

// TestFreeDeterministic: тот же вход — те же ключи в том же порядке; шум 1e-12 (как FMA) их не меняет.
func TestFreeDeterministic(t *testing.T) {
	for name, as := range freeSets() {
		want := Free(as, geomSq)
		noisy := make([]float64, len(as))
		for i, a := range as {
			noisy[i] = a * (1 + 1e-12*float64(i+1))
		}
		got := Free(noisy, geomSq)
		if len(got) != len(want) {
			t.Fatalf("%s: с шумом %d плиток, без — %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i].Key != want[i].Key {
				t.Errorf("%s: плитка %d — %q, без шума %q", name, i, got[i].Key, want[i].Key)
			}
		}
	}
}

// TestScatterQuality: «Разброс» засчитан заполненным (воздух — смысл вида): оценка конечная и та же,
// что в Cand; глубина Z — 1, как у лесенки.
func TestScatterQuality(t *testing.T) {
	p := anyFold(geomSq)
	for _, c := range scatterTiles(Free(same(3, aspP), p)) {
		q, ok := Quality(c.Result, Scatter, p)
		if !ok || math.IsNaN(q) || q != c.Q {
			t.Errorf("%q: Quality %v %v, в Cand %v", c.Key, q, ok, c.Q)
		}
		if depthOf(c.Key) != 1 {
			t.Errorf("%q: глубина %d", c.Key, depthOf(c.Key))
		}
	}
}

// TestFreeFoldRealAspects: реальные пропорции стока (1331×2000, 1333×2000, 1708×2000, QA 23.10):
// число — раскладки, отказанные только сгибом, а не «всегда 8». Почти 2:3 дают 8 / 2 / 8; если среди
// вертикальных есть шире остальных (0,854), центра нет (его фото не равно самому широкому), а сгиб
// ложится в фото у всех семи Z — на 4 и 5 фото скрыто 7.
func TestFreeFoldRealAspects(t *testing.T) {
	const a, b, w = 1331.0 / 2000, 1333.0 / 2000, 1708.0 / 2000
	for _, c := range []struct {
		name string
		as   []float64
		want int
	}{
		{"3 почти 2:3", []float64{a, b, b}, 8},
		{"4 почти 2:3", []float64{a, b, b, b}, 2},
		{"5 почти 2:3", []float64{a, b, b, b, b}, 8},
		{"4 с широким 0,854", []float64{a, b, b, w}, 7},
		{"5 с широким 0,854", []float64{a, b, b, w, b}, 7},
	} {
		if _, hidden := FreeFold(c.as, geomSq); hidden != c.want {
			t.Errorf("%s: скрыто %d, ждали %d", c.name, hidden, c.want)
		}
		if _, h := FreeFold(c.as, anyFold(geomSq)); h != 0 {
			t.Errorf("%s, любые фото: скрыто %d, ждали 0", c.name, h)
		}
	}
}
