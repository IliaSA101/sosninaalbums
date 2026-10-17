package grid_test

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Форма места (спека 044): добавление, переключение, скругление и перенос правками сетки.

// shapes — "форма:скругление" мест разворота i по порядку.
func shapes(p *model.Project, i int) []string {
	var out []string
	for _, pl := range p.Spreads[i].Grid {
		out = append(out, shapeOf(pl))
	}
	return out
}

func shapeOf(pl *model.Place) string {
	return pl.Shape + ":" + map[float64]string{0: "0", 12: "12", 25: "25", 50: "50"}[pl.Round]
}

// «Место ▸ Круглое» — то же квадратное место в форме круга; чужая форма — прямоугольник.
// Каждое добавление — один шаг; у «Авто» — закрепление той же правкой.
func TestAddPlaceRound(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 60, "a")})
	undo := edtest.UndoLen(e)
	if _, err := e.AddPlace("s1", 1, model.ShapeOval); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddPlace("s1", 0, "heart"); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != 3 || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("мест %d, шагов %d", len(g), edtest.UndoLen(e)-undo)
	}
	if c := g[1]; c.Shape != model.ShapeOval || c.Round != 0 || c.W != c.H || c.W < 90 {
		t.Fatalf("круглое: %+v", *c)
	}
	if r := g[2]; r.Shape != "" || r.W != r.H || r.W != g[1].W {
		t.Fatalf("чужая форма: %+v", *r)
	}
	// «Авто»: закрепление и круглое место — один шаг, отмена возвращает «Авто».
	a, id := edtest.AutoEd(t)
	undo = edtest.UndoLen(a)
	if _, err := a.AddPlace(id, 1, model.ShapeOval); err != nil {
		t.Fatal(err)
	}
	if g := edtest.P(a).Spreads[0].Grid; len(g) != 4 || g[3].Shape != model.ShapeOval || g[0].Shape != "" || edtest.UndoLen(a) != undo+1 {
		t.Fatalf("у «Авто»: %v, шагов %d", shapes(edtest.P(a), 0), edtest.UndoLen(a)-undo)
	}
	a.Undo()
	if edtest.P(a).Spreads[0].Grid != nil {
		t.Fatal("отмена не вернула «Авто»")
	}
}

// Переключатель формы — по образцу SetPlaceFit: группа одним шагом, рамка, фото и кадр
// стоят; круг снимает скругление; повтор, чужой ID и чужая форма — холостые.
func TestSetPlaceShape(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 60, "a"), edtest.Gp(130, 12, 80, 80, "b"), edtest.Gp(12, 150, 60, 60, "")})
	g := edtest.P(e).Spreads[0].Grid
	ids := []string{g[0].ID, "b"} // ID места и ID фото — обе ссылки
	e.SetPlaceRound("s1", ids, 25)
	e.SetFrame("a", 2, 0.5, -0.5)
	edtest.Tilt(e, "a", 10)
	before := slices.Clone([]model.Place{*g[0], *g[1], *g[2]})
	frame := edtest.P(e).Photos["a"].Frame
	undo := edtest.UndoLen(e)
	e.SetPlaceShape("s1", ids, model.ShapeOval)
	g = edtest.P(e).Spreads[0].Grid
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{"oval:0", "oval:0", ":0"}) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("формы %v, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	for i, pl := range g {
		b := before[i]
		if pl.X != b.X || pl.Y != b.Y || pl.W != b.W || pl.H != b.H || pl.Photo != b.Photo || pl.Rot != b.Rot {
			t.Fatalf("место %d сдвинулось: %+v → %+v", i, b, *pl)
		}
	}
	if f := edtest.P(e).Photos["a"].Frame; f == nil || *f != *frame {
		t.Fatalf("кадр изменился: %+v → %+v", frame, f)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceShape("s1", ids, model.ShapeOval)
	e.SetPlaceShape("s1", []string{"нет такого"}, "")
	e.SetPlaceShape("s1", ids, "heart")
	e.SetPlaceShape("нет", ids, "")
	edtest.Same(t, e, rev, undo, "повтор, чужой ID, чужая форма")
	// Отмена — один шаг, скругление вернулось.
	e.Undo()
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{":25", ":25", ":0"}) {
		t.Fatalf("после отмены: %v", got)
	}
	e.Redo()
	e.SetPlaceShape("s1", []string{"b"}, "")
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{"oval:0", ":0", ":0"}) {
		t.Fatalf("обратно в прямоугольник: %v", got)
	}
	edtest.MustGrid(t, e)
}

// У «Авто»: круг закрепляет разворот той же правкой, прямоугольник — холостая.
func TestSetPlaceShapeAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceShape(id, []string{"a"}, "")
	e.SetPlaceShape(id, []string{"a"}, "heart")
	e.SetPlaceShape(id, []string{"нет"}, model.ShapeOval) // закреплять нечего — откат
	e.SetPlaceRound(id, []string{"a"}, 0)
	edtest.Same(t, e, rev, undo, `"" , чужая форма и ID, скругление 0 у «Авто»`)
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая правка закрепила разворот")
	}
	e.SetPlaceShape(id, []string{"a", "c"}, model.ShapeOval)
	edtest.MustGrid(t, e)
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{"oval:0", ":0", "oval:0"}) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("формы %v, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("отмена не вернула «Авто»")
	}
	// Скругление у «Авто» тоже закрепляет.
	e.SetPlaceRound(id, []string{"b"}, 12)
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{":0", ":12", ":0"}) {
		t.Fatalf("скругление у «Авто»: %v", got)
	}
}

// Скругление: зажим 0…50 целым, NaN/Inf — холостые, у круга — холостая, группа — один шаг.
func TestSetPlaceRound(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 60, "a"), edtest.Gp(130, 12, 80, 80, "b"), edtest.Gp(12, 150, 60, 60, "")})
	g := edtest.P(e).Spreads[0].Grid
	e.SetPlaceShape("s1", []string{g[2].ID}, model.ShapeOval)
	all := []string{g[0].ID, g[1].ID, g[2].ID}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceRound("s1", all, math.NaN())
	e.SetPlaceRound("s1", all, math.Inf(1))
	e.SetPlaceRound("s1", all, math.Inf(-1))
	e.SetPlaceRound("s1", []string{g[2].ID}, 30) // у круга скругления нет
	e.SetPlaceRound("s1", all, 0)                // и так 0
	e.SetPlaceRound("нет", all, 20)              // чужой разворот
	edtest.Same(t, e, rev, undo, "NaN, Inf, круг, тот же 0, чужой разворот")
	for _, c := range []struct {
		in   float64
		want []string
	}{
		{70, []string{":50", ":50", "oval:0"}},
		{12.4, []string{":12", ":12", "oval:0"}},
		{-5, []string{":0", ":0", "oval:0"}},
	} {
		undo = edtest.UndoLen(e)
		e.SetPlaceRound("s1", all, c.in)
		if got := shapes(edtest.P(e), 0); !slices.Equal(got, c.want) || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%v: %v, шагов %d", c.in, got, edtest.UndoLen(e)-undo)
		}
	}
	e.SetPlaceRound("s1", all, 12)
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceRound("s1", all, 12.2) // то же после округления
	edtest.Same(t, e, rev, undo, "12,2 после 12")
	edtest.MustGrid(t, e)
}

// Деление, отражение, копия раскладки, дубль разворота и шаблон «сохранить → применить»
// сохраняют форму (решение №41: половины круга — овалы той же формы).
func TestShapeSurvivesGridOps(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 120, 120, "a"), edtest.Gp(150, 12, 100, 60, "b"), edtest.Gp(300, 40, 100, 100, "")}, []string{"x"})
	g := edtest.P(e).Spreads[0].Grid
	e.SetPlaceShape("s1", []string{g[0].ID, g[2].ID}, model.ShapeOval)
	e.SetPlaceRound("s1", []string{g[1].ID}, 25)
	want := []string{"oval:0", ":25", "oval:0"}
	// Шаблон: сохранить → файл → применить на другой разворот.
	boxes, scope, err := e.PlaceBoxes("s1", model.ScopeSpread)
	if err != nil || len(boxes) != 3 {
		t.Fatalf("PlaceBoxes: %v %v", err, boxes)
	}
	data, err := model.MarshalTemplates([]model.Template{{ID: model.NewID(), Name: "к", Scope: scope, Boxes: boxes}})
	if err != nil {
		t.Fatal(err)
	}
	back, ver, err := model.ParseTemplates(data)
	if err != nil || ver != 5 || len(back) != 1 {
		t.Fatalf("круг файла: v%d %v", ver, err)
	}
	if _, _, err := e.ApplyGrid("s2", model.ScopeSpread, back[0].Scope, back[0].Boxes); err != nil {
		t.Fatal(err)
	}
	if got := shapes(edtest.P(e), 1); !slices.Equal(got, want) {
		t.Fatalf("шаблон: %v", got)
	}
	// Копия раскладки (с отражением тоже) — на тот же разворот s2.
	e.SetPlaceShape("s2", []string{edtest.P(e).Spreads[1].Grid[0].ID}, "")
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	if got := shapes(edtest.P(e), 1); !slices.Equal(got, want) {
		t.Fatalf("копия раскладки: %v", got)
	}
	// Дубль разворота.
	if _, err := e.DuplicateGrid("s1", 1, false); err != nil {
		t.Fatal(err)
	}
	if got := shapes(edtest.P(e), 1); !slices.Equal(got, want) {
		t.Fatalf("дубль: %v", got)
	}
	// Отражение: места меняются сторонами, формы с ними.
	e.MirrorGrid("s1", model.ScopeSpread)
	g = edtest.P(e).Spreads[0].Grid
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, want) || g[0].X < 300 {
		t.Fatalf("отражение: %v, x %v", got, g[0].X)
	}
	// Деление круга — два овала; деление скруглённого — оба скруглены.
	if _, err := e.ArrangePlaces("s1", "splitH", []string{g[0].ID, g[1].ID}); err != nil {
		t.Fatal(err)
	}
	if got := shapes(edtest.P(e), 0); !slices.Equal(got, []string{"oval:0", "oval:0", ":25", ":25", "oval:0"}) {
		t.Fatalf("деление: %v", got)
	}
	edtest.MustGrid(t, e)
}

// Обложка — лист вне разворотов (032 §1): правки мест и текстов находят её через Sheet и
// держат места в её листе; бросок, обмен, автосборка и «Заполнить» соблюдают «одно фото —
// одно место» на весь проект; инварианты — по всем листам.

func TestCoverSheetPlaces(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(20, 20, 100, 80, "a")}, []string{"b", "c"})
	cv := func() *model.Spread { return edtest.P(e).Cover.Sheet }
	// Перенос к правому краю: зажим по листу обложки (два разворота, корешок 6 мм, заворот 20: 608 мм),
	// а не разворота (562).
	if w := edtest.P(e).Geo(cv()).W; w != 608 {
		t.Fatalf("ширина обложки %v", w)
	}
	e.SetPlaces(model.CoverID, []model.PlaceBox{{ID: edtest.CoverPlace, X: 600, Y: 100, W: 120, H: 80}}, "")
	if _, pl := cv().FindPlace(edtest.CoverPlace); pl == nil || pl.X != 608-120 {
		t.Fatalf("место обложки после переноса: %+v", pl)
	}
	// Новое место — на лицевой (страница 1 листа обложки — сторонка с заворотом).
	if _, err := e.AddPlace(model.CoverID, 1, ""); err != nil || len(cv().Grid) != 2 {
		t.Fatalf("AddPlace на обложку: %v, мест %d", err, len(cv().Grid))
	}
	if pl, front := cv().Grid[1], edtest.P(e).BdArea(cv(), model.ScopeRight); pl.X+pl.W/2 < front.X || pl.X+pl.W/2 > front.X+front.W {
		t.Fatalf("новое место не на лицевой: %+v", pl)
	}
	// Текст — по центру задней.
	if _, err := e.AddText(model.CoverID, 0, ""); err != nil || len(cv().Texts) != 2 {
		t.Fatalf("AddText на обложку: %v", err)
	}
	if tb := cv().Texts[1]; tb.X+tb.W/2 < 0 || tb.X+tb.W/2 > 301 {
		t.Fatalf("текст не на задней: %+v", tb)
	}
	e.DeleteText(cv().Texts[1].ID)
	if len(cv().Texts) != 1 {
		t.Fatal("DeleteText не нашёл текст обложки")
	}
	edtest.MustGrid(t, e)
}

func TestCoverSheetOnePhotoOnePlace(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(20, 20, 100, 80, "a")}, []string{"b", "c"})
	cover := func() string { _, pl := edtest.P(e).Cover.Sheet.FindPlace(edtest.CoverPlace); return pl.Photo }
	// Фото разворота на занятое место обложки — обмен (как между разворотами).
	if _, _, err := e.PlacePhotos(model.CoverID, edtest.CoverPlace, edtest.Photos("b")); err != nil || cover() != "b" {
		t.Fatalf("бросок на место обложки: %v, место %q", err, cover())
	}
	if got := edtest.P(e).Spreads[1].Photos; !slices.Equal(got, []string{"k", "c"}) {
		t.Fatalf("обмен с обложкой: разворот %v", got)
	}
	e.SwapPhotos("b", "c")
	if cover() != "c" || !slices.Equal(edtest.P(e).Spreads[1].Photos, []string{"k", "b"}) {
		t.Fatalf("SwapPhotos через обложку: %q %v", cover(), edtest.P(e).Spreads[1].Photos)
	}
	// Автосборка и «Заполнить» не берут фото с места обложки.
	if _, err := e.AutoBuild(edtest.Photos("c", "x", "y"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64}); err != nil {
		t.Fatal(err)
	}
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].Photos
	if cover() != "c" || slices.Contains(last, "c") || !slices.Equal(last, []string{"x", "y"}) {
		t.Fatalf("автосборка: обложка %q, новый разворот %v", cover(), last)
	}
	if _, left := e.FillPlaces(edtest.Photos("c"), ""); left != 0 || cover() != "c" {
		t.Fatalf("«Заполнить» забрал фото обложки: осталось %d, место %q", left, cover())
	}
	// Фото в развороте уходит с места обложки (место пустеет, остаётся).
	if _, err := e.AddPhotos(edtest.P(e).Spreads[1].ID, edtest.Photos("c"), -1); err != nil || cover() != "" {
		t.Fatalf("AddPhotos не снял фото с обложки: %v, место %q", err, cover())
	}
	edtest.MustGrid(t, e)
}

// Операции над порядком альбома и «Авто» обложку не находят (Find): холостая правка.
func TestCoverSheetRejectsSpreadOps(t *testing.T) {
	e := edtest.CoverEd(t, "k", nil, []string{"b", "c"})
	rev, undo, before := edtest.Rev(e), edtest.UndoLen(e), string(model.Marshal(edtest.P(e)))
	_, _ = e.AddPhotos(model.CoverID, edtest.Photos("b"), -1)
	e.SetLayout(model.CoverID, "H(0,1)")
	e.SplitSpread(model.CoverID, 1)
	e.MergeSpreads(model.CoverID)
	e.MoveSpread(model.CoverID, 1)
	e.DeleteSpread(model.CoverID)
	e.SetSpreadName(model.CoverID, "Обложка")
	e.SetGrid(model.CoverID, false)
	_, _, _ = e.ApplyGrid(model.CoverID, model.ScopeSpread, model.TplSpread, []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}})
	edtest.Same(t, e, rev, undo, "операции разворотов на обложке")
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("проект изменился")
	}
}
