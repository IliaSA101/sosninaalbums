package grid_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Бросок фото на обложку (127 G4, 0.26-127-12): остальные фото садятся только в места сторонки
// целевого места. Лицевая не занимает заднюю, корешок сторонкой не считается.

// place бросает фото ids на место placeID обложки; возвращает left.
func (c *cv) place(placeID string, ids ...string) int {
	c.t.Helper()
	_, left, _, err := c.e.PlacePhotos(model.CoverID, placeID, edtest.Photos(ids...))
	if err != nil {
		c.t.Fatal(err)
	}
	edtest.MustGrid(c.t, c.e)
	return left
}

// photosOf — фото мест сторонки по порядку Grid ("" — пусто).
func (c *cv) photosOf(scope string) (out []string) {
	for _, pl := range c.places(scope) {
		out = append(out, pl.Photo)
	}
	return out
}

// TestCoverPlacePhotosStaysOnSide (127-10): 3 фото на последнее место лицевой садятся в её же
// пустые места (по кругу сторонки), задняя пуста; один шаг отмены, Save → Load сохраняет.
func TestCoverPlacePhotosStaysOnSide(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "", "", "")
	c.fill(model.ScopeRight, "", "", "")
	front := c.places(model.ScopeRight)
	u0, was := edtest.UndoLen(c.e), snapshot(c.e)
	if left := c.place(front[2].ID, "h1", "v2", "s3"); left != 0 {
		t.Fatalf("left = %d", left)
	}
	if got := c.photosOf(model.ScopeRight); !slices.Equal(got, []string{"v2", "s3", "h1"}) {
		t.Fatalf("лицевая %v", got)
	}
	if got := c.photosOf(model.ScopeLeft); !slices.Equal(got, []string{"", "", ""}) {
		t.Fatalf("задняя занята: %v", got)
	}
	if edtest.UndoLen(c.e) != u0+1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(c.e)-u0)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e2 := c.reload(path)
	for _, sc := range []string{model.ScopeLeft, model.ScopeRight} {
		if got, want := len(placed(edtest.P(e2), sc)), len(placed(c.p(), sc)); got != want {
			t.Errorf("%s после Load: фото %d, было %d", sc, got, want)
		}
	}
	c.e.Undo()
	if string(snapshot(c.e)) != string(was) {
		t.Error("Undo не вернул проект до броска")
	}
}

// placed — фото мест сторонки.
func placed(p *model.Project, scope string) (out []string) {
	s := p.Cover.Sheet
	for _, pl := range s.Grid {
		if pl.Photo != "" && grid.InArea(p, s, pl.X, pl.W, scope) {
			out = append(out, pl.Photo)
		}
	}
	return out
}

// TestCoverPlacePhotosShort: на лицевой мест меньше, чем фото, — лишние left, задняя не берёт
// (даже пустая); фото, стоящее на задней, переносится на лицевую.
func TestCoverPlacePhotosShort(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "", "", "")
	c.fill(model.ScopeRight, "", "")
	if left := c.place(c.places(model.ScopeRight)[0].ID, "h1", "v2", "s3"); left != 1 {
		t.Fatalf("left = %d, ждали 1", left)
	}
	if got := c.photosOf(model.ScopeRight); !slices.Equal(got, []string{"h1", "v2"}) {
		t.Fatalf("лицевая %v", got)
	}
	if got := placed(c.p(), model.ScopeLeft); len(got) != 0 {
		t.Fatalf("задняя занята: %v", got)
	}
	// «s3» не встало и осталось в ленте, не в альбоме.
	if c.p().Placed("s3") {
		t.Error("лишнее фото попало в альбом")
	}

	// Фото задней, брошенное вместе с другими на лицевую, переезжает на лицевую.
	c = fsfera(t)
	c.fill(model.ScopeLeft, "p9", "")
	c.fill(model.ScopeRight, "", "", "")
	if left := c.place(c.places(model.ScopeRight)[0].ID, "h1", "p9"); left != 0 {
		t.Fatalf("left = %d", left)
	}
	if got := c.photosOf(model.ScopeRight); !slices.Equal(got, []string{"h1", "p9", ""}) {
		t.Fatalf("лицевая %v", got)
	}
	if got := placed(c.p(), model.ScopeLeft); len(got) != 0 {
		t.Fatalf("p9 остался на задней: %v", got)
	}
}

// TestCoverPlacePhotosSpine: место, центр которого на корешке, не принадлежит сторонке — туда
// встаёт только первое фото, остальные left; сторонки не тронуты.
func TestCoverPlacePhotosSpine(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "", "")
	c.fill(model.ScopeRight, "", "")
	var id string
	edtest.Change(c.e, func(p *model.Project) {
		s := p.Cover.Sheet
		a, _ := p.AreaOf(s, model.ScopeSpine)
		pl := s.Grid[0]
		pl.X = geom.Round2(a.X + a.W/2 - pl.W/2)
		id = pl.ID
	})
	if len(c.places(model.ScopeLeft))+len(c.places(model.ScopeRight)) != 3 {
		t.Fatal("место не ушло на корешок")
	}
	if left := c.place(id, "h1", "v2", "s3"); left != 2 {
		t.Fatalf("left = %d, ждали 2", left)
	}
	if got := placed(c.p(), model.ScopeLeft); len(got) != 0 {
		t.Errorf("задняя занята: %v", got)
	}
	if got := placed(c.p(), model.ScopeRight); len(got) != 0 {
		t.Errorf("лицевая занята: %v", got)
	}
}

// TestCoverPlacePhotosNoop: бросок того же фото на его место и на неизвестное место — холостая
// правка (rev и отмена прежние); пустой список — тоже.
func TestCoverPlacePhotosNoop(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeLeft, "", "")
	c.fill(model.ScopeRight, "h1", "")
	rev, undo := edtest.Rev(c.e), edtest.UndoLen(c.e)
	c.place(c.places(model.ScopeRight)[0].ID, "h1")
	c.place("нет-такого", "v2")
	c.place(c.places(model.ScopeRight)[0].ID)
	edtest.Same(t, c.e, rev, undo, "холостые броски на обложке")
}

// TestPlacePhotosSpreadWraps: на развороте поведение прежнее — «следующие» идут по кругу всего
// листа (Spread.NextEmpty), а не по странице.
func TestPlacePhotosSpreadWraps(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, ""), edtest.Gp(142, 12, 127, 127, ""), edtest.Gp(12, 150, 127, 127, "")})
	_, left, _, err := e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[2].ID, edtest.Photos("a", "b"))
	edtest.MustGrid(t, e)
	if err != nil || left != 0 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"b", "", "a"}) {
		t.Fatalf("места %v, left %d, err %v", edtest.PhotosOf(e, 0), left, err)
	}
}
