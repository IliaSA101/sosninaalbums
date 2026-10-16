package grid_test

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

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
