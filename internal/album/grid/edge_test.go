package grid_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Края правок мест: неизвестный лист, предел мест при делении, «Под фото» выше страницы.

// TestGridUnknownSheet — перенос мест и отражение на неизвестном листе — холостые без шага.
func TestGridUnknownSheet(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaces("нет-такого", []model.PlaceBox{{ID: "a", X: 20, Y: 20, W: 50, H: 50}}, "")
	e.MirrorGrid("нет-такого", model.ScopeSpread)
	edtest.Same(t, e, rev, undo, "неизвестный лист")
}

// TestSplitPlacesLimit — деление места на листе с пределом мест — отказ без шага, места те же.
func TestSplitPlacesLimit(t *testing.T) {
	places := make([]*model.Place, model.MaxPlaces)
	for i := range places {
		places[i] = edtest.Gp(float64(5+i%8*30), float64(5+i/8*30), 25, 25, "")
	}
	e := edtest.GridEd(t, places)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ArrangePlaces("s1", "splitH", []string{places[0].ID}); err == nil {
		t.Fatal("деление сверх предела мест прошло")
	}
	edtest.Same(t, e, rev, undo, "предел мест")
	if n := len(edtest.P(e).Spreads[0].Grid); n != model.MaxPlaces {
		t.Fatalf("мест %d", n)
	}
}

// TestFitPhotoTallerThanPage — «Под фото» узкого высокого фото в широком месте: по ширине место
// вышло бы выше страницы — встаёт по высоте страницы, ширина — по пропорции фото.
func TestFitPhotoTallerThanPage(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 200, 60, "a")})
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos["a"]
		r.W, r.H = 100, 1000
		p.Photos["a"] = r
	})
	if _, err := e.ArrangePlaces("s1", "fitPhoto", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	pl, ph := p.Spreads[0].Grid[0], p.Geo(p.Spreads[0]).H
	if pl.H > ph+1e-9 || pl.W >= 200 || pl.H < ph-1 {
		t.Fatalf("место %v×%v, страница высотой %v", pl.W, pl.H, ph)
	}
}
