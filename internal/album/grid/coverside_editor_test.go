package grid_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обёртки Editor над раскладками сторонки (127 G3): ApplyCoverGrid, StepCoverGrid,
// CoverGridChoices и память формы coverLast. Тела — grid/coverside.go (проверены на функциях в
// coverside_test.go и coverside_state_test.go); здесь — шаги отмены, rev, память и вид.

// side — сторонка scope в виде.
func side(t *testing.T, e interface{ View() model.View }, scope string) model.CoverSide {
	t.Helper()
	v := e.View()
	if v.Cover == nil {
		t.Fatal("в виде нет обложки")
	}
	for _, sd := range v.Cover.Sides {
		if sd.Scope == scope {
			return sd
		}
	}
	t.Fatalf("в виде нет сторонки %s", scope)
	return model.CoverSide{}
}

// snapshot — байты проекта.
func snapshot(e *album.Editor) []byte {
	p, _ := e.Snapshot()
	return model.Marshal(p)
}

// nudge двигает первое место сторонки на dx мм (ручная правка мест).
func (c *cv) nudge(scope string, dx float64) {
	edtest.Change(c.e, func(p *model.Project) {
		s := p.Cover.Sheet
		for _, pl := range s.Grid {
			if grid.InArea(p, s, pl.X, pl.W, scope) {
				pl.X += dx
				return
			}
		}
	})
}

// reload — Save в path и новый Editor из Load: память формы у него пуста.
func (c *cv) reload(path string) *album.Editor { return c.reloadFrom(c.e, path) }

func (c *cv) reloadFrom(e *album.Editor, path string) *album.Editor {
	c.t.Helper()
	if err := e.Save(path); err != nil {
		c.t.Fatal(err)
	}
	return edtest.Open(c.t, edtest.LoadFile(c.t, path), path)
}

// ed — ApplyCoverGrid через Editor с проверкой отсутствия ошибки.
func (c *cv) ed(scope string, frac []model.Box) int {
	c.t.Helper()
	_, left, err := c.e.ApplyCoverGrid(scope, frac)
	if err != nil {
		c.t.Fatal(err)
	}
	return left
}

// TestEditorApplyCoverGrid (127-3, 127-14): плитка — один шаг отмены; фото сверх мест уходят в
// ленту (left), отмена возвращает; та же форма — холостая (rev и отмена прежние); вид знает
// число мест сторонки, а другая сторонка цела.
func TestEditorApplyCoverGrid(t *testing.T) {
	c := fsfera(t)
	scope, other := model.ScopeRight, model.ScopeLeft
	c.clear(scope)
	c.fill(other, "h9", "v8")
	otherShape := rectsOfPlaces(c.p(), other)
	u0 := edtest.UndoLen(c.e)
	if left := c.ed(scope, c.choices(scope, 3).Builtin[0].Boxes); left != 0 {
		t.Fatalf("на пустую сторонку left = %d", left)
	}
	if edtest.UndoLen(c.e) != u0+1 || side(t, c.e, scope).Places != 3 || !side(t, c.e, scope).Known {
		t.Fatalf("плитка: undo %d → %d, сторонка %+v", u0, edtest.UndoLen(c.e), side(t, c.e, scope))
	}
	c.seat(scope, nil, "h1", "v2", "s3")
	three := rectsOfPlaces(c.p(), scope)

	u1 := edtest.UndoLen(c.e)
	if left := c.ed(scope, c.choices(scope, 2).Builtin[0].Boxes); left != 1 {
		t.Fatalf("3 фото на 2 места: left = %d, ждали 1", left)
	}
	if edtest.UndoLen(c.e) != u1+1 || side(t, c.e, scope).Places != 2 {
		t.Fatalf("замена 3→2: undo %d → %d, мест %d", u1, edtest.UndoLen(c.e), side(t, c.e, scope).Places)
	}
	if !slices.Equal(rectsOfPlaces(c.p(), other), otherShape) {
		t.Error("другая сторонка изменилась")
	}
	c.e.Undo()
	if got := rectsOfPlaces(c.p(), scope); !slices.Equal(got, three) {
		t.Errorf("после отмены: %v, было %v", got, three)
	}

	// Та же форма (после отмены стоят прежние три места) — холостая: ни шага, ни rev.
	rev, u1 := edtest.Rev(c.e), edtest.UndoLen(c.e)
	if c.ed(scope, c.choices(scope, 3).Builtin[0].Boxes) != 0 {
		t.Error("холостая плитка вернула left")
	}
	edtest.Same(t, c.e, rev, u1, "та же форма")
}

// TestEditorApplyCoverGridBad: отказы — NaN, пусто, не left/right, нет зоны, нет обложки — без шага,
// rev и памяти формы; ошибка текстом, вид текущий.
func TestEditorApplyCoverGridBad(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	good := c.choices(scope, 2).Builtin[0].Boxes
	u, rev := edtest.UndoLen(c.e), edtest.Rev(c.e)
	for name, call := range map[string]func() error{
		"NaN": func() error {
			_, _, err := c.e.ApplyCoverGrid(scope, []model.Box{{X: math.NaN(), W: 0.5, H: 0.5}})
			return err
		},
		"пусто":    func() error { _, _, err := c.e.ApplyCoverGrid(scope, nil); return err },
		"корешок":  func() error { _, _, err := c.e.ApplyCoverGrid(model.ScopeSpine, good); return err },
		"разворот": func() error { _, _, err := c.e.ApplyCoverGrid(model.ScopeSpread, good); return err },
		"мусор":    func() error { _, _, err := c.e.ApplyCoverGrid("нет такой", good); return err },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
		edtest.Same(t, c.e, rev, u, name)
	}
	if st := side(t, c.e, scope); st.Known || st.Places != 0 {
		t.Errorf("после отказов: %+v", st)
	}
	// Отказ не пишет память. Правая — 2 места, левая забита так, что форма на 3 места упирается в
	// MaxPlaces. После отказа ставим те же 3 места руками (без памяти): Known должен остаться false,
	// ошибочно записанная форма узнала бы их.
	other := model.ScopeLeft
	shape := []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.3}, {X: 0.5, Y: 0.5, W: 0.3, H: 0.3}, {X: 0.1, Y: 0.6, W: 0.3, H: 0.3}}
	c.ed(scope, good)
	filler := make([]model.Box, model.MaxPlaces-len(good))
	for i := range filler {
		filler[i] = model.Box{X: 0.01 * float64(i%50), Y: 0.01 * float64(i%50), W: 0.2, H: 0.2}
	}
	c.ed(other, filler)
	if len(c.places(other))+len(c.places(scope)) != model.MaxPlaces {
		t.Fatalf("подготовка: мест %d+%d", len(c.places(other)), len(c.places(scope)))
	}
	rev, u = edtest.Rev(c.e), edtest.UndoLen(c.e)
	if _, _, err := c.e.ApplyCoverGrid(scope, shape); err == nil {
		t.Fatal("сверх MaxPlaces: ждали ошибку")
	}
	edtest.Same(t, c.e, rev, u, "сверх MaxPlaces")
	box := c.box(scope)
	edtest.Change(c.e, func(p *model.Project) {
		s := p.Cover.Sheet
		var k int
		s.Grid = slices.DeleteFunc(s.Grid, func(pl *model.Place) bool {
			if grid.InArea(p, s, pl.X, pl.W, scope) {
				return true
			}
			k++
			return k == 1 // одно место левой: освободить квоту под три правых
		})
		for _, f := range shape {
			r := mm(box, f)
			s.Grid = append(s.Grid, &model.Place{ID: model.NewPlaceID(p.PlaceIDs()), X: r.X, Y: r.Y, W: r.W, H: r.H})
		}
	})
	if st := side(t, c.e, scope); st.Places != len(shape) || st.Known {
		t.Errorf("после отказа места узнаны (память записана): %+v", st)
	}
	rev, u = edtest.Rev(c.e), edtest.UndoLen(c.e)
	c.e.StepCoverGrid(model.ScopeSpine, 1)
	edtest.Same(t, c.e, rev, u, "Step: корешок")

	// Зона сторонки пуста (поля больше сторонки): ошибка текстом, правки нет.
	// Через Change: SetCoverSpec зажал бы поля.
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Spec.W, p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 50, 30, 30 })
	if len(c.p().CoverSides()) != 0 {
		t.Fatal("подготовка: сторонки остались")
	}
	rev, u = edtest.Rev(c.e), edtest.UndoLen(c.e)
	if _, _, err := c.e.ApplyCoverGrid(scope, good); err == nil {
		t.Error("пустая зона: ждали ошибку")
	}
	edtest.Same(t, c.e, rev, u, "пустая зона")
}

// TestEditorCoverGridNoCover: нет обложки — все три метода холостые, вид без обложки, паники нет.
func TestEditorCoverGridNoCover(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar())
	rev, u := edtest.Rev(e), edtest.UndoLen(e)
	box := []model.Box{{X: 0, Y: 0, W: 1, H: 1}}
	if _, left, err := e.ApplyCoverGrid(model.ScopeRight, box); err != nil || left != 0 {
		t.Errorf("ApplyCoverGrid без обложки: left %d, %v", left, err)
	}
	if v := e.StepCoverGrid(model.ScopeRight, 1); v.Cover != nil {
		t.Error("вид с обложкой из ниоткуда")
	}
	if ch := e.CoverGridChoices(model.ScopeRight, 2, nil); len(ch.ByPhoto)+len(ch.Builtin)+len(ch.Mine) != 0 || ch.Places != 0 {
		t.Errorf("галерея без обложки: %+v", ch)
	}
	edtest.Same(t, e, rev, u, "без обложки")
}

// TestEditorStepCoverGridSeries (127-5, 127-14): серия ‹ › по сторонке — один шаг отмены, круг в обе
// стороны возвращается к началу, отмена и возврат памяти формы не трогают, шаг другой сторонки рвёт серию.
func TestEditorStepCoverGridSeries(t *testing.T) {
	c := fsfera(t)
	scope, other := model.ScopeRight, model.ScopeLeft
	c.fill(scope, "l1", "p2", "h3")
	c.fill(other, "h4", "v5", "s6")
	c.ed(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	c.ed(other, c.choices(other, 3).ByPhoto[0].Boxes)
	start := rectsOfPlaces(c.p(), scope)
	m := len(c.choices(scope, 3).ByPhoto)
	if m < 3 {
		t.Fatalf("подготовка: в ряду %d раскладок", m)
	}

	u0, before := edtest.UndoLen(c.e), snapshot(c.e)
	var seen [][][4]float64
	for range m {
		c.e.StepCoverGrid(scope, 1)
		seen = append(seen, rectsOfPlaces(c.p(), scope))
	}
	if edtest.UndoLen(c.e) != u0+1 {
		t.Fatalf("серия › ×%d: шагов отмены +%d, ждали +1", m, edtest.UndoLen(c.e)-u0)
	}
	if !slices.Equal(seen[m-1], start) {
		t.Errorf("круг › ×%d не вернул начало: %v, было %v", m, seen[m-1], start)
	}
	if slices.Equal(seen[0], start) {
		t.Error("› ничего не сменил")
	}
	c.e.StepCoverGrid(scope, -1)
	if got := rectsOfPlaces(c.p(), scope); !slices.Equal(got, seen[m-2]) {
		t.Errorf("‹ после круга: %v, ждали %v", got, seen[m-2])
	}
	if edtest.UndoLen(c.e) != u0+1 {
		t.Error("‹ в той же серии добавил шаг")
	}

	// Отмена серии — одним шагом к началу; память формы цела: › снова листает.
	c.e.Undo()
	if got := rectsOfPlaces(c.p(), scope); !slices.Equal(got, start) {
		t.Errorf("отмена серии: %v, ждали начало %v", got, start)
	}
	if !bytes.Equal(snapshot(c.e), before) {
		t.Error("отмена серии — не байт в байт")
	}
	if st := side(t, c.e, scope); !st.Known {
		t.Errorf("после отмены форма не узнана: %+v", st)
	}
	c.e.StepCoverGrid(scope, 1)
	if got := rectsOfPlaces(c.p(), scope); !slices.Equal(got, seen[0]) {
		t.Errorf("› после отмены: %v, ждали %v", got, seen[0])
	}

	// Другая сторонка — свой ключ слияния: новый шаг, прежняя сторонка цела.
	u1, keep := edtest.UndoLen(c.e), rectsOfPlaces(c.p(), scope)
	c.e.StepCoverGrid(other, 1)
	c.e.StepCoverGrid(other, 1)
	if edtest.UndoLen(c.e) != u1+1 || !slices.Equal(rectsOfPlaces(c.p(), scope), keep) {
		t.Errorf("серия на другой сторонке: шагов +%d, лицевая %v", edtest.UndoLen(c.e)-u1, rectsOfPlaces(c.p(), scope))
	}
	// Плитка рвёт серию: › после неё — новый шаг.
	c.ed(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	u2 := edtest.UndoLen(c.e)
	c.e.StepCoverGrid(scope, 1)
	if edtest.UndoLen(c.e) != u2+1 {
		t.Errorf("› после плитки: шагов +%d, ждали +1", edtest.UndoLen(c.e)-u2)
	}
}

// TestEditorStepCoverGridNoop (127-7, 127-8): холостые случаи не двигают ни rev, ни отмену, ни память —
// форма не узнана (сдвиг руками), ноль мест, delta 0, одна раскладка в ряду, неизвестная сторонка.
func TestEditorStepCoverGridNoop(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "s3")
	c.ed(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	noop := func(what string, f func()) {
		t.Helper()
		rev, u := edtest.Rev(c.e), edtest.UndoLen(c.e)
		f()
		edtest.Same(t, c.e, rev, u, what)
	}
	noop("delta 0", func() { c.e.StepCoverGrid(scope, 0) })
	noop("корешок", func() { c.e.StepCoverGrid(model.ScopeSpine, 1) })
	noop("мусор", func() { c.e.StepCoverGrid("", -1) })

	c.nudge(scope, 3) // место сдвинули руками: «Ручная», › места не трогает
	if st := side(t, c.e, scope); st.Known {
		t.Fatalf("после сдвига форма узнана: %+v", st)
	}
	noop("ручная форма", func() { c.e.StepCoverGrid(scope, 1) })

	c.clear(scope) // ноль мест
	if st := side(t, c.e, scope); st.Known || st.Places != 0 {
		t.Fatalf("ноль мест: %+v", st)
	}
	noop("0 мест", func() { c.e.StepCoverGrid(scope, 1) })

	// В ряду одна раскладка, и она стоит: Alone, › и ‹ холостые.
	c.fill(scope, "h7")
	c.ed(scope, c.choices(scope, 1).ByPhoto[0].Boxes)
	if st := side(t, c.e, scope); !st.Known || !st.Alone {
		t.Fatalf("одно место: %+v", st)
	}
	noop("alone ›", func() { c.e.StepCoverGrid(scope, 1) })
	noop("alone ‹", func() { c.e.StepCoverGrid(scope, -1) })
}

// TestEditorCoverMemory (127-6, 127-14): память формы живёт в Editor — свой шаблон узнаётся после ▦,
// переживает замену фото, отмену и возврат; новый Editor (Save → Load) её не знает, а форма «По фото» и
// встроенная узнаются и так. Память не в файле: байты файла те же, что и без неё.
func TestEditorCoverMemory(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	c.ed(scope, c.choices(scope, 3).Builtin[0].Boxes)
	c.seat(scope, nil, "l1", "p2", "l3")
	c.ed(scope, mine("m", 3).Boxes)
	if st := side(t, c.e, scope); !st.Known || st.Places != 3 {
		t.Fatalf("свой шаблон сразу после ▦: %+v", st)
	}
	c.seat(scope, nil, "v4", "v5", "v6") // фото заменили, места стоят
	if st := side(t, c.e, scope); !st.Known {
		t.Fatalf("после замены фото: %+v", st)
	}
	c.e.Undo()
	c.e.Redo()
	if st := side(t, c.e, scope); !st.Known {
		t.Fatalf("после Undo и Redo: %+v", st)
	}

	// Save → Load: память исчезла (свой шаблон — «Ручная»), места те же.
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e2 := c.reload(path)
	if st := side(t, e2, scope); st.Known || st.Places != 3 {
		t.Errorf("после Load свой шаблон без памяти: %+v", st)
	}
	if got, want := rectsOfPlaces(edtest.P(e2), scope), rectsOfPlaces(c.p(), scope); !slices.Equal(got, want) {
		t.Errorf("места после Load: %v, было %v", got, want)
	}
	rev, u := edtest.Rev(e2), edtest.UndoLen(e2)
	e2.StepCoverGrid(scope, 1) // не узнана → холостая
	edtest.Same(t, e2, rev, u, "› без памяти")

	c.ed(scope, c.choices(scope, 3).ByPhoto[0].Boxes)
	e3 := c.reload(path)
	if st := side(t, e3, scope); !st.Known {
		t.Errorf("после Load «По фото» не узнана: %+v", st)
	}
	if v := e3.View(); v.Cover == nil || edtest.P(e3).Version != model.FormatVersion {
		t.Errorf("версия формата %d", edtest.P(e3).Version)
	}
}

// TestEditorCoverSaveLoad (127-14): обе сторонки — плитки и шаги разных рядов; после Save → Load места те же,
// повторная запись — байт в байт, итог — обычные места листа обложки.
func TestEditorCoverSaveLoad(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "h1", "v2", "s3", "l4")
	c.fill(model.ScopeRight, "p5", "h6")
	c.ed(model.ScopeLeft, c.choices(model.ScopeLeft, 4).ByPhoto[0].Boxes)
	c.e.StepCoverGrid(model.ScopeLeft, 1)
	c.e.StepCoverGrid(model.ScopeLeft, 1)
	c.ed(model.ScopeRight, c.choices(model.ScopeRight, 2).Builtin[0].Boxes)
	c.e.StepCoverGrid(model.ScopeRight, -1)

	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e2 := c.reload(path)
	for _, sc := range []string{model.ScopeLeft, model.ScopeRight} {
		if got, want := rectsOfPlaces(edtest.P(e2), sc), rectsOfPlaces(c.p(), sc); len(got) == 0 || !slices.Equal(got, want) {
			t.Errorf("%s после Load: %v, было %v", sc, got, want)
		}
	}
	// ID фото при первом Load перевыдаются; дальше запись устойчива — байт в байт.
	if err := e2.Save(path); err != nil {
		t.Fatal(err)
	}
	was, _ := os.ReadFile(path)
	e3 := c.reloadFrom(e2, path)
	if err := e3.Save(path); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(was, now) {
		t.Error("Save → Load → Save: байты разошлись")
	}
	if err := edtest.Reopen(edtest.P(e3)); err != nil {
		t.Error(err)
	}
}

// TestEditorCoverViewPrint: вид печати не считает состояния сторонок (Places, Known, Alone — нули), рабочий
// вид их несёт; зоны одни и те же.
func TestEditorCoverViewPrint(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2")
	c.ed(scope, c.choices(scope, 2).ByPhoto[0].Boxes)
	_, pv := c.e.Snapshot()
	var printed model.CoverSide
	for _, sd := range pv.Cover.Sides {
		if sd.Scope == scope {
			printed = sd
		}
	}
	work := side(t, c.e, scope)
	if work.Places != 2 || !work.Known {
		t.Fatalf("рабочий вид: %+v", work)
	}
	if printed.Places != 0 || printed.Known || printed.Alone || printed.Box != work.Box {
		t.Errorf("вид печати: %+v, рабочий %+v", printed, work)
	}
}

// TestEditorCoverGridChoices (127-12): Editor отдаёт то же, что grid по снятому входу; чтение — без правки;
// «Мои» приходят аргументом; неизвестная сторонка — пустые ряды.
func TestEditorCoverGridChoices(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.fill(scope, "h1", "v2", "s3")
	rev, u := edtest.Rev(c.e), edtest.UndoLen(c.e)
	tpls := []model.Template{mine("m3", 3), mine("m2", 2)}
	got := c.e.CoverGridChoices(scope, 3, tpls)
	want := grid.CoverGridChoicesOf(grid.CoverInput(c.p(), scope), 3, tpls)
	if got.Scope != scope || got.Places != 3 || got.Photos != 3 || len(got.ByPhoto) == 0 || len(got.Mine) != 1 || got.Mine[0].ID != "m3" {
		t.Errorf("галерея: %+v", got)
	}
	if len(got.ByPhoto) != len(want.ByPhoto) || len(got.Builtin) != len(want.Builtin) || got.Box != want.Box {
		t.Errorf("Editor и grid разошлись: %d/%d рядов", len(got.ByPhoto), len(want.ByPhoto))
	}
	if ch := c.e.CoverGridChoices(model.ScopeSpine, 3, tpls); ch.Box.W != 0 || len(ch.ByPhoto)+len(ch.Builtin)+len(ch.Mine) != 0 {
		t.Errorf("корешок: %+v", ch)
	}
	edtest.Same(t, c.e, rev, u, "CoverGridChoices")
}

// TestEditorCoverMemoryAfterStep (127-6): форма, поставленная ‹ ›, тоже в памяти — после замены фото
// портретами ряд «По фото» другой, форма в нём не найдена, но узнана по памяти, и › идёт по новому ряду с первой.
func TestEditorCoverMemoryAfterStep(t *testing.T) {
	c := fsfera(t)
	scope := model.ScopeRight
	c.clear(scope)
	c.ed(scope, c.choices(scope, 3).Builtin[0].Boxes)
	c.seat(scope, nil, "l1", "p2", "l3")
	c.e.StepCoverGrid(scope, 1)
	old := c.choices(scope, 3).ByPhoto[0].Boxes
	c.wantShape(scope, old, "› с первой")

	c.seat(scope, nil, "v4", "v5", "v6")
	if st := side(t, c.e, scope); !st.Known {
		t.Fatalf("после замены фото форма шага не узнана: %+v", st)
	}
	row := c.choices(scope, 3).ByPhoto
	if sameFrac(row[0].Boxes, old) {
		t.Fatal("подготовка: ряд не изменился")
	}
	c.e.StepCoverGrid(scope, 1)
	c.wantShape(scope, row[0].Boxes, "› по новому ряду — первая")
}
