package grid_test

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Раскладки сторонки обложки (127 G2): ApplyCoverGrid, StepCoverGrid, CoverSideStates,
// CoverGridChoicesOf. Обёртки Editor (G3) ещё нет — тест зовёт функции через edtest.Try, как их
// позовёт Editor, и держит память последней формы сам (cv.last).

// photoDims — размеры файла по первой букве ID: h 3:2, v 2:3, s 1:1, l 5:4, p 4:5.
var photoDims = map[byte][2]int{'h': {300, 200}, 'v': {200, 300}, 's': {300, 300}, 'l': {500, 400}, 'p': {400, 500}}

// cv — редактор с обложкой и память формы по сторонкам, как её держит Editor.
type cv struct {
	t    *testing.T
	e    *album.Editor
	last [2][]model.Box
}

// newCover — обложка группы group на странице формата name с пресетом preset (как sidesEd в view).
func newCover(t *testing.T, group, name, preset string) *cv {
	t.Helper()
	i := slices.IndexFunc(model.GroupFormats(group), func(f model.PrintFormat) bool { return f.Name == name })
	if i < 0 {
		t.Fatalf("нет формата %s %s", group, name)
	}
	f := model.GroupFormats(group)[i]
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	s := edtest.P(e).Settings
	s.PageW, s.PageH, s.DPI, s.Layflat, s.Safe, s.Bleed = f.PageW, f.PageH, 300, true, f.Safe, f.Bleed
	e.SetSettings(s)
	if _, err := e.ApplyCoverPreset(preset); err != nil {
		t.Fatal(err)
	}
	return &cv{t: t, e: e}
}

func fsfera(t *testing.T) *cv { return newCover(t, model.CoverGroupFsfera, "30×30", "fsfera-thin") }
func fabrika(t *testing.T) *cv {
	return newCover(t, model.CoverGroupFabrika, "30×20", "fabrika-photo")
}

func (c *cv) p() *model.Project { return edtest.P(c.e) }

// reg регистрирует фото ids (размеры — по первой букве ID). Вызывается внутри той же правки, что
// и посадка: PruneRefs в конце правки убирает ссылки на фото, которых нет в альбоме.
func reg(p *model.Project, ids ...string) {
	for _, id := range ids {
		if _, ok := p.Photos[id]; !ok && id != "" {
			d := photoDims[id[0]]
			p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: d[0], H: d[1]}
		}
	}
}

// places — места сторонки по порядку Grid.
func (c *cv) places(scope string) (out []*model.Place) {
	p := c.p()
	for _, pl := range p.Cover.Sheet.Grid {
		if grid.InArea(p, p.Cover.Sheet, pl.X, pl.W, scope) {
			out = append(out, pl)
		}
	}
	return out
}

func (c *cv) box(scope string) layout.Rect {
	for _, sd := range c.p().CoverSides() {
		if sd.Scope == scope {
			return sd.Box
		}
	}
	c.t.Fatalf("нет сторонки %s", scope)
	return layout.Rect{}
}

// apply — ApplyCoverGrid одной правкой; память формы пишется после успеха.
func (c *cv) apply(scope string, frac []model.Box) int {
	c.t.Helper()
	var left int
	_, err := edtest.Try(c.e, "", func(p *model.Project) (err error) {
		left, err = grid.ApplyCoverGrid(p, scope, frac)
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	i, _ := grid.CoverSideIndex(scope)
	c.last[i] = model.ClampBoxes(frac)
	return left
}

// step — StepCoverGrid под ключом слияния covergrid:<scope>; true — проект изменился.
func (c *cv) step(scope string, delta int) bool {
	c.t.Helper()
	rev := edtest.Rev(c.e)
	var shape []model.Box
	_, err := edtest.Try(c.e, "covergrid:"+scope, func(p *model.Project) (err error) {
		shape, err = grid.StepCoverGrid(p, scope, delta, c.last)
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	changed := edtest.Rev(c.e) != rev
	if changed != (shape != nil) {
		c.t.Fatalf("step %s %+d: shape %v, но изменён=%v", scope, delta, shape != nil, changed)
	}
	if shape != nil {
		i, _ := grid.CoverSideIndex(scope)
		c.last[i] = shape
	}
	return changed
}

func (c *cv) choices(scope string, n int, tpls ...model.Template) model.CoverGridChoices {
	return grid.CoverGridChoicesOf(grid.CoverInput(c.p(), scope), n, tpls)
}

func (c *cv) state(scope string) model.CoverSideState {
	i, _ := grid.CoverSideIndex(scope)
	return grid.CoverSideStates(c.p(), c.last)[i]
}

// clear снимает все места сторонки.
func (c *cv) clear(scope string) {
	edtest.Change(c.e, func(p *model.Project) {
		s := p.Cover.Sheet
		s.Grid = slices.DeleteFunc(s.Grid, func(pl *model.Place) bool { return grid.InArea(p, s, pl.X, pl.W, scope) })
		s.Photos = model.GridPhotos(s)
	})
	i, _ := grid.CoverSideIndex(scope)
	c.last[i] = nil
}

// fill ставит на сторонку len(ids) мест первой встроенной сеткой и сажает фото по порядку
// ("" — пустое место). Память формы — как после щелчка по плитке.
func (c *cv) fill(scope string, ids ...string) {
	c.t.Helper()
	c.clear(scope)
	c.apply(scope, c.choices(scope, len(ids)).Builtin[0].Boxes)
	c.seat(scope, nil, ids...)
}

// seat сажает фото в места сторонки по порядку без смены мест; tweak (может быть nil) правит
// записи фото той же правкой.
func (c *cv) seat(scope string, tweak func(p *model.Project), ids ...string) {
	c.t.Helper()
	edtest.Change(c.e, func(p *model.Project) {
		reg(p, ids...)
		if tweak != nil {
			tweak(p)
		}
		s := p.Cover.Sheet
		k := 0
		for _, pl := range s.Grid {
			if grid.InArea(p, s, pl.X, pl.W, scope) {
				pl.Photo = ids[k]
				k++
			}
		}
		s.Photos = model.GridPhotos(s)
	})
	if got := len(c.places(scope)); got != len(ids) {
		c.t.Fatalf("мест %d, фото %d", got, len(ids))
	}
}

// mm — доли зоны → мм листа.
func mm(box layout.Rect, f model.Box) layout.Rect {
	return layout.Rect{X: box.X + f.X*box.W, Y: box.Y + f.Y*box.H, W: f.W * box.W, H: f.H * box.H}
}

// rectsMM — доли зоны → прямоугольники мм от её угла.
func rectsMM(box layout.Rect, fr []model.Box) []layout.Rect {
	out := make([]layout.Rect, len(fr))
	for i, f := range fr {
		out[i] = layout.Rect{X: f.X * box.W, Y: f.Y * box.H, W: f.W * box.W, H: f.H * box.H}
	}
	return out
}

// wantShape — места сторонки совпали с долями frac (мм листа, ± 0,02) по порядку.
func (c *cv) wantShape(scope string, frac []model.Box, what string) {
	c.t.Helper()
	pls, box := c.places(scope), c.box(scope)
	if len(pls) != len(frac) {
		c.t.Fatalf("%s: мест %d, ждали %d", what, len(pls), len(frac))
	}
	for i, pl := range pls {
		w := mm(box, frac[i])
		if math.Abs(pl.X-w.X) > 0.02 || math.Abs(pl.Y-w.Y) > 0.02 || math.Abs(pl.W-w.W) > 0.02 || math.Abs(pl.H-w.H) > 0.02 {
			c.t.Fatalf("%s: место %d %v, ждали %v", what, i, []float64{pl.X, pl.Y, pl.W, pl.H}, w)
		}
	}
}

// inside — места сторонки внутри её безопасной зоны (± 0,01 мм) и не пересекают заворот, расстав и корешок.
func (c *cv) inside(scope, what string) {
	c.t.Helper()
	box := c.box(scope)
	const eps = 0.01
	for i, pl := range c.places(scope) {
		a := pl.Rect().AABB() // у повёрнутого места в зоне его описанный прямоугольник
		if a.X < box.X-eps || a.Y < box.Y-eps || a.X+a.W > box.X+box.W+eps || a.Y+a.H > box.Y+box.H+eps {
			c.t.Fatalf("%s: место %d %v вне зоны %+v", what, i, []float64{pl.X, pl.Y, pl.W, pl.H, pl.Rot}, box)
		}
		for _, z := range c.p().CoverZones() {
			if z.Kind != "wrap" && z.Kind != "hinge" && z.Kind != "spine" {
				continue
			}
			r := z.Rect
			if a.X < r.X+r.W-eps && a.X+a.W > r.X+eps && a.Y < r.Y+r.H-eps && a.Y+a.H > r.Y+eps {
				c.t.Fatalf("%s: место %d заходит в зону %s %+v", what, i, z.Kind, r)
			}
		}
	}
}

// mine — свой шаблон страницы на n мест полосами.
func mine(id string, n int) model.Template {
	t := model.Template{ID: id, Name: id, Scope: model.TplPage}
	for i := range n {
		t.Boxes = append(t.Boxes, model.Box{X: 0.02 + 0.96*float64(i)/float64(n), Y: 0.1, W: 0.96/float64(n) - 0.01, H: 0.8})
	}
	return t
}

// TestCoverGridInsideSide (127-2): места любой плитки и шага лежат внутри Box (± 0,01 мм), не в
// заворотах, расставах и корешке — у обеих сторонок, Фотосферы 30×30 и Фабрики 30×20, во всех
// раскладках фона (front, both, wrap).
func TestCoverGridInsideSide(t *testing.T) {
	for _, mk := range []struct {
		name string
		new  func(*testing.T) *cv
	}{{"Фотосфера 30×30", fsfera}, {"Фабрика 30×20", fabrika}} {
		for _, lay := range []string{model.CoverFront, model.CoverBoth, model.CoverWrap} {
			for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
				c := mk.new(t) // фото в месте только одно: на каждую сторонку своя обложка
				edtest.Change(c.e, func(p *model.Project) { p.Cover.Layout = lay })
				walkSide(c, mk.name+" "+lay+" "+scope, scope, lay == model.CoverFront)
				edtest.MustGrid(t, c.e)
			}
		}
	}
}

// TestCoverGridInsideRotated (127-2): повёрнутые коробки своего шаблона («Мои» с Rot) тоже держатся
// в зоне сторонки целиком — по описанному прямоугольнику: у края, в углу и крупнее зоны (сжимаются).
func TestCoverGridInsideRotated(t *testing.T) {
	tpl := model.Template{ID: "rot", Name: "rot", Scope: model.TplPage, Boxes: []model.Box{
		{X: 0.9, Y: 0.8, W: 0.3, H: 0.3, Rot: 30},     // за правым нижним краем
		{X: -0.1, Y: -0.05, W: 0.4, H: 0.2, Rot: -20}, // за левым верхним
		{X: 0.0, Y: 0.0, W: 1, H: 0.9, Rot: 45},       // габарит заведомо больше зоны
		{X: 0.4, Y: 0.4, W: 0.2, H: 0.2, Rot: 90},     // внутри: стоит как есть
	}}
	for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
		c := fsfera(t)
		c.apply(scope, tpl.Boxes)
		c.inside(scope, "повёрнутые "+scope)
		pls := c.places(scope)
		if len(pls) != len(tpl.Boxes) {
			t.Fatalf("%s: мест %d, ждали %d", scope, len(pls), len(tpl.Boxes))
		}
		for i, pl := range pls {
			if pl.Rot != tpl.Boxes[i].Rot {
				t.Errorf("%s место %d: угол %v, ждали %v", scope, i, pl.Rot, tpl.Boxes[i].Rot)
			}
		}
		// Внутреннее место (Rot 90 в центре) не сдвинуто: зажим трогает только вылезшие.
		box, in := c.box(scope), pls[3]
		if w := mm(box, tpl.Boxes[3]); math.Abs(in.X-w.X) > 0.02 || math.Abs(in.Y-w.Y) > 0.02 || math.Abs(in.W-w.W) > 0.02 {
			t.Errorf("%s: внутреннее место сдвинуто: %v, ждали %v", scope, []float64{in.X, in.Y, in.W}, w)
		}
		edtest.MustGrid(t, c.e)
	}
}

// TestCoverGridBlockTouchesZone (127-3, 0.26-127-29): блок любой встроенной плитки и «По фото» с
// полем 0 упирается в зону хотя бы по одной оси: масштаб в зону верный. Пара миллиметров щели по
// другой оси — не ошибка масштаба: layout бережёт пропорции мест (Crop 0), а у 1:1 в зоне 266×270
// или тройки с заданными пропорциями суммарная высота ≈ 268 мм.
func TestCoverGridBlockTouchesZone(t *testing.T) {
	for _, mk := range []struct {
		name string
		new  func(*testing.T) *cv
	}{{"Фотосфера 30×30", fsfera}, {"Фабрика 30×20", fabrika}} {
		c := mk.new(t)
		for n := 1; n <= 7; n++ {
			if testkit.Lite() && n%3 != 1 {
				continue
			}
			c.fill(model.ScopeRight, []string{"h1", "v2", "s3", "l4", "p5", "h6", "v7"}[:n]...)
			ch, box := c.choices(model.ScopeRight, n), c.box(model.ScopeRight)
			for _, tile := range append(slices.Clone(ch.ByPhoto), ch.Builtin...) {
				x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
				for _, b := range tile.Boxes {
					r := mm(box, b)
					x0, y0, x1, y1 = min(x0, r.X), min(y0, r.Y), max(x1, r.X+r.W), max(y1, r.Y+r.H)
				}
				const eps = 0.02
				if x0 < box.X-eps || y0 < box.Y-eps || x1 > box.X+box.W+eps || y1 > box.Y+box.H+eps {
					t.Fatalf("%s n=%d %s: блок вне зоны", mk.name, n, tile.ID)
				}
				wide, high := x1-x0 >= box.W-eps, y1-y0 >= box.H-eps
				if !wide && !high {
					t.Errorf("%s n=%d %s: блок %.2f×%.2f не касается зоны %.2f×%.2f ни по одной оси", mk.name, n, tile.ID, x1-x0, y1-y0, box.W, box.H)
				}
			}
		}
	}
}

// walkSide — все плитки трёх рядов и шаги ‹ › на сторонке с 1…7 местами; full — все числа мест,
// иначе (быстрый уровень, раскладки фона кроме front) — через два.
func walkSide(c *cv, what, scope string, full bool) {
	t := c.t
	t.Helper()
	photos := []string{"h1", "v2", "s3", "l4", "p5", "h6", "v7"}
	for n := 1; n <= len(photos); n++ {
		if testkit.Lite() && !full && n%3 != 1 {
			continue
		}
		c.fill(scope, photos[:n]...)
		ch := c.choices(scope, n, mine("m", n))
		if len(ch.ByPhoto) == 0 || len(ch.Builtin) == 0 || len(ch.Mine) != 1 {
			t.Fatalf("%s n=%d: ряды %d/%d/%d", what, n, len(ch.ByPhoto), len(ch.Builtin), len(ch.Mine))
		}
		for _, row := range [][]model.GridChoice{ch.ByPhoto, ch.Builtin, ch.Mine} {
			for _, tile := range row {
				c.apply(scope, tile.Boxes)
				if got := len(c.places(scope)); got != len(tile.Boxes) {
					t.Fatalf("%s: плитка %s на %d мест, на сторонке %d", what, tile.ID, len(tile.Boxes), got)
				}
				c.inside(scope, what+" плитка "+tile.ID)
			}
		}
		c.apply(scope, ch.ByPhoto[0].Boxes)
		for _, d := range []int{1, -1} {
			for range len(ch.ByPhoto) + 1 {
				c.step(scope, d)
				c.inside(scope, what+" шаг")
			}
		}
	}
}

// sheetRest — лист обложки без мест, порядка слоёв и фото мест: всё, что раскладка трогать не должна.
func sheetRest(t *testing.T, p *model.Project) string {
	t.Helper()
	s := *p.Cover.Sheet
	s.Grid, s.Photos, s.Order = nil, nil, nil
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestApplyCoverGridKeepsOther (127-3): плитка на лицевой не трогает заднюю, надписи, стикеры, фото
// фона и корешка; фото сверх мест уходят из листа (left), один шаг отмены, та же форма — холостая.
func TestApplyCoverGridKeepsOther(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "h1", "v2")
	c.fill(model.ScopeRight, "h3", "v4", "s5", "l6")
	edtest.Change(c.e, func(p *model.Project) {
		reg(p, "bgl", "bgr", "bgs", "stk")
		s := p.Cover.Sheet
		p.Cover.Layout = model.CoverBoth
		s.BgL, s.BgR = "#e8e0d0", "#d0e0e8"
		s.BgPhotoL = &model.Backdrop{Photo: "bgl", X: 0, Y: 0, W: 100, H: 100}
		s.BgPhotoR = &model.Backdrop{Photo: "bgr", X: 350, Y: 0, W: 100, H: 100}
		s.BgPhotoS = &model.Backdrop{Photo: "bgs", X: 300, Y: 0, W: 30, H: 100}
		s.Stickers = []*model.Sticker{{ID: "aaaaaaaaaaaa", Photo: "stk", X: 40, Y: 40, W: 20, H: 20}}
		s.Texts[0].Text = "Надпись"
	})
	before := sheetRest(t, c.p())
	for _, key := range []string{"bgPhotoL", "bgPhotoR", "bgPhotoS", "stickers", "texts"} {
		if !strings.Contains(before, `"`+key+`"`) {
			t.Fatalf("подготовка: в листе нет %s: %s", key, before)
		}
	}
	leftBefore, _ := json.Marshal(c.places(model.ScopeLeft))
	rightPhotos := []string{"h3", "v4", "s5", "l6"}
	tile := c.choices(model.ScopeRight, 2).Builtin[0]
	if tile.Drop != 2 {
		t.Fatalf("Drop плитки %d, ждали 2", tile.Drop)
	}
	whole := model.Marshal(c.p())
	undo := edtest.UndoLen(c.e)

	if left := c.apply(model.ScopeRight, tile.Boxes); left != 2 {
		t.Fatalf("left = %d, ждали 2", left)
	}
	if got := edtest.UndoLen(c.e); got != undo+1 {
		t.Fatalf("шагов отмены +%d, ждали +1", got-undo)
	}
	if after := sheetRest(t, c.p()); after != before {
		t.Errorf("тронуто лишнее:\n%s\nбыло\n%s", after, before)
	}
	if now, _ := json.Marshal(c.places(model.ScopeLeft)); string(now) != string(leftBefore) {
		t.Errorf("задняя изменилась:\n%s\nбыло\n%s", now, leftBefore)
	}
	pls := c.places(model.ScopeRight)
	if len(pls) != 2 || pls[0].Photo != rightPhotos[0] || pls[1].Photo != rightPhotos[1] {
		t.Fatalf("лицевая: %+v, ждали первые два фото по порядку", pls)
	}
	for _, id := range rightPhotos[2:] {
		if slices.Contains(c.p().Cover.Sheet.Photos, id) {
			t.Errorf("фото %s осталось на обложке, ждали ленту", id)
		}
	}
	c.inside(model.ScopeRight, "плитка")

	// Та же форма — холостая: ни rev, ни шага отмены.
	rev, undo2 := edtest.Rev(c.e), edtest.UndoLen(c.e)
	if left := c.apply(model.ScopeRight, tile.Boxes); left != 0 {
		t.Errorf("повтор: left %d", left)
	}
	edtest.Same(t, c.e, rev, undo2, "та же форма")

	// Один шаг отмены возвращает проект целиком.
	c.e.Undo()
	if back := model.Marshal(c.p()); string(back) != string(whole) {
		t.Error("Undo не вернул проект до плитки")
	}
}

// narrowPlace делает место сторонки с номером k узким 1:2 (высота вдвое больше ширины).
func (c *cv) narrowPlace(scope string, k int) {
	edtest.Change(c.e, func(p *model.Project) {
		for i, pl := range c.places(scope) {
			if i == k {
				pl.H = 2 * pl.W
			}
		}
	})
}

// TestCoverGridByPhotoNoCrop (127-4): места «По фото» несут пропорции Project.Aspects (± 0,1 мм) —
// свой кадр и четверть поворота учтены, — а у пустого места остаются свои W/H.
func TestCoverGridByPhotoNoCrop(t *testing.T) {
	tweak := func(p *model.Project) {
		r := p.Photos["h4"]
		r.Asp = 1 // свой кадр 1:1 у файла 3:2
		p.Photos["h4"] = r
		r = p.Photos["h5"]
		r.Frame = &model.Frame{S: 1, R: 90} // лежит боком: ячейке нужно 2:3
		p.Photos["h5"] = r
	}
	ids := []string{"h1", "v2", "", "s3", "h4", "h5"}
	for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
		c := fsfera(t) // фото в месте только одно: на каждую сторонку своя обложка
		c.fill(scope, ids...)
		c.seat(scope, tweak, ids...)
		// Пустое место делаем узким 1:2: у встроенной раскладки оно почти 3:2, и «всегда 1,5» не поймали бы.
		c.narrowPlace(scope, 2)
		emptyW, emptyH := c.places(scope)[2].W, c.places(scope)[2].H
		if c.places(scope)[2].Photo != "" || math.Abs(emptyW/emptyH-0.5) > 0.01 {
			t.Fatalf("подготовка: пустое место %.2f×%.2f, ждали 1:2", emptyW, emptyH)
		}
		asp := c.p().Aspects([]string{"h1", "v2", "s3", "h4", "h5"})
		if math.Abs(asp[3]-1) > 1e-9 || math.Abs(asp[4]-2.0/3) > 1e-9 {
			t.Fatalf("подготовка: пропорции %v", asp)
		}
		want := []float64{asp[0], asp[1], emptyW / emptyH, asp[2], asp[3], asp[4]}
		row := c.choices(scope, len(ids)).ByPhoto
		if len(row) < 2 {
			t.Fatalf("%s: в ряду «По фото» %d раскладок", scope, len(row))
		}
		for ri, ch := range row {
			box := c.box(scope)
			for i, b := range ch.Boxes {
				r := mm(box, b)
				if math.Abs(r.W-want[i]*r.H) > 0.1 {
					t.Fatalf("%s раскладка %d место %d: %.2f×%.2f, пропорция должна быть %.4f", scope, ri, i, r.W, r.H, want[i])
				}
			}
			if ch.Drop != 0 || ch.Scope != model.TplPage || ch.Name != "" {
				t.Errorf("карточка «По фото» %+v", ch)
			}
		}
		// Поставленная плитка несёт те же пропорции в местах проекта.
		c.apply(scope, row[1].Boxes)
		for i, pl := range c.places(scope) {
			if math.Abs(pl.W-want[i]*pl.H) > 0.1 {
				t.Fatalf("%s после плитки: место %d %.2f×%.2f, пропорция %.4f", scope, i, pl.W, pl.H, want[i])
			}
		}
	}
}

// TestStepCoverGridCycle (127-5): круг в обе стороны по ряду «По фото», серия — один шаг отмены
// (covergrid:<scope>), после плитки (не из ряда) › ставит первую раскладку ряда, ‹ — последнюю.
func TestStepCoverGridCycle(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "h3")
	row := c.choices(scope, 3).ByPhoto
	if len(row) < 3 {
		t.Fatalf("ряд «По фото»: %d", len(row))
	}
	tile := c.choices(scope, 3).Builtin[0].Boxes
	c.apply(scope, tile)
	if st := c.state(scope); !st.Known || st.Alone || st.Places != 3 {
		t.Fatalf("после плитки: %+v", st)
	}

	undo := edtest.UndoLen(c.e)
	whole := model.Marshal(c.p())
	if !c.step(scope, 1) {
		t.Fatal("› после плитки ничего не сделал")
	}
	c.wantShape(scope, row[0].Boxes, "› после плитки — первая")
	for i := 1; i < len(row); i++ {
		c.step(scope, 1)
		c.wantShape(scope, row[i].Boxes, "›")
	}
	c.step(scope, 1)
	c.wantShape(scope, row[0].Boxes, "› по кругу")
	c.step(scope, -1)
	c.wantShape(scope, row[len(row)-1].Boxes, "‹ с первой — последняя")
	for i := len(row) - 2; i >= 0; i-- {
		c.step(scope, -1)
		c.wantShape(scope, row[i].Boxes, "‹")
	}
	if got := edtest.UndoLen(c.e); got != undo+1 {
		t.Errorf("серия шагов: +%d шагов отмены, ждали +1", got-undo)
	}
	c.e.Undo()
	if back := model.Marshal(c.p()); string(back) != string(whole) {
		t.Error("один Undo не вернул проект до серии")
	}

	// ‹ после плитки — последняя раскладка ряда.
	c.apply(scope, tile)
	c.step(scope, -1)
	c.wantShape(scope, row[len(row)-1].Boxes, "‹ после плитки")
	c.inside(scope, "шаги")
}
