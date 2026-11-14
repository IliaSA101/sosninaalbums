package layout

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тесты подрезки (спека 104, design §9). Геометрия «Фабрики»: страница 300×305.

// fab — альбом 300×305 «любые фото» с полем m и промежутком g.
func fab(m, g float64) Params {
	return Params{PageW: 300, PageH: 305, Margin: m, Gap: g, Layflat: true, FoldAny: true}
}

// cropRng — детерминированный источник для наборов (тот же splitmix, что в variety).
type cropRng uint64

func (s *cropRng) unit() float64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return float64((z^z>>31)>>11) / (1 << 53)
}

// cropMixed — смесь 62 % горизонтальных, 33 % вертикальных, 5 % квадратов.
func cropMixed(n int, seed uint64) []float64 {
	r := cropRng(seed)
	out := make([]float64, n)
	for i := range out {
		switch x := r.unit(); {
		case x < 0.62:
			out[i] = L
		case x < 0.95:
			out[i] = P
		default:
			out[i] = S
		}
	}
	return out
}

func cropSame(n int, a float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = a
	}
	return out
}

// cropSets — наборы 3–12 фото: все 2:3, все 3:2 и четыре смеси на каждое n.
func cropSets() (sets [][]float64) {
	for n := 3; n <= 12; n++ {
		sets = append(sets, cropSame(n, P), cropSame(n, L))
		for s := uint64(1); s <= 4; s++ {
			sets = append(sets, cropMixed(n, s*101+uint64(n)))
		}
	}
	return sets
}

var cropGeoms = []Params{album30, {PageW: 300, PageH: 200, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}, fab(12, 3), fab(3, 3)}

// frame — рамка всех прямоугольников.
func frame(rs []Rect) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range rs {
		x0, y0, x1, y1 = math.Min(x0, r.X), math.Min(y0, r.Y), math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
	}
	return
}

// closes — рамка фото совпадает с областью в полях разворота (±e мм).
func closes(rs []Rect, p Params, e float64) bool {
	x0, y0, x1, y1 := frame(rs)
	return x0 <= p.Margin+e && y0 <= p.Margin+e && x1 >= 2*p.PageW-p.Margin-e && y1 >= p.PageH-p.Margin-e
}

func TestCacheKeyCrop(t *testing.T) {
	asp := []float64{L, P}
	p := fab(3, 3)
	base := CacheKey(asp, p, 24)
	if want := "300/305/3/3/true/true/false/24:1.5,0.6666666666666666,"; base != want {
		t.Fatalf("ключ без подрезки изменился: %q, ждали %q", base, want)
	}
	// Lock без Crop не действует: ключ прежний. Вне предела и NaN — как 0 или как предел.
	for _, q := range []Params{{Lock: 5}, {Crop: math.NaN()}, {Crop: -1}, {Crop: 0, Lock: 1}} {
		pp := p
		pp.Crop, pp.Lock = q.Crop, q.Lock
		if got := CacheKey(asp, pp, 24); got != base {
			t.Errorf("Crop=%v Lock=%d: ключ %q, ждали прежний", q.Crop, q.Lock, got)
		}
	}
	seen := map[string]string{base: "0"}
	for name, q := range map[string]Params{
		"5": {Crop: 0.05}, "10": {Crop: 0.10}, "5 lock1": {Crop: 0.05, Lock: 1}, "5 lock2": {Crop: 0.05, Lock: 2},
	} {
		pp := p
		pp.Crop, pp.Lock = q.Crop, q.Lock
		k := CacheKey(asp, pp, 24)
		if other, dup := seen[k]; dup {
			t.Errorf("%s и %s делят ключ %q", name, other, k)
		}
		seen[k] = name
	}
	// 0.5 зажимается до предела: тот же результат — тот же ключ.
	a, b := p, p
	a.Crop, b.Crop = 0.5, MaxFitCrop
	if CacheKey(asp, a, 24) != CacheKey(asp, b, 24) {
		t.Errorf("Crop 0.5 и предел дают разные ключи")
	}
}

func TestFitCrop(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{0, 0}, {-0.1, 0}, {math.NaN(), 0}, {math.Inf(-1), 0}, {0.05, 0.05}, {MaxFitCrop, MaxFitCrop}, {0.3, MaxFitCrop}, {math.Inf(1), MaxFitCrop},
	} {
		if got := fitCrop(Params{Crop: c.in}); got != c.want {
			t.Errorf("fitCrop(%v) = %v, ждали %v", c.in, got, c.want)
		}
	}
}

// ownerKey — случай владельца: шесть вертикальных на «Фабрике» 300×305, поля 3/3.
const ownerKey = "H(0,V(1,2),3,V(4,5))"

func TestBuildCropCloses(t *testing.T) {
	asp := cropSame(6, P)
	p := fab(3, 3)
	p.Crop = 0.05
	r, ok := Build(ownerKey, asp, p)
	if !ok {
		t.Fatal("Build отказал")
	}
	if !closes(r.Rects, p, 0.01) {
		x0, y0, x1, y1 := frame(r.Rects)
		t.Errorf("рамка %.3f,%.3f–%.3f,%.3f не закрывает область в полях", x0, y0, x1, y1)
	}
	if math.Abs(r.Crop-0.018) > 0.001 {
		t.Errorf("подрезка %.4f, ждали 0.018", r.Crop)
	}
	for i, c := range r.Rects {
		if f := c.W / c.H / asp[i]; f < 1-p.Crop-1e-9 || f > 1/(1-p.Crop)+1e-9 {
			t.Errorf("фото %d: пропорции ×%.4f вне предела", i, f)
		}
	}
	k, _ := parseKey(ownerKey)
	g := newGeom(p)
	if want := score(r.Rects, g.spread, g.spine, k.tree.node(asp, p.Gap).depth) - CropK*r.Crop; r.Score != want {
		t.Errorf("оценка %v, ждали формулу %v", r.Score, want)
	}
	// Это лучшая раскладка ряда «Авто», без подрезки — та же, но с полосами.
	if row := Layouts(asp, p, 24); len(row) == 0 || row[0].Key != ownerKey || row[0].Crop <= 0 {
		t.Errorf("лучшая раскладка ряда: %+v", row[:min(1, len(row))])
	}
	// Подрезка не больше предела при любом N: чем меньше предел, тем меньше f.
	for _, c := range []float64{0.02, 0.10, 0.15, 0.3} {
		q := p
		q.Crop = c
		r, ok := Build(ownerKey, asp, q)
		if !ok || r.Crop > math.Min(c, MaxFitCrop)+1e-9 {
			t.Errorf("Crop %v: подрезка %v", c, r.Crop)
		}
	}
}

func rectsEqual(a, b []Rect) bool { return reflect.DeepEqual(a, b) }

func TestBuildCropLimit(t *testing.T) {
	asp := cropSame(6, P)
	p := fab(3, 3)
	p0 := p
	p.Crop = 0.01 // нужно 1,8 %: не хватает предела
	got, ok := Build(ownerKey, asp, p)
	want, ok0 := Build(ownerKey, asp, p0)
	if !ok || !ok0 {
		t.Fatal("Build отказал")
	}
	if !rectsEqual(got.Rects, want.Rects) || got.Score != want.Score || got.Crop != 0 {
		t.Errorf("не хватает предела: %+v, ждали прежний %+v с Crop = 0", got, want)
	}
	// Блок уже закрыт: одно фото точно по пропорциям области.
	one := []float64{594.0 / 299}
	for _, c := range []float64{0.05, 0.15} {
		q := fab(3, 3)
		q.Crop = c
		got, _ := Build("0", one, q)
		want, _ := Build("0", one, fab(3, 3))
		if !rectsEqual(got.Rects, want.Rects) || got.Score != want.Score || got.Crop != 0 {
			t.Errorf("закрытый блок, Crop %v: %+v, ждали %+v", c, got, want)
		}
	}
	// Края: предел 0, отрицательный и NaN — прежний результат.
	for _, c := range []float64{0, -0.5, math.NaN()} {
		q := fab(3, 3)
		q.Crop = c
		got, _ := Build(ownerKey, asp, q)
		if !rectsEqual(got.Rects, want.Rects) || got.Score != want.Score || got.Crop != 0 {
			t.Errorf("Crop %v: %+v, ждали прежний", c, got)
		}
	}
	// Ряд при том же пределе: каждая строка — Build(key), Crop не больше предела.
	for _, r := range Layouts(asp, p, 24) {
		if r.Crop > 0.01+1e-9 {
			t.Errorf("%s: подрезка %v выше предела 0.01", r.Key, r.Crop)
		}
	}
}

// kindCases — ключи, которые Build при подрезке не растягивает: воздух, G, L/A, m, Q, B, страницы с модификатором.
var kindCases = []struct{ name, key string }{
	{"S", "S(H(0,1,2))"}, {"U", "U(H(0,1,2,3))"}, {"D", "D(H(0,1,2,3,4))"},
	{"G", "G(H(0,1,2),H(3,4))"}, {"G+S", "S(G(H(0,1),H(2,3)))"},
	{"L", "L(0,1,2)"}, {"A", "A(0,1,2,3,4)"},
	{"m", "H(m0,1,m2)"}, {"m+V", "H(m0,V(1,2))"},
	{"Q", "Q[H(0,1)|H(2,3)]"}, {"QU", "QU[0|1]"}, {"Q+G", "QD[G(0,1)|L(2,3,4)]"},
	{"B full", "B[full:0]"}, {"B right", "B[right:0]"}, {"B pages", "B[pages:0,1]"},
	{"P S", "P[S(H(0,1))|S(H(2,3))]"}, {"P O+I", "P[O(H(0,1))|I(H(2,3))]"},
	{"P U+D", "P[U(H(0,1))|D(H(2,3))]"}, {"P E", "P[E(0)|S(H(1,2))]"},
	{"P G", "P[G(0,1)|G(2,3)]"}, {"P L+m", "P[L(0,1,2)|H(m3,4)]"},
}

// sameAsZero — Build при подрезке даёт то же, что без неё, и Crop = 0.
func sameAsZero(t *testing.T, name, key string, asp []float64) {
	t.Helper()
	p0 := fab(3, 3)
	for _, c := range []float64{0.05, 0.15} {
		p := p0
		p.Crop = c
		got, ok := Build(key, asp, p)
		want, ok0 := Build(key, asp, p0)
		if ok != ok0 {
			t.Errorf("%s %s: Build ok=%t при подрезке, %t без", name, key, ok, ok0)
			continue
		}
		if ok && (!rectsEqual(got.Rects, want.Rects) || got.Score != want.Score || got.Crop != 0) {
			t.Errorf("%s %s Crop %v: растянут, а не должен: %+v", name, key, c, got)
		}
	}
}

// forcedStretch — растянулся ли бы корень (страницы) ключа без защитных проверок Build.
func forcedStretch(k parsedKey, asp []float64) bool {
	p := fab(3, 3)
	p.Crop = MaxFitCrop
	g := newGeom(p)
	rects := make([]Rect, len(asp))
	if k.kind == 'T' {
		_, _, ok := g.stretchTo(k.tree, asp, g.spread, p, rects)
		return ok
	}
	for side, kt := range k.pages {
		if kt == nil {
			continue
		}
		if _, _, ok := g.stretchTo(kt, asp, g.pages[side], p, rects); ok {
			return true
		}
	}
	return false
}

func TestCropKinds(t *testing.T) {
	// Для каждого ключа перебираем одинаковые фото разных пропорций: на части из них корень
	// (или страница) без защитных проверок растянулся бы — их-то Build обязан не трогать.
	for _, c := range kindCases {
		k, ok := parseKey(c.key)
		if !ok {
			t.Fatalf("%s: ключ %q не разобран", c.name, c.key)
		}
		n := len(k.leaves())
		if k.kind == 'B' {
			n = len(k.idx)
		}
		forced := 0
		for a := 0.2; a < 5; a *= 1.04 {
			asp := cropSame(n, a)
			sameAsZero(t, c.name, c.key, asp)
			if k.kind != 'B' && forcedStretch(k, asp) {
				forced++
			}
		}
		if k.kind != 'B' && forced == 0 {
			t.Errorf("%s %s: ни при каких пропорциях блок не растягивается — случай не проверяет защиту", c.name, c.key)
		}
	}
	checkMixedPages(t)
	// Без layflat блока на весь разворот нет — отказ как при 0.
	flat := fab(3, 3)
	flat.Layflat, flat.Crop = false, 0.05
	if _, ok := Build(ownerKey, cropSame(6, P), flat); ok {
		t.Errorf("блок на весь разворот без layflat построен")
	}
}

// checkMixedPages — P с одной страницей без модификатора: растянута только она, соседняя — как при 0.
func checkMixedPages(t *testing.T) {
	t.Helper()
	p0 := fab(3, 3)
	asp := []float64{0.46343, 0.46343, 0.46343}
	key := "P[S(0)|H(1,2)]"
	p := p0
	p.Crop = 0.05
	got, ok := Build(key, asp, p)
	want, _ := Build(key, asp, p0)
	if !ok {
		t.Fatal("Build отказал")
	}
	if got.Rects[0] != want.Rects[0] {
		t.Errorf("страница с S(…) изменилась: %+v, было %+v", got.Rects[0], want.Rects[0])
	}
	if got.Crop <= 0 || got.Rects[1] == want.Rects[1] {
		t.Errorf("сплошная страница не растянута: crop %v, %+v", got.Crop, got.Rects[1:])
	}
	g := newGeom(p)
	if _, y0, _, y1 := frame(got.Rects[1:]); math.Abs(y0-g.pages[1].Y) > 0.01 || math.Abs(y1-g.pages[1].Y-g.pages[1].H) > 0.01 {
		t.Errorf("растянутая страница не закрыла высоту: %.3f–%.3f", y0, y1)
	}
}

// stretchable — ключи сплошных блоков корпуса (T без модификатора и P) с областями.
func stretchable(asp []float64, p Params, fn func(kt *keyTree, box Rect, g geom)) {
	g := newGeom(p)
	p0 := noCrop(p)
	for _, r := range Cached(asp, p0, 48) {
		k, ok := parseKey(r.Key)
		if !ok {
			continue
		}
		switch {
		case k.kind == 'T' && k.mod == 0 && plainTree(k.tree):
			fn(k.tree, g.spread, g)
		case k.kind == 'P':
			for side, kt := range k.pages {
				if kt != nil && k.mods[side] == 0 && plainTree(kt) {
					fn(kt, g.pages[side], g)
				}
			}
		}
	}
}

// near — прямоугольники совпадают с точностью e мм.
func rectsNear(x, y Rect, e float64) bool {
	return math.Abs(x.X-y.X) <= e && math.Abs(x.Y-y.Y) <= e && math.Abs(x.W-y.W) <= e && math.Abs(x.H-y.H) <= e
}

// compareFormula — формула и деление пополам дают одно решение; true — блок растянут.
func compareFormula(t *testing.T, kt *keyTree, asp []float64, box Rect, g geom, p Params) bool {
	t.Helper()
	a, b := make([]Rect, len(asp)), make([]Rect, len(asp))
	ca, da, oka := g.stretchWith(kt, asp, box, p, a, true)
	cb, db, okb := g.stretchWith(kt, asp, box, p, b, false)
	if oka != okb {
		// Допуск на границе предела (1e-12) может дать разное решение у самого края.
		if oka && ca < p.Crop-1e-9 || okb && cb < p.Crop-1e-9 {
			t.Errorf("%s: формула ok=%t, деление пополам ok=%t", kt, oka, okb)
		}
		return false
	}
	if !oka {
		return false
	}
	if da != db || math.Abs(ca-cb) > 1e-9 {
		t.Errorf("%s: подрезка %v/%v глубина %d/%d", kt, ca, cb, da, db)
	}
	// Остальные фото у P лежат в других ячейках: сверяем только свои листья.
	for _, l := range kt.leaves(nil) {
		if !rectsNear(a[l], b[l], 1e-9) {
			t.Errorf("%s фото %d: %+v против %+v", kt, l, a[l], b[l])
		}
	}
	return true
}

func TestCropFormula(t *testing.T) {
	sets := cropSets()
	if testkit.Lite() {
		sets = sets[:len(sets)/3]
	}
	var n, used int
	for _, g0 := range cropGeoms {
		for _, c := range []float64{0.05, 0.15} {
			p := g0
			p.Crop = c
			for _, asp := range sets {
				stretchable(asp, p, func(kt *keyTree, box Rect, g geom) {
					n++
					if compareFormula(t, kt, asp, box, g, p) {
						used++
					}
				})
			}
		}
	}
	if used < 100 {
		t.Errorf("растянуто всего %d блоков из %d: корпус не проверяет формулу", used, n)
	}
}

func TestCropKeepsOwnFrame(t *testing.T) {
	asp := cropSame(6, P)
	p := fab(3, 3)
	p.Crop = 0.05
	base, _ := Build(ownerKey, asp, p)

	p.Lock = 1 << 3 // у фото 3 свой кадр
	r, ok := Build(ownerKey, asp, p)
	if !ok {
		t.Fatal("Build отказал")
	}
	if got := r.Rects[3].W / r.Rects[3].H; math.Abs(got-asp[3]) > 1e-9 {
		t.Errorf("фото со своим кадром подрезано: %.6f вместо %.6f", got, asp[3])
	}
	if !closes(r.Rects, p, 0.01) {
		t.Errorf("лист не закрыт: %+v", r.Rects)
	}
	if r.Crop <= base.Crop+1e-6 || r.Crop > 0.05 {
		t.Errorf("остальные режутся на %.4f: ждали больше %.4f (без Lock) и не больше 0.05", r.Crop, base.Crop)
	}
	for i, c := range r.Rects {
		f := c.W / c.H / asp[i]
		if i != 3 && (f < 0.95-1e-9 || f > 1/0.95+1e-9) {
			t.Errorf("фото %d: ×%.4f вне предела", i, f)
		}
	}

	// Несколько фото с кадром сразу: все не режутся, остальные закрывают.
	p.Lock = 1<<0 | 1<<4
	r, ok = Build(ownerKey, asp, p)
	if !ok {
		t.Fatal("Build отказал")
	}
	for _, i := range []int{0, 4} {
		if got := r.Rects[i].W / r.Rects[i].H; math.Abs(got-asp[i]) > 1e-9 {
			t.Errorf("фото %d со своим кадром подрезано: %.6f", i, got)
		}
	}

	// У всех свой кадр — как при Crop = 0.
	p.Lock = 1<<6 - 1
	all, _ := Build(ownerKey, asp, p)
	p.Crop, p.Lock = 0, 0
	zero, _ := Build(ownerKey, asp, p)
	if !rectsEqual(all.Rects, zero.Rects) || all.Score != zero.Score || all.Crop != 0 {
		t.Errorf("все с кадром: %+v, ждали %+v", all, zero)
	}

	// Lock не действует без Crop; биты выше числа фото не мешают.
	q := fab(3, 3)
	q.Lock = 0xff
	if got, _ := Build(ownerKey, asp, q); !rectsEqual(got.Rects, zero.Rects) {
		t.Errorf("Lock без Crop изменил раскладку")
	}
	q.Crop, q.Lock = 0.05, 1<<40
	if got, _ := Build(ownerKey, asp, q); !rectsEqual(got.Rects, base.Rects) {
		t.Errorf("бит Lock вне числа фото изменил раскладку")
	}
}

// foldCases — T-ключи корпуса, которые при подрезке растянулись бы: считает тех, чьё растяжение
// нарушает правило сгиба (блок обязан остаться нерастянутым), и тех, чьё проходит (обязан растянуться).
func foldCases(t *testing.T, g0 Params) (violate, pass int) {
	t.Helper()
	p := g0
	p.Crop = 0.15
	g := newGeom(p)
	for _, asp := range cropSets() {
		for _, r := range Cached(asp, noCrop(p), 48) {
			k, _ := parseKey(r.Key)
			if k.kind != 'T' || k.mod != 0 || !plainTree(k.tree) {
				continue
			}
			rects := make([]Rect, len(asp))
			if _, _, ok := g.stretchTo(k.tree, asp, g.spread, p, rects); !ok {
				continue
			}
			got, ok := Build(r.Key, asp, p)
			if !ok {
				t.Fatalf("Build(%s) отказал", r.Key)
			}
			if FoldOK(rects, nil, p) {
				pass++
				if got.Crop == 0 {
					t.Errorf("%s: растяжение проходит сгиб, а блок не растянут", r.Key)
				}
				continue
			}
			violate++
			want, _ := Build(r.Key, asp, noCrop(p))
			if !rectsEqual(got.Rects, want.Rects) || got.Score != want.Score || got.Crop != 0 {
				t.Errorf("%s: растяжение нарушает сгиб, а блок изменён: %+v", r.Key, got)
			}
		}
	}
	return violate, pass
}

func TestCropFold(t *testing.T) {
	// «Только широкие» (FoldAny=false): растянутый блок, нарушивший правило сгиба, не растягивается.
	wide := fab(3, 3)
	wide.FoldAny = false
	found, stretched := 0, 0
	for _, g0 := range []Params{wide, {PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}} {
		v, ps := foldCases(t, g0)
		found += v
		stretched += ps
	}
	if found == 0 || stretched == 0 {
		t.Errorf("корпус не различает случаи: нарушили сгиб %d, прошли %d", found, stretched)
	}
	// Ряд «только широких» при подрезке держит правило сгиба на каждой строке.
	for _, asp := range cropSets() {
		p := wide
		p.Crop = 0.15
		for _, r := range Layouts(asp, p, 24) {
			if !FoldOK(r.Rects, asp, p) {
				t.Errorf("%v %s: фото на сгибе вне правила", asp, r.Key)
			}
		}
	}
}

// checkPrefix — ряды на 12, 24 и 48 при подрезке: каждый префикс следующего; строки — Build(p).
func checkPrefix(t *testing.T, asp []float64, p Params) {
	t.Helper()
	var rows [][]Result
	for _, lim := range []int{12, 24, 48} {
		rows = append(rows, Layouts(asp, p, lim))
	}
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]
		if len(prev) > len(rows[i]) {
			t.Fatalf("n=%d: ряд короче при большем limit", len(asp))
		}
		for j, r := range prev {
			if r.Key != rows[i][j].Key {
				t.Errorf("n=%d: ряд %d не префикс следующего на месте %d: %s / %s", len(asp), i, j, r.Key, rows[i][j].Key)
				break
			}
		}
	}
	for _, r := range rows[2] {
		b, ok := Build(r.Key, asp, p)
		if !ok || !rectsEqual(b.Rects, r.Rects) || b.Score != r.Score || b.Crop != r.Crop {
			t.Errorf("n=%d: %s — не Build(p)", len(asp), r.Key)
		}
	}
}

func TestLayoutsCropPrefix(t *testing.T) {
	sets := cropSets()
	for n := 17; n <= 64; n += 11 {
		sets = append(sets, cropSame(n, P), cropMixed(n, uint64(n)*7))
	}
	if testkit.Lite() {
		sets = sets[:12]
	}
	for _, g0 := range cropGeoms[:3] {
		p := g0
		p.Crop = 0.10
		for _, asp := range sets {
			checkPrefix(t, asp, p)
		}
	}
	// limit вне 1…48: набор всё равно целиком и без повторов.
	asp := cropSame(6, P)
	p := fab(3, 3)
	p.Crop = 0.05
	for _, lim := range []int{0, 49, 1000} {
		seen := map[string]bool{}
		for _, r := range Layouts(asp, p, lim) {
			if seen[r.Key] {
				t.Errorf("limit %d: ключ %s повторяется", lim, r.Key)
			}
			seen[r.Key] = true
		}
		if len(seen) == 0 {
			t.Errorf("limit %d: пусто", lim)
		}
	}
	if rs := Layouts(nil, p, 24); len(rs) != 0 {
		t.Errorf("пустой набор: %v", rs)
	}
}

// checkPageRows — ряд страницы при подрезке: порядок ключей как без неё, строки — Build(p).
func checkPageRows(t *testing.T, key, name string, asp []float64, p Params, plain, crop []Result) {
	t.Helper()
	if len(plain) != len(crop) {
		t.Errorf("%s %s: %d строк вместо %d", key, name, len(crop), len(plain))
		return
	}
	for i := range plain {
		if plain[i].Key != crop[i].Key {
			t.Errorf("%s %s: порядок ключей изменился на %d", key, name, i)
		}
		b, ok := Build(crop[i].Key, asp, p)
		if !ok || !rectsEqual(b.Rects, crop[i].Rects) || b.Score != crop[i].Score || b.Crop != crop[i].Crop {
			t.Errorf("%s %s %s: не Build(p)", key, name, crop[i].Key)
		}
	}
}

func TestPageChoicesCrop(t *testing.T) {
	for _, asp := range [][]float64{{P, P, P, P}, {L, P, L, P, P}, {P, P, P, P, P, P}} {
		p0 := fab(3, 3)
		p := p0
		p.Crop = 0.10
		for _, r := range Layouts(asp, p0, 24) {
			if !IsPaged(r.Key) {
				continue
			}
			for side := range 2 {
				in0, re0, blk0, ok0 := PageChoices(r.Key, asp, p0, side, 24)
				in, re, blk, ok := PageChoices(r.Key, asp, p, side, 24)
				if ok != ok0 || blk != blk0 {
					t.Fatalf("%s side %d: ok/block изменились", r.Key, side)
				}
				checkPageRows(t, r.Key, "inOrder", asp, p, in0, in)
				checkPageRows(t, r.Key, "rearranged", asp, p, re0, re)
				cached, _ := CachedPagesInOrder(r.Key, asp, p, side, 24)
				checkPageRows(t, r.Key, "cached", asp, p, in, cached)
			}
		}
	}
}

// checkCrop — инварианты ряда с подрезкой: всё в полях (вне «в край»), без пересечений,
// пропорции в пределе подрезки (с Lock — точные), число прямоугольников, сгиб.
func checkCrop(t *testing.T, asp []float64, p Params, r Result) {
	t.Helper()
	if len(r.Rects) != len(asp) {
		t.Fatalf("%s: %d прямоугольников вместо %d", r.Key, len(r.Rects), len(asp))
	}
	const eps = 0.01
	bleed := bleedPhotos(r.Key)
	tcrop := fitCrop(p)
	for i, a := range r.Rects {
		if a.X < -eps || a.Y < -eps || a.X+a.W > 2*p.PageW+eps || a.Y+a.H > p.PageH+eps {
			t.Errorf("%s: фото %d вне разворота: %+v", r.Key, i, a)
		}
		if bleed[i] {
			continue
		}
		if a.X < p.Margin-eps || a.Y < p.Margin-eps || a.X+a.W > 2*p.PageW-p.Margin+eps || a.Y+a.H > p.PageH-p.Margin+eps {
			t.Errorf("%s: фото %d заходит на поля: %+v", r.Key, i, a)
		}
		f := a.W / a.H / asp[i]
		lo, hi := 1-tcrop-1e-6, 1/(1-tcrop)+1e-6
		if p.Lock&(1<<uint(i)) != 0 {
			lo, hi = 1-1e-3, 1+1e-3
		}
		if f < lo || f > hi {
			t.Errorf("%s: фото %d: пропорции ×%.4f вне [%.4f, %.4f]", r.Key, i, f, lo, hi)
		}
		for j := i + 1; j < len(r.Rects); j++ {
			b := r.Rects[j]
			if a.X+eps < b.X+b.W && b.X+eps < a.X+a.W && a.Y+eps < b.Y+b.H && b.Y+eps < a.Y+a.H {
				t.Errorf("%s: фото %d и %d перекрываются", r.Key, i, j)
			}
		}
	}
	if r.Crop < 0 || r.Crop > tcrop+1e-9 {
		t.Errorf("%s: Crop %v вне предела %v", r.Key, r.Crop, tcrop)
	}
	if !FoldOK(r.Rects, asp, p) {
		t.Errorf("%s: фото на сгибе вне правила", r.Key)
	}
}

// TestCropInvariants — все ряды, которые видит интерфейс, при подрезке (в том числе с Lock и
// правилом сгиба): инварианты и круг Build(key) = строка ряда.
func TestCropInvariants(t *testing.T) {
	for _, params := range invariantParams {
		for _, lock := range []uint64{0, 1 << 1, 1<<0 | 1<<2} {
			for _, asp := range invariantCases {
				p := params
				p.Crop, p.Lock = 0.10, lock
				for name, rs := range rows(asp, p) {
					for _, r := range rs {
						checkCrop(t, asp, p, r)
						checkRoundTrip(t, asp, p, name, r)
					}
				}
			}
		}
	}
}

// TestRearrangedCrop — «С перестановкой» при подрезке строится через Build(p) и не повторяет формы ряда.
func TestRearrangedCrop(t *testing.T) {
	asp := []float64{P, L, P, L, P}
	p := fab(3, 3)
	p.Crop = 0.10
	re, _ := Rearranged(asp, p, 24)
	if len(re) == 0 {
		t.Fatal("пусто")
	}
	forms := map[string]bool{}
	for _, r := range Cached(asp, p, 24) {
		forms[shapeKey(r.Key, asp)] = true
	}
	for _, r := range re {
		if forms[shapeKey(r.Key, asp)] {
			t.Errorf("%s: форма уже есть в ряду «Авто»", r.Key)
		}
		b, ok := Build(r.Key, asp, p)
		if !ok || !rectsEqual(b.Rects, r.Rects) || b.Score != r.Score {
			t.Errorf("%s: не Build(p)", r.Key)
		}
	}
	// Без подрезки ряд прежний (CachedRearranged различает запись по Crop).
	p0 := fab(3, 3)
	r0, _ := Rearranged(asp, p0, 24)
	c0, _ := CachedRearranged(asp, p0, 24)
	if len(c0) != len(r0) {
		t.Errorf("кэш без подрезки: %d строк, ждали %d", len(c0), len(r0))
	}
}

func TestLayoutsCropCorpus(t *testing.T) {
	if testkit.Lite() {
		t.Skip("полный корпус: без -short и -race")
	}
	for _, c := range []struct {
		crop, min float64
	}{{0.05, 0.30}, {0.15, 0.70}, {0, 0}} {
		closed, total := 0, 0
		for _, g0 := range cropGeoms {
			p := g0
			p.Crop = c.crop
			for _, asp := range cropSets() {
				row := Layouts(asp, p, 24)
				total++
				if closes(row[0].Rects, p, 0.5) {
					closed++
				}
			}
		}
		share := float64(closed) / float64(total)
		t.Logf("Crop %.2f: лучшая закрывает лист у %d из %d (%.0f %%)", c.crop, closed, total, 100*share)
		if share < c.min {
			t.Errorf("Crop %.2f: закрывают %.0f %%, ждали не меньше %.0f %%", c.crop, 100*share, 100*c.min)
		}
		if c.crop == 0 && share > 0.05 {
			t.Errorf("без подрезки лист закрывают %.0f %%: корпус не показывает пользу", 100*share)
		}
	}
}

func TestWarmCropCost(t *testing.T) {
	if testkit.Lite() {
		t.Skip("замер CPU: без -short и -race")
	}
	if !cpuTimeKnown {
		t.Skip("процессорное время недоступно на этой ОС")
	}
	sets := cropSets()
	for n := 17; n <= 64; n += 11 {
		sets = append(sets, cropSame(n, P), cropMixed(n, uint64(n)*7), cropMixed(n, uint64(n)*13))
	}
	run := func(crop float64) float64 {
		ResetCache()
		t0 := cpuSeconds()
		for _, g0 := range cropGeoms {
			p := g0
			p.Crop = crop
			for _, asp := range sets {
				Cached(asp, p, 24)
				CachedRearranged(asp, p, 24)
			}
		}
		return cpuSeconds() - t0
	}
	// Порядок чередуем, берём лучший из трёх: всплеск нагрузки портит один прогон.
	base, with := math.Inf(1), math.Inf(1)
	for range 3 {
		base = math.Min(base, run(0))
		with = math.Min(with, run(0.10))
	}
	t.Logf("CPU холодного прогрева: без подрезки %.2f с, с 10 %% — %.2f с (%+.0f %%)", base, with, 100*(with/base-1))
	if with > 1.2*base {
		t.Errorf("прогрев с подрезкой %.2f с дороже 1,2 × %.2f с", with, base)
	}
}

// TestCropKeysDistinctCaches — кэши разных пределов не смешиваются: ряд при 5 % не равен ряду при 0.
func TestCropKeysDistinctCaches(t *testing.T) {
	asp := cropSame(6, P)
	p := fab(3, 3)
	r0 := Cached(asp, p, 24)
	p.Crop = 0.05
	r5 := Cached(asp, p, 24)
	if r0[0].Crop != 0 || r5[0].Crop <= 0 {
		t.Errorf("кэш смешал пределы: Crop %v и %v", r0[0].Crop, r5[0].Crop)
	}
	if !strings.HasPrefix(r5[0].Key, "H(") {
		t.Errorf("лучший ключ %q", r5[0].Key)
	}
}
