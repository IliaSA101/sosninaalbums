package grid_test

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Шаблон сторонки обложки «зона + поле» (0.27-132 G1): «Сохранить как шаблон…» со сторонки и «Мои»
// со страницы на сторонку живут в долях одной рамки — безопасная зона плюс поле альбома с каждой стороны.

// frameOf — рамка шаблона сторонки: зона и поле альбома m со всех сторон (то же, что coverFrame).
func frameOf(box layout.Rect, m float64) layout.Rect {
	return layout.Rect{X: box.X - m, Y: box.Y - m, W: box.W + 2*m, H: box.H + 2*m}
}

// setMargin — поле альбома (мм) до всего остального: зона от него не зависит, рамка — да.
func (c *cv) setMargin(m float64) {
	edtest.Change(c.e, func(p *model.Project) { p.Settings.Margin = m })
}

// frame — рамка шаблона сторонки scope в нынешнем поле альбома.
func (c *cv) frame(scope string) layout.Rect { return frameOf(c.box(scope), c.p().Settings.Margin) }

// seatMM ставит на сторонку n пустых мест и задаёт места по мм листа (как их мог поставить фотограф:
// за пунктир, повёрнутыми); b.Rot, Shape и Round берутся как есть.
func (c *cv) seatMM(scope string, boxes ...model.Box) {
	c.t.Helper()
	ids := make([]string, len(boxes))
	c.fill(scope, ids...)
	edtest.Change(c.e, func(p *model.Project) {
		k := 0
		for _, pl := range p.Cover.Sheet.Grid {
			if grid.InArea(p, p.Cover.Sheet, pl.X, pl.W, scope) {
				b := boxes[k]
				pl.X, pl.Y, pl.W, pl.H, pl.Rot, pl.Shape, pl.Round = b.X, b.Y, b.W, b.H, b.Rot, b.Shape, b.Round
				k++
			}
		}
	})
}

// save — «Сохранить как шаблон» со сторонки целиком: PlaceBoxes → templates.json → чтение файла.
func (c *cv) save(scope string) model.Template {
	c.t.Helper()
	boxes, tplScope, err := c.e.PlaceBoxes(model.CoverID, scope)
	if err != nil {
		c.t.Fatal(err)
	}
	return viaFile(c.t, model.Template{ID: "t1", Name: "Шаблон 1", Scope: tplScope, Boxes: boxes})
}

// viaFile — шаблон после записи в templates.json и чтения (округление долей до 1e-5).
func viaFile(t *testing.T, tpl model.Template) model.Template {
	t.Helper()
	data, err := model.MarshalTemplates([]model.Template{tpl})
	if err != nil {
		t.Fatal(err)
	}
	list, _, err := model.ParseTemplates(data)
	if err != nil || len(list) != 1 {
		t.Fatalf("чтение шаблонов: %v, %d", err, len(list))
	}
	return list[0]
}

// mineOf — плитка «Мои» с шаблоном tpl на сторонке scope (доли зоны, как уходят в ApplyCoverGrid).
func (c *cv) mineOf(scope string, tpl model.Template) []model.Box {
	c.t.Helper()
	ch := c.choices(scope, len(tpl.Boxes), tpl)
	if len(ch.Mine) != 1 {
		c.t.Fatalf("в «Мои» %d плиток", len(ch.Mine))
	}
	return ch.Mine[0].Boxes
}

// placesMM — места сторонки в мм листа по порядку.
func (c *cv) placesMM(scope string) (out []model.Box) {
	for _, pl := range c.places(scope) {
		out = append(out, model.Box{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Rot: pl.Rot, Shape: pl.Shape, Round: pl.Round})
	}
	return out
}

// inside — место целиком в зоне box (его описанный прямоугольник, ± tol мм).
func inside(b model.Box, box layout.Rect, tol float64) bool {
	r := geom.RotRect{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}.AABB()
	return r.X >= box.X-tol && r.Y >= box.Y-tol && r.X+r.W <= box.X+box.W+tol && r.Y+r.H <= box.Y+box.H+tol
}

func nearBox(a, b model.Box, tol float64) bool {
	n := func(x, y float64) bool { return math.Abs(x-y) <= tol }
	return n(a.X, b.X) && n(a.Y, b.Y) && n(a.W, b.W) && n(a.H, b.H) && a.Rot == b.Rot && a.Shape == b.Shape && a.Round == b.Round
}

// tplCovers — обложки теста: Фотосфера 30×30 и Фабрика 30×20 с полем 0 и нынешним 12 мм.
func tplCovers(t *testing.T) (out []struct {
	name string
	c    *cv
}) {
	for _, mk := range []struct {
		name string
		f    func(*testing.T) *cv
	}{{"Фотосфера 30×30", fsfera}, {"Фабрика 30×20", fabrika}} {
		for _, m := range []float64{0, 12} {
			c := mk.f(t)
			c.setMargin(m)
			out = append(out, struct {
				name string
				c    *cv
			}{fmt.Sprintf("%s, поле %g", mk.name, m), c})
		}
	}
	return out
}

// TestCoverPlaceBoxesRoundTrip (132-1): места в зоне — «сохранить → файл (1e-5) → Мои → поставить» —
// холостая правка (rev и отмена прежние); угол, овал и скругление сохраняются. Места «у края» и
// «навылет» — первая постановка равна плитке (один шаг), повторная холостая.
func TestCoverPlaceBoxesRoundTrip(t *testing.T) {
	for _, tc := range tplCovers(t) {
		for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
			t.Run(tc.name+" "+scope, func(t *testing.T) {
				c := tc.c
				z := c.box(scope)
				// Три места в зоне: прямоугольник во всю высоту у левого края, овал, скруглённое с наклоном.
				c.seatMM(scope,
					model.Box{X: z.X, Y: z.Y, W: 90, H: z.H},
					model.Box{X: z.X + 100, Y: z.Y + 10, W: 60, H: 80, Shape: model.ShapeOval},
					model.Box{X: z.X + 120, Y: z.Y + 110, W: 70, H: 50, Rot: 7.3, Round: 20})
				want := c.placesMM(scope)
				tpl := c.save(scope)
				if tpl.Scope != model.TplPage || len(tpl.Boxes) != 3 {
					t.Fatalf("шаблон: %s, %d мест", tpl.Scope, len(tpl.Boxes))
				}
				rev, u := edtest.Rev(c.e), edtest.UndoLen(c.e)
				c.ed(scope, c.mineOf(scope, tpl))
				for i, b := range c.placesMM(scope) {
					if !nearBox(b, want[i], 0.01) {
						t.Errorf("место %d: %+v, было %+v", i, b, want[i])
					}
				}
				edtest.Same(t, c.e, rev, u, "места в зоне")
			})
		}
	}
}

// TestCoverPlaceBoxesEdges (132-1): «у края», «навылет» и повёрнутое у края — первая постановка равна
// плитке «Мои» (проект меняется на один шаг), вторая холостая; результат целиком в зоне.
func TestCoverPlaceBoxesEdges(t *testing.T) {
	for _, tc := range tplCovers(t) {
		t.Run(tc.name, func(t *testing.T) {
			c, scope := tc.c, model.ScopeRight
			z := c.box(scope)
			c.seatMM(scope,
				model.Box{X: z.X - 8, Y: z.Y + 20, W: 100, H: 60},                          // у левого края, за пунктир
				model.Box{X: z.X - 20, Y: z.Y + z.H - 50, W: z.W + 40, H: 80},              // навылет по обе стороны
				model.Box{X: z.X + z.W - 40, Y: z.Y + 90, W: 90, H: 70, Rot: 12},           // повёрнутое у правого края
				model.Box{X: z.X + 100, Y: z.Y - 15, W: 50, H: 50, Shape: model.ShapeOval}) // над верхним краем
			tpl := c.save(scope)
			fr := c.frame(scope)
			for i, b := range tpl.Boxes { // шаблон уже в зоне: плитка равна тому, что встанет
				m := model.Box{X: fr.X + b.X*fr.W, Y: fr.Y + b.Y*fr.H, W: b.W * fr.W, H: b.H * fr.H, Rot: b.Rot}
				if !inside(m, z, 0.01) {
					t.Errorf("шаблон, место %d вне зоны: %+v", i, m)
				}
			}
			mine := c.mineOf(scope, tpl)
			for i, b := range mine {
				if m := mm(z, b); !inside(model.Box{X: m.X, Y: m.Y, W: m.W, H: m.H, Rot: b.Rot}, z, 0.01) {
					t.Errorf("плитка, место %d вне зоны: %+v", i, m)
				}
			}
			rev, u := edtest.Rev(c.e), edtest.UndoLen(c.e)
			c.ed(scope, mine)
			if edtest.Rev(c.e) != rev+1 || edtest.UndoLen(c.e) != u+1 {
				t.Fatalf("первая постановка: rev %d → %d, отмена %d → %d", rev, edtest.Rev(c.e), u, edtest.UndoLen(c.e))
			}
			for i, b := range c.placesMM(scope) {
				m := mm(z, mine[i])
				if !nearBox(b, model.Box{X: m.X, Y: m.Y, W: m.W, H: m.H, Rot: mine[i].Rot, Shape: mine[i].Shape, Round: mine[i].Round}, 0.01) || !inside(b, z, 0.01) {
					t.Errorf("место %d: %+v, плитка %+v", i, b, m)
				}
			}
			// Повторное сохранение даёт тот же шаблон, повторная постановка — холостая.
			rev, u = edtest.Rev(c.e), edtest.UndoLen(c.e)
			c.ed(scope, c.mineOf(scope, c.save(scope)))
			edtest.Same(t, c.e, rev, u, "повторная постановка")
		})
	}
}

// TestCoverSaveClipKeepsGap (132-1, 0.27-132-15): место за пунктиром рядом с соседом при сохранении
// обрезается по зоне, а не сдвигается: зазор между ними после «сохранить → поставить» прежний.
func TestCoverSaveClipKeepsGap(t *testing.T) {
	for _, tc := range tplCovers(t) {
		t.Run(tc.name, func(t *testing.T) {
			c, scope := tc.c, model.ScopeLeft
			z := c.box(scope)
			const gap = 4.0
			a := model.Box{X: z.X - 10, Y: z.Y, W: 10 + 120, H: z.H} // выступает за пунктир на 10 мм
			b := model.Box{X: a.X + a.W + gap, Y: z.Y, W: 100, H: z.H}
			c.seatMM(scope, a, b)
			c.ed(scope, c.mineOf(scope, c.save(scope)))
			got := c.placesMM(scope)
			if g := got[1].X - (got[0].X + got[0].W); math.Abs(g-gap) > 0.02 {
				t.Errorf("зазор %.3f мм, был %v: левое место сдвинуто, а не обрезано", g, gap)
			}
			if math.Abs(got[0].X-z.X) > 0.01 || math.Abs(got[0].W-120) > 0.02 { // правый край на месте, левый — по зоне
				t.Errorf("обрезанное место: %+v, зона %+v", got[0], z)
			}
			if !inside(got[0], z, 0.01) || !inside(got[1], z, 0.01) {
				t.Errorf("места вне зоны: %+v", got)
			}
		})
	}
}

// TestCoverPlaceBoxesErrors (132-2): отказы — текстом для тоста; ничего не меняется.
func TestCoverPlaceBoxesErrors(t *testing.T) {
	c := fsfera(t)
	c.clear(model.ScopeLeft)
	c.clear(model.ScopeRight)
	cases := []struct{ scope, want string }{
		{model.ScopeLeft, "На задней нет мест"},
		{model.ScopeRight, "На лицевой нет мест"},
		{model.ScopeSpread, "неизвестная область"},
		{model.ScopeSpine, "неизвестная область"},
		{"", "неизвестная область"},
	}
	for _, tc := range cases {
		_, _, err := c.e.PlaceBoxes(model.CoverID, tc.scope)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%q без мест: %v, ждали %q", tc.scope, err, tc.want)
		}
		if tc.want != "неизвестная область" && !errors.Is(err, model.ErrNoPlaces) {
			t.Errorf("%q: ошибка не ErrNoPlaces", tc.scope)
		}
	}
	// Места только на корешке: ни к одной сторонке они не относятся.
	cg := c.p().CoverGeo()
	c.seatMM(model.ScopeRight, model.Box{X: cg.X[3], Y: 100, W: cg.X[4] - cg.X[3], H: 50})
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Sheet.Grid[0].X = cg.X[3] })
	for _, sc := range []string{model.ScopeLeft, model.ScopeRight} {
		if _, _, err := c.e.PlaceBoxes(model.CoverID, sc); !errors.Is(err, model.ErrNoPlaces) {
			t.Errorf("место на корешке попало в %s: %v", sc, err)
		}
	}
	// Нет безопасной зоны.
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Spec.W, p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 50, 30, 30 })
	if _, _, err := c.e.PlaceBoxes(model.CoverID, model.ScopeRight); err == nil || !strings.Contains(err.Error(), "Безопасная зона сторонки пуста") {
		t.Errorf("зоны нет: %v", err)
	}
	// Нет обложки.
	e := edtest.GridEd(t, edtest.Calendar())
	if _, _, err := e.PlaceBoxes(model.CoverID, model.ScopeRight); err == nil || err.Error() != "Обложки нет" {
		t.Errorf("без обложки: %v", err)
	}
	// Чтение не меняет проект.
	c2 := fsfera(t)
	c2.fill(model.ScopeRight, "h1", "v2")
	rev, u := edtest.Rev(c2.e), edtest.UndoLen(c2.e)
	if _, _, err := c2.e.PlaceBoxes(model.CoverID, model.ScopeRight); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, c2.e, rev, u, "сохранение шаблона")
}

// TestCoverFrameMargins (132-4): сторонка → страница — поле 11,0–11,7 мм до обреза, верха, низа и
// сгиба; страница → сторонка — 0,3–1,2 мм от пунктира (двойного поля 11,4 мм нет).
func TestCoverFrameMargins(t *testing.T) {
	for _, mk := range []struct {
		name string
		f    func(*testing.T) *cv
	}{{"Фотосфера", fsfera}, {"Фабрика", fabrika}} {
		t.Run(mk.name, func(t *testing.T) {
			c := mk.f(t)
			scope := model.ScopeRight
			z := c.box(scope)
			c.seatMM(scope, model.Box{X: z.X, Y: z.Y, W: z.W, H: z.H}) // место во всю зону
			tpl := c.save(scope)
			sid := edtest.NewSpread(c.e, -1, edtest.Photos("a")).Spreads[0].ID
			for _, side := range []string{model.ScopeLeft, model.ScopeRight} {
				for k, v := range c.pageMargins(tpl, sid, side) {
					if v < 11.0 || v > 11.7 {
						t.Errorf("%s: поле %d = %.2f мм, ждали 11,0–11,7", side, k, v)
					}
				}
			}

			// Страница → сторонка: место страницы с полем альбома от всех краёв.
			p := c.p()
			_, s := p.Find(sid)
			pg, _ := p.AreaOf(s, model.ScopeLeft)
			m := p.Settings.Margin
			page := model.Template{ID: "pg", Name: "pg", Scope: model.TplPage,
				Boxes: []model.Box{{X: m / pg.W, Y: m / pg.H, W: 1 - 2*m/pg.W, H: 1 - 2*m/pg.H}}}
			b := mm(z, c.mineOf(scope, page)[0])
			for k, v := range [4]float64{b.X - z.X, b.Y - z.Y, z.X + z.W - (b.X + b.W), z.Y + z.H - (b.Y + b.H)} {
				if v < 0.3 || v > 1.2 {
					t.Errorf("страница → сторонка: отступ %d = %.2f мм от пунктира, ждали 0,3–1,2", k, v)
				}
			}
		})
	}
}

// pageMargins ставит шаблон tpl на страницу side разворота sid и отдаёт отступы единственного
// места от левого, верхнего, правого и нижнего краёв страницы, мм.
func (c *cv) pageMargins(tpl model.Template, sid, side string) [4]float64 {
	c.t.Helper()
	if _, _, err := c.e.ApplyGrid(sid, side, tpl.Scope, tpl.Boxes); err != nil {
		c.t.Fatal(err)
	}
	p := c.p()
	_, s := p.Find(sid)
	pg, _ := p.AreaOf(s, side)
	var pl *model.Place
	for _, g := range s.Grid {
		if grid.InArea(p, s, g.X, g.W, side) {
			pl = g
		}
	}
	return [4]float64{pl.X - pg.X, pl.Y - pg.Y, pg.X + pg.W - (pl.X + pl.W), pg.Y + pg.H - (pl.Y + pl.H)}
}

// TestCoverMineBleedNoOverlap (0.27-132-15): шаблон страницы «в край» из двух половин с промежутком
// 3 мм — на сторонке обрезается по зоне, а не сдвигается: соседи не наползают (зазор ≈ 3,1 мм).
func TestCoverMineBleedNoOverlap(t *testing.T) {
	for _, mk := range []struct {
		name string
		f    func(*testing.T) *cv
	}{{"Фотосфера", fsfera}, {"Фабрика", fabrika}} {
		t.Run(mk.name, func(t *testing.T) {
			c := mk.f(t)
			scope := model.ScopeLeft
			pageW := c.p().Settings.PageW
			tpl := model.Template{ID: "h", Name: "h", Scope: model.TplPage, Boxes: []model.Box{
				{X: 0, Y: 0, W: (pageW/2 - 1.5) / pageW, H: 1}, {X: (pageW/2 + 1.5) / pageW, Y: 0, W: (pageW/2 - 1.5) / pageW, H: 1}}}
			z := c.box(scope)
			ch := c.mineOf(scope, tpl)
			a, b := mm(z, ch[0]), mm(z, ch[1])
			if g := b.X - (a.X + a.W); g < 3.0 || g > 3.3 {
				t.Errorf("зазор %.2f мм, ждали ≈ 3,1 (наезд = сдвиг, а не обрезка)", g)
			}
			if math.Abs(a.X-z.X) > 1e-6 || math.Abs(b.X+b.W-(z.X+z.W)) > 1e-6 {
				t.Errorf("края не обрезаны по зоне: %+v %+v, зона %+v", a, b, z)
			}
		})
	}
}

// TestCoverMineRotatedEdge (0.27-132-15, риск): повёрнутое место шаблона страницы у края не
// обрезается, а удерживается в зоне сдвигом (holdRotated): угол и размер те же, описанный
// прямоугольник целиком в зоне. Поведение зафиксировано; обрезка повёрнутого — BACKLOG.
func TestCoverMineRotatedEdge(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	z := c.box(scope)
	tpl := model.Template{ID: "r", Name: "r", Scope: model.TplPage, Boxes: []model.Box{
		{X: 0, Y: 0.05, W: 0.3, H: 0.3, Rot: 10}, {X: 0.7, Y: 0.6, W: 0.3, H: 0.4, Rot: -15}}}
	ch := c.mineOf(scope, tpl)
	f := c.frame(scope)
	for i, b := range ch {
		m := mm(z, b)
		src := tpl.Boxes[i]
		if b.Rot != src.Rot || math.Abs(m.W-src.W*f.W) > 0.01 || math.Abs(m.H-src.H*f.H) > 0.01 {
			t.Errorf("повёрнутое место %d изменилось: %+v, шаблон %+v", i, m, src)
		}
		if !inside(model.Box{X: m.X, Y: m.Y, W: m.W, H: m.H, Rot: b.Rot}, z, 0.01) {
			t.Errorf("повёрнутое место %d вне зоны: %+v", i, m)
		}
	}
	rev, u := edtest.Rev(c.e), edtest.UndoLen(c.e)
	c.ed(scope, ch)
	c.ed(scope, ch)
	if edtest.Rev(c.e) != rev+1 || edtest.UndoLen(c.e) != u+1 {
		t.Errorf("две постановки: rev %d → %d, отмена %d → %d", rev, edtest.Rev(c.e), u, edtest.UndoLen(c.e))
	}
}

// TestCoverTplOtherFormat (132-1, п. 5 спеки): шаблон Фотосферы 30×30 на Фабрике 30×20 — в зоне,
// место во всю зону обрезается на ≈ 3 мм сверху и снизу и держится в 1 мм от пунктира по бокам;
// то же поле 12 → 5 мм — отступ ≈ 6,4 мм.
func TestCoverTplOtherFormat(t *testing.T) {
	src := fsfera(t)
	scope := model.ScopeRight
	zs := src.box(scope)
	src.seatMM(scope, model.Box{X: zs.X, Y: zs.Y, W: zs.W, H: zs.H})
	tpl := src.save(scope)

	dst := fabrika(t)
	zd := dst.box(scope)
	b := mm(zd, dst.mineOf(scope, tpl)[0])
	if !inside(model.Box{X: b.X, Y: b.Y, W: b.W, H: b.H}, zd, 0.01) {
		t.Fatalf("вне зоны: %+v", b)
	}
	if side := b.X - zd.X; side < 0.9 || side > 1.2 {
		t.Errorf("отступ по бокам %.2f мм, ждали ≈ 1,06", side)
	}
	// Место Фотосферы выходит за зону Фабрики на ≈ 3 мм сверху и снизу — и обрезано по ней.
	fz := frameOf(zd, dst.p().Settings.Margin)
	if over := zd.Y - (fz.Y + tpl.Boxes[0].Y*fz.H); over < 2.9 || over > 3.1 {
		t.Errorf("до обрезки место за пунктиром на %.2f мм, ждали ≈ 3", over)
	}
	if math.Abs(b.Y-zd.Y) > 0.01 || math.Abs(b.H-zd.H) > 0.01 {
		t.Errorf("высота не обрезана по зоне: %+v, зона %+v", b, zd)
	}

	narrow := fsfera(t)
	narrow.setMargin(5)
	zn := narrow.box(scope)
	n := mm(zn, narrow.mineOf(scope, tpl)[0])
	if gap := n.X - zn.X; gap < 6.3 || gap > 6.6 {
		t.Errorf("поле 12 → 5 мм: отступ %.2f мм, ждали ≈ 6,4", gap)
	}
}

// TestCoverSaveClipTiny (132-1, края): место целиком за пунктиром или с вылезшим на волос краем не
// пропадает и не уходит за зону — ужимается до geom.MinPlace у ближнего края; от шаблона до
// постановки то же самое.
func TestCoverSaveClipTiny(t *testing.T) {
	for _, tc := range tplCovers(t) {
		t.Run(tc.name, func(t *testing.T) {
			c, scope := tc.c, model.ScopeLeft
			z := c.box(scope)
			c.seatMM(scope,
				model.Box{X: z.X - 40, Y: z.Y + 10, W: 30, H: 50},     // целиком левее зоны
				model.Box{X: z.X + z.W + 2, Y: z.Y + 80, W: 8, H: 40}, // целиком правее (центр ещё на странице)
				model.Box{X: z.X - 29, Y: z.Y + 150, W: 30, H: 40})    // в зоне 1 мм из 30
			tpl := c.save(scope)
			if len(tpl.Boxes) != 3 {
				t.Fatalf("мест в шаблоне %d, ждали 3", len(tpl.Boxes))
			}
			for i, b := range c.mineOf(scope, tpl) {
				m := mm(z, b)
				if m.W < geom.MinPlace-0.01 || m.H < geom.MinPlace-0.01 || !inside(model.Box{X: m.X, Y: m.Y, W: m.W, H: m.H}, z, 0.01) {
					t.Errorf("место %d: %+v (зона %+v)", i, m, z)
				}
			}
			c.ed(scope, c.mineOf(scope, tpl))
			for i, b := range c.placesMM(scope) {
				if !inside(b, z, 0.01) {
					t.Errorf("после постановки место %d вне зоны: %+v", i, b)
				}
			}
		})
	}
}

// TestEditorApplyCoverGridSameShapeKnown (132-9): после Save → Load память формы пуста, свой шаблон —
// «Ручная». Холостая постановка того же шаблона (места уже такие) проект не трогает, но Editor
// запоминает форму, и вид знает её (known) — UI снимает «· Ручная ·» при том же rev.
func TestEditorApplyCoverGridSameShapeKnown(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	tpl := mine("m", 3)
	c.ed(scope, c.mineOf(scope, tpl))
	// На задней — те же доли в её зоне: общая память формы узнала бы и её после постановки справа.
	c.ed(model.ScopeLeft, c.mineOf(model.ScopeLeft, tpl))
	e2 := c.reload(filepath.Join(testkit.TempDir(t), "a.salbum"))
	if st := side(t, e2, scope); st.Known || st.Places != 3 {
		t.Fatalf("после Load: %+v", st)
	}
	rev, u := edtest.Rev(e2), edtest.UndoLen(e2)
	v, left, err := e2.ApplyCoverGrid(scope, c.mineOf(scope, tpl))
	if err != nil || left != 0 {
		t.Fatalf("холостая постановка: left %d, %v", left, err)
	}
	edtest.Same(t, e2, rev, u, "та же форма")
	if sd := v.Cover.Sides[1]; sd.Scope != scope || !sd.Known || sd.Places != 3 {
		t.Errorf("вид из ответа холостой постановки: %+v", sd)
	}
	if st := side(t, e2, scope); !st.Known {
		t.Errorf("вид Editor: %+v", st)
	}
	// Другая сторонка форму не получает.
	if st := side(t, e2, model.ScopeLeft); st.Known || st.Places != 3 {
		t.Errorf("задняя после постановки справа: %+v", st)
	}
}

// TestCoverMineHostile (11): шаблоны приходят из templates.json, а не из кода. Шаблон
// с NaN, бесконечностью, нулями, отрицательными и огромными долями — без паники, плитка конечная
// и целиком в зоне; нулевой и пустой шаблон в ряд не попадают.
func TestCoverMineHostile(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	z := c.box(scope)
	nan, inf := math.NaN(), math.Inf(1)
	bad := [][]model.Box{
		{{X: nan, Y: nan, W: nan, H: nan}},
		{{X: inf, Y: -inf, W: inf, H: inf}},
		{{X: 1e300, Y: -1e300, W: 1e300, H: 1e-300}},
		{{X: -5, Y: -5, W: 0, H: 0}},
		{{X: 0.5, Y: 0.5, W: -1, H: -1, Rot: 1e300}},
		{{X: 0, Y: 0, W: 1, H: 1, Rot: nan}},
	}
	for i, boxes := range bad {
		ch := c.choices(scope, 1, model.Template{ID: fmt.Sprint("bad", i), Name: "x", Scope: model.TplPage, Boxes: boxes})
		for _, m := range ch.Mine {
			for _, b := range m.Boxes {
				mm := mm(z, b)
				for _, v := range []float64{b.X, b.Y, b.W, b.H, b.Rot} {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Errorf("вариант %d: нечисловая доля %+v", i, b)
					}
				}
				if mm.X < z.X-0.01 || mm.Y < z.Y-0.01 || mm.X+mm.W > z.X+z.W+0.01 || mm.Y+mm.H > z.Y+z.H+0.01 {
					t.Errorf("вариант %d: место вне зоны: %+v", i, mm)
				}
			}
		}
	}
	if ch := c.choices(scope, 1, model.Template{ID: "e", Name: "e", Scope: model.TplPage}); len(ch.Mine) != 0 {
		t.Errorf("пустой шаблон в ряду: %+v", ch.Mine)
	}
}
