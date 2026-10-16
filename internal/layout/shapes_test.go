package layout

import (
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Грамматика и геометрия ключей 051 (c0-grammar.md): таблица, формы, эталон shapes.sha, фазз.

var (
	grammarP     = Params{PageW: 300, PageH: 300, Margin: 10, Gap: 3, Layflat: true}
	grammarFlat  = Params{PageW: 300, PageH: 300, Margin: 10, Gap: 3}
	grammarSheet = Params{PageW: 300, PageH: 300, Margin: 10, Gap: 3, Layflat: true, Sheet: true}
	grammarCfg   = []Params{grammarP, grammarFlat, grammarSheet}
)

// grammarAsp — n пропорций по циклу {1,5; 0,667; 1; 2,2; 0,8}.
func grammarAsp(n int) []float64 {
	cyc := []float64{1.5, 0.667, 1, 2.2, 0.8}
	out := make([]float64, n)
	for i := range out {
		out[i] = cyc[i%len(cyc)]
	}
	return out
}

// grammarCase — ключ и чего ждать: parse — разбор проходит, build — Build строит при
// layflat / без layflat / на листе (Sheet). У принятых ещё parse(s).String() == s.
type grammarCase struct {
	key   string
	parse bool
	build [3]bool
}

var (
	spreadOnly = [3]bool{true, false, true}
	pagedOnly  = [3]bool{true, true, false}
	never      = [3]bool{}
)

var grammarCases = []grammarCase{
	// Старые: Build — как в 0.12 (сверено с копией разбора 0.12).
	{"0", true, spreadOnly}, {"V(H(0,1),2)", true, spreadOnly}, {"H(H(0,1),2)", true, spreadOnly},
	{"P[S(0)|H(1,2)]", true, pagedOnly}, {"P[O(0)|I(1)]", true, pagedOnly},
	{"P[E(0)|-]", true, pagedOnly}, // на квадратных пропорциях (squareAsp): обрезки нет
	{"P[-|0]", true, pagedOnly}, {"B[full:0]", true, spreadOnly},
	{"B[right:0]", true, pagedOnly}, {"B[pages:1,0]", true, pagedOnly},
	// Разворот 051: только layflat.
	{"S(H(0,1,2))", true, spreadOnly}, {"U(H(0,1,2,3))", true, spreadOnly}, {"D(H(0,1,2,3,4))", true, spreadOnly},
	{"G(0,1)", true, spreadOnly}, {"G(H(0,1,2),H(3,4))", true, spreadOnly}, {"S(G(H(0,1),H(2,3)))", true, spreadOnly},
	{"L(0,1,2)", true, spreadOnly}, {"A(0,1,2,3,4)", true, spreadOnly},
	{"H(m0,1,m2)", true, spreadOnly}, {"H(m0,V(1,2))", true, spreadOnly},
	// Q и страницы 051: на листе нет.
	{"Q[H(0,1)|H(2,3)]", true, pagedOnly}, {"QU[0|1]", true, pagedOnly}, {"QD[G(0,1)|L(2,3,4)]", true, pagedOnly},
	{"Q[H(m0,1)|H(2,m3)]", true, pagedOnly}, {"P[U(G(0,H(1,2)))|D(3)]", true, pagedOnly},
	{"P[G(0,1)|S(L(2,3,4))]", true, pagedOnly},
	// m — только прямой ребёнок корневого H, и не все дети.
	{"m0", false, never}, {"S(m0)", false, never}, {"V(m0,1)", false, never}, {"H(H(m0,1),2)", false, never},
	{"H(m0,m1)", false, never}, {"L(m0,1,2)", false, never}, {"G(H(m0,1),2)", false, never},
	{"P[m0|1]", false, never}, {"P[E(m0)|1]", false, never}, {"mm0", false, never}, {"H(m,1)", false, never},
	// G, L/A — только корень и своё число детей.
	{"G(0)", false, never}, {"G(H(0,1))", false, never}, {"G(V(0,1),2)", false, never}, {"H(G(0,1),2)", false, never},
	{"V(0,L(1,2,3))", false, never}, {"L(0,1)", false, never}, {"A(0,1,2,3,4,5)", false, never},
	{"L(H(0,1),2,3)", false, never},
	// Модификаторы.
	{"S(S(0))", false, never}, {"S(P[0|1])", false, never}, {"S(Q[0|1])", false, never}, {"O(0)", false, never},
	{"E(0)", false, never}, {"SU(0)", false, never}, {"P[U(E(0))|1]", false, never}, {"P[UD(0)|1]", false, never},
	{"P[S(-)|0]", false, never},
	// Q.
	{"Q[-|0]", false, never}, {"Q[E(0)|1]", false, never}, {"Q[S(0)|1]", false, never}, {"Q[0]", false, never},
	{"Q[0|1|2]", false, never}, {"QS[0|1]", false, never}, {"QUD[0|1]", false, never}, {"UQ[0|1]", false, never},
	// Запись: одна каноническая.
	{"", false, never}, {"H(01,2)", false, never}, {"m01", false, never}, {"H(0, 1)", false, never},
	{"H(0,1)\n", false, never}, {"h(0,1)", false, never}, {"H(0,,1)", false, never}, {"H(0,1))", false, never},
	{"H(12345,0)", false, never},
	{"P[-|E(0)]", true, never}, {"P[E(0)|E(1)]", true, never}, // это B[right:0] и B[pages:0,1]
	// Листья.
	{"H(0,0)", true, never}, {"H(0,2)", true, never}, {"Q[0|0]", true, never},
}

// squareAsp — старые ключи «в край» на страницу: проверяются на квадратных фото (на 1,5 обрезка
// 33 % > 30 % — отказ и в 0.12), чтобы Build строил их, как строил в 0.12.
var squareAsp = map[string]bool{"P[E(0)|-]": true, "B[right:0]": true, "B[pages:1,0]": true}

func TestKeyGrammar051(t *testing.T) {
	for _, c := range grammarCases {
		k, ok := parseKey(c.key)
		if ok != c.parse {
			t.Errorf("%q: разбор %v, ждали %v", c.key, ok, c.parse)
			continue
		}
		// Неканонические записи разбираются, но Build их отвергает.
		canon := c.key != "P[-|E(0)]" && c.key != "P[E(0)|E(1)]"
		if ok && (k.String() == c.key) != canon {
			t.Errorf("%q: каноническая запись %q", c.key, k.String())
		}
		asp := grammarAsp(max(1, len(leavesOf(c.key))))
		if squareAsp[c.key] {
			for i := range asp {
				asp[i] = 1
			}
		}
		for i, p := range grammarCfg {
			r, ok := Build(c.key, asp, p)
			if ok != c.build[i] {
				t.Errorf("%q: Build %v при %+v, ждали %v", c.key, ok, p, c.build[i])
			}
			if ok {
				checkResult(t, asp, p, r)
			}
		}
	}
}

// Ключ длиннее MaxKeyLen не разбирается, даже если записан верно; на пределе — разбирается.
func TestKeyMaxLen(t *testing.T) {
	at := "H(" + strings.Repeat("0,", 1022) + "0)"
	over := "H(" + strings.Repeat("0,", 1022) + "10)"
	if len(at) != MaxKeyLen || len(over) != MaxKeyLen+1 {
		t.Fatalf("длины %d, %d", len(at), len(over))
	}
	if _, ok := parseKey(at); !ok {
		t.Error("ключ ровно MaxKeyLen не разобран")
	}
	if _, ok := parseKey(over); ok {
		t.Error("ключ длиннее MaxKeyLen разобран")
	}
}

// Глубокий ключ из файла не роняет программу: при стеке 8 МБ без предела длины
// разбор H(×200 000 падал бы фатально (recover этого не ловит).
func TestKeyDeepNoCrash(t *testing.T) {
	old := debug.SetMaxStack(8 << 20)
	defer debug.SetMaxStack(old)
	const d = 200000
	key := strings.Repeat("H(", d) + "0" + strings.Repeat(",1)", d)
	if _, ok := parseKey(key); ok {
		t.Fatal("ключ глубины 200 000 разобран")
	}
	if _, ok := Build(key, grammarAsp(2), grammarP); ok || IsPaged(key) {
		t.Fatal("Build или IsPaged принял глубокий ключ")
	}
}

// shapeKeys — ключи форм 051 на n ≥ 2 фото: все корни и модификаторы.
func shapeKeys(n int) []string {
	seq := func(a, b int) string {
		s := make([]string, 0, b-a)
		for i := a; i < b; i++ {
			s = append(s, strconv.Itoa(i))
		}
		return strings.Join(s, ",")
	}
	h := func(a, b int) string {
		if b-a == 1 {
			return strconv.Itoa(a)
		}
		return "H(" + seq(a, b) + ")"
	}
	half := n / 2
	g2 := "G(" + h(0, half) + "," + h(half, n) + ")"
	keys := []string{
		g2, "S(" + g2 + ")", "U(" + h(0, n) + ")", "D(" + h(0, n) + ")", "S(" + h(0, n) + ")", "U(" + g2 + ")",
		"Q[" + h(0, half) + "|" + h(half, n) + "]", "QU[" + h(0, half) + "|" + h(half, n) + "]",
		"QD[" + h(0, half) + "|" + h(half, n) + "]",
		"P[U(" + h(0, half) + ")|D(" + h(half, n) + ")]", "P[" + h(0, half) + "|U(" + h(half, n) + ")]",
	}
	if n == 2 {
		keys = append(keys, "H(m0,1)", "H(0,m1)")
	} else {
		keys = append(keys, "H(m0,"+seq(1, n-1)+",m"+strconv.Itoa(n-1)+")", "D(H(m0,V("+seq(1, n)+")))")
	}
	if n >= 3 && n <= 5 {
		keys = append(keys, "L("+seq(0, n)+")", "A("+seq(0, n)+")", "U(L("+seq(0, n)+"))")
	}
	if n >= 4 {
		q := half / 2
		gl := "G(" + h(0, q) + "," + h(q, half) + ")"
		keys = append(keys, "Q["+gl+"|"+h(half, n)+"]", "P[S("+gl+")|"+h(half, n)+"]",
			"Q[H(m0,"+seq(1, half)+")|"+h(half, n)+"]")
	}
	if n >= 6 {
		a, b := n/3, 2*n/3
		keys = append(keys, "G("+h(0, a)+","+h(a, b)+","+h(b, n)+")", "P[L(0,1,2)|"+h(3, n)+"]")
		if n <= 8 {
			keys = append(keys, "QD[L(0,1,2)|A("+seq(3, n)+")]")
		}
	}
	return keys
}

// Новые ключи: инварианты (поля, пропорции, без пересечений, корешок без layflat),
// повторный Build побитово тот же, ряды PageChoices у ключей с такой второй страницей.
func TestShapesInvariants(t *testing.T) {
	built := map[string]bool{}
	tried, ok := 0, 0
	for _, p := range append(slices.Clone(invariantParams), grammarSheet) {
		for _, asp := range invariantCases {
			if len(asp) < 2 {
				continue
			}
			for _, key := range shapeKeys(len(asp)) {
				tried++
				r, done := Build(key, asp, p)
				if !done {
					continue
				}
				ok++
				built[key[:1]] = true
				checkResult(t, asp, p, r)
				if again, _ := Build(key, asp, p); !sameResult(r, again) || r.Key != key {
					t.Errorf("%s: повторный Build другой", key)
				}
				for side := range 2 {
					in, re, _, _ := PageChoices(key, asp, p, side, 8)
					for _, c := range append(in, re...) {
						checkResult(t, asp, p, c)
						checkRoundTrip(t, asp, p, key, c)
					}
				}
			}
		}
	}
	t.Logf("построено %d из %d", ok, tried)
	for _, c := range "GSUDQPHL" {
		if !built[string(c)] {
			t.Errorf("ни один ключ на %c не построен", c)
		}
	}
}

// bbox — общий прямоугольник фото ids.
func bbox(rs []Rect, ids ...int) Rect {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, i := range ids {
		x0, y0 = min(x0, rs[i].X), min(y0, rs[i].Y)
		x1, y1 = max(x1, rs[i].X+rs[i].W), max(y1, rs[i].Y+rs[i].H)
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func mustBuild(t *testing.T, key string, asp []float64, p Params) []Rect {
	t.Helper()
	r, ok := Build(key, asp, p)
	if !ok {
		t.Fatalf("Build(%s) отказал", key)
	}
	checkResult(t, asp, p, r)
	return r.Rects
}

// Геометрия G и L/A (design §2): G — одна высота у всех фото, ряды через gap, каждый по
// центру блока; L/A — одна высота, шаг h/2 вниз / вверх, по x подряд через gap.
func TestShapesRowsLadder(t *testing.T) {
	p, asp := grammarP, grammarAsp(5)
	rs := mustBuild(t, "G(H(0,1,2),H(3,4))", asp, p)
	for i := range rs {
		if !near(rs[i].H, rs[0].H) {
			t.Errorf("G: высота фото %d %.4f, у 0 — %.4f", i, rs[i].H, rs[0].H)
		}
	}
	r1, r2, all := bbox(rs, 0, 1, 2), bbox(rs, 3, 4), bbox(rs, 0, 1, 2, 3, 4)
	if !near(r2.Y-(r1.Y+r1.H), p.Gap) || !near(r1.X+r1.W/2, all.X+all.W/2) || !near(r2.X+r2.W/2, all.X+all.W/2) {
		t.Errorf("G: ряды %+v %+v", r1, r2)
	}
	for _, op := range []string{"L", "A"} {
		rs = mustBuild(t, op+"(0,1,2,3)", asp[:4], p)
		for i := 1; i < 4; i++ {
			step := rs[i].Y - rs[i-1].Y
			want := rs[0].H / 2
			if op == "A" {
				want = -want
			}
			if !near(rs[i].H, rs[0].H) || !near(step, want) || !near(rs[i].X, rs[i-1].X+rs[i-1].W+p.Gap) {
				t.Errorf("%s: фото %d %+v после %+v", op, i, rs[i], rs[i-1])
			}
		}
	}
}

// Геометрия m и Q: m — фото kappa·h по центру ячейки (h — высота обычного фото ряда);
// Q — блоки одной высоты по центру своих страниц (решение 0.13 №39), U — к верхнему полю, D — к нижнему.
func TestShapesSmallPair(t *testing.T) {
	p, asp := grammarP, grammarAsp(4)
	top, bottom := p.Margin, p.PageH-p.Margin
	rs := mustBuild(t, "H(m0,1,m2)", asp[:3], p)
	for _, i := range []int{0, 2} {
		if !near(rs[i].H, kappa*rs[1].H) || !near(rs[i].Y+rs[i].H/2, rs[1].Y+rs[1].H/2) {
			t.Errorf("m%d: %+v при соседе %+v", i, rs[i], rs[1])
		}
	}
	for _, c := range []struct {
		key string
		ay  float64
	}{{"Q[H(0,1)|H(2,3)]", 0.5}, {"QU[H(0,1)|H(2,3)]", 0}, {"QD[H(0,1)|H(2,3)]", 1}} {
		rs = mustBuild(t, c.key, asp, p)
		l, r := bbox(rs, 0, 1), bbox(rs, 2, 3)
		if !near(l.H, r.H) || !near(l.Y, r.Y) || !near(l.X+l.W/2, p.PageW/2) || !near(r.X+r.W/2, p.PageW*1.5) {
			t.Errorf("%s: блоки %+v %+v", c.key, l, r)
		}
		if want := top + (bottom-top-l.H)*c.ay; !near(l.Y, want) {
			t.Errorf("%s: верх блока %.4f, ждали %.4f", c.key, l.Y, want)
		}
	}
}

// Модификаторы разворота: U/D — к верхнему / нижнему полю, по горизонтали по центру; S — в
// smallBox; U без места по высоте (портреты 0,5 во всю высоту) строится и выглядит как без U.
func TestShapesSpreadMods(t *testing.T) {
	p, asp := grammarP, grammarAsp(3)
	top, bottom := p.Margin, p.PageH-p.Margin
	for _, c := range []struct {
		key string
		y   func(b Rect) float64
		at  float64
	}{{"U(H(0,1,2))", func(b Rect) float64 { return b.Y }, top}, {"D(H(0,1,2))", func(b Rect) float64 { return b.Y + b.H }, bottom}} {
		b := bbox(mustBuild(t, c.key, asp, p), 0, 1, 2)
		if !near(c.y(b), c.at) || !near(b.X+b.W/2, p.PageW) {
			t.Errorf("%s: блок %+v", c.key, b)
		}
	}
	b, sb := bbox(mustBuild(t, "S(G(0,1))", asp[:2], p), 0, 1), smallBox(newGeom(p).spread)
	if b.X < sb.X-1e-6 || b.Y < sb.Y-1e-6 || b.X+b.W > sb.X+sb.W+1e-6 || b.Y+b.H > sb.Y+sb.H+1e-6 {
		t.Errorf("S(G): блок %+v вне %+v", b, sb)
	}
	// У страницы: U — верх блока на поле, D — низ блока на нижнем поле, по горизонтали по центру.
	rs := mustBuild(t, "P[U(H(0,1))|D(H(2,3))]", grammarAsp(4), p)
	l, r := bbox(rs, 0, 1), bbox(rs, 2, 3)
	if !near(l.Y, top) || !near(r.Y+r.H, bottom) || !near(l.X+l.W/2, p.PageW/2) || !near(r.X+r.W/2, 1.5*p.PageW) {
		t.Errorf("P[U|D]: блоки %+v %+v", l, r)
	}
	if near(l.Y+l.H, bottom) || near(r.Y, top) {
		t.Errorf("P[U|D]: блоки заняли всю высоту (нет сдвига): %+v %+v", l, r)
	}
	port := []float64{0.5, 0.5}
	u, ok1 := Build("U(H(0,1))", port, p)
	c, ok2 := Build("H(0,1)", port, p)
	if !ok1 || !ok2 || !slices.Equal(u.Rects, c.Rects) {
		t.Errorf("U(H(0,1)) на портретах: %v %v, %+v против %+v", ok1, ok2, u.Rects, c.Rects)
	}
}

const goldenShapesPath = "testdata/golden/shapes.sha"

// TestGoldenShapes — Build всех shapeKeys на 2…10 фото, goldenGeoms × три набора goldenAspects
// (и лист): Rects и оценки побитово. -update — только при осознанной смене геометрии 051
// (после выпуска 0.13 — никогда: ключи в файлах v9).
func TestGoldenShapes(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	var got []goldenGroup
	for g, p := range append(slices.Clone(goldenGeoms), grammarSheet) {
		for n := 2; n <= 10; n++ {
			gr := goldenGroup{name: fmt.Sprintf("p%d/n%d", g, n)}
			for set := range 3 {
				asp := goldenAspects(uint64(g)+1, n, set)
				var d dump
				for _, key := range shapeKeys(n) {
					r, ok := Build(key, asp, p)
					d.str(key)
					if ok {
						d.results([]Result{r})
					} else {
						d.u64(0)
					}
				}
				gr.cases = append(gr.cases, goldenCase{fmt.Sprintf("набор %d", set), d.sum()})
			}
			got = append(got, gr)
		}
	}
	checkGolden(t, goldenShapesPath, got)
}

// chainKey — H(H(…H(0,1)…),n−1): самый длинный ключ, который Build принимает на n фото.
func chainKey(n int) string {
	k := "0"
	for i := 1; i < n; i++ {
		k = "H(" + k + "," + strconv.Itoa(i) + ")"
	}
	return k
}

// FuzzParseKey: без паники в разборе, Build (layflat / без / лист), IsPaged, IsRearranged и
// relabel; длинный ключ — отказ; разобранный — листья по порядку записи (цифры — только
// позиции); построенный — канонический и строится повторно побитово так же.
// go test ./internal/layout -run '^$' -fuzz FuzzParseKey -fuzztime 20s
func FuzzParseKey(f *testing.F) {
	for _, c := range grammarCases {
		f.Add(c.key)
	}
	f.Add(chainKey(MaxPhotos))
	longest := ""
	for _, r := range Layouts(grammarAsp(MaxPhotos), grammarP, 48) {
		if len(r.Key) > len(longest) {
			longest = r.Key
		}
	}
	f.Add(longest)
	f.Add("H(" + strings.Repeat("0,", 1022) + "10)") // MaxKeyLen + 1
	f.Add(strings.Repeat("H(", 1023) + "0)")         // глубина разбора у предела длины, скобки не закрыты
	f.Fuzz(fuzzKey)
}

// fuzzAsp — пропорции n фото из байтов входа (0,5…2,5): раскладки на разных кадрах, а не на одном
// наборе; пустой вход — квадраты.
func fuzzAsp(s string, n int) []float64 {
	asp := make([]float64, n)
	for i := range asp {
		asp[i] = 1
		if s != "" {
			asp[i] = 0.5 + float64(s[(i*7+3)%len(s)])/128
		}
	}
	return asp
}

// fuzzKey — свойства FuzzParseKey на одной строке.
func fuzzKey(t *testing.T, s string) {
	k, ok := parseKey(s)
	if ok && len(s) > MaxKeyLen {
		t.Fatalf("разобран ключ длиной %d", len(s))
	}
	paged := IsPaged(s)
	if ok {
		if ls := leavesOf(s); !slices.Equal(ls, k.leaves()) {
			t.Fatalf("%q: leavesOf %v, разбор %v", s, ls, k.leaves())
		}
		if paged != (k.kind == 'P' || k.bleed == "right" || k.bleed == "pages") {
			t.Fatalf("%q: IsPaged %v у вида %c", s, paged, k.kind)
		}
		IsRearranged(s)
		idx := make([]int, 10000)
		for i := range idx {
			idx[i] = i + 1
		}
		relabel(s, idx)
	}
	asp := fuzzAsp(s, min(max(1, len(leavesOf(s))), MaxPhotos))
	for _, p := range grammarCfg {
		for side := range 2 { // страницы разворота: без паники на любом входе
			in, re, _, pok := PageChoices(s, asp, p, side, 4)
			if pok && !ok {
				t.Fatalf("%q: PageChoices принял неразобранный ключ", s)
			}
			for _, r := range slices.Concat(in, re) {
				checkResult(t, asp, p, r)
			}
		}
		r, built := Build(s, asp, p)
		if !built {
			continue
		}
		if !ok || r.Key != s || k.String() != s {
			t.Fatalf("%q: построен, разбор %v, ключ %q, запись %q", s, ok, r.Key, k.String())
		}
		if again, _ := Build(s, asp, p); !sameResult(r, again) {
			t.Fatalf("%q: повторный Build другой", s)
		}
		checkResult(t, asp, p, r)
	}
}
