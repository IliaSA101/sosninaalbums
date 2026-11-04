package export

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCoverJobSpinePhoto — 00.jpg с фото корешка (061): фото корешка поверх плашки
// (OverPlates), задняя под ней; одно фото корешка — уже не пустая обложка.
func TestCoverJobSpinePhoto(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 100, 100
	for _, id := range []string{"s1", "s2", "s3"} {
		p.Spreads = append(p.Spreads, &model.Spread{ID: id})
	}
	e := mustEditor(t, p, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	ph := func(id string) model.PhotoInfo {
		return model.PhotoInfo{ID: id, Path: filepath.Join(dir, id+".jpg"), W: 300, H: 200}
	}
	if _, err := e.SetBackdrop(model.CoverID, model.ScopeSpine, ph("b")); err != nil {
		t.Fatal(err)
	}
	_, v := e.Snapshot()
	if BlankCover(v) {
		t.Fatal("обложка с одним фото корешка — пустая")
	}
	if _, err := e.SetBackdrop(model.CoverID, model.ScopeLeft, ph("a")); err != nil {
		t.Fatal(err)
	}
	e.SetCoverSpineBg("#102030")
	q, v := e.Snapshot()
	j := NewCoverJob(q, v, 20, 90)
	if j == nil || len(j.Spread.Plates) != 1 || len(j.Spread.Backdrops) != 2 {
		t.Fatalf("задание обложки: %+v", j)
	}
	area := q.BdArea(q.Cover.Sheet, model.ScopeSpine)
	for _, b := range j.Spread.Backdrops {
		spine := b.Path == q.Photos["b"].Path
		if b.OverPlates != spine {
			t.Errorf("фон %s: OverPlates %v", b.Path, b.OverPlates)
		}
		if spine && b.Area != area {
			t.Errorf("область фото корешка %+v, ждали %+v", b.Area, area)
		}
	}
	if j.Spread.Plates[0].Color != render.ParseHex("#102030") {
		t.Fatalf("плашка %+v", j.Spread.Plates)
	}
}
