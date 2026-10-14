package grid_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Выбор сеток и места по шаблонам (Editor).

func TestGridChoices(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c")).Spreads[0].ID

	// n по умолчанию — число фото; первой идёт сетка из фото разворота.
	c := e.GridChoices(sid, 0, "spread", nil)
	if c.N != 3 || len(c.Builtin) == 0 || len(c.Builtin) > grid.BuiltinLimit || len(c.Mine) != 0 {
		t.Fatalf("N=%d builtin=%d mine=%d", c.N, len(c.Builtin), len(c.Mine))
	}
	best := layout.Cached(edtest.P(e).Aspects([]string{"a", "b", "c"}), edtest.P(e).Params(), 1)[0]
	for i, b := range fracMM(t, edtest.P(e), c.Builtin[0].Boxes, model.TplSpread, "spread") {
		if !nearMM(b, best.Rects[i]) {
			t.Fatalf("первая сетка — не лучшая раскладка фото: %v ≠ %v", b, best.Rects[i])
		}
	}
	ids, shapes := map[string]bool{}, map[string]bool{}
	for _, g := range c.Builtin {
		sh := grid.ShapeKey(fracMM(t, edtest.P(e), g.Boxes, model.TplSpread, "spread"))
		if len(g.Boxes) != 3 || g.Scope != model.TplSpread || g.Name != "" || ids[g.ID] || shapes[sh] {
			t.Fatalf("карточка %+v: не 3 места, не разворот, повтор ID или формы", g)
		}
		ids[g.ID], shapes[sh] = true, true
		for _, b := range g.Boxes {
			if !inUnit(b) {
				t.Fatalf("доля вне 0..1: %+v", b)
			}
		}
	}

	// Фильтр n, «7+» и зажим n.
	if c = e.GridChoices(sid, 5, "spread", nil); c.N != 5 || len(c.Builtin) == 0 || len(c.Builtin[0].Boxes) != 5 {
		t.Fatalf("n=5: %+v", c.N)
	}
	tpls := []model.Template{tplOf(model.TplPage, 5), tplOf(model.TplSpread, 7), tplOf(model.TplSpread, 9), tplOf(model.TplPage, 2)}
	for _, g := range e.GridChoices(sid, 9, "spread", tpls).Builtin {
		if len(g.Boxes) != 7 {
			t.Fatalf("«7+» у встроенных — только 7 мест, получено %d", len(g.Boxes))
		}
	}
	for n, want := range map[int]int{5: 1, 7: 2, 2: 1, 3: 0, 9: 2} {
		if got := len(e.GridChoices(sid, n, "spread", tpls).Mine); got != want {
			t.Fatalf("Mine при n=%d: %d, ждали %d", n, got, want)
		}
	}

	// Неизвестный разворот — пустые ряды, не nil (в JSON — []).
	if c = e.GridChoices("нет", 3, "spread", tpls); len(c.Builtin) != 0 || len(c.Mine) != 0 || c.Builtin == nil || c.Mine == nil {
		t.Fatalf("неизвестный разворот: %+v", c)
	}
}

func TestGridChoicesDrop(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d", "e")).Spreads[0].ID
	cells, _ := grid.TplCells(edtest.P(e), edtest.P(e).Spreads[0])
	left := 0
	for _, r := range cells {
		if grid.InArea(edtest.P(e), nil, r.X, r.W, model.ScopeLeft) {
			left++
		}
	}
	right := len(cells) - left
	cases := []struct {
		tpl   model.Template
		scope string
		want  int
	}{
		{tplOf(model.TplSpread, 2), "spread", 3},
		{tplOf(model.TplSpread, 5), "spread", 0},
		{tplOf(model.TplSpread, 9), "spread", 0},
		{tplOf(model.TplPage, 2), "spread", 1}, // на обе страницы: 4 места
		{tplOf(model.TplPage, 1), "left", max(0, left-1)},
		{tplOf(model.TplPage, 1), "right", max(0, right-1)},
	}
	for _, c := range cases {
		got := e.GridChoices(sid, len(c.tpl.Boxes), c.scope, []model.Template{c.tpl}).Mine
		if len(got) != 1 || got[0].Drop != c.want {
			t.Errorf("%s×%d на %s: %+v, ждали drop %d", c.tpl.Scope, len(c.tpl.Boxes), c.scope, got, c.want)
		}
	}
	// У сетки считаются места с фото: пустое место фото не добавляет.
	edtest.Change(e, func(p *model.Project) {
		p.Freeze(p.Spreads[0])
		p.Spreads[0].Grid[0].Photo = ""
		p.Spreads[0].Photos = model.GridPhotos(p.Spreads[0])
	})
	if got := e.GridChoices(sid, 2, "spread", []model.Template{tplOf(model.TplSpread, 2)}).Mine[0].Drop; got != 2 {
		t.Fatalf("сетка с 4 фото и шаблон на 2 места: drop %d, ждали 2", got)
	}
}

// Места разворота → доли → мм возвращают те же прямоугольники.
func TestPlaceBoxes(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c")).Spreads[0].ID
	pw := edtest.P(e).Settings.PageW
	cells, _ := grid.TplCells(edtest.P(e), edtest.P(e).Spreads[0])

	boxes, scope, err := e.PlaceBoxes(sid, "spread")
	if err != nil || scope != model.TplSpread || len(boxes) != 3 {
		t.Fatalf("spread: %v %q %d", err, scope, len(boxes))
	}
	for i, b := range fracMM(t, edtest.P(e), boxes, scope, "spread") {
		if !nearMM(b, rectOfBox(cells[i])) {
			t.Fatalf("мм → доли → мм: %v ≠ %v", b, cells[i])
		}
	}
	for _, side := range []string{"left", "right"} {
		bs, sc, err := e.PlaceBoxes(sid, side)
		if sc != model.TplPage || err != nil && !errors.Is(err, model.ErrNoPlaces) {
			t.Fatalf("%s: %v %q", side, err, sc)
		}
		for _, b := range bs {
			if !inUnit(b) {
				t.Fatalf("%s: доля вне 0..1 %+v", side, b)
			}
		}
		for _, b := range fracMM(t, edtest.P(e), bs, sc, side) {
			if c := b.X + b.W/2; (side == "left") != (c < pw) {
				t.Fatalf("%s: место на чужой странице: %+v", side, b)
			}
		}
	}
	if _, _, err := e.PlaceBoxes(sid, "всё"); err == nil {
		t.Fatal("неизвестная область принята")
	}
	if _, _, err := e.PlaceBoxes("нет", "spread"); err == nil {
		t.Fatal("неизвестный разворот принят")
	}
	// Пустая сетка из одной страницы: на другой нет мест.
	edtest.Change(e, func(p *model.Project) {
		p.Freeze(p.Spreads[0])
		p.Spreads[0].Grid = p.Spreads[0].Grid[:1]
		p.Spreads[0].Photos = nil
	})
	left, _, errL := e.PlaceBoxes(sid, "left")
	right, _, errR := e.PlaceBoxes(sid, "right")
	if (errL == nil) == (errR == nil) || len(left)+len(right) != 1 {
		t.Fatalf("одно место: left %v/%v right %v/%v", left, errL, right, errR)
	}
	for scope, want := range map[string]string{"left": "На левой странице нет мест", "right": "На правой странице нет мест"} {
		_, _, err := e.PlaceBoxes(sid, scope)
		if err != nil && (err.Error() != want || !errors.Is(err, model.ErrNoPlaces)) {
			t.Fatalf("%s: текст %q", scope, err)
		}
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Grid = nil; p.Spreads[0].Photos = nil })
	if _, _, err := e.PlaceBoxes(sid, "spread"); err == nil || err.Error() != "На развороте нет мест" || !errors.Is(err, model.ErrNoPlaces) {
		t.Fatalf("пустой разворот: %v", err)
	}
}

// areaFrac отдаёт то же, что PlaceBoxes: доли после общего зажима.
func TestAreaFracMatchesPlaceBoxes(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c")).Spreads[0].ID
	edtest.FreezeAt(t, e, 0)
	for _, scope := range []string{model.ScopeLeft, model.ScopeRight, model.ScopeSpread} {
		want, _, err := e.PlaceBoxes(sid, scope)
		got := grid.AreaFrac(edtest.P(e), edtest.P(e).Spreads[0], scope)
		if err != nil {
			if len(got) != 0 {
				t.Fatalf("%s: %v", scope, got)
			}
			continue
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s: %v ≠ %v", scope, got, want)
		}
	}
}

func inUnit(b model.Box) bool {
	return b.W >= model.MinBoxFrac && b.H >= model.MinBoxFrac && b.X >= 0 && b.Y >= 0 && b.X+b.W <= 1+1e-9 && b.Y+b.H <= 1+1e-9
}

func tplOf(scope string, n int) model.Template {
	t := model.Template{ID: model.NewID(), Name: "t", Scope: scope}
	for i := range n {
		t.Boxes = append(t.Boxes, model.Box{X: float64(i) / 20, Y: 0.1, W: 0.04, H: 0.5})
	}
	return t
}

// fracMM — доли шаблона в мм через fracBoxes (то, чем пользуется ApplyGrid).
func fracMM(t *testing.T, p *model.Project, frac []model.Box, tplScope, scope string) []layout.Rect {
	t.Helper()
	out, err := grid.FracBoxes(p, nil, tplScope, scope, frac)
	if err != nil {
		t.Fatalf("fracBoxes(%s, %s): %v", tplScope, scope, err)
	}
	rs := make([]layout.Rect, len(out))
	for i, b := range out {
		rs[i] = rectOfBox(b)
	}
	return rs
}

func rectOfBox(b model.Box) layout.Rect { return layout.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H} }
