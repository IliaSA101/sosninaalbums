package grid_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// fillEd — обложка с пустыми местами (две на лицевой, одно на задней, одно на корешке) и два разворота
// по два места; на лицевой остаётся занятое CoverPlace.
func fillEd(t *testing.T) *album.Editor {
	t.Helper()
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(12, 12, 100, 100, ""), edtest.Gp(150, 12, 100, 100, "")}, []string{"x"})
	edtest.Change(e, func(p *model.Project) {
		c := p.Cover.Sheet
		for i, x := range []float64{100, 330, 450, 294} { // задняя, лицевая ×2, корешок (центры 110, 340, 460, 304)
			c.Grid = append(c.Grid, &model.Place{ID: fmt.Sprintf("c0ffee0000d%d", i), X: x, Y: 150, W: 20, H: 20})
		}
		s := p.Spreads[1]
		s.Grid = []*model.Place{{ID: "000000000b01", X: 12, Y: 12, W: 50, H: 50, Photo: "x"}, {ID: "000000000b02", X: 150, Y: 12, W: 50, H: 50}}
	})
	edtest.MustGrid(t, e)
	return e
}

// seatPhotos — фото мест обложки и разворотов по порядку Grid: «cover:…», «s1:…».
func seatPhotos(e *album.Editor) (out []string) {
	p := edtest.P(e)
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			out = append(out, s.ID+":"+pl.ID[len(pl.ID)-3:]+"="+pl.Photo)
		}
	}
	return out
}

// «Заполнить» с обложки: лицевая → задняя → корешок → развороты с первого; один шаг отмены,
// круг Save → Load сохраняет посадку, отмена возвращает прежнее состояние.
func TestFillPlacesFromCover(t *testing.T) {
	e := fillEd(t)
	before := string(model.Marshal(edtest.P(e)))
	undo := edtest.UndoLen(e)
	_, left, _ := e.FillPlaces(edtest.Photos("p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9"), model.CoverID)
	edtest.MustGrid(t, e)
	c := edtest.P(e).Cover.Sheet
	byID := map[string]string{}
	for _, pl := range c.Grid {
		byID[pl.ID] = pl.Photo
	}
	// Лицевая: d1, d2 → p1, p2; задняя: d0 → p3; корешок: d3 → p4; затем развороты: s1 → p5, p6; s2 (первое место занято) → p7.
	want := map[string]string{"c0ffee0000d1": "p1", "c0ffee0000d2": "p2", "c0ffee0000d0": "p3", "c0ffee0000d3": "p4", edtest.CoverPlace: "k"}
	for id, ph := range want {
		if byID[id] != ph {
			t.Errorf("место %s: %q, ждали %q (%v)", id, byID[id], ph, byID)
		}
	}
	if got := edtest.PhotosOf(e, 0); !slices.Equal(got, []string{"p5", "p6"}) {
		t.Errorf("s1: %v", got)
	}
	if got := edtest.PhotosOf(e, 1); !slices.Equal(got, []string{"x", "p7"}) {
		t.Errorf("s2: %v", got)
	}
	if left != 2 {
		t.Errorf("не встало %d, ждали 2 (p8, p9)", left)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Errorf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	// Круг записи: посадка та же.
	q := edtest.SaveLoad(t, e)
	for _, pl := range q.Cover.Sheet.Grid {
		// ID фото в файле — по пути (Rekey): сверяем файлы.
		if got, was := q.Photos[pl.Photo].Path, edtest.P(e).Photos[byID[pl.ID]].Path; got != was {
			t.Errorf("после Save→Load место %s: %q, было %q", pl.ID, got, was)
		}
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != before {
		t.Error("один Undo не вернул проект")
	}
}

// С разворота обложка не трогается; фото больше мест — остаток; фото меньше — часть мест пустует.
func TestFillPlacesFromSpreadKeepsCover(t *testing.T) {
	e := fillEd(t)
	_, left, _ := e.FillPlaces(edtest.Photos("p1", "p2", "p3", "p4", "p5", "p6"), "s2")
	if left != 5 || !slices.Equal(edtest.PhotosOf(e, 1), []string{"x", "p1"}) || !slices.Equal(edtest.PhotosOf(e, 0), []string{"", ""}) {
		t.Fatalf("left %d, %v %v", left, edtest.PhotosOf(e, 0), edtest.PhotosOf(e, 1))
	}
	for _, pl := range edtest.P(e).Cover.Sheet.Grid {
		if pl.ID != edtest.CoverPlace && pl.Photo != "" {
			t.Errorf("обложка тронута: %s %q", pl.ID, pl.Photo)
		}
	}
	// С обложки одного фото: оно уходит на лицевую, на разворотах пусто.
	e = fillEd(t)
	e.FillPlaces(edtest.Photos("p1"), model.CoverID)
	got := seatPhotos(e)
	if !slices.Contains(got, "cover:0d1=p1") || slices.Contains(got, "s1:001=p1") || len(edtest.PhotosOf(e, 0)) != 2 || edtest.PhotosOf(e, 0)[0] != "" {
		t.Errorf("одно фото: %v", got)
	}
}

// Холостая: нет фото, фото уже в альбоме, нет пустых мест — ни шага отмены, ни rev.
func TestFillPlacesFromCoverNoop(t *testing.T) {
	e := fillEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, ph := range [][]model.PhotoInfo{nil, edtest.Photos("k"), edtest.Photos("x")} {
		if _, left, _ := e.FillPlaces(ph, model.CoverID); left != 0 {
			t.Errorf("фото %v: left %d", ph, left)
		}
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостая правка: rev %d→%d, undo %d→%d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	// Все места заняты: нечего садить — остаток целиком, без шага.
	e.FillPlaces(edtest.Photos("p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"), model.CoverID)
	rev = edtest.Rev(e)
	if _, left, _ := e.FillPlaces(edtest.Photos("q1", "q2"), model.CoverID); left != 2 || edtest.Rev(e) != rev {
		t.Errorf("мест нет: left %d, rev %d→%d", left, rev, edtest.Rev(e))
	}
}

// Обложки нет: CoverID — как неизвестный лист, с первого разворота.
func TestFillPlacesCoverIDWithoutCover(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "")}, []string{"x"})
	if _, left, _ := e.FillPlaces(edtest.Photos("p1", "p2"), model.CoverID); left != 1 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"p1"}) {
		t.Fatalf("left %d, %v", left, edtest.PhotosOf(e, 0))
	}
}
