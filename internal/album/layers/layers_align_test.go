package layers_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// rectsOf — aabb мест первого разворота по порядку.
func alignRects(e *album.Editor) []layout.Rect {
	var out []layout.Rect
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		out = append(out, model.PlaceAABB(pl))
	}
	return out
}

func closeMM(a, b float64) bool { return math.Abs(a-b) < 0.011 }

func TestAlignSel(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(120, 40, 50, 50, "b"), edtest.Gp(200, 100, 80, 60, "c")})
	g := edtest.PlaceIDsOf(e)
	tx := edtest.AddText(t, e, "s1", 0)
	e.SetTextBox(tx, 40, 200, 100)

	undo := edtest.UndoLen(e)
	e.AlignItems("s1", g, "left", "sel")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d", edtest.UndoLen(e)-undo)
	}
	for i, r := range alignRects(e) {
		if r.X != 10 {
			t.Fatalf("left: место %d X=%v", i, r.X)
		}
	}
	if r := alignRects(e)[2]; r.Y != 100 || r.W != 80 {
		t.Fatalf("left трогает лишнее: %+v", r)
	}
	e.AlignItems("s1", g, "bottom", "sel") // низ — по самому нижнему (100+60)
	for i, r := range alignRects(e) {
		if !closeMM(r.Y+r.H, 160) {
			t.Fatalf("bottom: место %d низ=%v", i, r.Y+r.H)
		}
	}
	e.AlignItems("s1", g, "hcenter", "sel") // центр выделения: 10…110 → 60
	for i, r := range alignRects(e) {
		if !closeMM(r.X+r.W/2, 60) {
			t.Fatalf("hcenter: место %d центр=%v", i, r.X+r.W/2)
		}
	}
	// Текст в выборе едет вместе с местами; ref фото вместо ID места тоже годится.
	e.AlignItems("s1", []string{"a", tx}, "left", "sel")
	if got := edtest.P(e).Spreads[0].Texts[0]; !closeMM(got.X, alignRects(e)[0].X) {
		t.Fatalf("текст X=%v, место %v", got.X, alignRects(e)[0].X)
	}
	edtest.MustGrid(t, e)
}

func TestAlignPageMargins(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 50, 50, "a"), edtest.Gp(330, 100, 50, 50, "b")})
	g := edtest.PlaceIDsOf(e)
	m, pw := edtest.P(e).Settings.Margin, edtest.P(e).Settings.PageW
	e.AlignItems("s1", g, "left", "page")
	if r := alignRects(e); r[0].X != m || !closeMM(r[1].X, pw+m) {
		t.Fatalf("left page: %+v", r)
	}
	e.AlignItems("s1", g, "right", "page")
	if r := alignRects(e); !closeMM(r[0].X+r[0].W, pw-m) || !closeMM(r[1].X+r[1].W, 2*pw-m) {
		t.Fatalf("right page: %+v", r)
	}
	e.AlignItems("s1", g[:1], "vcenter", "page") // одного достаточно
	if r := alignRects(e)[0]; !closeMM(r.Y+r.H/2, edtest.P(e).Settings.PageH/2) {
		t.Fatalf("vcenter page: %+v", r)
	}
	e.AlignItems("s1", g, "top", "spread")
	e.AlignItems("s1", g, "left", "spread")
	for i, r := range alignRects(e) {
		if r.X != m || r.Y != m {
			t.Fatalf("spread: место %d %+v", i, r)
		}
	}
	edtest.MustGrid(t, e)
}

func TestDistribute(t *testing.T) {
	// Порядок слоёв ≠ порядку по центрам: распределение идёт по центрам (a, b, c).
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(400, 10, 50, 50, "c"), edtest.Gp(10, 60, 50, 50, "a"), edtest.Gp(100, 110, 100, 50, "b")})
	g := edtest.PlaceIDsOf(e)
	e.AlignItems("s1", g, "distH", "sel")
	r := alignRects(e)
	gap1, gap2 := r[2].X-(r[1].X+r[1].W), r[0].X-(r[2].X+r[2].W)
	if r[1].X != 10 || r[0].X != 400 || !closeMM(gap1, gap2) || !closeMM(gap1, 120) {
		t.Fatalf("distH sel: %+v (промежутки %v, %v)", r, gap1, gap2)
	}
	if e.AlignOverlap("s1", g, "distH", "sel") {
		t.Fatal("помещаются, а AlignOverlap — нет")
	}
	// Поле страницы: a и b (левая) встают к полям левой, c (правая) один — стоит.
	e.AlignItems("s1", g, "distH", "page")
	r = alignRects(e)
	m, pw, ph := edtest.P(e).Settings.Margin, edtest.P(e).Settings.PageW, edtest.P(e).Settings.PageH
	if r[1].X != m || !closeMM(r[2].X+r[2].W, pw-m) || r[0].X != 400 {
		t.Fatalf("distH page: %+v", r)
	}
	e.AlignItems("s1", g, "distV", "spread") // по центрам Y: c, a, b
	r = alignRects(e)
	if r[0].Y != m || !closeMM(r[2].Y+r[2].H, ph-m) || !closeMM(r[1].Y-(r[0].Y+r[0].H), r[2].Y-(r[1].Y+r[1].H)) {
		t.Fatalf("distV spread: %+v", r)
	}
	edtest.MustGrid(t, e)

	// Равные центры — по слою: нижний (раньше в стеке) идёт первым.
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(100, 10, 100, 50, "a"), edtest.Gp(125, 70, 50, 50, "b"), edtest.Gp(400, 130, 50, 50, "c")})
	g = edtest.PlaceIDsOf(e)
	e.AlignItems("s1", g, "distH", "sel")
	if r := alignRects(e); !closeMM(r[0].X, 100) || !closeMM(r[1].X, 275) {
		t.Fatalf("равные центры: нижний должен идти первым: %+v", r)
	}
	// Меньше трёх при опоре sel — холостая.
	undo := edtest.UndoLen(e)
	e.AlignItems("s1", g[:2], "distH", "sel")
	if edtest.UndoLen(e) != undo {
		t.Fatal("distH на двух объектах не холостая")
	}
}

func TestAlignRotatedAABB(t *testing.T) {
	// 100×20 под 90°: aabb 20×100, центр (150, 70) → X 140…160.
	e := edtest.GridEd(t, []*model.Place{{X: 100, Y: 60, W: 100, H: 20, Rot: 90, Photo: "a"}, edtest.Gp(300, 200, 50, 50, "b")})
	g := edtest.PlaceIDsOf(e)
	if r := alignRects(e)[0]; !closeMM(r.X, 140) || !closeMM(r.W, 20) {
		t.Fatalf("aabb до: %+v", r)
	}
	e.AlignItems("s1", g, "left", "page")
	pl := edtest.P(e).Spreads[0].Grid[0]
	if r := model.PlaceAABB(pl); !closeMM(r.X, edtest.P(e).Settings.Margin) || pl.Rot != 90 || pl.W != 100 || pl.H != 20 {
		t.Fatalf("после: aabb %+v, место %+v", r, *pl)
	}
	e.AlignItems("s1", g, "bottom", "sel")
	if a, b := alignRects(e)[0], alignRects(e)[1]; !closeMM(a.Y+a.H, b.Y+b.H) {
		t.Fatalf("низ aabb: %+v %+v", a, b)
	}
	edtest.MustGrid(t, e)
}

func TestAlignFreezesAuto(t *testing.T) {
	e := edtest.GridEd(t, nil, []string{"a", "b"})
	if len(edtest.P(e).Spreads[1].Grid) != 0 {
		t.Fatal("s2 должен быть «Авто»")
	}
	undo := edtest.UndoLen(e)
	// Проверка наложения у «Авто» считает на копии: проект не меняется.
	was := string(model.Marshal(edtest.P(e)))
	if e.AlignOverlap("s2", []string{"a", "b"}, "distH", "spread") || string(model.Marshal(edtest.P(e))) != was {
		t.Fatal("AlignOverlap у «Авто»: наложение или изменил проект")
	}
	// Одно фото при опоре sel — мало объектов: закрепление не должно остаться.
	e.AlignItems("s2", []string{"a"}, "left", "sel")
	if len(edtest.P(e).Spreads[1].Grid) != 0 || edtest.UndoLen(e) != undo {
		t.Fatal("холостая правка закрепила «Авто»")
	}
	e.AlignItems("s2", []string{"a", "b"}, "top", "page")
	s := edtest.P(e).Spreads[1]
	if len(s.Grid) != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("мест %d, шагов отмены %d", len(s.Grid), edtest.UndoLen(e)-undo)
	}
	for _, pl := range s.Grid {
		if pl.Y != edtest.P(e).Settings.Margin {
			t.Fatalf("top page: %+v", *pl)
		}
	}
	edtest.MustGrid(t, e)
	e.Undo()
	if len(edtest.P(e).Spreads[1].Grid) != 0 {
		t.Fatal("отмена не вернула «Авто»")
	}
	// Одни тексты «Авто» не закрепляют.
	t1, t2 := edtest.AddText(t, e, "s2", 0), edtest.AddText(t, e, "s2", 0)
	e.SetTextBox(t1, 40, 50, 80)
	e.SetTextBox(t2, 90, 120, 80)
	e.AlignItems("s2", []string{t1, t2}, "left", "sel")
	s = edtest.P(e).Spreads[1]
	if len(s.Grid) != 0 || !closeMM(s.Texts[0].X, 40) || !closeMM(s.Texts[1].X, 40) {
		t.Fatalf("тексты в «Авто»: сетка %d, %+v %+v", len(s.Grid), *s.Texts[0], *s.Texts[1])
	}
}

func TestAlignNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(10, 120, 50, 50, "b")})
	g := edtest.PlaceIDsOf(e)
	was := string(model.Marshal(edtest.P(e)))
	undo := edtest.UndoLen(e)
	for _, c := range []struct {
		ids    []string
		op, to string
	}{
		{g, "left", "sel"},     // уже стоят
		{g[:1], "left", "sel"}, // один объект при опоре sel
		{g, "wat", "sel"},      // неизвестная операция
		{g, "left", "wat"},     // неизвестная опора
		{g, "left", ""},        // пустая опора
		{nil, "left", "page"},  // никого
		{[]string{"нет", "нет2"}, "left", "sel"}, // чужие ID
		{g, "distH", "sel"},                      // два объекта — мало
	} {
		e.AlignItems("s1", c.ids, c.op, c.to)
		e.AlignItems("нет такого", c.ids, c.op, c.to)
	}
	if edtest.UndoLen(e) != undo || string(model.Marshal(edtest.P(e))) != was {
		t.Fatal("холостые вызовы изменили проект или отмену")
	}
}

// wantCentres — центры рамок по оси (X или Y) равны want (мм).
func wantCentres(t *testing.T, e *album.Editor, horiz bool, want map[int]float64) {
	t.Helper()
	r := alignRects(e)
	for i, c := range want {
		got := r[i].Y + r[i].H/2
		if horiz {
			got = r[i].X + r[i].W/2
		}
		if !closeMM(got, c) {
			t.Fatalf("место %d: центр %v, ждали %v; %+v", i, got, c, r)
		}
	}
}

func TestDistributeOverlapPages(t *testing.T) {
	// Левая страница (0…281): e, f, g помещаются — равные промежутки 73,5 в поле 12…269.
	// Правая (281…562): ширины 257+100+152+33 > поля 293…550 — равный шаг центров:
	// b у начала (центр 343), d у конца (533,5), шаг 63,5. Порядок по центрам: b, a, c, d.
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(290, 10, 257, 40, "a"), edtest.Gp(300, 60, 100, 40, "b"), edtest.Gp(400, 110, 152, 40, "c"), edtest.Gp(500, 160, 33, 40, "d"),
		edtest.Gp(20, 10, 40, 40, "e"), edtest.Gp(100, 100, 40, 40, "f"), edtest.Gp(200, 200, 30, 30, "g"),
	})
	g := edtest.PlaceIDsOf(e)
	if !e.AlignOverlap("s1", g, "distH", "page") {
		t.Fatal("правая страница не помещается, а AlignOverlap — нет")
	}
	e.AlignItems("s1", g, "distH", "page")
	wantCentres(t, e, true, map[int]float64{1: 343, 0: 406.5, 2: 470, 3: 533.5})
	if r := alignRects(e); r[1].X != 293 || !closeMM(r[3].X+r[3].W, 550) {
		t.Fatalf("крайние правой не у полей: %+v", r)
	}
	if r := alignRects(e); r[4].X != 12 || !closeMM(r[5].X, 125.5) || !closeMM(r[6].X, 239) {
		t.Fatalf("левая страница: %+v", r)
	}
	if e.AlignOverlap("s1", g[4:], "distH", "page") {
		t.Fatal("левая помещается, а AlignOverlap — да")
	}
	// Уже распределено — холостая (и внахлёст, и с промежутками).
	undo := edtest.UndoLen(e)
	e.AlignItems("s1", g, "distH", "page")
	if edtest.UndoLen(e) != undo {
		t.Fatal("повторное распределение не холостое")
	}
	// По одному на странице — никто не двигается.
	e.AlignItems("s1", []string{g[2], g[5]}, "distH", "page")
	if edtest.UndoLen(e) != undo {
		t.Fatal("по одному на странице — должно быть холостым")
	}
	edtest.MustGrid(t, e)
}

func TestDistributeOverlapV(t *testing.T) {
	// Высоты 200+150+100 > поля 12…272: a сверху (центр 112), c снизу (222), шаг 55.
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 200, "a"), edtest.Gp(100, 50, 50, 150, "b"), edtest.Gp(200, 100, 50, 100, "c")})
	g := edtest.PlaceIDsOf(e)
	if !e.AlignOverlap("s1", g, "distV", "spread") {
		t.Fatal("AlignOverlap: не помещаются")
	}
	e.AlignItems("s1", g, "distV", "spread")
	wantCentres(t, e, false, map[int]float64{0: 112, 1: 167, 2: 222})
	if r := alignRects(e); r[0].Y != 12 || !closeMM(r[2].Y+r[2].H, 272) {
		t.Fatalf("крайние не у полей: %+v", r)
	}
	undo := edtest.UndoLen(e)
	e.AlignItems("s1", g, "distV", "spread")
	if edtest.UndoLen(e) != undo {
		t.Fatal("повторное распределение не холостое")
	}
	if e.AlignOverlap("s1", g, "distH", "spread") {
		t.Fatal("по ширине помещаются")
	}
	edtest.MustGrid(t, e)
}

func TestDistributeOverlapSel(t *testing.T) {
	// Сумма 300 > пролёта 10…200: крайние a и c стоят, центр b — посередине (105).
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 40, "a"), edtest.Gp(50, 60, 100, 40, "b"), edtest.Gp(100, 110, 100, 40, "c")})
	g := edtest.PlaceIDsOf(e)
	if !e.AlignOverlap("s1", g, "distH", "sel") {
		t.Fatal("AlignOverlap: не помещаются")
	}
	e.AlignItems("s1", g, "distH", "sel")
	if r := alignRects(e); r[0].X != 10 || r[2].X != 100 || !closeMM(r[1].X, 55) {
		t.Fatalf("distH sel внахлёст: %+v", r)
	}
	if e.AlignOverlap("s1", g, "left", "sel") || e.AlignOverlap("s1", g[:2], "distH", "sel") {
		t.Fatal("AlignOverlap вне распределения или на малом выборе")
	}
}

func TestDistributeWideIdempotent(t *testing.T) {
	// Половины крайних больше поля (c0 > c1): все центры — в середину поля, повтор
	// холостой, объекты остаются на своей странице. Поле левой 12…269, высоты 12…272.
	for _, c := range []struct {
		name   string
		places []*model.Place
		op     string
		centre float64 // 0 — не проверять (широкое место зажато разворотом)
	}{
		{"distH", []*model.Place{edtest.Gp(0, 10, 270, 40, "a"), edtest.Gp(10, 60, 260, 40, "b")}, "distH", 140.5},
		{"distH шире страницы", []*model.Place{edtest.Gp(0, 10, 545, 40, "a"), edtest.Gp(270, 60, 20, 40, "b")}, "distH", 0},
		{"distV", []*model.Place{edtest.Gp(10, 0, 40, 270, "a"), edtest.Gp(60, 10, 40, 260, "b")}, "distV", 142},
	} {
		e := edtest.GridEd(t, c.places)
		g := edtest.PlaceIDsOf(e)
		e.AlignItems("s1", g, c.op, "page")
		if c.centre != 0 {
			wantCentres(t, e, c.op == "distH", map[int]float64{0: c.centre, 1: c.centre})
		}
		undo, was := edtest.UndoLen(e), alignRects(e)
		for range 2 {
			e.AlignItems("s1", g, c.op, "page")
		}
		if edtest.UndoLen(e) != undo {
			t.Fatalf("%s: повторное распределение не холостое: %+v → %+v", c.name, was, alignRects(e))
		}
		for i, r := range alignRects(e) {
			if edtest.P(e).AlignSheet(nil).PageAt(r.X+r.W/2) == 1 {
				t.Fatalf("%s: место %d ушло на правую страницу: %+v", c.name, i, r)
			}
		}
		edtest.MustGrid(t, e)
	}
}

func TestDistributeTouching(t *testing.T) {
	// Встык ровно в поле 12…269: ширины (X+W)−X дают промежуток ≈ −1e-14 — это не
	// наложение, и повтор холостой.
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 10, 244.08, 40, "a"), edtest.Gp(256.08, 60, 12.92, 40, "b")})
	g := edtest.PlaceIDsOf(e)
	if e.AlignOverlap("s1", g, "distH", "page") {
		t.Fatal("встык — не наложение")
	}
	undo := edtest.UndoLen(e)
	e.AlignItems("s1", g, "distH", "page")
	if edtest.UndoLen(e) != undo {
		t.Fatalf("встык: распределение не холостое: %+v", alignRects(e))
	}
}
