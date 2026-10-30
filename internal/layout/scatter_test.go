package layout

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// «Разброс» (спека 105): корень Z и лист n. Грамматика, геометрия, эталон scatter.sha.

// scatterCases — случаи таблицы грамматики Z и n (тот же разбор, что у grammarCases).
var scatterCases = []grammarCase{
	// Z: 1–5 фото, у каждого уровень a…g; на разворот только layflat, на странице и листе — как корень.
	{"Z(d0)", true, spreadOnly}, {"Z(a0,d1,g2)", true, spreadOnly}, {"Z(g0,a1)", true, spreadOnly},
	{"Z(a0,b1,c2,d3,e4)", true, spreadOnly}, {"Z(g0,f1,e2,d3,c4)", true, spreadOnly},
	{"Z(a1,d0,g2)", true, spreadOnly}, // позиции не по порядку: ключ тот же, что у перестановки
	{"S(Z(a0,d1,g2))", true, spreadOnly}, {"U(Z(a0,g1))", true, spreadOnly}, {"D(Z(g0,a1,g2))", true, spreadOnly},
	{"P[Z(g0,g1)|Z(a2)]", true, pagedOnly}, {"P[Z(a0)|Z(d1,g2)]", true, pagedOnly},
	{"P[S(Z(a0,g1))|Z(d2)]", true, pagedOnly},
	{"Q[Z(a0,b1)|Z(g2)]", true, pagedOnly}, {"P[-|Z(a0,g1)]", true, pagedOnly},
	// Лист n — как m, только kappaN; ряд H из m и n вместе допустим, из одних m/n — нет.
	{"H(n0,1,n2)", true, spreadOnly}, {"H(n0,n1,2,n3,n4)", true, spreadOnly}, {"H(n0,V(1,2),n3)", true, spreadOnly},
	{"H(m0,n1,2)", true, spreadOnly}, {"P[H(n0,1)|H(2,n3)]", true, pagedOnly}, {"S(H(n0,1,n2))", true, spreadOnly},
	// Отказы Z: число детей, уровень, позиция, место в дереве.
	{"Z()", false, never}, {"Z(0)", false, never}, {"Z(h0)", false, never}, {"Z(A0)", false, never},
	{"Z(a0,b1,c2,d3,e4,f5)", false, never}, {"Z(a)", false, never}, {"Z(a01)", false, never},
	{"Z(a0,)", false, never}, {"Z(a0, b1)", false, never}, {"Z(a0,1)", false, never}, {"Z(0,a1)", false, never},
	{"Z(aa0)", false, never}, {"Z(n0,1)", false, never},
	{"Z(a0", false, never}, {"Z(a0))", false, never}, {"Z(a0)Z", false, never}, {"Z[a0]", false, never},
	{"H(Z(a0),1)", false, never}, {"V(Z(a0,b1),2)", false, never}, {"Z(Z(a0))", false, never},
	{"G(Z(a0),1)", false, never}, {"L(Z(a0),1,2)", false, never}, {"a0", false, never}, {"H(a0,1)", false, never},
	{"S(S(Z(a0)))", false, never}, {"E(Z(a0))", false, never}, {"B[full:Z(a0)]", false, never},
	// Отказы n: только прямой ребёнок корневого H, и не все дети.
	{"n0", false, never}, {"S(n0)", false, never}, {"V(n0,1)", false, never}, {"H(H(n0,1),2)", false, never},
	{"H(n0,n1)", false, never}, {"H(n0,m1)", false, never}, {"L(n0,1,2)", false, never}, {"G(H(n0,1),2)", false, never},
	{"P[n0|1]", false, never}, {"P[E(n0)|1]", false, never}, {"nn0", false, never}, {"H(n,1)", false, never},
	{"H(n01,2)", false, never}, {"H(N0,1)", false, never}, {"Z(n0)", false, never},
	// Листья: позиции 0…n−1 по разу.
	{"Z(a0,b0)", true, never}, {"Z(a0,b2)", true, never}, {"H(n0,1,n3)", true, never},
}

func TestKeyGrammarScatter(t *testing.T) {
	for _, c := range scatterCases {
		checkGrammarCase(t, c)
	}
}

// Запись ключа — единственная каноническая: уровень пишется всегда, в порядке записи.
func TestScatterKeyCanon(t *testing.T) {
	for _, key := range []string{"Z(a0,d1,g2)", "Z(d0)", "H(n0,1,n2)", "P[Z(g0,g1)|Z(a2)]", "H(m0,n1,2)"} {
		k, ok := parseKey(key)
		if !ok || k.String() != key {
			t.Errorf("%q: запись %q (разбор %v)", key, k.String(), ok)
		}
	}
	// Позиции снимаются без цифр уровня: буквы a…g, n — не часть позиции.
	if got := leavesOf("Z(a2,d0,g1)"); !slices.Equal(got, []int{2, 0, 1}) {
		t.Errorf("leavesOf = %v", got)
	}
	if got := relabel("Z(a0,d1,g2)", []int{5, 7, 9}); got != "Z(a5,d7,g9)" {
		t.Errorf("relabel = %q", got)
	}
	if got := shapeKey("H(n0,1,n2)", []float64{0.5, 1, 0.5}); got != "H(n0.500,1.000,n0.500)" {
		t.Errorf("shapeKey = %q", got)
	}
	// Перестановка порядка ленты видна: Z(a1,d0,g2) — не в порядке ленты.
	if !IsRearranged("Z(a1,d0,g2)") || IsRearranged("Z(a0,d1,g2)") {
		t.Error("IsRearranged не различает порядок листьев у Z")
	}
}

// scatterP — пример из спеки 105: разворот 600×305, поля 12, отступ 3.
var scatterP = Params{PageW: 300, PageH: 305, Margin: 12, Gap: 3, Layflat: true}

// Геометрия Z (design §3): одна высота 10/16 области, воздух поровну у краёв и между фото,
// уровни — доли свободной высоты.
func TestScatterGeometry(t *testing.T) {
	asp := []float64{P, P, P}
	rs := mustBuild(t, "Z(a0,d1,g2)", asp, scatterP)
	box := newGeom(scatterP).spread
	h := box.H * 10 / 16
	w := h * P
	air := (box.W - 3*w) / 4
	for i, level := range []float64{0, 0.5, 1} {
		want := Rect{box.X + air + float64(i)*(w+air), box.Y + (box.H-h)*level, w, h}
		if !near(rs[i].X, want.X) || !near(rs[i].Y, want.Y) || !near(rs[i].W, want.W) || !near(rs[i].H, want.H) {
			t.Errorf("фото %d: %+v, ждали %+v", i, rs[i], want)
		}
	}
	// Числа из спеки: 117×176 мм, x 68/241/415, y 12/65/117.
	for i, want := range [][2]float64{{68, 12}, {241, 65}, {415, 117}} {
		if math.Abs(rs[i].W-117) > 0.5 || math.Abs(rs[i].H-176) > 0.5 ||
			math.Abs(rs[i].X-want[0]) > 0.5 || math.Abs(rs[i].Y-want[1]) > 0.5 {
			t.Errorf("фото %d: %+v, ждали 117×176 в %v", i, rs[i], want)
		}
	}
	// Воздух поровну: у левого и правого краёв и между соседями.
	gaps := []float64{rs[0].X - box.X, rs[1].X - rs[0].X - rs[0].W, rs[2].X - rs[1].X - rs[1].W,
		box.X + box.W - rs[2].X - rs[2].W}
	for i, g := range gaps {
		if math.Abs(g-gaps[0]) > 0.01 || g < box.H/16 {
			t.Errorf("воздух %d = %.3f, у первого %.3f", i, g, gaps[0])
		}
	}
}

// Уровни: все семь долей свободной высоты, края — вплотную к верху и низу области.
func TestScatterLevels(t *testing.T) {
	box := newGeom(scatterP).spread
	for i, level := range zLevels {
		letter := string(rune('a' + i))
		rs := mustBuild(t, "Z("+letter+"0)", []float64{P}, scatterP)
		h := rs[0].H
		if want := box.Y + (box.H-h)*level; !near(rs[0].Y, want) {
			t.Errorf("уровень %s: y = %.4f, ждали %.4f", letter, rs[0].Y, want)
		}
	}
	if want := []float64{0, 0.25, 1.0 / 3, 0.5, 2.0 / 3, 0.75, 1}; !slices.Equal(zLevels[:], want) {
		t.Errorf("zLevels = %v: доли в формате, менять нельзя", zLevels)
	}
}

// Узкая область: высота ограничена воздухом H/16, а не 10/16 области; совсем узкая — отказ.
func TestScatterNarrow(t *testing.T) {
	p := Params{PageW: 120, PageH: 300, Margin: 10, Gap: 3, Layflat: true, Sheet: true}
	asp := []float64{P, P, P, P, P}
	rs := mustBuild(t, "Z(a0,b1,c2,d3,e4)", asp, p)
	box := newGeom(p).spread
	air0 := box.H / 16
	wantH := (box.W - 6*air0) / (5 * P)
	if wantH >= box.H*10/16 {
		t.Fatalf("тест не на пределе: %.2f ≥ 10/16 области", wantH)
	}
	for i, r := range rs {
		if !near(r.H, wantH) {
			t.Errorf("фото %d: h = %.4f, ждали %.4f (по воздуху H/16)", i, r.H, wantH)
		}
	}
	// Воздух между соседними не меньше H/16 (на пределе — ровно).
	if g := rs[1].X - rs[0].X - rs[0].W; g < air0-1e-9 {
		t.Errorf("воздух %.4f < H/16 = %.4f", g, air0)
	}
	// Совсем узко: воздух съел всю ширину — отказ.
	if _, ok := Build("Z(a0,b1,c2,d3,e4)", asp, Params{PageW: 40, PageH: 300, Margin: 10, Gap: 3, Layflat: true, Sheet: true}); ok {
		t.Error("Build принял Z, где воздух съел всю ширину")
	}
	// Большой отступ перекрывает H/16: воздух не меньше gap.
	pg := scatterP
	pg.Gap = 40
	rs = mustBuild(t, "Z(a0,d1,g2)", []float64{P, P, P}, pg)
	if g := rs[1].X - rs[0].X - rs[0].W; g < 40-1e-9 {
		t.Errorf("воздух %.2f < отступ 40", g)
	}
}

// Лист n: «центр крупнее» с боками kappaN вместо kappa (m даёт 0,5).
func TestCenterN(t *testing.T) {
	asp := []float64{P, P, P}
	rs := mustBuild(t, "H(n0,1,n2)", asp, scatterP)
	if math.Abs(rs[1].W-187) > 0.5 || math.Abs(rs[1].H-281) > 0.5 {
		t.Errorf("центр %+v, ждали 187×281", rs[1])
	}
	for _, i := range []int{0, 2} {
		if math.Abs(rs[i].W-131) > 0.5 || math.Abs(rs[i].H-197) > 0.5 {
			t.Errorf("бок %d: %+v, ждали 131×197", i, rs[i])
		}
	}
	// Бока по центру высоты ряда, высота бока = 0,7 высоты центра.
	for _, i := range []int{0, 2} {
		if !near(rs[i].H, 0.7*rs[1].H) || !near(rs[i].Y+rs[i].H/2, rs[1].Y+rs[1].H/2) {
			t.Errorf("бок %d не на 0,7 по центру: %+v при центре %+v", i, rs[i], rs[1])
		}
	}
	// m — прежние 0,5.
	m := mustBuild(t, "H(m0,1,m2)", asp, scatterP)
	if !near(m[0].H, 0.5*m[1].H) {
		t.Errorf("бок m: %+v при центре %+v", m[0], m[1])
	}
	// Любые пропорции: бока мельче центра.
	mix := mustBuild(t, "H(n0,1,n2)", []float64{L, P, S}, scatterP)
	if mix[0].H >= mix[1].H || mix[2].H >= mix[1].H {
		t.Errorf("бока крупнее центра: %+v", mix)
	}
}

// Подрезка (104) к Z и n не применяется: у «Разброса» воздух задуман, ячейка n — тоже.
func TestScatterNoCrop(t *testing.T) {
	asp := []float64{P, P, P}
	for _, key := range []string{"Z(a0,d1,g2)", "H(n0,1,n2)", "P[Z(g0,g1)|Z(a2)]", "S(Z(a0,d1,g2))", "H(n0,V(1,2))"} {
		for _, p := range []Params{scatterP, album30, album30wide} {
			want, ok := Build(key, asp, p)
			p.Crop, p.Lock = MaxFitCrop, 1<<1
			got, ok2 := Build(key, asp, p)
			if ok != ok2 || ok && (!sameResult(want, got) || got.Crop != 0) {
				t.Errorf("%s: Crop меняет Build: %+v → %+v", key, want, got)
			}
		}
	}
	if plainTree(&keyTree{leaf: -1, op: 'Z'}) || plainTree(&keyTree{leaf: 0, small: true, big: true}) {
		t.Error("plainTree считает Z или n сплошным")
	}
}

// Прижимать Z некуда: U/D/O/I дают тот же вид, что без модификатора; S вписывает в меньшую
// область (decisions 105-g1-2).
func TestScatterMods(t *testing.T) {
	asp := []float64{P, P, P, P}
	base := mustBuild(t, "P[Z(d0,d1)|Z(d2,d3)]", asp, scatterP)
	for _, key := range []string{"P[O(Z(d0,d1))|I(Z(d2,d3))]", "P[U(Z(d0,d1))|D(Z(d2,d3))]"} {
		if got := mustBuild(t, key, asp, scatterP); !slices.Equal(got, base) {
			t.Errorf("%s: %+v, без модификаторов %+v", key, got, base)
		}
	}
	whole := mustBuild(t, "Z(d0,d1,d2,d3)", asp, scatterP)
	for _, key := range []string{"U(Z(d0,d1,d2,d3))", "D(Z(d0,d1,d2,d3))"} {
		if got := mustBuild(t, key, asp, scatterP); !slices.Equal(got, whole) {
			t.Errorf("%s: %+v, без модификатора %+v", key, got, whole)
		}
	}
	if small := mustBuild(t, "S(Z(d0,d1,d2,d3))", asp, scatterP); small[0].H >= whole[0].H {
		t.Errorf("S не уменьшил: %.2f при %.2f", small[0].H, whole[0].H)
	}
	// Q выравнивает страницы по размаху фото, а он у ровного ряда уже высоты области.
	if q := mustBuild(t, "Q[Z(d0,d1)|Z(d2,d3)]", asp, scatterP); q[0].H >= base[0].H {
		t.Errorf("Q[Z|Z]: h = %.2f, у P[Z|Z] %.2f", q[0].H, base[0].H)
	}
}

// Z вне layflat: на разворот нет, страницы — есть; фото не идёт через корешок.
func TestScatterSpine(t *testing.T) {
	asp := []float64{P, P, P}
	flat := scatterP
	flat.Layflat = false
	if _, ok := Build("Z(a0,d1,g2)", asp, flat); ok {
		t.Error("Z на разворот без layflat построен")
	}
	for _, key := range []string{"P[Z(g0,g1)|Z(a2)]", "P[Z(a0)|Z(d1,g2)]"} {
		r, ok := Build(key, asp, flat)
		if !ok {
			t.Fatalf("%s без layflat не построен", key)
		}
		checkResult(t, asp, flat, r)
	}
}

// profiles — уровни по фото слева направо для 1…5 фото (для 3–5 — таблица design §4.1).
var profiles = map[int][]string{
	1: {"a", "d", "g"},
	2: {"ag", "ga", "dd", "be"},
	3: {"adg", "gda", "aga", "gag", "gga", "agg", "ddd"},
	4: {"aceg", "geca", "agga", "gaag", "ggga", "aggg", "dddd"},
	5: {"abdfg", "gfdba", "adgda", "gdadg", "gggga", "agggg", "ddddd"},
}

// zKey — Z-ключ фото [from, to) с уровнями lv[from:to].
func zKey(lv string, from, to int) string {
	kids := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		kids = append(kids, string(lv[i])+strconv.Itoa(i))
	}
	return "Z(" + strings.Join(kids, ",") + ")"
}

// scatterKeys — ключи Z и n на n ≤ 5 фото: профили на разворот и по страницам, модификаторы, центр.
func scatterKeys(n int) []string {
	var keys []string
	for _, lv := range profiles[n] {
		keys = append(keys, zKey(lv, 0, n), "S("+zKey(lv, 0, n)+")", "U("+zKey(lv, 0, n)+")")
		for j := 1; j < n; j++ {
			keys = append(keys, "P["+zKey(lv, 0, j)+"|"+zKey(lv, j, n)+"]")
		}
		if n >= 2 {
			keys = append(keys, "Q["+zKey(lv, 0, n/2)+"|"+zKey(lv, n/2, n)+"]")
		}
	}
	switch n {
	case 2:
		keys = append(keys, "H(n0,1)", "H(0,n1)")
	case 3:
		keys = append(keys, "H(n0,1,n2)", "H(n1,0,n2)", "H(m0,n1,2)")
	case 4:
		keys = append(keys, "H(n0,1,2,n3)", "H(n0,1,n2,n3)", "H(n0,V(1,2),n3)", "H(m0,n1,2,3)")
	case 5:
		keys = append(keys, "H(n0,n1,2,n3,n4)", "H(n0,1,n2,3,n4)", "P[H(n0,1)|H(n2,3,n4)]", "H(m0,n1,2,3,4)")
	}
	return keys
}

// Инварианты Z и n на всех наборах: поля, пропорции, без пересечений, корешок без layflat;
// повторный Build побитово тот же; ряды PageChoices у ключей с такой второй страницей.
func TestScatterInvariants(t *testing.T) {
	built := map[byte]bool{}
	for _, p := range append(slices.Clone(invariantParams), grammarSheet) {
		for _, asp := range invariantCases {
			if len(asp) > 5 {
				continue
			}
			for _, key := range scatterKeys(len(asp)) {
				r, ok := Build(key, asp, p)
				if !ok {
					continue
				}
				built[key[0]] = true
				checkResult(t, asp, p, r)
				if again, _ := Build(key, asp, p); !sameResult(r, again) || r.Key != key {
					t.Errorf("%s: повторный Build другой", key)
				}
				for side := range 2 {
					in, re, _, _ := PageChoices(key, asp, p, side, 8)
					for _, c := range slices.Concat(in, re) {
						checkResult(t, asp, p, c)
						checkRoundTrip(t, asp, p, key, c)
					}
				}
			}
		}
	}
	for _, c := range "ZHPSUQ" {
		if !built[byte(c)] {
			t.Errorf("ни один ключ на %c не построен", c)
		}
	}
}

const goldenScatterPath = "testdata/golden/scatter.sha"

// TestGoldenScatter — Build ключей Z и n (scatterKeys) на 1…5 фото, goldenGeoms × три набора
// goldenAspects (и лист): Rects и оценки побитово. После выпуска 0.24 — никогда не
// перезаписывать: ключи в файлах v18.
func TestGoldenScatter(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	var got []goldenGroup
	built, total := 0, 0
	for g, p := range append(slices.Clone(goldenGeoms), grammarSheet) {
		for n := 1; n <= 5; n++ {
			gr := goldenGroup{name: fmt.Sprintf("p%d/n%d", g, n)}
			for set := range 3 {
				asp := goldenAspects(uint64(g)+1, n, set)
				var d dump
				for _, key := range scatterKeys(n) {
					r, ok := Build(key, asp, p)
					d.str(key)
					total++
					if ok {
						built++
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
	// Эталон из одних отказов ничего не держит: строится не меньше трети ключей.
	if built*3 < total {
		t.Errorf("построено %d из %d ключей", built, total)
	}
	checkGolden(t, goldenScatterPath, got)
}
