package roundtrip_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 114: сдвиг фото по местам ручного разворота переживает Save → Load: места те же, фото в новом
// порядке со своими кадрами, Photos совпадает с порядком мест.
func TestShiftGridSaveLoad(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(12, 12, 100, 80, "a"), edtest.Gp(130, 12, 60, 60, "b"), edtest.Gp(200, 12, 50, 90, ""),
		edtest.Gp(12, 120, 90, 70, "c"), edtest.Gp(120, 120, 120, 50, "d"),
	})
	e.SetFrame("a", 2, 0.3, -0.2)
	e.SetFlip([]string{"c"}, "v")
	e.SetCaption("d", "подпись", "")
	if _, err := e.MovePhoto("a", "s1", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := e.MovePhoto("d", "s1", 0); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := edtest.Open(t, p, path)
	edtest.MustGrid(t, got)
	// Load перекладывает ID по пути файла — порядок сверяем по путям.
	var order []string
	for _, pl := range p.Spreads[0].Grid {
		order = append(order, filepath.Base(p.Photos[pl.Photo].Path))
	}
	if !slices.Equal(order, []string{"d.jpg", "b.jpg", ".", "c.jpg", "a.jpg"}) {
		t.Fatalf("после Load: %v", order)
	}
	if !slices.Equal(p.Spreads[0].Photos, model.GridPhotos(p.Spreads[0])) {
		t.Fatalf("Photos %v ≠ места", p.Spreads[0].Photos)
	}
	byFile := func(q *model.Project, name string) model.PhotoRef {
		for _, ref := range q.Photos {
			if filepath.Base(ref.Path) == name {
				return ref
			}
		}
		t.Fatalf("фото %s нет", name)
		return model.PhotoRef{}
	}
	if a := byFile(p, "a.jpg"); !reflect.DeepEqual(a.Frame, byFile(edtest.P(e), "a.jpg").Frame) || a.Frame == nil {
		t.Fatalf("кадр a не дошёл: %+v", a.Frame)
	}
	if byFile(p, "d.jpg").Caption == nil || !byFile(p, "c.jpg").FlipV {
		t.Fatal("подпись d или зеркало c не дошли")
	}
}
