package layout

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

var album30 = Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}

const (
	L = 1.5     // горизонтальное 3:2
	P = 2.0 / 3 // вертикальное 2:3
	S = 1.0     // квадрат
)

var invariantCases = [][]float64{
	{L}, {P}, {L, L}, {P, P}, {L, P}, {L, P, L},
	{P, P, P, P}, {L, L, L, L}, {P, L, P, L}, {P, L, P, L, P},
	{P, P, L, P, P, L, L}, {L, P, S, L, P, S, L, P},
	{P, P, P, P, P, P, P, P, P, P, P, P}, // 12 — ряды/колонки
}

var invariantParams = []Params{album30, {PageW: 300, PageH: 200, Margin: 10, Gap: 4, Layflat: false}}

// rows — все ряды, которые видит интерфейс: порядок ленты, перестановки,
// ряды страниц для первых постраничных раскладок и для раскладок «в край»,
// а также ряды второй страницы, когда на первой стоит модификатор (E, S, O, I).
func rows(asp []float64, p Params) map[string][]Result {
	out := map[string][]Result{}
	out["layouts"] = Layouts(asp, p, 24)
	out["rearranged"], _ = Rearranged(asp, p, 24)
	pages := func(key string, side int) []Result {
		in, re, _, ok := PageChoices(key, asp, p, side, 24)
		if ok {
			out[fmt.Sprintf("%s side %d", key, side)] = in
			out[fmt.Sprintf("%s side %d rearranged", key, side)] = re
		}
		return in
	}
	n := 0
	for _, r := range append(append([]Result{}, out["layouts"]...), out["rearranged"]...) {
		if !IsPaged(r.Key) || n >= 3 && !strings.HasPrefix(r.Key, "B[") {
			continue
		}
		n++
		for side := range 2 {
			for _, c := range pages(r.Key, side) {
				if k, _ := pageForm(c.Key); k.mods[side] != 0 {
					pages(c.Key, 1-side)
				}
			}
		}
	}
	return out
}

// bleedPhotos — фото, которые стоят «в край» (с обрезкой и без полей).
func bleedPhotos(key string) map[int]bool {
	out := map[int]bool{}
	k, ok := pageForm(key)
	if !ok {
		k, _ = parseKey(key)
		if k.kind == 'B' {
			for _, i := range k.idx {
				out[i] = true
			}
		}
		return out
	}
	for side, t := range k.pages {
		if k.mods[side] == 'E' {
			out[t.leaf] = true
		}
	}
	return out
}

func TestInvariants(t *testing.T) {
	for _, params := range invariantParams {
		for _, asp := range invariantCases {
			all := rows(asp, params)
			if len(all["layouts"]) == 0 {
				t.Fatalf("нет раскладок для %v", asp)
			}
			for name, rs := range all {
				seen := map[string]bool{}
				for _, r := range rs {
					if seen[r.Key] {
						t.Errorf("%v %s: ключ %s повторяется", asp, name, r.Key)
					}
					seen[r.Key] = true
					checkResult(t, asp, params, r)
				}
			}
		}
	}
}

func checkResult(t *testing.T, asp []float64, p Params, r Result) {
	t.Helper()
	if len(r.Rects) != len(asp) {
		t.Fatalf("%s: %d прямоугольников вместо %d", r.Key, len(r.Rects), len(asp))
	}
	bleed := bleedPhotos(r.Key)
	const eps = 0.01
	for i, a := range r.Rects {
		if a.X < -eps || a.Y < -eps || a.X+a.W > 2*p.PageW+eps || a.Y+a.H > p.PageH+eps {
			t.Errorf("%s: фото %d вне разворота: %+v", r.Key, i, a)
		}
		if !bleed[i] {
			if a.X < p.Margin-eps || a.Y < p.Margin-eps || a.X+a.W > 2*p.PageW-p.Margin+eps || a.Y+a.H > p.PageH-p.Margin+eps {
				t.Errorf("%s: фото %d заходит на поля: %+v", r.Key, i, a)
			}
			if got := a.W / a.H; math.Abs(got-asp[i])/asp[i] > 0.001 {
				t.Errorf("%s: фото %d искажено: %.4f вместо %.4f", r.Key, i, got, asp[i])
			}
		}
		if !p.Layflat && a.X < p.PageW-eps && a.X+a.W > p.PageW+eps {
			t.Errorf("%s: фото %d идёт через корешок без layflat", r.Key, i)
		}
		for j := i + 1; j < len(r.Rects); j++ {
			b := r.Rects[j]
			if a.X+eps < b.X+b.W && b.X+eps < a.X+a.W && a.Y+eps < b.Y+b.H && b.Y+eps < a.Y+a.H {
				t.Errorf("%s: фото %d и %d перекрываются", r.Key, i, j)
			}
		}
	}
}

func TestBuildRoundTrip(t *testing.T) {
	for _, params := range invariantParams {
		for _, asp := range invariantCases {
			for name, rs := range rows(asp, params) {
				for _, r := range rs {
					checkRoundTrip(t, asp, params, name, r)
				}
			}
		}
	}
}

// checkRoundTrip — Build(r.Key) даёт те же оценку и прямоугольники, что и ряд.
func checkRoundTrip(t *testing.T, asp []float64, params Params, name string, r Result) {
	t.Helper()
	b, ok := Build(r.Key, asp, params)
	if !ok {
		t.Fatalf("%v %s: Build(%s) отказал", asp, name, r.Key)
	}
	if math.Abs(b.Score-r.Score) > 1e-6 {
		t.Errorf("%v %s: %s score %.9f, ждали %.9f", asp, name, r.Key, b.Score, r.Score)
	}
	for i := range r.Rects {
		a, c := r.Rects[i], b.Rects[i]
		if math.Abs(a.X-c.X) > 1e-6 || math.Abs(a.Y-c.Y) > 1e-6 || math.Abs(a.W-c.W) > 1e-6 || math.Abs(a.H-c.H) > 1e-6 {
			t.Errorf("%v %s: %s фото %d: %+v вместо %+v", asp, name, r.Key, i, c, a)
		}
	}
}

func TestBuildRejects(t *testing.T) {
	asp := []float64{L, P, L}
	flat := album30
	flat.Layflat = false
	for _, c := range []struct {
		key string
		asp []float64
		p   Params
	}{
		{"", asp, album30},
		{"H(0,1", asp, album30},
		{"H(0,1,2)x", asp, album30},
		{"H(0)", []float64{L}, album30},
		{"P[0,1|2]", asp, album30},
		{"P[V(0,1)]", asp, album30},
		{"Q(0,1,2)", asp, album30},
		{"H(00,1,2)", asp, album30},
		{"H(0,1)", asp, album30},      // листьев меньше, чем фото
		{"H(0,1,2,3)", asp, album30},  // больше
		{"H(0,1,1)", asp, album30},    // повтор
		{"P[V(0,2)|2]", asp, album30}, // повтор через страницы
		{"H(0,1,2)", asp, flat},       // на весь разворот без layflat
		{"B[full:0]", []float64{L}, flat},
		{"B[full:1]", []float64{L}, album30},
		{"B[pages:0,0]", []float64{L, L}, album30},
		{"B[side:0]", []float64{L}, album30},
		{"B[pages:0,1]", []float64{3, 3}, album30}, // обрезка больше 30 %
		{"H(0,1)", []float64{L, 0}, album30},
		{"P[-|E(0)]", []float64{S}, album30},       // это B[right:0]
		{"P[E(0)|E(1)]", []float64{S, S}, album30}, // это B[pages:0,1]
		{"P[E(H(0,1))|2]", asp, album30},           // в край только одно фото
		{"P[E(0)|V(1,2)]", asp, album30},           // обрезка больше 30 %
		{"P[X(0)|V(1,2)]", asp, album30},           // нет такого модификатора
		{"P[S(0|V(1,2)]", asp, album30},            // не закрыта скобка
		{"P[S(-)|H(0,1,2)]", asp, album30},         // модификатор пустой страницы
		{"P[S(0)|S(H(1,2)))]", asp, album30},       // лишняя скобка
	} {
		if r, ok := Build(c.key, c.asp, c.p); ok {
			t.Errorf("Build(%q, %v, layflat=%t) = %+v, ждали отказ", c.key, c.asp, c.p.Layflat, r)
		}
	}
	// Слишком много промежутков в тесном месте: фото меньше миллиметра.
	tiny := Params{PageW: 20, PageH: 20, Margin: 2, Gap: 7}
	if _, ok := Build("P[V(0,1,2)|-]", []float64{L, L, L}, tiny); ok {
		t.Errorf("фото меньше 1 мм построено")
	}
}

func TestRearrangedBetter(t *testing.T) {
	asp := []float64{P, L, P, L}
	base := Layouts(asp, album30, 24)
	re, block := Rearranged(asp, album30, 24)
	if len(re) == 0 {
		t.Fatalf("перестановок нет, причина %q", block)
	}
	if re[0].Score <= base[0].Score {
		t.Fatalf("перестановка %s (%.3f) не лучше %s (%.3f)", re[0].Key, re[0].Score, base[0].Key, base[0].Score)
	}
	// Вертикальные кадры (0 и 2) на одной странице, горизонтальные — на другой.
	found := false
	for _, r := range re {
		left := func(i int) bool { return r.Rects[i].X < album30.PageW }
		if strings.HasPrefix(r.Key, "P[") && left(0) == left(2) && left(1) == left(3) && left(0) != left(1) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("нет варианта «вертикальные на одной странице»")
	}
	t.Logf("лучшая обычная %.3f %s, с перестановкой %.3f %s", base[0].Score, base[0].Key, re[0].Score, re[0].Key)
}

func TestRearrangedDistinct(t *testing.T) {
	for _, params := range invariantParams {
		for _, asp := range invariantCases {
			base := Layouts(asp, params, 24)
			shapes := map[string]bool{}
			for _, r := range base {
				shapes[shapeKey(r.Key, asp)] = true
			}
			re, _ := Rearranged(asp, params, 24)
			seen := map[string]bool{}
			for _, r := range re {
				sh := shapeKey(r.Key, asp)
				switch {
				case !IsRearranged(r.Key):
					t.Errorf("%v: тождественная перестановка %s", asp, r.Key)
				case shapes[sh]:
					t.Errorf("%v: форма %s уже есть в обычном ряду", asp, r.Key)
				case seen[sh]:
					t.Errorf("%v: две перестановки одной формы %s", asp, sh)
				}
				seen[sh] = true
				if r.Score < base[0].Score-rearrangeSlack {
					t.Errorf("%v: %s хуже лучшей обычной больше допуска", asp, r.Key)
				}
			}
		}
	}
	re, block := Rearranged([]float64{L, L, L, L}, album30, 24)
	if len(re) != 0 || block != BlockSame {
		t.Errorf("одинаковые пропорции: %d перестановок, причина %q", len(re), block)
	}
	re, block = Rearranged([]float64{L, P, L, P, L, P, L, P}, album30, 24)
	if len(re) != 0 || block != BlockMany {
		t.Errorf("8 фото: %d перестановок, причина %q", len(re), block)
	}
}

func TestRearrangeBudget(t *testing.T) {
	asp := []float64{0.5, 0.6, 0.75, 1, 1.25, 1.5, 1.8}
	re, _, ps := rearranged(asp, album30, 24)
	if !ps.stopped {
		t.Fatalf("бюджет не кончился: перебрано %d деревьев", ps.used)
	}
	// Сверх бюджета — только блоки в порядке ленты, их считаем всегда.
	if ps.used > 2*permBudget {
		t.Errorf("перебрано %d деревьев при бюджете %d", ps.used, permBudget)
	}
	if len(re) == 0 {
		t.Errorf("нет ни одной перестановки")
	}
	t.Logf("деревьев %d, перестановок %d", ps.used, len(re))
}

func TestPageChoicesKeepOtherPage(t *testing.T) {
	for _, params := range invariantParams {
		for _, asp := range invariantCases {
			cur := Layouts(asp, params, 24)
			re, _ := Rearranged(asp, params, 24)
			for _, r := range append(cur, re...) {
				if !IsPaged(r.Key) {
					continue
				}
				for side := range 2 {
					checkOtherPageKept(t, asp, params, r, side)
				}
			}
		}
	}
	if _, _, _, ok := PageChoices("H(0,1)", []float64{L, L}, album30, 0, 24); ok {
		t.Errorf("непостраничный ключ принят")
	}
	if _, _, _, ok := PageChoices("P[-|0]", []float64{L}, album30, 0, 24); ok {
		t.Errorf("пустая страница принята")
	}
	if _, _, _, ok := PageChoices("B[full:0]", []float64{S}, album30, 1, 24); ok {
		t.Errorf("фото в край на весь разворот принято как страница")
	}
}

// checkOtherPageKept — ряды страницы side постраничные и не двигают фото второй страницы.
func checkOtherPageKept(t *testing.T, asp []float64, params Params, r Result, side int) {
	t.Helper()
	in, rr, _, ok := PageChoices(r.Key, asp, params, side, 24)
	if !ok {
		return
	}
	if len(in) == 0 {
		t.Fatalf("%v %s сторона %d: пустой ряд", asp, r.Key, side)
	}
	for _, c := range append(in, rr...) {
		if !IsPaged(c.Key) {
			t.Errorf("%s: ключ %s не постраничный", r.Key, c.Key)
		}
		for i, rc := range r.Rects {
			onOther := (rc.X < params.PageW) == (side == 1)
			if onOther && rc != c.Rects[i] {
				t.Errorf("%s → %s: фото %d второй страницы сдвинулось", r.Key, c.Key, i)
			}
		}
	}
}

func rowKeys(rs []Result) map[string]Result {
	out := map[string]Result{}
	for _, r := range rs {
		out[r.Key] = r
	}
	return out
}

func TestPageVariantsSingle(t *testing.T) {
	for _, p := range invariantParams {
		for side := range 2 {
			in, _, _, ok := PageChoices("P[0|1]", []float64{P, L}, p, side, 24)
			if !ok || len(in) < 2 {
				t.Fatalf("сторона %d: страница из одного фото не листается (%d вариантов)", side, len(in))
			}
		}
	}
	// Вертикальное фото на квадратной странице: в край нельзя (обрезка 33 %),
	// а по горизонтали есть место — есть прижатые к краю.
	m := album30.Margin
	in, _, _, _ := PageChoices("P[0|1]", []float64{P, L}, album30, 0, 24)
	ks := rowKeys(in)
	fit, ok := ks["P[0|1]"]
	if !ok {
		t.Fatalf("нет вписанного варианта: %v", ks)
	}
	if _, ok := ks["P[E(0)|1]"]; ok {
		t.Errorf("в край с обрезкой 33 %% построено")
	}
	f := fit.Rects[0]
	if o, ok := ks["P[O(0)|1]"]; !ok || math.Abs(o.Rects[0].X-m) > 1e-9 || o.Rects[0].W != f.W {
		t.Errorf("к внешнему краю: %+v", o.Rects)
	}
	if i, ok := ks["P[I(0)|1]"]; !ok || math.Abs(i.Rects[0].X+i.Rects[0].W-(album30.PageW-m)) > 1e-9 {
		t.Errorf("к корешку: %+v", i.Rects)
	}
	sm, ok := ks["P[S(0)|1]"]
	if !ok || math.Abs(sm.Rects[0].W-smallScale*f.W) > 1e-9 || math.Abs(sm.Rects[0].H-smallScale*f.H) > 1e-9 {
		t.Errorf("поменьше: %+v при вписанном %+v", sm.Rects, f)
	}
	if c := sm.Rects[0]; math.Abs(c.X+c.W/2-(f.X+f.W/2)) > 1e-9 || math.Abs(c.Y+c.H/2-(f.Y+f.H/2)) > 1e-9 {
		t.Errorf("поменьше не по центру: %+v", c)
	}
	// Правая страница: внешний край справа.
	in, _, _, _ = PageChoices("P[0|1]", []float64{L, P}, album30, 1, 24)
	if o, ok := rowKeys(in)["P[0|O(1)]"]; !ok || math.Abs(o.Rects[1].X+o.Rects[1].W-(2*album30.PageW-m)) > 1e-9 {
		t.Errorf("к внешнему краю справа: %+v", o.Rects)
	}
	// Горизонтальное фото на странице 3:2: в край без обрезки, прижимать некуда.
	flat := invariantParams[1]
	in, _, _, _ = PageChoices("P[0|1]", []float64{L, P}, flat, 0, 24)
	ks = rowKeys(in)
	if e, ok := ks["P[E(0)|1]"]; !ok || e.Rects[0] != (Rect{0, 0, flat.PageW, flat.PageH}) {
		t.Errorf("в край: %v", ks)
	}
	for _, k := range []string{"P[O(0)|1]", "P[I(0)|1]"} {
		if _, ok := ks[k]; ok {
			t.Errorf("%s построен, хотя сдвигать некуда", k)
		}
	}
	// Страница в край без второй страницы — ключ B[right:0], и его тоже можно листать.
	in, _, _, ok = PageChoices("P[-|0]", []float64{S}, album30, 1, 24)
	if _, has := rowKeys(in)["B[right:0]"]; !ok || !has {
		t.Fatalf("нет B[right:0] в ряду страницы: %v", rowKeys(in))
	}
	in2, _, _, ok := PageChoices("B[right:0]", []float64{S}, album30, 1, 24)
	if !ok || len(in2) != len(in) {
		t.Errorf("B[right:0] листается не так, как P[-|0]: %d и %d", len(in2), len(in))
	}
	if !IsPaged("B[pages:1,0]") || IsPaged("B[full:0]") || IsPaged("H(0,1)") {
		t.Errorf("IsPaged ошибается")
	}
}

// На странице из нескольких фото есть ровно один вариант «поменьше» — от лучшего блока.
func TestPageVariantsSmall(t *testing.T) {
	for _, asp := range [][]float64{{L, L, P}, {P, P, P, L, L}} {
		found := false
		for _, r := range Layouts(asp, album30, 24) {
			k, _ := parseKey(r.Key)
			if k.kind != 'P' {
				continue
			}
			for side, pg := range k.pages {
				if len(pg.leaves(nil)) < 2 {
					continue
				}
				found = true
				in, _, _, _ := PageChoices(r.Key, asp, album30, side, 24)
				small := 0
				for _, c := range in {
					if ck, _ := parseKey(c.Key); ck.mods[side] == 'S' {
						small++
					}
				}
				if small != 1 {
					t.Errorf("%s сторона %d: вариантов «поменьше» %d, ждали 1", r.Key, side, small)
				}
			}
		}
		if !found {
			t.Errorf("%v: нет страницы из нескольких фото", asp)
		}
	}
}

// TestPreview печатает лучшую раскладку для типичных наборов: go test -run Preview -v
func TestPreview(t *testing.T) {
	if !testing.Verbose() {
		t.Skip()
	}
	for _, asp := range [][]float64{
		{L}, {P}, {L, L}, {P, P}, {P, P, L}, {P, P, P, P}, {L, L, L, L}, {P, L, P, L},
		{P, P, L, P, L, L, P}, {L, L, L, L, L, L},
	} {
		res := Layouts(asp, album30, 2)
		for i, r := range res {
			fmt.Printf("%v вариант %d  score=%.3f  %s\n%s\n", asp, i+1, r.Score, r.Key, ascii(r, album30))
		}
		re, block := Rearranged(asp, album30, 2)
		for i, r := range re {
			fmt.Printf("%v перестановка %d  score=%.3f  %s\n%s\n", asp, i+1, r.Score, r.Key, ascii(r, album30))
		}
		if len(re) == 0 {
			fmt.Printf("%v без перестановок (%q)\n\n", asp, block)
		}
	}
}

func ascii(r Result, p Params) string {
	const cols, rows = 60, 10
	sx, sy := cols/(2*p.PageW), rows/p.PageH
	g := make([][]byte, rows)
	for y := range g {
		g[y] = []byte(strings.Repeat(".", cols))
		g[y][cols/2] = '|'
	}
	for i, rc := range r.Rects {
		ch := byte('0' + i%10)
		for y := int(rc.Y * sy); y < int((rc.Y+rc.H)*sy) && y < rows; y++ {
			for x := int(rc.X * sx); x < int((rc.X+rc.W)*sx) && x < cols; x++ {
				g[y][x] = ch
			}
		}
	}
	var b strings.Builder
	for _, l := range g {
		b.Write(l)
		b.WriteByte('\n')
	}
	return b.String()
}

func BenchmarkEight(b *testing.B) {
	asp := []float64{P, P, L, P, L, L, P, S}
	for b.Loop() {
		Layouts(asp, album30, 12)
	}
}

func BenchmarkRearranged7(b *testing.B) {
	asp := []float64{P, L, P, L, P, L, P}
	for b.Loop() {
		resetCache()
		Rearranged(asp, album30, 24)
	}
}

func BenchmarkRearranged7Distinct(b *testing.B) {
	asp := []float64{0.5, 0.6, 0.75, 1, 1.25, 1.5, 1.8}
	for b.Loop() {
		resetCache()
		Rearranged(asp, album30, 24)
	}
}

// BenchmarkLayouts16 — худший случай Layouts: 16 случайных пропорций (5 наборов по кругу).
func BenchmarkLayouts16(b *testing.B) {
	var sets [5][]float64
	for i := range sets {
		sets[i] = goldenAspects(uint64(i), 16, 2)
	}
	i := 0
	for b.Loop() {
		Layouts(sets[i%len(sets)], album30, 24)
		i++
	}
}

// TestLayoutSheetNoSpine — лист 031 (Sheet): ширина 2·PageW без корешка. Постраничных
// вариантов нет ни в одном ряду (и в 17–64, и в перестановках, и в автосборке), Build их
// отвергает, штрафа за сгиб нет; инварианты и round-trip — как у разворота.
func TestLayoutSheetNoSpine(t *testing.T) {
	sheet := Params{PageW: 102, PageH: 304, Margin: 8, Gap: 3, Layflat: true, Sheet: true}
	if g := newGeom(sheet); g.spine != 0 {
		t.Fatalf("у листа корешок %v", g.spine)
	}
	many := func(n int) []float64 {
		a := make([]float64, n)
		for i := range a {
			a[i] = []float64{P, L, S}[i%3]
		}
		return a
	}
	cases := append(append([][]float64{}, invariantCases...), many(17), many(25), many(64))
	for _, asp := range cases {
		all := rows(asp, sheet)
		if len(all["layouts"]) == 0 {
			t.Fatalf("нет раскладок листа для %d фото", len(asp))
		}
		for name, rs := range all {
			for _, r := range rs {
				if IsPaged(r.Key) {
					t.Errorf("%d фото %s: постраничный ключ %s у листа", len(asp), name, r.Key)
				}
				checkResult(t, asp, sheet, r)
				checkRoundTrip(t, asp, sheet, name, r)
			}
		}
		if _, _, _, ok := PageChoices(all["layouts"][0].Key, asp, sheet, 0, 24); ok {
			t.Errorf("%d фото: у листа есть ряды страницы", len(asp))
		}
	}
	// Ключи страниц, которые разворот строит, лист отвергает.
	flat := sheet
	flat.Sheet = false
	for _, asp := range [][]float64{{L}, {L, P}, {P, P, L}} {
		for _, r := range Layouts(asp, flat, 0) {
			if _, ok := Build(r.Key, asp, sheet); IsPaged(r.Key) == ok {
				t.Errorf("%s: Build листа ok=%v", r.Key, ok)
			}
		}
	}
	// Широкое фото через середину: у листа без штрафа сгиба, у разворота — со штрафом.
	asp := []float64{1.2}
	r := Layouts(asp, sheet, 1)[0]
	if b, ok := Build(r.Key, asp, flat); !ok || b.Score >= r.Score {
		t.Errorf("%s: лист %.4f, разворот %.4f — штраф корешка у листа не снят", r.Key, r.Score, b.Score)
	}
	if res := Cached(asp, sheet, 1); res[0].Score != r.Score {
		t.Errorf("кэш смешал лист и разворот: %.4f вместо %.4f", res[0].Score, r.Score)
	}
	// Автосборка листа: окна 17+ без страниц не падают и покрывают все фото.
	if pl := PlanGroups(many(40), sheet, 2, 32); len(pl.Groups) != 2 || pl.Groups[1][1] != 40 {
		t.Errorf("автосборка листа: %+v", pl)
	}
}
