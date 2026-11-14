package grid_test

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Связи поворота места с прочими правками (design-rotation §2.1, таблица «Связи»).

// rotEd — gridEd, где места после закрепления повёрнуты на углы rots (по порядку).
func rotEd(t *testing.T, places []*model.Place, rots ...float64) *album.Editor {
	t.Helper()
	e := edtest.GridEd(t, places)
	for i, deg := range rots {
		e.SetPlaceAngle("s1", edtest.P(e).Spreads[0].Grid[i].ID, deg)
	}
	edtest.MustGrid(t, e)
	return e
}

func centerOf(pl *model.Place) (float64, float64) { return pl.X + pl.W/2, pl.Y + pl.H/2 }

func near2(a, b float64) bool { return math.Abs(a-b) <= 0.01+1e-9 }

// same и fitPhoto у повёрнутого: центр стоит, угол тот же.
func TestArrangeRotKeepsCenter(t *testing.T) {
	e := rotEd(t, []*model.Place{edtest.Gp(200, 80, 80, 80, "b"), edtest.Gp(100, 100, 100, 60, "a")}, 0, 30)
	pl := edtest.P(e).Spreads[0].Grid[1]
	cx, cy := centerOf(pl)
	for _, op := range []string{"same", "fitPhoto"} {
		if _, err := e.ArrangePlaces("s1", op, []string{"b", "a"}); err != nil {
			t.Fatal(err)
		}
		if x, y := centerOf(pl); !near2(x, cx) || !near2(y, cy) || pl.Rot != 30 {
			t.Fatalf("%s: %+v, центр (%v, %v), было (%v, %v)", op, *pl, x, y, cx, cy)
		}
	}
	if pl.W != 80 || math.Abs(pl.H-80/1.5) > 0.01 {
		t.Fatalf("размер: %+v", *pl)
	}
	edtest.MustGrid(t, e)
}

// Деление повёрнутого места — в его осях: половины на оси места с промежутком Gap,
// угол тот же (место вдали от края: у края половины зажимаются по отдельности).
func TestSplitRotated(t *testing.T) {
	for _, horiz := range []bool{true, false} {
		e := rotEd(t, []*model.Place{edtest.Gp(221, 112, 120, 60, "a")}, 90)
		op := map[bool]string{true: "splitH", false: "splitV"}[horiz]
		if _, err := e.ArrangePlaces("s1", op, []string{"a"}); err != nil {
			t.Fatal(err)
		}
		g := edtest.P(e).Spreads[0].Grid
		if len(g) != 2 || g[0].Rot != 90 || g[1].Rot != 90 || g[0].Photo != "a" || g[1].Photo != "" {
			t.Fatalf("%s: %+v %+v", op, *g[0], *g[1])
		}
		gap := edtest.P(e).Settings.Gap
		x0, y0 := centerOf(g[0])
		x1, y1 := centerOf(g[1])
		if horiz { // ось места под 90° — вертикаль экрана
			half := math.Floor((120-gap)/2*100) / 100
			if g[0].W != half || g[1].W != half || g[0].H != 60 || !near2(x0, 281) || !near2(x1, 281) || !near2(y1-y0, half+gap) {
				t.Fatalf("%s: %+v %+v", op, *g[0], *g[1])
			}
		} else { // поперечная ось места — горизонталь, к −x
			half := math.Floor((60-gap)/2*100) / 100
			if g[0].H != half || g[1].H != half || g[0].W != 120 || !near2(y0, 142) || !near2(y1, 142) || !near2(x0-x1, half+gap) {
				t.Fatalf("%s: %+v %+v", op, *g[0], *g[1])
			}
		}
		edtest.MustGrid(t, e)
	}
}

// Смена формата: центр — по kx, ky, угол стоит, стороны — длины образов осей места.
// 300×300 → 300×200, место 100×60 под 90°: на экране 60×67, а не 40×100.
func TestRescaleGridRot(t *testing.T) {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 300, 300
	p.Photos["a"] = model.PhotoRef{Path: edtest.Abs("a.jpg"), W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"},
		Grid: []*model.Place{{ID: "000000000001", X: 200, Y: 120, W: 100, H: 60, Photo: "a", Rot: 90}}}}
	e := edtest.Open(t, p, "")
	st := edtest.P(e).Settings
	st.PageH = 200
	e.SetSettings(st)
	pl := edtest.P(e).Spreads[0].Grid[0]
	a := model.PlaceAABB(pl)
	if pl.Rot != 90 || !near2(a.W, 60) || !near2(a.H, 66.67) || !near2(a.X+a.W/2, 250) || !near2(a.Y+a.H/2, 100) {
		t.Fatalf("%+v, на экране %+v", *pl, a)
	}
	edtest.MustGrid(t, e)
}

// Область места — по центру (опора поворота): 100×60 под 90°, центр справа от
// корешка — «правая»; корешок — по описанному прямоугольнику.
func TestRotPlaceArea(t *testing.T) {
	e := rotEd(t, []*model.Place{edtest.Gp(250, 100, 100, 60, "a"), edtest.Gp(270, 200, 100, 20, "")}, 90, 90)
	p, s := edtest.P(e), edtest.P(e).Spreads[0]
	pl := s.Grid[0]
	if !grid.InArea(p, s, pl.X, pl.W, model.ScopeRight) || grid.InArea(p, s, pl.X, pl.W, model.ScopeLeft) || len(grid.AreaFrac(p, s, model.ScopeRight)) != 2 {
		t.Fatal("место под 90° с центром справа — не «правая»")
	}
	p.Settings.Layflat = false
	v := e.View().Spreads[0].Places
	// Первое: описанный 270..330 — через корешок 281; второе: неповёрнутое 270..370
	// пересекло бы, описанный 310..330 — нет.
	if !v[0].Spine || v[1].Spine || v[0].Rot != 90 {
		t.Fatalf("корешок: %+v", v)
	}
}

// Слои — по описанному прямоугольнику: «Выше» перешагивает место, которое задевает
// только описанный прямоугольник повёрнутой рамки.
func TestArrangeLayersRotAABB(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(50, 95, 100, 10, "a"), edtest.Gp(125, 75, 20, 20, "b")})
	ids := edtest.PlaceIDsOf(e)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ArrangeLayers("s1", "up", []string{ids[0]})
	edtest.Same(t, e, rev, undo, "ровные не перекрыты")
	e.SetPlaceAngle("s1", ids[0], 45)
	e.ArrangeLayers("s1", "up", []string{ids[0]})
	if got := edtest.PlaceIDsOf(e); got[0] != ids[1] || got[1] != ids[0] {
		t.Fatalf("порядок %v", got)
	}
}

// Группа: граница повёрнутого места — hold; угол едет как есть, clampPlace после не сдвигает.
func TestGroupXfRotated(t *testing.T) {
	e := rotEd(t, []*model.Place{edtest.Gp(200, 100, 100, 20, "a")}, 90)
	e.TransformGroup("s1", []string{"a"}, 1, 0, 0, -9999, 0, "")
	pl := edtest.P(e).Spreads[0].Grid[0]
	if pl.X != -40 || pl.Y != 100 || pl.Rot != 90 || model.Hold(pl).X != 0 {
		t.Fatalf("%+v", *pl)
	}
	e.TransformGroup("s1", []string{"a"}, 0.5, 0, 0, 0, 0, "")
	if pl.W != 50 || pl.H != 10 || pl.Rot != 90 {
		t.Fatalf("масштаб: %+v", *pl)
	}
	edtest.MustGrid(t, e)
}

// Отражение: X' = A0 + A1 − X − W, угол −Rot; описанный прямоугольник отражается
// (до 0.01); пара ±угол симметрична — холостая.
func TestMirrorGridRot(t *testing.T) {
	e := rotEd(t, []*model.Place{edtest.Gp(20, 30, 100, 60, "a")}, 30)
	pl := edtest.P(e).Spreads[0].Grid[0]
	sw := 2 * edtest.P(e).Settings.PageW
	a := model.PlaceAABB(pl)
	e.MirrorGrid("s1", model.ScopeSpread)
	b := model.PlaceAABB(pl)
	if pl.Rot != -30 || !near2(b.X, sw-a.X-a.W) || !near2(b.Y, a.Y) || !near2(b.W, a.W) || !near2(b.H, a.H) {
		t.Fatalf("%+v: aabb %+v, было %+v", *pl, b, a)
	}
	e.MirrorGrid("s1", model.ScopeRight) // место целиком на правой (по hold) — отражается в странице
	if pl.Rot != 30 {
		t.Fatalf("страница: %+v", *pl)
	}
	// Пара ±30 симметрична: отражение холостое; та же пара +30/+30 — нет.
	e = rotEd(t, []*model.Place{edtest.Gp(20, 30, 100, 60, "a"), edtest.Gp(sw-120, 30, 100, 60, "")}, 30, -30)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.MirrorGrid("s1", model.ScopeSpread)
	edtest.Same(t, e, rev, undo, "симметричная пара")
	e.SetPlaceAngle("s1", edtest.P(e).Spreads[0].Grid[1].ID, 30)
	if e.MirrorGrid("s1", model.ScopeSpread); edtest.P(e).Spreads[0].Grid[0].Rot != -30 {
		t.Fatal("несимметричная пара не отразилась")
	}
	edtest.MustGrid(t, e)
}

// Подпись повёрнутого места: посчитана для неповёрнутой рамки, угол рамки и строки
// повёрнуты вокруг центра места, Rot = углу места, Overflow 0.
func TestCaptionRotatesWithPlace(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(100, 100, 120, 80, "a")})
	ref := edtest.P(e).Photos["a"]
	ref.Caption = &model.Caption{Text: "Первый танец", Style: model.DefaultCaptionStyle()}
	edtest.P(e).Photos["a"] = ref
	c0 := e.View().Spreads[0].Captions[0]
	e.SetPlaceAngle("s1", "a", 30)
	c1 := e.View().Spreads[0].Captions[0]
	dx, dy := geom.Rotate(30, c0.X-160, c0.Y-140)
	if c1.Rot != 30 || c1.Overflow != 0 || math.Abs(c1.X-160-dx) > 1e-9 || math.Abs(c1.Y-140-dy) > 1e-9 || c1.W != c0.W {
		t.Fatalf("подпись %+v, ровная %+v", c1, c0)
	}
	for k := range c0.Lines {
		if math.Abs(c1.Lines[k].X-c0.Lines[k].X-(c1.X-c0.X)) > 1e-9 || math.Abs(c1.Lines[k].Y-c0.Lines[k].Y-(c1.Y-c0.Y)) > 1e-9 {
			t.Fatalf("строка %d: %+v, ровная %+v", k, c1.Lines[k], c0.Lines[k])
		}
	}
	if c0.Rot != 0 {
		t.Fatal("у ровного места подпись с углом")
	}
}

// Угол в копировании и шаблонах, порядок слоёв в копиях (спека 010, G6).

// rotPlaceOf — повёрнутое место разворота i (одно на тест); нет — nil.
func rotPlaceOf(e *album.Editor, i int) *model.Place {
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		if pl.Rot != 0 {
			return pl
		}
	}
	return nil
}

func sameAABB(a, b layout.Rect) bool {
	return near2(a.X, b.X) && near2(a.Y, b.Y) && near2(a.W, b.W) && near2(a.H, b.H)
}

func TestCopyGridKeepsRot(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(320, 100, 100, 60, "c")}, []string{"d", "e"})
	e.SetPlaceAngle("s1", "c", 30)
	src := edtest.P(e).Spreads[0].Grid
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	got := edtest.P(e).Spreads[1].Grid
	if len(got) != 2 {
		t.Fatalf("мест %d", len(got))
	}
	for i := range got {
		if got[i].Rot != src[i].Rot || !sameAABB(model.PlaceAABB(got[i]), model.PlaceAABB(src[i])) {
			t.Fatalf("место %d: %+v, источник %+v", i, *got[i], *src[i])
		}
	}
	if _, err := e.DuplicateGrid("s1", 1, false); err != nil {
		t.Fatal(err)
	}
	if pl := edtest.P(e).Spreads[1].Grid[1]; pl.Rot != 30 || !sameAABB(model.PlaceAABB(pl), model.PlaceAABB(src[1])) {
		t.Fatalf("размножение: %+v", *pl)
	}
	edtest.MustGrid(t, e)
}

// Повёрнутое место у края (вылет aabb за лист, hold вплотную): копия, зеркальная
// копия, шаблон и «Отразить» ставят его на то же место (по aabb, до 0,01), а не
// сдвигают внутрь.
func TestCopyGridRotEdge(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(0, 0, 100, 60, "a")}, []string{"d"}, []string{"e"}, []string{"f"})
	edtest.Change(e, func(p *model.Project) {
		pl := p.Spreads[0].Grid[0]
		pl.X, pl.Rot = -20, 30 // hold: r = 30, центр (30, 30) — у левого и верхнего края
		p.ClampPlace(p.Spreads[0], pl)
	})
	pl := edtest.P(e).Spreads[0].Grid[0]
	if pl.X != -20 || pl.Y != 0 || pl.Rot != 30 {
		t.Fatalf("исходное место: %+v", *pl)
	}
	a := model.PlaceAABB(pl)
	pw, sw := edtest.P(e).Settings.PageW, 2*edtest.P(e).Settings.PageW
	if a.X >= 0 || a.Y >= 0 {
		t.Fatalf("место не вылетает: %+v", a)
	}
	mirror := func(axis2 float64) layout.Rect { return layout.Rect{X: axis2 - a.X - a.W, Y: a.Y, W: a.W, H: a.H} }

	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, false, false); err != nil {
		t.Fatal(err)
	}
	if c := rotPlaceOf(e, 1); c == nil || c.Rot != 30 || !sameAABB(model.PlaceAABB(c), a) {
		t.Fatalf("копия: %+v", c)
	}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s3", Scope: model.ScopeRight}}, false, true); err != nil {
		t.Fatal(err)
	}
	if c := rotPlaceOf(e, 2); c == nil || c.Rot != -30 || !sameAABB(model.PlaceAABB(c), mirror(sw)) {
		t.Fatalf("зеркальная копия: %+v, ждём %+v", c, mirror(sw))
	}
	boxes, scope, err := e.PlaceBoxes("s1", model.ScopeLeft)
	if err != nil || len(boxes) != 1 || boxes[0].Rot != 30 || boxes[0].X >= 0 {
		t.Fatalf("PlaceBoxes: %v %+v", err, boxes)
	}
	if _, _, err := e.ApplyGrid("s4", model.ScopeLeft, scope, boxes); err != nil {
		t.Fatal(err)
	}
	if c := rotPlaceOf(e, 3); c == nil || c.Rot != 30 || !sameAABB(model.PlaceAABB(c), a) {
		t.Fatalf("шаблон: %+v", c)
	}
	e.MirrorGrid("s1", model.ScopeLeft)
	if pl := edtest.P(e).Spreads[0].Grid[0]; pl.Rot != -30 || !sameAABB(model.PlaceAABB(pl), mirror(pw)) {
		t.Fatalf("отражение: %+v, ждём %+v", *pl, mirror(pw))
	}
	edtest.MustGrid(t, e)
}

// Шаблон v2 с углом → GridChoices → ApplyGrid ставит угол.
func TestGridChoicesRot(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b")).Spreads[0].ID
	data := []byte(`{"version":2,"templates":[{"id":"0123456789ab","name":"Н","scope":"page","boxes":[` +
		`{"x":0.1,"y":0.1,"w":0.3,"h":0.3},{"x":0.5,"y":0.5,"w":0.4,"h":0.3,"rot":-15}]}]}`)
	tpls, ver, err := model.ParseTemplates(data)
	if err != nil || ver != 2 {
		t.Fatalf("%v %d", err, ver)
	}
	c := e.GridChoices(sid, 2, model.ScopeLeft, tpls)
	if len(c.Mine) != 1 || c.Mine[0].Boxes[1].Rot != -15 {
		t.Fatalf("галерея: %+v", c.Mine)
	}
	if _, _, err := e.ApplyGrid(sid, model.ScopeLeft, c.Mine[0].Scope, c.Mine[0].Boxes); err != nil {
		t.Fatal(err)
	}
	var rots []float64
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if grid.InArea(edtest.P(e), nil, pl.X, pl.W, model.ScopeLeft) {
			rots = append(rots, pl.Rot)
		}
	}
	if !slices.Equal(rots, []float64{0, -15}) {
		t.Fatalf("углы мест: %v", rots)
	}
	edtest.MustGrid(t, e)
}

// orderEd — сетка s1 [a слева, пустое справа], текст t1 под a; s2 — «Авто».
func orderEd(t *testing.T) (e *album.Editor, t1 string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 120, 120, "a"), edtest.Gp(293, 12, 127, 127, "")}, []string{"d"})
	t1 = edtest.TextAt(t, e, "s1", 40, 40)
	e.ArrangeLayers("s1", "back", []string{t1})
	g := edtest.PlaceIDsOf(e)
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, g[0], g[1]}) {
		t.Fatalf("исходный порядок: %v", got)
	}
	return e, t1
}

// CopyGrid(design): текст под фото в копии остаётся под ним; новые места и тексты —
// одним куском на позиции первого нового места, текст цели — как был.
func TestCopyGridKeepsOrder(t *testing.T) {
	e, _ := orderEd(t)
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	keep := edtest.AddText(t, e, "s2", 1)
	s2 := edtest.P(e).Spreads[1]
	right := s2.Grid[1].ID
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, true, false); err != nil {
		t.Fatal(err)
	}
	var tNew string
	for _, tb := range s2.Texts {
		if tb.ID != keep {
			tNew = tb.ID
		}
	}
	aNew := s2.Grid[0].ID
	if want := []string{tNew, aNew, right, keep}; tNew == "" || !slices.Equal(s2.Stack(), want) || !slices.Equal(s2.Order, want) {
		t.Fatalf("стек %v, Order %v, ждём %v", s2.Stack(), s2.Order, want)
	}
	edtest.MustGrid(t, e)
	// Без design тексты не копируются — порядок мест прежний.
	if _, _, err := e.CopyGrid("s1", model.ScopeRight, []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}, false, false); err != nil {
		t.Fatal(err)
	}
	if st := s2.Stack(); len(st) != 4 || st[0] != tNew || st[1] != aNew || st[3] != keep {
		t.Fatalf("без design: %v", st)
	}
	edtest.MustGrid(t, e)
}

// DuplicateGrid: Order по карте ID; без design текстов нет — Order nil.
func TestDuplicateGridKeepsOrder(t *testing.T) {
	e, _ := orderEd(t)
	if _, err := e.DuplicateGrid("s1", 2, true); err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{1, 2} {
		ns := edtest.P(e).Spreads[i]
		want := []string{ns.Texts[0].ID, ns.Grid[0].ID, ns.Grid[1].ID}
		if len(ns.Texts) != 1 || !slices.Equal(ns.Order, want) {
			t.Fatalf("копия %d: Order %v, ждём %v", i, ns.Order, want)
		}
	}
	if _, err := e.DuplicateGrid("s1", 1, false); err != nil {
		t.Fatal(err)
	}
	if ns := edtest.P(e).Spreads[1]; ns.Order != nil || len(ns.Texts) != 0 {
		t.Fatalf("без design: Order %v, текстов %d", ns.Order, len(ns.Texts))
	}
	edtest.MustGrid(t, e)
}

// Поворот — вокруг центра: меняется только Rot; углы рамки — вокруг центра места.
func TestSetPlaceAngleAboutCenter(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(100, 50, 120, 80, "a")})
	e.SetPlaceAngle("s1", "a", 30)
	pl := edtest.P(e).Spreads[0].Grid[0]
	if pl.X != 100 || pl.Y != 50 || pl.W != 120 || pl.H != 80 || pl.Rot != 30 {
		t.Fatalf("%+v", *pl)
	}
	cx, cy := model.PlaceBoxOf(pl).Center()
	if math.Abs(cx-160) > 1e-9 || math.Abs(cy-90) > 1e-9 {
		t.Fatalf("центр %v, %v", cx, cy)
	}
	v := e.SetPlaceAngle("s1", pl.ID, 0)
	if *pl != (model.Place{ID: pl.ID, X: 100, Y: 50, W: 120, H: 80, Photo: "a"}) || v.Spreads[0].Places[0].Rot != 0 {
		t.Fatalf("назад: %+v", *pl)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceAngle("s1", "a", 360)
	e.SetPlaceAngle("s1", "a", math.NaN())
	e.SetPlaceAngle("s1", "нет такого", 30)
	edtest.Same(t, e, rev, undo, "тот же угол, NaN, чужое")
	if v := e.SetPlaceAngle("s1", "a", 725); v.Spreads[0].Places[0].Rot != 5 || v.Spreads[0].Frames[0].CellRot != 5 {
		t.Fatalf("вид: %+v %+v", v.Spreads[0].Places[0], v.Spreads[0].Frames[0])
	}
}

// У «Авто» поворот закрепляет разворот той же правкой; углы прочих мест — 0
// (freeze углов не даёт). Один Undo — снова «Авто».
func TestSetPlaceAngleFreezesAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceAngle(id, "b", 0)
	edtest.Same(t, e, rev, undo, "угол 0 у «Авто»")
	v := e.SetPlaceAngle(id, "b", 15)
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 3 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("закрепление: мест %d, undo %d", len(s.Grid), edtest.UndoLen(e))
	}
	for _, pl := range s.Grid {
		if want := map[bool]float64{true: 15}[pl.Photo == "b"]; pl.Rot != want {
			t.Fatalf("место %s: угол %v", pl.Photo, pl.Rot)
		}
	}
	if !v.Spreads[0].Grid {
		t.Fatal("вид не сетка")
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("Undo не вернул «Авто»")
	}
}

// Поворот места (спека 010, design-rotation §2.1): опора — центр, зажим — hold.

// Поворот у края место не сдвигает (hold от угла не зависит); туда и обратно — те же X, Y, W, H.
func TestRotateNoShift(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(0, 0, 100, 60, "a"), edtest.Gp(462, 224, 100, 60, "")})
	want := [2]model.Place{*edtest.P(e).Spreads[0].Grid[0], *edtest.P(e).Spreads[0].Grid[1]}
	for _, deg := range []float64{30, 90, 145, -90, 0} {
		for i := range 2 {
			pl := edtest.P(e).Spreads[0].Grid[i]
			e.SetPlaceAngle("s1", pl.ID, deg)
			if pl.X != want[i].X || pl.Y != want[i].Y || pl.W != want[i].W || pl.H != want[i].H || pl.Rot != deg {
				t.Fatalf("%v°: %+v, было %+v", deg, *pl, want[i])
			}
		}
	}
}

// SetPlaces под ненулевым углом не увеличивает вылет описанного прямоугольника:
// полоса 200×10 под 1° у края не уезжает за лист, поворот к 0° потом не прыгает;
// уже вылетающая — дальше не уходит, внутрь — свободно.
func TestSetPlacesRotNoGrowOverhang(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(300, 100, 200, 10, "a")})
	pl := edtest.P(e).Spreads[0].Grid[0]
	e.SetPlaceAngle("s1", "a", 1)
	sw := 2 * edtest.P(e).Settings.PageW
	e.SetPlaces("s1", []model.PlaceBox{{ID: "a", X: 500, Y: 100, W: 200, H: 10}}, "")
	if a := model.PlaceAABB(pl); a.X+a.W > sw+1e-9 || pl.Rot != 1 {
		t.Fatalf("ушло за лист: %+v, aabb %+v", *pl, a)
	}
	x := pl.X
	e.SetPlaceAngle("s1", "a", 0)
	if pl.X != x || pl.Rot != 0 {
		t.Fatalf("поворот к 0° прыгнул: %v → %v", x, pl.X)
	}
	// Уже вылетающее (hold пускает центр до r = 5 мм от края): дальше не уходит.
	e.SetPlaceAngle("s1", "a", 1)
	pl.X = -50
	e.SetPlaces("s1", []model.PlaceBox{{ID: "a", X: -80, Y: 100, W: 200, H: 10}}, "")
	if pl.X != -50 {
		t.Fatalf("вылет вырос: X %v", pl.X)
	}
	e.SetPlaces("s1", []model.PlaceBox{{ID: "a", X: -20, Y: 100, W: 200, H: 10}}, "")
	if pl.X != -20 {
		t.Fatalf("внутрь не пустило: X %v", pl.X)
	}
	edtest.MustGrid(t, e)
}
