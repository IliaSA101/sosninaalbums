package forms

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Наборы и геометрии тестов: пропорции детерминированы своим генератором, не зависят от версии Go.

var (
	geomSq   = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}
	geomWide = layout.Params{PageW: 300, PageH: 200, Margin: 12, Gap: 3, Layflat: true}
	geomFlat = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: false}
	geomSh   = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, Sheet: true}
	geoms    = []namedGeom{{"30x30", geomSq}, {"30x20", geomWide}, {"30x30-flat", geomFlat}, {"30x30-sheet", geomSh}}
)

// namedGeom — геометрия теста с именем для сообщений и строк эталона.
type namedGeom struct {
	name string
	p    layout.Params
}

const (
	aspL = 1.5       // горизонтальное 3:2
	aspP = 2.0 / 3.0 // вертикальное 2:3
)

type rng uint64

func (s *rng) unit() float64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return float64((z^z>>31)>>11) / (1 << 53)
}

func same(n int, a float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = a
	}
	return out
}

// mixed — смесь 62 % горизонтальных, 33 % вертикальных, 5 % квадратов.
func mixed(n int, seed uint64) []float64 {
	r := rng(seed)
	out := make([]float64, n)
	for i := range out {
		switch x := r.unit(); {
		case x < 0.62:
			out[i] = aspL
		case x < 0.95:
			out[i] = aspP
		default:
			out[i] = 1
		}
	}
	return out
}

// series — серии: ориентация держится с вероятностью 0.6, горизонтальных долей 0.65.
func series(n int, seed uint64) []float64 {
	r := rng(seed)
	out := make([]float64, n)
	cur := aspL
	for i := range out {
		if r.unit() > 0.6 {
			cur = aspP
			if r.unit() < 0.65 {
				cur = aspL
			}
		}
		out[i] = cur
	}
	return out
}

// sets — наборы инвариантов: по размерам и составу (одинаковые, смесь, серии).
func sets() map[string][]float64 {
	out := map[string][]float64{}
	for _, n := range []int{2, 3, 5, 7, 10, 16, 25, 40} {
		out[fmt.Sprintf("гориз%d", n)] = same(n, aspL)
		out[fmt.Sprintf("смесь%d", n)] = mixed(n, uint64(n))
	}
	for _, n := range []int{4, 8, 12, 20} {
		out[fmt.Sprintf("верт%d", n)] = same(n, aspP)
		out[fmt.Sprintf("серия%d", n)] = series(n, uint64(n))
	}
	return out
}

func sortedNames(m map[string][]float64) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// checkCand — независимая проверка раскладки генератора (layout не трогаем): Build даёт то же,
// вид и оценка пересчитываются, в порядке ленты, внутри разворота и полей, пропорции точны (0.1 %),
// без пересечений, без layflat не через корешок; у листа и без layflat нет запрещённых видов ключа.
func checkCand(t *testing.T, name string, as []float64, p layout.Params, c Cand) {
	t.Helper()
	r, ok := layout.Build(c.Key, as, p)
	if !ok || !slices.Equal(r.Rects, c.Rects) || r.Score != c.Score {
		t.Fatalf("%s: %q — Build отказал или дал другое", name, c.Key)
	}
	if layout.IsRearranged(c.Key) {
		t.Fatalf("%s: %q — не в порядке ленты", name, c.Key)
	}
	if f := Of(c.Key, c.Rects, p); f != c.Form {
		t.Fatalf("%s: %q — вид %s, Of даёт %s", name, c.Key, c.Form, f)
	}
	if q, ok := Quality(c.Result, c.Form, p); !ok || q != c.Q {
		t.Fatalf("%s: %q — Quality %v/%v, в кандидате %v", name, c.Key, q, ok, c.Q)
	}
	pagedKey := strings.HasPrefix(c.Key, "P[") || strings.HasPrefix(c.Key, "Q")
	if p.Sheet && pagedKey || !p.Layflat && !pagedKey {
		t.Fatalf("%s: %q — вид ключа не для этой геометрии (лист %v, layflat %v)", name, c.Key, p.Sheet, p.Layflat)
	}
	const eps = 1e-6
	for i, x := range c.Rects {
		if x.X < p.Margin-eps || x.Y < p.Margin-eps || x.X+x.W > 2*p.PageW-p.Margin+eps || x.Y+x.H > p.PageH-p.Margin+eps {
			t.Fatalf("%s: %q — фото %d вне полей", name, c.Key, i)
		}
		if a := x.W / x.H; math.Abs(a-as[i])/as[i] > 0.001 {
			t.Fatalf("%s: %q — фото %d искажено: %.4f вместо %.4f", name, c.Key, i, a, as[i])
		}
		if !p.Layflat && x.X < p.PageW-foldEps && x.X+x.W > p.PageW+foldEps {
			t.Fatalf("%s: %q — без layflat фото %d через корешок", name, c.Key, i)
		}
		for j := range i {
			y := c.Rects[j]
			if x.X+eps < y.X+y.W && y.X+eps < x.X+x.W && x.Y+eps < y.Y+y.H && y.Y+eps < x.Y+x.H {
				t.Fatalf("%s: %q — фото %d и %d пересекаются", name, c.Key, i, j)
			}
		}
	}
}

// TestFormsInvariants: все ключи Gen на наборах 2–40 фото × 4 геометрии (в т. ч. лист и без
// layflat) строятся, повторяют Build (круг ключ → раскладка), проходят инварианты и не повторяют
// ключ; на каждой геометрии что-то есть; бюджет Build на разворот — buildLimit.
func TestFormsInvariants(t *testing.T) {
	all := sets()
	for _, g := range geoms {
		total := 0
		for _, name := range sortedNames(all) {
			as := all[name]
			cs := Gen(as, g.p)
			seen := map[string]bool{}
			for _, c := range cs {
				checkCand(t, g.name+"/"+name, as, g.p, c)
				if seen[c.Key] {
					t.Fatalf("%s/%s: ключ %q дважды", g.name, name, c.Key)
				}
				seen[c.Key] = true
			}
			total += len(cs)
			if b, lim := builds(as, g.p), buildLimit(len(as)); b > lim {
				t.Errorf("%s/%s: Build %d > %d", g.name, name, b, lim)
			}
		}
		if total == 0 {
			t.Errorf("%s: генератор ничего не дал", g.name)
		}
	}
}

// buildLimit — бюджет Build на разворот: 80 до 16 фото, 200 с ManyFrom (генераторы многих фото 072).
func buildLimit(k int) int {
	if k >= layout.ManyFrom {
		return 200
	}
	return 80
}

// builds — сколько раз генератор вызывает Build на наборе.
func builds(as []float64, p layout.Params) int {
	g := newGen(as, p, 0)
	g.run()
	return g.builds
}

// catalogRange — на каких фото на развороте вид обязан быть хотя бы в одном наборе: диапазон
// design §5 за вычетом того, что не выдерживают полы (на 36 фото ряды, на 16 — рамка).
var catalogRange = map[Form][2]int{
	Strip: {3, 10}, Rows: {6, 35}, Grid: {4, 30}, Air: {2, 16}, Asym: {5, 40},
	Contrast: {3, 7}, Ladder: {3, 5}, Frame: {7, 15},
}

// TestFormsCatalog: каждая форма появляется на каждом k своего диапазона — на каком-нибудь из
// наборов (одинаковые горизонтальные и вертикальные, смеси, серии) при layflat; и ни одна форма
// не выходит за свой диапазон k (строит только свой генератор: проверка Of по ключу).
func TestFormsCatalog(t *testing.T) {
	seen := map[Form]map[int]bool{}
	for n := 2; n <= 40; n++ {
		variants := [][]float64{same(n, aspL), same(n, aspP)}
		for s := uint64(1); s <= 4; s++ {
			variants = append(variants, mixed(n, s), series(n, s))
		}
		for _, g := range geoms[:3] {
			for _, as := range variants {
				for _, c := range Gen(as, g.p) {
					if seen[c.Form] == nil {
						seen[c.Form] = map[int]bool{}
					}
					seen[c.Form][n] = true
				}
			}
		}
	}
	for f, r := range catalogRange {
		for n := r[0]; n <= r[1]; n++ {
			if !seen[f][n] {
				t.Errorf("форма %s не появилась на %d фото", f, n)
			}
		}
	}
	// Формы, которых генератор не строит, в его выдаче не бывает с таким названием «из воздуха»:
	// мозаика допустима (H(…) из одного ряда без воздуха), остальное — свой генератор.
	for f := range seen {
		if _, ok := catalogRange[f]; !ok && f != Mosaic && f != Hero && f != Pair && f != Spine {
			t.Errorf("неожиданная форма %s в выдаче Gen", f)
		}
	}
}

// TestGenEdges: пустой и плохой вход — пусто без паники; край диапазона (2 и 64 фото); крошечная
// страница и поля шире страницы; лист и без layflat — только допустимые виды ключа.
func TestGenEdges(t *testing.T) {
	bad := func(a float64) []float64 { return append(same(5, aspL), a) }
	for name, c := range map[string]struct {
		as []float64
		p  layout.Params
	}{
		"nil":            {nil, geomSq},
		"одно фото":      {same(1, aspL), geomSq},
		"65 фото":        {same(layout.MaxPhotos+1, aspL), geomSq},
		"NaN":            {bad(math.NaN()), geomSq},
		"Inf":            {bad(math.Inf(1)), geomSq},
		"ноль":           {bad(0), geomSq},
		"минус":          {bad(-1), geomSq},
		"страница 0":     {same(6, aspL), layout.Params{PageH: 300, Margin: 12, Layflat: true}},
		"высота 0":       {same(6, aspL), layout.Params{PageW: 300, Margin: 12, Layflat: true}},
		"поля шире":      {same(6, aspL), layout.Params{PageW: 20, PageH: 20, Margin: 12, Gap: 3, Layflat: true}},
		"нулевые поля":   {same(6, aspL), layout.Params{PageW: 300, PageH: 300, Layflat: true}},
		"узкие полоски":  {same(8, 50), geomSq},
		"высокие полосы": {same(8, 0.02), geomSq},
	} {
		for _, x := range Gen(c.as, c.p) {
			checkCand(t, name, c.as, c.p, x)
		}
		if len(c.as) < 2 || len(c.as) > layout.MaxPhotos || slices.ContainsFunc(c.as, func(a float64) bool { return !(a > 0) || math.IsInf(a, 0) }) || !(c.p.PageW > 0) || !(c.p.PageH > 0) {
			if cs := Gen(c.as, c.p); len(cs) != 0 {
				t.Errorf("%s: ждали пусто, получили %d", name, len(cs))
			}
		}
	}
	for _, n := range []int{2, layout.MaxPhotos} {
		for _, g := range geoms {
			for _, c := range Gen(same(n, aspL), g.p) {
				checkCand(t, fmt.Sprintf("%s/%d", g.name, n), same(n, aspL), g.p, c)
			}
		}
	}
	if len(Gen(same(2, aspL), geomSq)) == 0 {
		t.Error("2 фото: ни одной формы")
	}
}

// TestGenDeterministic: тот же вход — те же ключи в том же порядке; шум 1e-12 в пропорциях (как
// FMA на arm64) не меняет ключи (ничьи — в квантах). Кроме sets — многие фото 072 (разрез DP, планы
// главного): серии и stock на 20, 25, 30.
func TestGenDeterministic(t *testing.T) {
	keys := func(as []float64, p layout.Params) []string {
		var out []string
		for _, c := range Gen(as, p) {
			out = append(out, c.Key)
		}
		return out
	}
	r := rng(41)
	all := sets()
	st := testkit.StockAspects(t)
	for _, n := range []int{20, 25, 30} {
		all[fmt.Sprintf("серия%d", n)] = series(n, uint64(n))
		all[fmt.Sprintf("серия%d+", n)] = series(n, uint64(n+7))
		all[fmt.Sprintf("stock%d", n)] = st[:n]
		all[fmt.Sprintf("stock30+%d", n)] = st[30 : 30+n]
	}
	for _, g := range geoms[:3] {
		for _, name := range sortedNames(all) {
			as := all[name]
			first := keys(as, g.p)
			if !slices.Equal(keys(as, g.p), first) {
				t.Fatalf("%s/%s: генератор недетерминирован", g.name, name)
			}
			noisy := slices.Clone(as)
			for i := range noisy {
				noisy[i] *= 1 + (r.unit()*2-1)*1e-12
			}
			if !slices.Equal(keys(noisy, g.p), first) {
				t.Errorf("%s/%s: шум 1e-12 поменял ключи", g.name, name)
			}
		}
	}
}

// TestGenDoesNotChangeInput: Gen не меняет срез пропорций вызывающего.
func TestGenDoesNotChangeInput(t *testing.T) {
	as := mixed(12, 5)
	before := slices.Clone(as)
	Gen(as, geomSq)
	if !slices.Equal(as, before) {
		t.Fatal("Gen изменил входной срез")
	}
}

// TestGenSpeed: холодный Gen на 16 и 40 фото — страж от взрыва (цель design §12: ≤ 5 / ≤ 10 мс
// у BenchmarkForms16/40; в тесте запас на шум и занятую машину).
func TestGenSpeed(t *testing.T) {
	testkit.CostTest(t)
	for _, c := range []struct {
		n   int
		lim time.Duration
	}{{16, 40 * time.Millisecond}, {40, 80 * time.Millisecond}} {
		as := mixed(c.n, 3)
		d := testkit.Fastest(5, func() { Gen(as, geomSq) })
		t.Logf("Gen %d: %v", c.n, d)
		if lim := testkit.Limit(c.lim); d > lim {
			t.Fatalf("Gen на %d фото %v > %v", c.n, d, lim)
		}
	}
}

// BenchmarkForms16 — холодный Gen на 16 фото (design §12: ≤ 5 мс).
func BenchmarkForms16(b *testing.B) {
	as := mixed(16, 3)
	for b.Loop() {
		Gen(as, geomSq)
	}
}

// BenchmarkForms40 — холодный Gen на 40 фото (design §12: ≤ 10 мс).
func BenchmarkForms40(b *testing.B) {
	as := mixed(40, 3)
	for b.Loop() {
		Gen(as, geomSq)
	}
}
