package grid_test

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

func TestSyncGridAddFillsEmpty(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "")})
	edtest.AddPhotos(e, "s1", []model.PhotoInfo{edtest.Photo("c")}, 0)
	edtest.MustGrid(t, e)
	if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("места: %v", got)
	}
}

func TestAddToFullGridFails(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, []string{"x"})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.AddPhotos("s1", edtest.Photos("c", "d"), -1); !errors.Is(err, model.ErrNoPlace) {
		t.Fatalf("ошибка %v", err)
	}
	if _, err := e.MovePhoto("x", "s1", 0); !errors.Is(err, model.ErrNoPlace) {
		t.Fatalf("перенос: ошибка %v", err)
	}
	edtest.Same(t, e, rev, undo, "полная сетка")
	// Всё или ничего: из двух фото при одном пустом месте не встаёт ни одно.
	edtest.Change(e, func(p *model.Project) { _, _ = p.AddPlace(p.Spreads[0], 1, 1, p.PlaceIDs()) })
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.AddPhotos("s1", edtest.Photos("c", "d"), -1); !errors.Is(err, model.ErrNoPlace) || edtest.HasRef(edtest.P(e), "c") {
		t.Fatalf("ошибка %v, c в записях: %v", err, edtest.HasRef(edtest.P(e), "c"))
	}
	edtest.Same(t, e, rev, undo, "всё или ничего")
}

func TestMoveBetweenPlaces(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "")})
	e.SetFrame("a", 2, 0.3, 0)
	_, left, _, err := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[1].ID, []model.PhotoInfo{edtest.Photo("a")})
	edtest.MustGrid(t, e)
	if err != nil || left != 0 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"", "a"}) || edtest.P(e).Photos["a"].Frame == nil {
		t.Fatalf("места %v, left %d, err %v", edtest.PhotosOf(e, 0), left, err)
	}
}

func TestReorderInGridNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.AddPhotos(e, "s1", []model.PhotoInfo{edtest.Photo("b")}, 0)
	// MovePhoto внутри сетки — сдвиг (114), см. TestMoveInGridShifts; здесь бросок своего фото на свой лист.
	if _, _, err := e.DropPhotos("s1", "", []model.PhotoInfo{edtest.Photo("b")}, -1, ""); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "перестановка в сетке")
}

// 114: MovePhoto своего фото ручного разворота — сдвиг по местам: места (геометрия, форма, «Целиком»,
// поворот, табличка) стоят, фото (кадр, зеркало, тон, прозрачность, подпись) едут; шаг один, своя
// щель и обложка — холостые.
func TestMoveInGridShifts(t *testing.T) {
	places := []*model.Place{
		edtest.Gp(12, 12, 100, 80, "a"), edtest.Gp(130, 12, 60, 60, "b"), edtest.Gp(200, 12, 50, 90, ""),
		edtest.Gp(12, 120, 90, 70, "c"), edtest.Gp(120, 120, 120, 50, "d"),
	}
	e := edtest.GridEd(t, places)
	ids := []string{}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		ids = append(ids, pl.ID)
	}
	e.SetPlaceFit("s1", ids[:1], model.FitContain)
	e.SetPlaceShape("s1", ids[:1], model.ShapeOval)
	e.SetPlaceAngle("s1", ids[0], 10)
	e.SetPlacePerson("s1", ids[0], model.Person{Last: "Иванов"}, "")
	e.SetFrame("a", 2, 0.3, -0.2)
	e.SetFlip([]string{"a"}, "h")
	e.SetTone("a", 10, -5)
	e.SetPhotoFade([]string{"a"}, 0.4, "")
	e.SetCaption("a", "подпись", "")
	edtest.MustGrid(t, e)

	geo := func() []model.Place {
		var out []model.Place
		for _, pl := range edtest.P(e).Spreads[0].Grid {
			c := *pl
			c.Photo = ""
			out = append(out, c)
		}
		return out
	}
	was, refA := geo(), edtest.P(e).Photos["a"]
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)

	if _, err := e.MovePhoto("a", "s1", 3); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"b", "c", "", "a", "d"}) {
		t.Fatalf("места: %v", got)
	}
	if got := edtest.P(e).Spreads[0].Photos; !slices.Equal(got, []string{"b", "c", "a", "d"}) {
		t.Fatalf("Photos: %v", got)
	}
	if !reflect.DeepEqual(geo(), was) {
		t.Fatalf("места сдвинулись:\n%+v\n%+v", geo(), was)
	}
	if !reflect.DeepEqual(edtest.P(e).Photos["a"], refA) {
		t.Fatalf("свойства фото не поехали: %+v ≠ %+v", edtest.P(e).Photos["a"], refA)
	}
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d→%d, undo %d→%d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}

	// Своя щель (сразу до и после фото; a теперь третье из четырёх) — холостая.
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	for _, at := range []int{2, 3} {
		if _, err := e.MovePhoto("a", "s1", at); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, fmt.Sprintf("своя щель %d", at))
	}
	// Неизвестное фото — не правка.
	if _, err := e.MovePhoto("zz", "s1", 0); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "неизвестное фото")

	// −1 и больше числа фото — в конец.
	e.Undo()
	if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"a", "b", "", "c", "d"}) {
		t.Fatalf("Undo: %v", got)
	}
	edtest.MustGrid(t, e)
	for _, at := range []int{-1, 99} {
		if _, err := e.MovePhoto("a", "s1", at); err != nil {
			t.Fatal(err)
		}
		if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"b", "c", "", "d", "a"}) {
			t.Fatalf("в конец (%d): %v", at, got)
		}
		e.Undo()
	}
	e.Redo() // Redo возвращает последний сдвиг в конец
	if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"b", "c", "", "d", "a"}) {
		t.Fatalf("Redo: %v", got)
	}
}

// 114: обложка не переставляется MovePhoto — через p.Find её нет, фото обложки идёт путём AddPhotos.
func TestMoveCoverPhotoNoop(t *testing.T) {
	e := edtest.CoverEd(t, "cover", []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.MovePhoto("cover", model.CoverID, 0); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "обложка")
	if got := edtest.P(e).Cover.Sheet.Photos; !slices.Equal(got, []string{"cover"}) {
		t.Fatalf("обложка: %v", got)
	}
}

func TestRemoveEmptiesPlace(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	e.RemovePhotos([]string{"a"})
	edtest.MustGrid(t, e)
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"", "b"}) || edtest.HasRef(edtest.P(e), "a") {
		t.Fatalf("места %v", edtest.PhotosOf(e, 0))
	}
}

func TestPlacePhotosEmpty(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "")})
	_, left, _, _ := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[1].ID, edtest.Photos("c"))
	edtest.MustGrid(t, e)
	if left != 0 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "c"}) {
		t.Fatalf("места %v, left %d", edtest.PhotosOf(e, 0), left)
	}
}

func TestPlacePhotosReplace(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")})
	e.SetCaption("a", "подпись", "")
	_, left, _, _ := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[0].ID, edtest.Photos("c"))
	edtest.MustGrid(t, e)
	if left != 1 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"c"}) || edtest.HasRef(edtest.P(e), "a") {
		t.Fatalf("места %v, left %d", edtest.PhotosOf(e, 0), left)
	}
	e.Undo() // «Вернуть» = Undo: прежнее фото и подпись на месте
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"a"}) || edtest.P(e).Photos["a"].Caption == nil {
		t.Fatalf("после отмены: %v", edtest.PhotosOf(e, 0))
	}
}

func TestPlacePhotosSwap(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	_, left, _, _ := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[0].ID, edtest.Photos("b", "c"))
	edtest.MustGrid(t, e)
	if left != 1 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"b", "a"}) {
		t.Fatalf("места %v, left %d", edtest.PhotosOf(e, 0), left)
	}
}

func TestPlacePhotosSwapWithAuto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, []string{"c", "d"})
	e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[0].ID, edtest.Photos("d"))
	edtest.MustGrid(t, e)
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"d"}) || !slices.Equal(edtest.P(e).Spreads[1].Photos, []string{"c", "a"}) {
		t.Fatalf("сетка %v, «Авто» %v", edtest.PhotosOf(e, 0), edtest.P(e).Spreads[1].Photos)
	}
}

func TestPlacePhotosMulti(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(142, 12, 100, 100, ""), edtest.Gp(12, 150, 100, 100, "")}, []string{"x"})
	_, left, _, _ := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[1].ID, edtest.Photos("c", "x", "d"))
	edtest.MustGrid(t, e)
	// c — на целевое, x (из альбома) — в следующее пустое, d — мест нет: остаётся в ленте.
	if left != 1 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "c", "x"}) || edtest.HasRef(edtest.P(e), "d") || len(edtest.P(e).Spreads) != 1 {
		t.Fatalf("места %v, left %d, разворотов %d", edtest.PhotosOf(e, 0), left, len(edtest.P(e).Spreads))
	}
}

func TestPlacePhotosSelf(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[0].ID, edtest.Photos("a"))
	e.PlacePhotos("s1", "000000000099", edtest.Photos("c"))
	e.PlacePhotos("s9", edtest.P(e).Spreads[0].Grid[0].ID, edtest.Photos("c"))
	edtest.Same(t, e, rev, undo, "своё место, чужое место")
}

func TestSwapAcrossGridAndAuto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "")}, []string{"c", "d"})
	e.SwapPhotos("a", "d")
	edtest.MustGrid(t, e)
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"d", ""}) || !slices.Equal(edtest.P(e).Spreads[1].Photos, []string{"c", "a"}) {
		t.Fatalf("сетка %v, «Авто» %v", edtest.PhotosOf(e, 0), edtest.P(e).Spreads[1].Photos)
	}
}

func TestAutoBuildSkipsGrid(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")})
	e.AutoBuild(edtest.Photos("a", "x", "y"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	edtest.MustGrid(t, e)
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"a"}) {
		t.Fatalf("сетка %v", edtest.PhotosOf(e, 0))
	}
	for _, s := range edtest.P(e).Spreads[1:] {
		if slices.Contains(s.Photos, "a") {
			t.Fatalf("фото сетки в автосборке: %v", s.Photos)
		}
	}
}

func TestSplitMergeGridNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")}, []string{"c"})
	e.MoveSpread("s2", 0) // «Авто» перед сеткой: склейка «Авто» с сеткой тоже холостая
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SplitSpread("s1", 1)
	e.MergeSpreads("s1")
	e.MergeSpreads("s2")
	edtest.Same(t, e, rev, undo, "разрез и склейка")
}

func TestStepLayoutGridNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, sc := range []string{"spread", "left", "right"} {
		e.StepLayout("s1", sc, 1)
	}
	edtest.Same(t, e, rev, undo, "листание")
}

func TestSetLayoutLeavesGrid(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	key := edtest.PickKey(t, e, 0, edtest.AnyKey)
	v := e.SetLayout("s1", key)
	if s := edtest.P(e).Spreads[0]; s.Grid != nil || s.Layout != key || v.Spreads[0].Layout != key || v.Spreads[0].Grid {
		t.Fatalf("ключ %q, сетка %v", s.Layout, s.Grid)
	}
	e.Undo()
	if len(edtest.P(e).Spreads[0].Grid) != 2 {
		t.Fatal("Undo не вернул сетку")
	}
}

func TestRescaleGridOnSettings(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 142, 100, 130, "")})
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 140.5, 142
	e.SetSettings(st)
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	want := []layout.Rect{{X: 6, Y: 6, W: 63.5, H: 63.5}, {X: 146.5, Y: 71, W: 50, H: 65}}
	for i, w := range want {
		if !nearMM(rectOfPlace(g[i]), w) {
			t.Fatalf("место %d: %+v, ждали %+v", i, *g[i], w)
		}
	}
}

func TestDeleteLastPlaceIsAuto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b")})
	if _, err := e.AddText("s1", 1, ""); err != nil {
		t.Fatal(err)
	}
	e.DeletePlaces("s1", []string{edtest.P(e).Spreads[0].Grid[0].ID})
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"b"}) || edtest.HasRef(edtest.P(e), "a") {
		t.Fatalf("места %v", edtest.PhotosOf(e, 0))
	}
	e.DeletePlaces("s1", []string{edtest.P(e).Spreads[0].Grid[0].ID})
	edtest.MustGrid(t, e)
	if len(edtest.P(e).Spreads) != 1 || edtest.P(e).Spreads[0].Grid != nil || len(edtest.P(e).Spreads[0].Photos) != 0 {
		t.Fatalf("развороты %d, сетка %v", len(edtest.P(e).Spreads), edtest.P(e).Spreads[0].Grid)
	}
}

func TestAddPlaceFreezesAuto(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a"))
	id := v.Spreads[0].ID
	rects := v.Spreads[0].Rects
	undo := edtest.UndoLen(e)
	if _, err := e.AddPlace(id, 1, ""); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != 2 || edtest.UndoLen(e) != undo+1 || g[0].Photo != "a" || !nearMM(rectOfPlace(g[0]), rects[0]) || g[1].X < edtest.P(e).Settings.PageW {
		t.Fatalf("места %d, undo %d", len(g), edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("один Undo не вернул «Авто»")
	}
	// Разворот без фото (только текст): сетка с нуля.
	v, _ = e.NewSpread(-1, nil)
	if _, err := e.AddPlace(v.Spreads[1].ID, 0, ""); err != nil || len(edtest.P(e).Spreads[1].Grid) != 1 {
		t.Fatalf("пустой разворот: %v, мест %d", err, len(edtest.P(e).Spreads[1].Grid))
	}
}

func TestArrangeSame(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 60, "a"), edtest.Gp(150, 100, 40, 40, "")})
	g := edtest.P(e).Spreads[0].Grid
	e.ArrangePlaces("s1", "same", []string{g[0].ID, g[1].ID})
	if p1 := *edtest.P(e).Spreads[0].Grid[1]; p1.W != 100 || p1.H != 60 || p1.X != 120 || p1.Y != 90 {
		t.Fatalf("место: %+v", p1)
	}
	e.ArrangePlaces("s1", "sameH", []string{g[1].ID, g[0].ID})
	edtest.MustGrid(t, e)
}

func TestArrangeSplit(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 100, 100, "")})
	if _, err := e.ArrangePlaces("s1", "splitH", []string{edtest.P(e).Spreads[0].Grid[0].ID}); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != 3 || g[0].Photo != "a" || g[0].W != 62 || g[1].X != 77 || g[1].W != 62 || g[1].Photo != "" || g[2].X != 293 {
		t.Fatalf("места: %+v %+v %+v", *g[0], *g[1], *g[2])
	}
	e.ArrangePlaces("s1", "splitV", []string{g[1].ID})
	if g = edtest.P(e).Spreads[0].Grid; len(g) != 4 || g[1].H != 62 || g[2].Y != 77 {
		t.Fatalf("⇅: %+v %+v", *g[1], *g[2])
	}
	// Слои: «Выше» — в конец, «Ниже» — в начало.
	last := g[3].ID
	e.ArrangePlaces("s1", "back", []string{last})
	if edtest.P(e).Spreads[0].Grid[0].ID != last {
		t.Fatal("«Ниже» не сработало")
	}
	e.ArrangePlaces("s1", "front", []string{last})
	if edtest.P(e).Spreads[0].Grid[3].ID != last {
		t.Fatal("«Выше» не сработало")
	}
}

func TestArrangeSplitTooSmall(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 12, 127, "a")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ArrangePlaces("s1", "splitH", []string{edtest.P(e).Spreads[0].Grid[0].ID}); !errors.Is(err, grid.ErrSplitSmall) {
		t.Fatalf("ошибка %v", err)
	}
	edtest.Same(t, e, rev, undo, "малое место")
	e.ArrangePlaces("s1", "nope", []string{edtest.P(e).Spreads[0].Grid[0].ID})
	edtest.Same(t, e, rev, undo, "неизвестная операция")
}

func TestArrangeFitPhoto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(150, 12, 50, 50, "")})
	g := edtest.P(e).Spreads[0].Grid
	e.ArrangePlaces("s1", "fitPhoto", []string{g[0].ID, g[1].ID})
	g = edtest.P(e).Spreads[0].Grid
	if g[0].W != 100 || g[0].H != 66.67 || g[0].Y != 28.67 || g[1].H != 50 {
		t.Fatalf("места: %+v %+v", *g[0], *g[1])
	}
}

func TestPlacesNudgeOneUndo(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")})
	pl := *edtest.P(e).Spreads[0].Grid[0]
	undo := edtest.UndoLen(e)
	for i := 1; i <= 3; i++ {
		e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: pl.X + float64(i), Y: pl.Y, W: pl.W, H: pl.H}}, "sel-1")
	}
	if edtest.P(e).Spreads[0].Grid[0].X != 15 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("x %v, шагов %d", edtest.P(e).Spreads[0].Grid[0].X, edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid[0].X != 12 {
		t.Fatal("Undo не вернул место")
	}
	// Перетаскивание без merge — шаг на действие.
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: 20, Y: 12, W: 100, H: 100}}, "")
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: 30, Y: 12, W: 100, H: 100}}, "")
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов %d", edtest.UndoLen(e)-undo)
	}
}

func TestSetPlacesNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")})
	pl := *edtest.P(e).Spreads[0].Grid[0]
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: 12.001, Y: 12, W: 100, H: 100}}, "")
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: math.NaN(), Y: 0, W: 1, H: 1}}, "")
	e.SetPlaces("s1", []model.PlaceBox{{ID: "000000000099", X: 50, Y: 50, W: 50, H: 50}}, "")
	edtest.Same(t, e, rev, undo, "то же место, NaN, чужой ID")
	// За край — зажим внутрь разворота, размер тот же.
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: 1000, Y: -5, W: 100, H: 100}}, "")
	if g := edtest.P(e).Spreads[0].Grid[0]; g.X != 462 || g.Y != 0 || g.W != 100 {
		t.Fatalf("зажим: %+v", *g)
	}
}

func TestViewPlaces(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(200, 12, 150, 100, ""), edtest.Gp(12, 150, 100, 100, "b")})
	st := edtest.P(e).Settings
	st.Layflat = false
	v := e.SetSettings(st)
	sv := v.Spreads[0]
	if !sv.Grid || sv.Layout != "" || sv.Variant != -1 || sv.Variants != 0 || sv.Pages != nil || v.EmptyPlaces != 1 {
		t.Fatalf("вид: grid %v, layout %q, variant %d, pages %v, пустых %d", sv.Grid, sv.Layout, sv.Variant, sv.Pages, v.EmptyPlaces)
	}
	if len(sv.Places) != 3 || sv.Places[1].Photo != "" || !sv.Places[1].Spine || sv.Places[0].Spine || sv.Places[2].Photo != "b" {
		t.Fatalf("места: %+v", sv.Places)
	}
	if !slices.Equal(sv.Photos, []string{"a", "b"}) || len(sv.Rects) != 2 || len(sv.Frames) != 2 ||
		sv.Rects[1] != (layout.Rect{X: 12, Y: 150, W: 100, H: 100}) {
		t.Fatalf("фото %v, ячейки %+v", sv.Photos, sv.Rects)
	}
	st.Layflat = true
	if v = e.SetSettings(st); v.Spreads[0].Places[1].Spine {
		t.Fatal("при layflat корешок не помечается")
	}
	if v := edtest.Open(t, model.New(), "").View(); v.EmptyPlaces != 0 {
		t.Fatal("пустой альбом")
	}
}

func TestContainFrame(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")})
	e.SetFrame("a", 2, 0.5, 0.5)
	v := e.SetPlaceFit("s1", []string{edtest.P(e).Spreads[0].Grid[0].ID}, model.FitContain)
	f := v.Spreads[0].Frames[0]
	want := layout.Rect{X: 12, Y: 12 + (100-200.0/3)/2, W: 100, H: 200.0 / 3}
	if !nearMM(f.Img, want) || f.S != 1 || f.X != 0 || f.Y != 0 || f.DPI != 76 {
		t.Fatalf("contain: %+v, ждали %+v, dpi 76", f, want)
	}
	if edtest.P(e).Photos["a"].Frame == nil {
		t.Fatal("кадр фото стёрт: он нужен, если вернуть «С обрезкой»")
	}
	v = e.SetPlaceFit("s1", []string{edtest.P(e).Spreads[0].Grid[0].ID}, "")
	if f := v.Spreads[0].Frames[0]; f.S != 2 || f.Img.W != 2*100*1.5 {
		t.Fatalf("cover: %+v", f)
	}
	rev := edtest.Rev(e)
	e.SetPlaceFit("s1", []string{edtest.P(e).Spreads[0].Grid[0].ID}, "stretch")
	if edtest.Rev(e) != rev {
		t.Fatal("неизвестный fit поменял проект")
	}
}

// TestPlaces64: «Авто» на 64 фото закрепляется в сетку из 64 мест (предел MaxPlaces, 027);
// 65-е место — ошибка без правки; «Вернуть авто» с 64 мест работает.
func TestPlaces64(t *testing.T) {
	ids := make([]string, 64)
	for i := range ids {
		ids[i] = fmt.Sprint("p", i)
	}
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos(ids...)).Spreads[0].ID
	edtest.FreezeAt(t, e, 0)
	if n := len(edtest.P(e).Spreads[0].Grid); n != 64 {
		t.Fatalf("мест после закрепления: %d", n)
	}
	rev := e.View().Rev
	if v, err := e.AddPlace(sid, 1, ""); err == nil || v.Rev != rev || len(edtest.P(e).Spreads[0].Grid) != 64 {
		t.Fatalf("65-е место: ошибка %v, мест %d", err, len(edtest.P(e).Spreads[0].Grid))
	}
	v := e.SetGrid(sid, false)
	if v.Spreads[0].Grid || len(v.Spreads[0].Rects) != 64 || !slices.Equal(v.Spreads[0].Photos, ids) {
		t.Fatalf("«Вернуть авто»: сетка %v, ячеек %d", v.Spreads[0].Grid, len(v.Spreads[0].Rects))
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Пропорции нового места не число, ≤ 0 или бесконечность — 1:1, как у образца.
func TestAddPlaceBadAspect(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}}
	s := p.Spreads[0]
	ref, err := p.AddPlace(s, 0, 1, p.PlaceIDs())
	if err != nil || ref.W != ref.H {
		t.Fatalf("образец 1:1: %v, %+v", err, ref)
	}
	for _, a := range []float64{0, -2, math.NaN(), math.Inf(1), math.Inf(-1)} {
		pl, err := p.AddPlace(s, 0, a, p.PlaceIDs())
		if err != nil || pl.W != ref.W || pl.H != ref.H {
			t.Fatalf("aspect %v: %v, %+v, ждали сторону %v", a, err, pl, ref.W)
		}
	}
	if len(s.Grid) != 6 {
		t.Fatalf("мест %d, ждали 6", len(s.Grid))
	}
}
