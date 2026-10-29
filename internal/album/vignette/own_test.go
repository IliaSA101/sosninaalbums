package vignette

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

// ownInput — вход с отступом подписи и полосой учителя выше ученической (как в программе).
func ownInput(rs []Rect, t, p int) Input {
	return Input{Regions: rs, Gap: 3, Pad: 1, Teachers: t, Pupils: p, Ratio: 0.75, HS: hs8(), HST: hs8() + 3.4}
}

// shapeOf — шаблон из плана каталога: то, что сохранит «Сохранить как шаблон» с нетронутой виньетки.
func shapeOf(p Plan, area Rect, ratio float64) Shape {
	sh := Shape{Aspect: area.W / area.H, Ratio: ratio}
	frac := func(r Rect) Rect {
		return Rect{(r.X - area.X) / area.W, (r.Y - area.Y) / area.H, r.W / area.W, r.H / area.H}
	}
	for _, s := range p.Slots {
		sh.Boxes = append(sh.Boxes, frac(s.Box))
		sh.Roles = append(sh.Roles, s.Role)
	}
	if p.Head != nil {
		h := frac(*p.Head)
		sh.Head = &h
	}
	return sh
}

func countRole(sh Shape, role string) int {
	n := 0
	for _, r := range sh.Roles {
		if r == role {
			n++
		}
	}
	return n
}

// checkOwn — инварианты плана: слотов столько, сколько людей и групповых, нет заглушек,
// места с подписями внутри областей, без пересечений друг с другом и с шапкой.
func checkOwn(t *testing.T, name string, p Plan, in Input, groups int) {
	t.Helper()
	if len(p.Slots) != in.Teachers+in.Pupils+groups {
		t.Errorf("%s: слотов %d, надо %d", name, len(p.Slots), in.Teachers+in.Pupils+groups)
	}
	var cells []Rect
	for i, s := range p.Slots {
		if s.Box.W == 0 {
			t.Errorf("%s: слот %d (%s) — заглушка", name, i, s.Role)
			continue
		}
		c := withCaption(s, in)
		if !inAny(c, in.Regions) {
			t.Errorf("%s: слот %d (%s) вне областей: %+v", name, i, s.Role, c)
		}
		for _, o := range cells {
			if overlap(o, c) {
				t.Errorf("%s: перекрытие %+v и %+v", name, o, c)
			}
		}
		if p.Head != nil && overlap(*p.Head, c) {
			t.Errorf("%s: слот %d на шапке", name, i)
		}
		cells = append(cells, c)
	}
}

// ownSource — исходная виньетка шаблона каталога и сочетания чисел, куда её переносят.
type ownSource struct {
	tpl      string
	area     Rect
	rs       []Rect
	teachers int
	pupils   int
}

type ownDst struct {
	teachers, pupils int
	area             Rect
	rs               []Rect
}

var (
	areaSheet  = Rect{0, 0, 204, 304}
	areaSheet2 = Rect{0, 0, 304, 404}
	areaSpread = Rect{0, 0, 600, 300}
)

func ownSources() []ownSource {
	rsSheet, rsSpread := sheetRegions(204, 304), spreadRegions()
	return []ownSource{
		{DefaultTpl, areaSheet, rsSheet, 1, 25},
		{"head", areaSheet, rsSheet, 1, 25},
		{"t-top", areaSheet, rsSheet, 2, 25},
		{"g-bottom", areaSheet, rsSheet, 1, 25},
		{"split-t", areaSpread, rsSpread, 2, 28},
		{"head-t-left-g", areaSpread, rsSpread, 2, 28},
	}
}

// ownDsts — 8 сочетаний: тот же класс, меньше и больше учеников, учителей больше и ни одного,
// другой формат листа; у разворота — свои.
func ownDsts(s ownSource) []ownDst {
	if len(s.rs) == 1 {
		rs1, rs2 := s.rs, sheetRegions(304, 404)
		return []ownDst{
			{s.teachers, s.pupils, s.area, rs1}, {s.teachers, 20, s.area, rs1}, {s.teachers, 30, s.area, rs1},
			{2, 25, s.area, rs1}, {0, 25, s.area, rs1}, {3, 18, s.area, rs1},
			{s.teachers, s.pupils, areaSheet2, rs2}, {1, 33, areaSheet2, rs2},
		}
	}
	return []ownDst{
		{s.teachers, s.pupils, s.area, s.rs}, {s.teachers, 20, s.area, s.rs}, {s.teachers, 40, s.area, s.rs},
		{1, 40, s.area, s.rs}, {0, 28, s.area, s.rs}, {3, 28, s.area, s.rs},
		{s.teachers, 1, s.area, s.rs}, {4, 12, s.area, s.rs},
	}
}

// 6 шаблонов каталога → свой шаблон → 8 сочетаний: без перекрытий и вылетов за область;
// при исходных числах и формате (точный режим) места равны исходному плану.
func TestOwnCatalogMatrix(t *testing.T) {
	for _, s := range ownSources() {
		src := Build(Find(s.tpl), ownInput(s.rs, s.teachers, s.pupils))
		sh := shapeOf(src, s.area, 0.75)
		groups := countRole(sh, RoleGroup)
		for _, d := range ownDsts(s) {
			in := ownInput(d.rs, d.teachers, d.pupils)
			name := fmt.Sprintf("%s %gx%g T%d P%d", s.tpl, d.area.W, d.area.H, d.teachers, d.pupils)
			got := BuildOwn(sh, d.area, in)
			checkOwn(t, name, got, in, groups)
			if d.area == s.area && d.teachers == s.teachers && d.pupils == s.pupils {
				if !reflect.DeepEqual(roundPlan(got), roundPlan(src)) {
					t.Errorf("%s: точный режим не равен исходному плану", name)
				}
			}
		}
	}
}

// roundPlan — план с округлением до 1e-6 (доли туда-обратно).
func roundPlan(p Plan) Plan {
	r := func(v float64) float64 { return math.Round(v*1e6) / 1e6 }
	rr := func(b Rect) Rect { return Rect{r(b.X), r(b.Y), r(b.W), r(b.H)} }
	out := Plan{PupilW: r(p.PupilW), TeacherW: r(p.TeacherW)}
	for _, s := range p.Slots {
		out.Slots = append(out.Slots, Slot{rr(s.Box), s.Role})
	}
	if p.Head != nil {
		h := rr(*p.Head)
		out.Head = &h
	}
	return out
}

// Все шаблоны каталога (лист и разворот) как свои: перенос на другие числа держит инварианты.
func TestOwnWholeCatalog(t *testing.T) {
	for _, sheet := range []bool{true, false} {
		area, rs := areaSpread, spreadRegions()
		if sheet {
			area, rs = areaSheet, sheetRegions(204, 304)
		}
		for _, tp := range Catalog(sheet) {
			sh := shapeOf(Build(tp, ownInput(rs, 2, 26)), area, 0.75)
			groups := countRole(sh, RoleGroup)
			for _, c := range [][2]int{{2, 26}, {1, 14}, {0, 30}, {3, 38}, {2, 1}} {
				in := ownInput(rs, c[0], c[1])
				checkOwn(t, fmt.Sprintf("%s T%d P%d", tp.ID, c[0], c[1]), BuildOwn(sh, area, in), in, groups)
			}
		}
	}
}

// Точный режим — и когда исходный план собран с другими числами подписей: места берутся из шаблона,
// а не пересчитываются; повторный вызов даёт тот же план, вход не меняется.
func TestOwnExactIdempotent(t *testing.T) {
	rs := sheetRegions(204, 304)
	src := Build(Find(DefaultTpl), ownInput(rs, 1, 25))
	sh := shapeOf(src, areaSheet, 0.75)
	boxes0 := append([]Rect(nil), sh.Boxes...)
	head0 := *sh.Head
	in := ownInput(rs, 1, 25)
	a, b := BuildOwn(sh, areaSheet, in), BuildOwn(sh, areaSheet, in)
	if !reflect.DeepEqual(a, b) {
		t.Error("повторный вызов дал другой план")
	}
	if !reflect.DeepEqual(sh.Boxes, boxes0) || *sh.Head != head0 {
		t.Error("BuildOwn изменил шаблон")
	}
	// пропорция области в пределах 2 % — всё ещё точный режим
	near := Rect{0, 0, 204 * 1.015, 304}
	if p := BuildOwn(sh, near, in); p.PupilW <= 0 || len(p.Slots) != 26 {
		t.Errorf("область в пределах 2%%: %+v", p.PupilW)
	}
	// другой портрет — не точный режим: ширина портрета меняется вместе с пропорцией
	in2 := in
	in2.Ratio = 1
	if p := BuildOwn(sh, areaSheet, in2); reflect.DeepEqual(roundPlan(p), roundPlan(src)) {
		t.Error("другой портрет оставил исходный план")
	}
}

// Места листа округлены до 0,01 мм (зажим места), а нижний ряд плана стоит вплотную к краю
// области: шаблон с такого листа всё равно точный (fitTol), все шаблоны каталога.
func TestOwnExactAfterMMRounding(t *testing.T) {
	for _, sheet := range []bool{true, false} {
		area, rs := areaSpread, spreadRegions()
		if sheet {
			area, rs = areaSheet, sheetRegions(204, 304)
		}
		for _, tp := range Catalog(sheet) {
			in := ownInput(rs, 2, 26)
			src := Build(tp, in)
			for i := range src.Slots {
				b := &src.Slots[i].Box
				// как ClampPlace: X, Y, W, H до сотых мм (край может выступить за область на ≤ 0,01)
				b.X, b.Y = math.Round(b.X*100)/100, math.Round(b.Y*100)/100
				b.W, b.H = math.Round(b.W*100)/100, math.Round(b.H*100)/100
			}
			got := BuildOwn(shapeOf(src, area, 0.75), area, in)
			if !probePlanEqual(got, src) {
				t.Errorf("%s (лист %v): округление мест до 0,01 мм увело шаблон из точного режима", tp.ID, sheet)
			}
		}
	}
}

// Учителей стало больше, чем мест у шаблона: зона растёт, портрет не падает до размера одной ячейки.
func TestOwnZoneGrows(t *testing.T) {
	rs := sheetRegions(204, 304)
	src := Build(Find("t-left"), ownInput(rs, 1, 25))
	sh := shapeOf(src, areaSheet, 0.75)
	one := src.TeacherW
	in := ownInput(rs, 2, 25)
	got := BuildOwn(sh, areaSheet, in)
	checkOwn(t, "2 учителя", got, in, 0)
	if got.TeacherW <= one*0.7 {
		t.Errorf("портрет учителя при 2 вместо 1: %.1f, у одного %.1f — зона не выросла", got.TeacherW, one)
	}
}

// Учитель не крупнее двух портретов ученика, как у каталога (учитель один, ученик мелкий).
func TestOwnTeacherCap(t *testing.T) {
	rs := sheetRegions(204, 304)
	src := Build(Find("t-top"), ownInput(rs, 1, 12))
	sh := shapeOf(src, areaSheet, 0.75)
	for _, p := range []int{40, 55, 63} {
		in := ownInput(rs, 1, p)
		got := BuildOwn(sh, areaSheet, in)
		checkOwn(t, fmt.Sprintf("P%d", p), got, in, 0)
		if got.PupilW <= 0 || got.TeacherW > teachMax*got.PupilW+tol {
			t.Errorf("P%d: учитель %.1f крупнее %v × ученик %.1f", p, got.TeacherW, teachMax, got.PupilW)
		}
	}
}

// Дыра у края < 10 % области заполняется (рамка дотягивается до края), больший отступ — замысел.
func TestOwnEdgeSnap(t *testing.T) {
	area := Rect{0, 0, 200, 300}
	rs := []Rect{area}
	in := ownInput(rs, 0, 20)
	in.HS = 8
	mk := func(bottom float64) Shape {
		// ряд из 5 учеников сверху; нижний край мест с подписями — bottom (доля высоты)
		sh := Shape{Aspect: 200.0 / 300, Ratio: 0.75}
		for i := range 5 {
			sh.Boxes = append(sh.Boxes, Rect{0.02 + float64(i)*0.19, 0.02, 0.17, bottom - 0.02 - in.HS/300})
			sh.Roles = append(sh.Roles, RolePupil)
		}
		return sh
	}
	small := BuildOwn(mk(0.95), area, in) // дыра снизу и справа 5 % — дотягивается до края
	full := BuildOwn(mk(1), area, in)     // композиция до края области
	large := BuildOwn(mk(0.70), area, in) // дыра 30 % — замысел, остаётся
	bottomOf := func(p Plan) float64 {
		m := 0.0
		for _, s := range p.Slots {
			m = math.Max(m, s.Box.Y+s.Box.H+in.HS)
		}
		return m
	}
	checkOwn(t, "5 %", small, in, 0)
	checkOwn(t, "30 %", large, in, 0)
	if bottomOf(large) > 0.70*300+tol {
		t.Errorf("зона вышла за замысел: низ %.1f при границе %.1f", bottomOf(large), 0.70*300)
	}
	if math.Abs(small.PupilW-full.PupilW) > tol {
		t.Errorf("дыра 5 %% не заполнена: портрет %.2f, у композиции до края %.2f", small.PupilW, full.PupilW)
	}
	if large.PupilW >= small.PupilW {
		t.Errorf("зона 30 %% выросла: портрет %.2f, у 5 %% %.2f", large.PupilW, small.PupilW)
	}
}

// Групповое посреди класса — препятствие внутри зоны: ученики обходят его, групповое на месте.
func TestOwnObstacle(t *testing.T) {
	rs := sheetRegions(204, 304)
	sh := shapeOf(Build(Find("grid"), ownInput(rs, 0, 24)), areaSheet, 0.75)
	grp := Rect{0.3, 0.4, 0.4, 0.4 * 204 / 304 / 1.5}
	sh.Boxes = append(sh.Boxes, grp)
	sh.Roles = append(sh.Roles, RoleGroup)
	for _, p := range []int{16, 20, 23, 30} {
		in := ownInput(rs, 0, p)
		got := BuildOwn(sh, areaSheet, in)
		checkOwn(t, fmt.Sprintf("P%d", p), got, in, 1)
		last := got.Slots[len(got.Slots)-1]
		if last.Role != RoleGroup || last.Box != toMM(areaSheet, grp) {
			t.Errorf("P%d: групповое сдвинуто: %+v", p, last)
		}
	}
}

// Края: нет учителей, один ученик, 64 места, пустой шаблон, шаблон без мест одной роли.
func TestOwnEdges(t *testing.T) {
	rs := sheetRegions(204, 304)
	sh := shapeOf(Build(Find(DefaultTpl), ownInput(rs, 1, 25)), areaSheet, 0.75)
	for _, c := range [][2]int{{0, 25}, {1, 1}, {0, 1}, {1, 0}, {0, 0}, {2, 62}, {0, 64}, {1, 63}} {
		in := ownInput(rs, c[0], c[1])
		got := BuildOwn(sh, areaSheet, in)
		if len(got.Slots) != c[0]+c[1] {
			t.Errorf("T%d P%d: слотов %d", c[0], c[1], len(got.Slots))
		}
		if c[0]+c[1] > 0 && c[0]+c[1] <= 48 {
			checkOwn(t, fmt.Sprintf("T%d P%d", c[0], c[1]), got, in, 0)
		}
	}
	// 64 места на развороте
	sp := spreadRegions()
	shSp := shapeOf(Build(Find("head-t-left-g"), ownInput(sp, 2, 28)), areaSpread, 0.75)
	in := ownInput(sp, 2, 62)
	checkOwn(t, "разворот 64", BuildOwn(shSp, areaSpread, in), in, 1)
	// шаблон только с учителями: ученики встают в общую сетку
	shT := Shape{Boxes: []Rect{{0.1, 0.1, 0.3, 0.3}}, Roles: []string{RoleTeacher}, Aspect: 204.0 / 304, Ratio: 0.75}
	in = ownInput(rs, 1, 10)
	checkOwn(t, "только учителя", BuildOwn(shT, areaSheet, in), in, 0)
	// пустой шаблон: заглушки в углу, без паники
	got := BuildOwn(Shape{Aspect: 1, Ratio: 0.75}, areaSheet, ownInput(rs, 1, 3))
	if len(got.Slots) != 4 || got.Head != nil {
		t.Errorf("пустой шаблон: %+v", got)
	}
}

// Мусор на входе: роли короче мест, NaN и Inf в долях, нулевая область, нет областей.
func TestOwnDegenerate(t *testing.T) {
	rs := sheetRegions(204, 304)
	nan, inf := math.NaN(), math.Inf(1)
	sh := Shape{
		Boxes:  []Rect{{0.1, 0.1, 0.3, 0.3}, {nan, 0, 0.1, 0.1}, {0.5, 0.5, inf, 0.1}, {0.6, 0.6, -1, 0.1}, {0.5, 0.1, 0.3, 0.3}},
		Roles:  []string{RoleTeacher}, // короче мест: остальные — ученики
		Head:   &Rect{0, 0, nan, 0.1},
		Aspect: 204.0 / 304, Ratio: 0.75,
	}
	in := ownInput(rs, 1, 5)
	got := BuildOwn(sh, areaSheet, in)
	if len(got.Slots) != 6 || got.Head != nil {
		t.Errorf("мусор: слотов %d, шапка %v", len(got.Slots), got.Head)
	}
	for _, area := range []Rect{{}, {0, 0, -5, 10}, {0, 0, nan, 10}, {0, 0, inf, 10}} {
		if p := BuildOwn(sh, area, in); len(p.Slots) != 6 {
			t.Errorf("область %+v: слотов %d", area, len(p.Slots))
		}
	}
	if p := BuildOwn(sh, areaSheet, Input{Teachers: 1, Pupils: 2}); len(p.Slots) != 0 {
		t.Errorf("нет областей: %+v", p)
	}
	for _, r := range []float64{0, -1, nan, inf} {
		in := ownInput(rs, 1, 5)
		in.Ratio = r
		if p := BuildOwn(sh, areaSheet, in); len(p.Slots) != 6 {
			t.Errorf("портрет %v: слотов %d", r, len(p.Slots))
		}
	}
}

// Зонды из варианта B (замер go 22): 6 шаблонов каталога, форматы листа и разворота.
var probeTemplates = []struct {
	tpl    string
	spread bool
}{
	{"head-t-left", false}, {"g-top", false}, {"t-top", false}, {"head", false},
	{"split-t", true}, {"head-t-left-g", true},
}

type probeFmt struct {
	name string
	area Rect
	rs   []Rect
}

func probeSheet(w, h float64) probeFmt {
	return probeFmt{fmt.Sprintf("%gx%g", w, h), Rect{0, 0, w, h}, []Rect{{8, 8, w - 16, h - 16}}}
}

func probeSpread(w, h float64) probeFmt {
	return probeFmt{fmt.Sprintf("2x%gx%g", w/2, h), Rect{0, 0, w, h},
		[]Rect{{12, 12, w/2 - 24, h - 24}, {w/2 + 12, 12, w/2 - 24, h - 24}}}
}

func probeFormats(spread bool) []probeFmt {
	if spread {
		return []probeFmt{probeSpread(600, 300)}
	}
	return []probeFmt{probeSheet(204, 304), probeSheet(304, 404)}
}

func probePlanEqual(a, b Plan) bool {
	if len(a.Slots) != len(b.Slots) {
		return false
	}
	for i := range a.Slots {
		x, y := a.Slots[i].Box, b.Slots[i].Box
		if a.Slots[i].Role != b.Slots[i].Role ||
			math.Abs(x.X-y.X)+math.Abs(x.Y-y.Y)+math.Abs(x.W-y.W)+math.Abs(x.H-y.H) > 1e-6 {
			return false
		}
	}
	return true
}

// TestOwnExactB: свой шаблон из плана каталога с теми же числами и форматом — тот же план (±1e-6).
func TestOwnExactB(t *testing.T) {
	for _, c := range probeTemplates {
		f := probeFormats(c.spread)[0]
		for _, n := range [][2]int{{1, 20}, {2, 25}, {0, 25}, {3, 18}} {
			in := ownInput(f.rs, n[0], n[1])
			pl := Build(Find(c.tpl), in)
			own := BuildOwn(shapeOf(pl, f.area, in.Ratio), f.area, in)
			if !probePlanEqual(own, pl) {
				t.Errorf("%s %s T%d P%d: точный режим не совпал с исходным планом", c.tpl, f.name, n[0], n[1])
			}
			if (own.Head == nil) != (pl.Head == nil) {
				t.Errorf("%s: шапка не совпала", c.tpl)
			}
		}
	}
}

// Групповое у нижнего края области: полоса подписи не выходит за область (обрезка), а точный
// режим при другом размере листа той же пропорции не оставляет места как есть.
func TestOwnGroupClipAndScaledSheet(t *testing.T) {
	rs := sheetRegions(204, 304)
	sh := Shape{Aspect: 204.0 / 304, Ratio: 0.75, Boxes: []Rect{{0.1, 0.9, 0.5, 0.08}}, Roles: []string{RoleGroup}}
	in := ownInput(rs, 0, 0)
	p := BuildOwn(sh, areaSheet, in)
	if len(p.Slots) != 1 || !inAny(withCaption(p.Slots[0], in), rs) {
		t.Errorf("групповое с полосой вне области: %+v", p.Slots)
	}
	if want := (Rect{20.4, 273.6, 102, 24.32}); p.Slots[0].Box.W != want.W || p.Slots[0].Box.H >= want.H {
		t.Errorf("групповое не обрезано: %+v", p.Slots[0].Box)
	}
	// тот же шаблон, пропорция та же, лист вдвое меньше: ученики не «как есть» из долей
	src := Build(Find("head"), ownInput(rs, 1, 25))
	sh2 := shapeOf(src, areaSheet, 0.75)
	half := Rect{0, 0, 102, 152}
	rs2 := []Rect{{8, 8, 86, 136}}
	in2 := ownInput(rs2, 1, 25)
	checkOwn(t, "лист вдвое меньше", BuildOwn(sh2, half, in2), in2, 0)
}

// TestOwnGeometryB: 6 шаблонов × 5 сочетаний чисел × форматы листа и разворота (в т. ч. другая
// пропорция и размер) и портрет 1:1.
func TestOwnGeometryB(t *testing.T) {
	nums := [][2]int{{1, 20}, {1, 30}, {2, 25}, {0, 25}, {3, 18}}
	for _, c := range probeTemplates {
		src := probeFormats(c.spread)[0]
		sh := shapeOf(Build(Find(c.tpl), ownInput(src.rs, 1, 25)), src.area, 0.75)
		dsts := probeFormats(c.spread)
		if c.spread {
			dsts = append(dsts, probeSpread(600, 400), probeSpread(300, 150))
		}
		for _, d := range dsts {
			for _, n := range nums {
				in := ownInput(d.rs, n[0], n[1])
				name := fmt.Sprintf("%s %s T%d P%d", c.tpl, d.name, n[0], n[1])
				checkOwn(t, name, BuildOwn(sh, d.area, in), in, countRole(sh, RoleGroup))
			}
		}
		in := ownInput(src.rs, 1, 25)
		in.Ratio = 1
		checkOwn(t, c.tpl+" портрет 1:1", BuildOwn(sh, src.area, in), in, countRole(sh, RoleGroup))
	}
}

// Страница рамки — по её центру (граница своя); вне всех областей, в том числе в зазоре, — последняя.
// inter: общая часть; касание краем, зазор и нулевая ширина — не пересечение, в любом порядке аргументов.
func TestOwnPageOfInter(t *testing.T) {
	c := &ownCtx{in: Input{Regions: []Rect{{0, 0, 100, 100}, {110, 0, 100, 100}}}}
	for name, tc := range map[string]struct {
		b    Rect
		want int
	}{
		"левая":         {Rect{10, 10, 20, 20}, 0},
		"правая":        {Rect{120, 10, 20, 20}, 1},
		"центр на краю": {Rect{90, 40, 20, 20}, 0},
		"в зазоре":      {Rect{101, 40, 8, 20}, 1},
		"вне, ниже":     {Rect{10, 500, 20, 20}, 1},
		"вне, левее":    {Rect{-50, 10, 20, 20}, 1},
	} {
		if got := c.pageOf(tc.b); got != tc.want {
			t.Errorf("%s: страница %d, ждали %d", name, got, tc.want)
		}
	}
	a := Rect{0, 0, 10, 10}
	for name, tc := range map[string]struct {
		b    Rect
		ok   bool
		want Rect
	}{
		"пересечение":  {Rect{5, 6, 10, 10}, true, Rect{5, 6, 5, 4}},
		"внутри":       {Rect{2, 2, 3, 3}, true, Rect{2, 2, 3, 3}},
		"касание":      {Rect{10, 0, 5, 5}, false, Rect{}},
		"зазор по X":   {Rect{11, 0, 5, 5}, false, Rect{}},
		"зазор по Y":   {Rect{0, 20, 5, 5}, false, Rect{}},
		"нулевая шире": {Rect{3, 3, 0, 5}, false, Rect{}},
	} {
		z, ok := inter(a, tc.b)
		z2, ok2 := inter(tc.b, a)
		if ok != tc.ok || z != tc.want || ok2 != ok || z2 != z {
			t.Errorf("%s: %+v %v, обратно %+v %v, ждали %+v %v", name, z, ok, z2, ok2, tc.want, tc.ok)
		}
	}
}

// Точный режим по размеру (жалоба 0.11): шаблон с размером листа на листе того же размера (±0,05 мм),
// с тем же портретом и числами ставит места ровно как сохранены — ученика на шапке, наложение
// полосы подписи, групповое за полем; без размера, с другим размером, числом или портретом — пересчёт.
func TestOwnExactSize(t *testing.T) {
	in := ownInput(sheetRegions(204, 304), 1, 24)
	sh := shapeOf(Build(Find(DefaultTpl), in), areaSheet, 0.75)
	p0 := slices.Index(sh.Roles, RolePupil)
	sh.Boxes[p0].Y = 20.0 / 304                            // ученик на шапке
	sh.Boxes[p0+1].Y += 3.0 / 304                          // полоса подписи на следующем ряду
	sh.Boxes = append(sh.Boxes, Rect{0.1, 0.9, 0.5, 0.09}) // групповое: с полосой за полем
	sh.Roles = append(sh.Roles, RoleGroup)
	sh.W, sh.H = 204, 304
	want := func(i int) Rect { return toMM(areaSheet, sh.Boxes[i]) }
	got := BuildOwn(sh, areaSheet, in)
	if len(got.Slots) != len(sh.Boxes) || got.Head == nil || *got.Head != toMM(areaSheet, *sh.Head) || got.PupilW != want(p0).W {
		t.Fatalf("слотов %d из %d, шапка %+v, портрет %v", len(got.Slots), len(sh.Boxes), got.Head, got.PupilW)
	}
	byRole := map[string][]Rect{}
	for _, s := range got.Slots {
		byRole[s.Role] = append(byRole[s.Role], s.Box)
	}
	for i, r := range sh.Roles {
		if b := byRole[r][0]; b != want(i) {
			t.Fatalf("место %d (%s): %+v, сохранено %+v", i, r, b, want(i))
		}
		byRole[r] = byRole[r][1:]
	}
	near := Rect{0, 0, 204.04, 303.96}
	if !slices.ContainsFunc(BuildOwn(sh, near, in).Slots, func(s Slot) bool { return s.Box == toMM(near, sh.Boxes[p0]) }) {
		t.Error("размер в допуске 0,05 мм — не точный режим")
	}
	in2, in3, old, nan := in, in, sh, sh
	in2.Pupils--
	in3.Ratio = 1
	old.W, old.H = 0, 0
	nan.W = math.NaN()
	for name, c := range map[string]struct {
		sh   Shape
		area Rect
		in   Input
	}{"без размера": {old, areaSheet, in}, "NaN": {nan, areaSheet, in}, "лист на 1 мм шире": {sh, Rect{0, 0, 205, 304}, in},
		"меньше учеников": {sh, areaSheet, in2}, "другой портрет": {sh, areaSheet, in3}} {
		if slices.ContainsFunc(BuildOwn(c.sh, c.area, c.in).Slots, func(s Slot) bool { return s.Box == want(p0) }) {
			t.Errorf("%s: ученик на шапке остался как сохранён — ждали пересчёт", name)
		}
	}
}
