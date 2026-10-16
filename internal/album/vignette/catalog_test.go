package vignette

import (
	"math"
	"reflect"
	"slices"
	"testing"
)

// Тесты видов 049 (design §1.3, §7): крупно, центр, шапка в ряду, руководитель, шахматка.

// hs3 — полоса подписи в три строки 8 пт (StaffText: «Фамилия / Имя Отчество / Предмет»).
func hs3() float64 { return 1 + 3*1.2*8*0.3528 }

func TestVignetteFamilies(t *testing.T) {
	seen := map[string]bool{}
	for _, tp := range Catalog(false) {
		switch tp.Family {
		case FamSide, FamBig, FamInrow, FamGroup, FamRound, FamStaff:
		default:
			t.Fatalf("%s: вид %q", tp.ID, tp.Family)
		}
		seen[tp.Family] = true
		if len(tp.ID) > 32 { // maxTplID модели: длиннее Load обрежет
			t.Fatalf("ID %q длиннее 32", tp.ID)
		}
		round, staff := tp.Family == FamRound, tp.Family == FamStaff
		if tp.Stagger != round || (tp.Shape == ShapeOval) != round || tp.Shape != "" && tp.Shape != ShapeOval {
			t.Fatalf("%s: шахматка %v, форма %q у вида %s", tp.ID, tp.Stagger, tp.Shape, tp.Family)
		}
		if (tp.TText == StaffText) != staff || tp.TText != "" && tp.TText != StaffText || (tp.Teach == PosLead) != staff {
			t.Fatalf("%s: подпись %q, учителя %q у вида %s", tp.ID, tp.TText, tp.Teach, tp.Family)
		}
		if tp.Big != (tp.Family == FamBig) || (tp.Group != PosNone) != (tp.Family == FamGroup) {
			t.Fatalf("%s: крупно %v, групповое %q у вида %s", tp.ID, tp.Big, tp.Group, tp.Family)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("видов %d: %v", len(seen), seen)
	}
}

// TestVignetteBigTeacher — учитель крупно ≥ 2× ученика при ученике ≥ 23 мм; 2 учителя на
// развороте встают столбиком.
func TestVignetteBigTeacher(t *testing.T) {
	for _, id := range []string{"t-left-big", "head-t-left-big", "t-left-big-cell"} {
		p := Build(Find(id), input(sheetRegions(204, 304), 1, 24))
		if p.TeacherW < 2*p.PupilW || p.PupilW < 23 {
			t.Errorf("%s 1/24: учитель %.1f, ученик %.1f", id, p.TeacherW, p.PupilW)
		}
		usual := Build(Find("t-left"), input(sheetRegions(204, 304), 1, 24))
		if p.TeacherW/p.PupilW <= usual.TeacherW/usual.PupilW {
			t.Errorf("%s: ×%.2f не крупнее «Учителя слева» ×%.2f", id, p.TeacherW/p.PupilW, usual.TeacherW/usual.PupilW)
		}
	}
	p := Build(Find("t-left-big"), input(spreadRegions(), 2, 25))
	tb := boxes(p, RoleTeacher)
	if math.Abs(tb[0].X-tb[1].X) > tol || math.Abs(tb[0].Y-tb[1].Y) < tol || p.TeacherW < 2*p.PupilW {
		t.Fatalf("2 учителя на развороте не столбиком или мелко: %+v, ученик %.1f", tb, p.PupilW)
	}
}

// TestTwoThreeTeachers — 2–3 воспитателя ≥ 1,3× ученика без новых шаблонов (design §1.3.1):
// 2 — в прежних «слева/справа» на любом формате, 3 — на книжном листе; на альбомном листе и
// развороте прежние зоны дают 3 учителям ×1,0–1,2 (их геометрию 049 не меняет: листы стали
// бы «Ручными») — там 3 крупнее ставит «Учитель крупно слева».
func TestTwoThreeTeachers(t *testing.T) {
	for _, f := range [][]Rect{sheetRegions(204, 304), sheetRegions(304, 204), spreadRegions()} {
		for _, id := range []string{"t-left", "head-t-left", "t-right", "head-t-right", "t-left-big", "head-t-left-big"} {
			for _, nt := range []int{2, 3} {
				if nt == 3 && (len(f) == 2 || f[0].W > f[0].H) && !Find(id).Big {
					continue
				}
				p := Build(Find(id), input(f, nt, 24))
				if p.TeacherW < teachMore*p.PupilW-tol {
					t.Errorf("%v %s %d/24: учитель %.1f, ученик %.1f", f[0], id, nt, p.TeacherW, p.PupilW)
				}
			}
		}
	}
}

// centerSide — ученики сбоку от учителей (по вертикали перекрываются с портретом учителя):
// число слева и справа и низ нижнего портрета каждой стороны.
func centerSide(ps []Rect, tb []Rect) (nl, nr int, bl, br float64) {
	l, r := tb[0].X, tb[len(tb)-1].X+tb[len(tb)-1].W
	top, bottom := tb[0].Y, tb[0].Y+tb[0].H
	for _, b := range ps {
		if b.Y >= bottom-tol || b.Y+b.H <= top+tol || b.X > 300 && l < 300 {
			continue
		}
		if b.X+b.W <= l+tol {
			nl, bl = nl+1, math.Max(bl, b.Y+b.H)
		} else if b.X >= r-tol {
			nr, br = nr+1, math.Max(br, b.Y+b.H)
		}
	}
	return nl, nr, bl, br
}

// TestVignetteCenterRow — учитель по центру нижней полосы, бока поровну и прижаты к низу
// учителя; на развороте шапка и учитель — на левой странице.
func TestVignetteCenterRow(t *testing.T) {
	cases := []struct {
		id     string
		rs     []Rect
		nt, np int
	}{
		{"head-t-center", sheetRegions(304, 204), 1, 21},
		{"t-center", sheetRegions(304, 204), 1, 21},
		{"head-t-center", sheetRegions(204, 304), 1, 24},
		{"t-center", sheetRegions(204, 304), 2, 25},
		{"head-t-center", spreadRegions(), 1, 25},
		{"head-t-center", spreadRegions(), 2, 25},
	}
	for _, c := range cases {
		in := input(c.rs, c.nt, c.np)
		p := Build(Find(c.id), in)
		checkPlan(t, Find(c.id), in, p)
		tb, ps := boxes(p, RoleTeacher), boxes(p, RolePupil)
		r0 := c.rs[0]
		mid := (tb[0].X + tb[len(tb)-1].X + tb[len(tb)-1].W) / 2
		if math.Abs(mid-(r0.X+r0.W/2)) > 0.05 || p.TeacherW < teachMore*p.PupilW {
			t.Fatalf("%s %d/%d: учитель %+v не по центру %.2f или мелко (ученик %.1f)", c.id, c.nt, c.np, tb, r0.X+r0.W/2, p.PupilW)
		}
		tBottom := tb[0].Y + tb[0].H
		for _, b := range ps {
			if b.Y+b.H > tBottom+tol {
				t.Fatalf("%s: ученик %+v ниже учителя", c.id, b)
			}
		}
		nl, nr, bl, br := centerSide(ps, tb)
		if nl+nr == 0 || nl-nr < 0 || nl-nr > 1 || math.Abs(bl-tBottom) > tol || math.Abs(br-tBottom) > tol {
			t.Fatalf("%s %d/%d: бока %d и %d, низ %.3f и %.3f, учитель %.3f", c.id, c.nt, c.np, nl, nr, bl, br, tBottom)
		}
		// Порядок: верхние ряды раньше боковых.
		if ps[0].Y > tb[0].Y || ps[len(ps)-1].Y+ps[len(ps)-1].H < tBottom-tol && len(c.rs) == 1 {
			t.Fatalf("%s: порядок учеников %+v … %+v", c.id, ps[0], ps[len(ps)-1])
		}
		if len(c.rs) == 2 && (p.Head == nil || p.Head.X+p.Head.W > 300 || tb[len(tb)-1].X+tb[0].W > 300) {
			t.Fatalf("центр на развороте: шапка %+v, учитель %+v — не на левой", p.Head, tb)
		}
	}
}

// TestCenterFallsBack — учителей много (ряд упирается в 0,6 полосы) или нет: центр не
// лучше общей сетки — строится общая сетка.
func TestCenterFallsBack(t *testing.T) {
	for _, c := range []struct {
		center, inline string
		rs             []Rect
		nt, np         int
	}{
		{"t-center", "grid", sheetRegions(204, 304), 18, 25},
		{"head-t-center", "head", spreadRegions(), 18, 25},
		{"t-center", "grid", sheetRegions(204, 304), 0, 25},
		{"t-center", "grid", sheetRegions(204, 304), 3, 0},
	} {
		in := input(c.rs, c.nt, c.np)
		if got, want := Build(Find(c.center), in), Build(Find(c.inline), in); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s %d/%d: не общая сетка, учитель %.1f ученик %.1f", c.center, c.nt, c.np, got.TeacherW, got.PupilW)
		}
	}
}

// rowsOf — ряды мест по Y (с точностью tol), сверху вниз.
func rowsOf(cells []Rect) [][]Rect {
	sorted := slices.Clone(cells)
	slices.SortFunc(sorted, func(a, b Rect) int {
		if math.Abs(a.Y-b.Y) > tol {
			return cmpF(a.Y, b.Y)
		}
		return cmpF(a.X, b.X)
	})
	var rows [][]Rect
	for _, c := range sorted {
		if n := len(rows); n > 0 && math.Abs(rows[n-1][0].Y-c.Y) <= tol {
			rows[n-1] = append(rows[n-1], c)
			continue
		}
		rows = append(rows, []Rect{c})
	}
	return rows
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// checkHeadCell — шапка в хвосте последнего ряда своей области: ≥ 2 ячеек, справа от людей
// ряда, ниже рядов нет, ряды выше полные, ряд с шапкой той же ширины, что полный.
func checkHeadCell(t *testing.T, name string, p Plan, in Input, people []Rect, pw float64) {
	t.Helper()
	h, gx := *p.Head, math.Max(in.Gap, 2)
	if h.W < 2*pw+gx-tol {
		t.Fatalf("%s: шапка %.1f мм < 2 ячеек (%.1f)", name, h.W, pw)
	}
	var reg Rect
	for _, r := range in.Regions {
		if inside(h, r) {
			reg = r
		}
	}
	if len(in.Regions) == 2 && reg.X < 300 {
		t.Fatalf("%s: шапка в ряду на левой странице %+v", name, h)
	}
	var mine []Rect
	for _, b := range people {
		if inside(b, reg) {
			mine = append(mine, b)
		}
	}
	rows := rowsOf(mine)
	cells := int(math.Round((h.W + gx) / (pw + gx)))
	full := cells
	if n := len(rows); n > 0 && math.Abs(rows[n-1][0].Y-h.Y) <= tol {
		last := rows[n-1]
		full += len(last)
		if last[len(last)-1].X+pw > h.X+tol || math.Abs(last[0].X-rows[0][0].X) > tol {
			t.Fatalf("%s: люди ряда шапки %+v, шапка %+v", name, last, h)
		}
		rows = rows[:n-1]
	}
	for _, row := range rows {
		if row[0].Y > h.Y-tol || len(row) != full {
			t.Fatalf("%s: ряд %d мест (полный %d) на Y %.1f, шапка на %.1f", name, len(row), full, row[0].Y, h.Y)
		}
	}
}

func TestVignetteHeadCell(t *testing.T) {
	formats := [][]Rect{sheetRegions(204, 304), sheetRegions(304, 204), sheetRegions(304, 404), spreadRegions()}
	for _, id := range []string{"cell-head", "t-left-cell", "t-left-big-cell"} {
		tp := Find(id)
		for _, rs := range formats {
			for _, nt := range []int{0, 1, 2} {
				for np := 0; np+nt <= 64; np++ {
					headCellCase(t, tp, input(rs, nt, np))
				}
			}
		}
	}
}

// headCellCase — один случай TestVignetteHeadCell: людей нет — полоса сверху, иначе шапка в ряду.
func headCellCase(t *testing.T, tp Tpl, in Input) {
	t.Helper()
	p := Build(tp, in)
	if in.Teachers+in.Pupils == 0 {
		if p.HeadCell || p.Head == nil || p.Head.Y != in.Regions[0].Y {
			t.Fatalf("%s без людей: шапка %+v, в ряду %v", tp.ID, p.Head, p.HeadCell)
		}
		return
	}
	if !p.HeadCell {
		t.Fatalf("%s %v %d/%d: шапка не в ряду", tp.ID, in.Regions[0], in.Teachers, in.Pupils)
	}
	// В общей сетке — все люди (у «Сетки» и когда учеников нет), иначе ученики.
	people, pw := boxes(p, RolePupil), p.PupilW
	if tp.Teach == PosInline || in.Pupils == 0 {
		people, pw = append(people, boxes(p, RoleTeacher)...), math.Max(pw, p.TeacherW)
	}
	checkHeadCell(t, tp.ID, p, in, people, pw)
}

// TestVignetteStagger — шахматка: соседние полуряды занимают чередующиеся столбцы, шаг
// полуряда — половина ячейки с промежутком; у круга портрет квадратный.
func TestVignetteStagger(t *testing.T) {
	for _, id := range []string{"round", "head-round", "head-t-left-round"} {
		for _, rs := range [][]Rect{sheetRegions(204, 304), sheetRegions(304, 204), spreadRegions()} {
			for _, np := range []int{6, 12, 25, 33, 64} {
				in := input(rs, 0, np)
				in.Ratio = 1
				p := Build(Find(id), in)
				checkPlan(t, Find(id), in, p)
				var page []Rect // разворот: только левая страница — свой блок
				for _, b := range boxes(p, RolePupil) {
					if b.X < rs[0].X+rs[0].W {
						page = append(page, b)
					}
				}
				rows := rowsOf(page)
				hp := (p.PupilW + in.HS + math.Max(in.Gap, in.Pad)) / 2
				for i := 1; i < len(rows); i++ {
					if math.Abs(rows[i][0].Y-rows[i-1][0].Y-hp) > tol || sharesX(rows[i], rows[i-1]) {
						t.Fatalf("%s %v %d: полуряды %d и %d %+v / %+v", id, rs[0], np, i-1, i, rows[i-1], rows[i])
					}
				}
				if len(rows) < 2 {
					t.Fatalf("%s %v %d: полурядов %d", id, rs[0], np, len(rows))
				}
			}
		}
	}
}

func sharesX(a, b []Rect) bool {
	for _, x := range a {
		for _, y := range b {
			if math.Abs(x.X-y.X) < tol {
				return true
			}
		}
	}
	return false
}

// TestStaggerCapacityMatchesPlace — вместимость шахматки равна тому, что раскладка ставит, и
// все места с подписями внутри области без наложений (прототип B ставил ряды за областью).
func TestStaggerCapacityMatchesPlace(t *testing.T) {
	areas := [][]Rect{{{8, 8, 188, 288}}, {{8, 8, 288, 188}}, {{0, 0, 20, 30}}, {{12, 12, 276, 276}, {312, 12, 276, 276}},
		{{0, 0, 100, 41}}}
	for _, rs := range areas {
		for _, a := range []float64{0.75, 1} {
			g := grid{gx: 3, gy: 3, a: a, hs: hs8(), stagger: true}
			for _, pw := range []float64{4, 9.5, 17.3, 25, 33.3, 40, 61} {
				n := g.capacity(rs, pw)
				for _, k := range []int{n + 1, n, n - 1, 1} {
					cells := g.place(k, rs, pw)
					if want := min(max(k, 0), n); len(cells) != want {
						t.Fatalf("%v a %.2f pw %.1f: вместимость %d, поставлено %d из %d", rs, a, pw, n, len(cells), k)
					}
					checkCells(t, cells, rs, g.hs)
				}
			}
		}
	}
}

func checkCells(t *testing.T, cells, rs []Rect, hs float64) {
	t.Helper()
	full := make([]Rect, len(cells))
	for i, c := range cells {
		full[i] = Rect{c.X, c.Y, c.W, c.H + hs}
		if !inAny(full[i], rs) {
			t.Fatalf("место %+v с подписью вне областей", c)
		}
		for j := range i {
			if overlap(full[i], full[j]) {
				t.Fatalf("места %+v и %+v пересекаются", full[j], full[i])
			}
		}
	}
}

// TestVignetteLead — руководитель крупно: страница предметников (13/0) — руководитель ≥ 2×
// остальных; разворот — руководитель слева, остальные справа; 1/24 — класс.
func TestVignetteLead(t *testing.T) {
	staff := func(rs []Rect, nt, np int) Input {
		in := input(rs, nt, np)
		in.HST = hs3()
		return in
	}
	for _, id := range []string{"lead-left", "head-lead-left"} {
		in := staff(sheetRegions(304, 204), 13, 0)
		p := Build(Find(id), in)
		checkPlan(t, Find(id), in, p)
		tb := boxes(p, RoleTeacher)
		if p.LeadW != tb[0].W || p.TeacherW != p.LeadW || p.PupilW != 0 || distinct(widths(tb[1:])) != 1 || tb[0].W < 2*tb[1].W {
			t.Fatalf("%s 13/0: руководитель %.1f, остальные %v, ученик %.1f", id, tb[0].W, widths(tb[1:]), p.PupilW)
		}
		for _, rs := range [][]Rect{sheetRegions(304, 204), sheetRegions(204, 304)} {
			p = Build(Find(id), staff(rs, 1, 24))
			if p.LeadW < 2*p.PupilW || len(boxes(p, RolePupil)) != 24 {
				t.Fatalf("%s 1/24 %v: руководитель %.1f, ученик %.1f", id, rs[0], p.LeadW, p.PupilW)
			}
		}
	}
	// Остальные учителя — первыми в общей сетке.
	p := Build(Find("head-lead-left"), staff(sheetRegions(304, 204), 4, 20))
	tb, ps := boxes(p, RoleTeacher), boxes(p, RolePupil)
	if tb[1].Y > ps[0].Y+tol || math.Abs(tb[1].W-p.PupilW) > tol {
		t.Fatalf("учитель №2 %+v не первым в сетке (ученик %+v)", tb[1], ps[0])
	}
	// Разворот: шапка и руководитель слева, предметники справа.
	in := staff(spreadRegions(), 13, 0)
	p = Build(Find("split-lead"), in)
	checkPlan(t, Find("split-lead"), in, p)
	checkSpine(t, Find("split-lead"), in, p, true)
	if p.Head == nil || p.Head.X+p.Head.W > 300 || p.LeadW < 2*boxes(p, RoleTeacher)[1].W {
		t.Fatalf("split-lead: шапка %+v, руководитель %.1f", p.Head, p.LeadW)
	}
	// На листе Split-руководитель — тот же «Шапка, руководитель крупно».
	in = staff(sheetRegions(204, 304), 5, 20)
	if a, b := Build(Find("split-lead"), in), Build(Find("head-lead-left"), in); !reflect.DeepEqual(a, b) {
		t.Fatal("split-lead на листе не как head-lead-left")
	}
	// Учителей нет — общая сетка.
	in = staff(sheetRegions(204, 304), 0, 25)
	if p := Build(Find("head-lead-left"), in); p.LeadW != 0 || !reflect.DeepEqual(p, Build(Find("head"), in)) {
		t.Fatal("руководитель без учителей — не «Шапка и сетка»")
	}
}

func widths(bs []Rect) []float64 {
	out := make([]float64, len(bs))
	for i, b := range bs {
		out[i] = b.W
	}
	return out
}

// 33 превью на 64 местах — то, что зовёт VignetteChoices (049 §1.5).
func BenchmarkBuildNew(b *testing.B) {
	for _, f := range []struct {
		name string
		rs   []Rect
	}{{"sheet204", sheetRegions(204, 304)}, {"spread300", spreadRegions()}} {
		in := input(f.rs, 3, 61)
		tpls := Catalog(false)[20:]
		b.Run(f.name, func(b *testing.B) {
			for b.Loop() {
				for _, tp := range tpls {
					Build(tp, in)
				}
			}
		})
	}
}
