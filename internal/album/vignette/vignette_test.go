package vignette

import (
	"fmt"
	"math"
	"testing"
)

const tol = 1e-6

// hs8 — полоса подписи «отступ 1 + две строки 8 пт» (как в прототипе 031 design §1).
func hs8() float64 { return 1 + 2*1.2*8*0.3528 }

// sheetRegions — лист w×h с полем 8 мм.
func sheetRegions(w, h float64) []Rect { return []Rect{{8, 8, w - 16, h - 16}} }

// spreadRegions — разворот 2×300×300, поле 12 мм и у корешка (x = 300).
func spreadRegions() []Rect { return []Rect{{12, 12, 276, 276}, {312, 12, 276, 276}} }

func input(rs []Rect, t, p int) Input {
	return Input{Regions: rs, Gap: 3, Teachers: t, Pupils: p, Ratio: 0.75, HS: hs8(), HST: hs8() + 1.5}
}

// distinct — число разных значений с точностью до tol.
func distinct(vs []float64) int {
	var seen []float64
	for _, v := range vs {
		dup := false
		for _, s := range seen {
			if math.Abs(s-v) < tol {
				dup = true
			}
		}
		if !dup {
			seen = append(seen, v)
		}
	}
	return len(seen)
}

func boxes(p Plan, role string) []Rect {
	var out []Rect
	for _, s := range p.Slots {
		if s.Role == role {
			out = append(out, s.Box)
		}
	}
	return out
}

func TestVignetteGridFit(t *testing.T) {
	cases := []struct {
		name       string
		rs         []Rect
		n          int
		cols, rows int     // 0 — не проверять
		minW       float64 // прототип 031 design §1 (там округлено до 0,1)
	}{
		{"204x304/25", sheetRegions(204, 304), 25, 5, 5, 35.15},
		{"204x304/33", sheetRegions(204, 304), 33, 6, 6, 28.25},
		{"204x304/64", sheetRegions(204, 304), 64, 8, 8, 19.15},
		{"304x404/25", sheetRegions(304, 404), 25, 5, 5, 50.55},
		{"304x404/64", sheetRegions(304, 404), 64, 8, 8, 28.55},
		{"spread/25", spreadRegions(), 25, 0, 0, 52.75},
		{"spread/64", spreadRegions(), 64, 0, 0, 33.75},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Build(Find("grid"), input(c.rs, 0, c.n))
			ps := boxes(p, RolePupil)
			if len(ps) != c.n {
				t.Fatalf("учеников %d, want %d", len(ps), c.n)
			}
			if p.PupilW < c.minW {
				t.Errorf("портрет %.2f мм, want ≥ %.2f", p.PupilW, c.minW)
			}
			var ys []float64
			perRow := map[int]int{} // ряд — по Y с точностью до мм
			cols := 0
			for _, b := range ps {
				ys = append(ys, b.Y)
				perRow[int(math.Round(b.Y))]++
				cols = max(cols, perRow[int(math.Round(b.Y))])
				if math.Abs(b.W-p.PupilW) > tol {
					t.Errorf("ширина %.3f ≠ PupilW %.3f", b.W, p.PupilW)
				}
			}
			if c.cols > 0 && (cols != c.cols || distinct(ys) != c.rows) {
				t.Errorf("сетка %d×%d, want %d×%d", cols, distinct(ys), c.cols, c.rows)
			}
		})
	}
}

// overlap — пересечение по площади (касание краями не считается).
func overlap(a, b Rect) bool {
	return a.X < b.X+b.W-tol && b.X < a.X+a.W-tol && a.Y < b.Y+b.H-tol && b.Y < a.Y+a.H-tol
}

func inside(r, area Rect) bool {
	return r.X >= area.X-tol && r.Y >= area.Y-tol && r.X+r.W <= area.X+area.W+tol && r.Y+r.H <= area.Y+area.H+tol
}

// withCaption — рамка места вместе с полосой подписи под ней.
func withCaption(s Slot, in Input) Rect {
	hs := in.HS
	if s.Role == RoleTeacher {
		hs = in.HST
	}
	r := s.Box
	r.H += hs
	return r
}

// checkPlan — инварианты плана: число и роли мест, пропорции, места с подписями внутри
// областей (безопасная зона), без пересечений, шапка отдельно, сгиб свободен.
func checkPlan(t *testing.T, tp Tpl, in Input, p Plan) {
	t.Helper()
	want := map[string]int{RolePupil: in.Pupils, RoleTeacher: in.Teachers}
	if tp.Group != PosNone {
		want[RoleGroup] = 1
	}
	got := map[string]int{}
	for _, s := range p.Slots {
		got[s.Role]++
	}
	for _, role := range []string{RolePupil, RoleTeacher, RoleGroup} {
		if got[role] != want[role] {
			t.Fatalf("%s: %d, want %d", role, got[role], want[role])
		}
	}
	if (p.Head != nil) != (tp.Head != PosNone) {
		t.Fatalf("шапка %v у шаблона %q", p.Head != nil, tp.Head)
	}
	var full []Rect
	for i, s := range p.Slots {
		checkSlot(t, s, in, p)
		r := withCaption(s, in)
		if !inAny(r, in.Regions) {
			t.Fatalf("место %d %s %+v вне областей", i, s.Role, r)
		}
		full = append(full, r)
	}
	if p.Head != nil {
		if !inAny(*p.Head, in.Regions) {
			t.Fatalf("шапка %+v вне областей", *p.Head)
		}
		full = append(full, *p.Head)
	}
	for i := range full {
		for j := i + 1; j < len(full); j++ {
			if overlap(full[i], full[j]) {
				t.Fatalf("пересекаются %d %+v и %d %+v", i, full[i], j, full[j])
			}
		}
	}
}

func checkSlot(t *testing.T, s Slot, in Input, p Plan) {
	t.Helper()
	b := s.Box
	if b.W <= 0 || b.H <= 0 {
		t.Fatalf("%s: пустая рамка %+v", s.Role, b)
	}
	a := in.Ratio
	switch s.Role {
	case RoleGroup:
		a = groupA
	case RolePupil:
		if math.Abs(b.W-p.PupilW) > tol {
			t.Fatalf("ученик %.3f ≠ PupilW %.3f", b.W, p.PupilW)
		}
	case RoleTeacher:
		if math.Abs(b.W-p.TeacherW) > tol {
			t.Fatalf("учитель %.3f ≠ TeacherW %.3f", b.W, p.TeacherW)
		}
	}
	if math.Abs(b.W/b.H-a) > 1e-6 {
		t.Fatalf("%s: пропорция %.4f, want %.4f", s.Role, b.W/b.H, a)
	}
	if s.Role == RolePupil && b.W < 3 {
		t.Fatalf("ученик %.2f мм — генератор сдался", b.W)
	}
}

func inAny(r Rect, rs []Rect) bool {
	for _, area := range rs {
		if inside(r, area) {
			return true
		}
	}
	return false
}

func TestVignetteTemplatesGeometry(t *testing.T) {
	formats := []struct {
		name string
		rs   []Rect
	}{
		{"204x304", sheetRegions(204, 304)},
		{"304x404", sheetRegions(304, 404)},
		{"spread300", spreadRegions()},
	}
	for _, f := range formats {
		for _, tp := range Catalog(false) {
			for _, ratio := range []string{"2:3", "3:4", "4:5", "1:1"} {
				for _, nt := range []int{0, 1, 2, 3, 5, 18} {
					for _, np := range []int{0, 1, 6, 25, 33, 64} {
						if nt+np > 64 {
							continue
						}
						name := fmt.Sprintf("%s/%s/%s/t%d/p%d", f.name, tp.ID, ratio, nt, np)
						in := input(f.rs, nt, np)
						in.Ratio = ParseRatio(ratio)
						p := Build(tp, in)
						t.Run(name, func(t *testing.T) {
							checkPlan(t, tp, in, p)
							checkSpine(t, tp, in, p, len(f.rs) == 2)
						})
					}
				}
			}
		}
	}
}

// checkSpine — на развороте места не пересекают сгиб (x = 300), у Split с местами слева
// класс — только на правой странице.
func checkSpine(t *testing.T, tp Tpl, in Input, p Plan, spread bool) {
	t.Helper()
	if !spread {
		return
	}
	for _, s := range p.Slots {
		if s.Box.X < 300-tol && s.Box.X+s.Box.W > 300+tol {
			t.Fatalf("%s на сгибе: %+v", s.Role, s.Box)
		}
	}
	leftBusy := (tp.Teach == PosPage && in.Teachers > 0) || tp.Group == PosPage
	if !tp.Split || !leftBusy {
		return
	}
	for _, b := range boxes(p, RolePupil) {
		if b.X < 300 {
			t.Fatalf("Split: ученик на левой странице %+v", b)
		}
	}
}

func TestVignetteOnePupil(t *testing.T) {
	in := input(sheetRegions(204, 304), 0, 1)
	p := Build(Find("grid"), in)
	checkPlan(t, Find("grid"), in, p)
	want := math.Min(188, (288-in.HS)*0.75)
	if b := p.Slots[0].Box; math.Abs(b.W-want) > 1e-3 || math.Abs(b.X+b.W/2-102) > tol {
		t.Errorf("один ученик %+v, want ширину %.2f по центру", b, want)
	}
}

func TestVignetteTeacherZones(t *testing.T) {
	for _, nt := range []int{1, 2, 3} {
		in := input(sheetRegions(204, 304), nt, 24)
		left := Build(Find("t-left"), in)
		top := Build(Find("head-t-top"), in)
		for _, pb := range boxes(left, RolePupil) {
			for _, tb := range boxes(left, RoleTeacher) {
				if tb.X+tb.W > pb.X+tol {
					t.Fatalf("t-left, учителей %d: учитель %+v правее ученика %+v", nt, tb, pb)
				}
			}
		}
		for _, pb := range boxes(top, RolePupil) {
			for _, tb := range boxes(top, RoleTeacher) {
				if tb.Y > pb.Y-tol || tb.Y < top.Head.Y+top.Head.H {
					t.Fatalf("head-t-top, учителей %d: учитель %+v не между шапкой и учеником %+v", nt, tb, pb)
				}
			}
		}
		if left.TeacherW < left.PupilW {
			t.Errorf("t-left, учителей %d: учитель %.1f мельче ученика %.1f", nt, left.TeacherW, left.PupilW)
		}
	}
	// Учителя справа на развороте — на правой странице, класс начинается слева.
	p := Build(Find("t-right"), input(spreadRegions(), 2, 25))
	for _, tb := range boxes(p, RoleTeacher) {
		if tb.X < 300 {
			t.Fatalf("t-right: учитель на левой странице %+v", tb)
		}
	}
	if ps := boxes(p, RolePupil); ps[0].X > 300 {
		t.Fatalf("t-right: первый ученик не на левой странице %+v", ps[0])
	}
}

func TestVignetteSplit(t *testing.T) {
	p := Build(Find("split-t"), input(spreadRegions(), 1, 25))
	if p.Head == nil || p.Head.X > 300 {
		t.Fatalf("split-t: шапка %+v не слева", p.Head)
	}
	if tb := boxes(p, RoleTeacher)[0]; tb.X > 300 || tb.W > teachMax*p.PupilW+tol {
		t.Fatalf("split-t: учитель %+v (ученик %.1f)", tb, p.PupilW)
	}
	// Учителей нет — слева только шапка: класс на обеих страницах.
	p = Build(Find("split-t"), input(spreadRegions(), 0, 25))
	onLeft := 0
	for _, b := range boxes(p, RolePupil) {
		if b.X < 300 {
			onLeft++
		}
	}
	if onLeft == 0 || onLeft == 25 {
		t.Fatalf("split-t без учителей: слева %d из 25", onLeft)
	}
	// На листе шаблон Split — зоны того же листа.
	in := input(sheetRegions(204, 304), 2, 25)
	for _, id := range []string{"split-t", "split-g", "split-tg"} {
		checkPlan(t, Find(id), in, Build(Find(id), in))
	}
}

func TestVignetteCatalog(t *testing.T) {
	all, sheet := Catalog(false), Catalog(true)
	if len(all) != 20 || len(sheet) != 17 {
		t.Fatalf("шаблонов %d / %d в листе, want 20 / 17", len(all), len(sheet))
	}
	seen := map[string]bool{}
	for _, tp := range all {
		if seen[tp.ID] || tp.Name == "" {
			t.Fatalf("повтор или пустое имя: %+v", tp)
		}
		seen[tp.ID] = true
		if Find(tp.ID) != tp {
			t.Fatalf("Find(%q) = %+v", tp.ID, Find(tp.ID))
		}
	}
	for _, tp := range sheet {
		if tp.Split {
			t.Fatalf("в листе шаблон Split %q", tp.ID)
		}
	}
	if Find("чужой").ID != DefaultTpl || Find("").ID != DefaultTpl {
		t.Fatal("чужой ID — не умолчание")
	}
}

func TestVignetteParseRatio(t *testing.T) {
	for s, want := range map[string]float64{"2:3": 2.0 / 3, "3:4": 0.75, "4:5": 0.8, "1:1": 1, "": 0.75, "16:9": 0.75} {
		if got := ParseRatio(s); math.Abs(got-want) > tol {
			t.Errorf("ParseRatio(%q) = %v, want %v", s, got, want)
		}
	}
	if ParseRatio(DefaultRatio) != 0.75 {
		t.Error("умолчание не 3:4")
	}
}

// Мусорный вход не роняет генератор, число слотов сохраняется.
func TestVignetteDegenerate(t *testing.T) {
	if p := Build(Find("grid"), Input{Pupils: 5}); len(p.Slots) != 0 {
		t.Fatalf("без областей: %d мест", len(p.Slots))
	}
	cases := []Input{
		{Regions: []Rect{{0, 0, 0, 0}}, Teachers: 2, Pupils: 5},
		{Regions: []Rect{{0, 0, 50, 5}}, Teachers: 2, Pupils: 5, HS: 20, HST: 20, Gap: 3},
		{Regions: []Rect{{math.NaN(), 0, math.Inf(1), -4}}, Teachers: -1, Pupils: 3, Ratio: math.NaN(), Gap: math.NaN()},
	}
	for i, in := range cases {
		for _, tp := range Catalog(false) {
			p := Build(tp, in)
			want := max(in.Teachers, 0) + in.Pupils
			if tp.Group != PosNone {
				want++
			}
			if len(p.Slots) != want {
				t.Fatalf("случай %d, %s: %d мест, want %d", i, tp.ID, len(p.Slots), want)
			}
			for _, s := range p.Slots {
				b := s.Box
				if math.IsNaN(b.X+b.Y+b.W+b.H) || math.IsInf(b.X+b.Y+b.W+b.H, 0) || b.W < 0 || b.H < 0 {
					t.Fatalf("случай %d, %s: рамка %+v", i, tp.ID, b)
				}
			}
		}
	}
	// Низкая область: полоса подписи уступает местам.
	p := Build(Find("grid"), Input{Regions: []Rect{{0, 0, 100, 6}}, Pupils: 4, HS: 10, Gap: 1})
	for _, b := range boxes(p, RolePupil) {
		if b.W <= 0 || b.Y < 0 || b.Y+b.H > 6+tol {
			t.Fatalf("низкая область: %+v", b)
		}
	}
}

// TestVignetteFitEdges — края подбора размера: пустая область не даёт столбцов и не мешает
// соседней; места не влезают даже крошечными — размер 0; делить нечего — нули; бесконечная
// координата Y области обнуляется.
func TestVignetteFitEdges(t *testing.T) {
	g := grid{gx: 2, gy: 2, a: 0.75, hs: 4}
	if c, r := g.dims(Rect{0, 0, 100, 0}, 10); c != 0 || r != 0 {
		t.Fatalf("область без высоты: %d×%d", c, r)
	}
	rs := []Rect{{0, 0, 100, 0}, {0, 0, 60, 60}}
	if got, want := g.capacity(rs, 10), g.capacity(rs[1:], 10); got != want || want == 0 {
		t.Fatalf("вместимость с пустой областью %d, без неё %d", got, want)
	}
	if pw := g.maxPW(100000, rs[1:]); pw != 0 {
		t.Fatalf("100000 мест в 60×60 мм: портрет %v", pw)
	}
	if got := share(5, []int{0, 0}); got[0] != 0 || got[1] != 0 {
		t.Fatalf("делить некуда: %v", got)
	}
	if got := share(5, []int{3, 3}); got[0]+got[1] != 5 {
		t.Fatalf("делёж 5 по 3+3: %v", got)
	}
	p := Build(Find("grid"), Input{Regions: []Rect{{5, math.Inf(1), 100, 100}}, Pupils: 3})
	if len(p.Slots) != 3 {
		t.Fatalf("бесконечный Y: мест %d", len(p.Slots))
	}
	for _, s := range p.Slots {
		if math.IsInf(s.Box.Y, 0) || math.IsNaN(s.Box.Y) {
			t.Fatalf("рамка %+v", s.Box)
		}
	}
}

// TestVignettePadRows — отступ подписи больше промежутка (4 и 3 мм): ряды разнесены на
// полосу подписи и ещё Pad (подпись кончается на Pad выше ряда ниже); при Pad ≤ Gap — Gap.
func TestVignettePadRows(t *testing.T) {
	for _, c := range []struct{ gap, pad, want float64 }{{3, 4, 4}, {3, 1, 3}, {3, 0, 3}} {
		in := input(sheetRegions(204, 304), 0, 25)
		in.Gap, in.Pad = c.gap, c.pad
		ps := boxes(Build(Find("grid"), in), RolePupil)
		if len(ps) != 25 {
			t.Fatalf("pad %v: мест %d", c.pad, len(ps))
		}
		gaps := 0
		for _, a := range ps {
			for _, b := range ps {
				if b.Y > a.Y+tol && math.Abs(a.X-b.X) < tol {
					if d := b.Y - (a.Y + a.H + in.HS); d < c.want-tol {
						t.Fatalf("pad %v: между рядами %.3f мм, ждали ≥ %v", c.pad, d, c.want)
					}
					gaps++
				}
			}
		}
		if gaps == 0 {
			t.Fatalf("pad %v: рядов нет", c.pad)
		}
	}
}

// Двадцать превью галереи на 64 местах — то, что зовёт VignetteChoices (031 G5).
func BenchmarkVignetteChoices(b *testing.B) {
	for _, f := range []struct {
		name string
		rs   []Rect
	}{{"sheet204", sheetRegions(204, 304)}, {"spread300", spreadRegions()}} {
		in := input(f.rs, 4, 60)
		tpls := Catalog(false)
		b.Run(f.name, func(b *testing.B) {
			for b.Loop() {
				for _, tp := range tpls {
					Build(tp, in)
				}
			}
		})
	}
}
