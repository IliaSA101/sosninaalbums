package album_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Номер шага макета в конвейере правки (131 design §3): FixLook до Normalize.

// TestFixLookBeforeNormalize — правка снимает с листа с шагом и места, и фото: лист удаляется, как
// без шага (FixLook после Normalize оставил бы пустой лист с одним номером шага). Соседний лист с
// шагом и фоном стоит; отмена возвращает лист с шагом; холостая правка — без шага отмены.
func TestFixLookBeforeNormalize(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a"), edtest.Gp(130, 10, 80, 80, "b")}, []string{"c"})
	edtest.Change(e, func(p *model.Project) {
		p.Look = &model.LookRef{Key: "my:0123456789abcdef"}
		p.Spreads[0].LookStep = 2
		p.Spreads[1].BgL, p.Spreads[1].LookStep = "#ffeeee", 3
	})
	steps := func() (ids []string, st []int) {
		for _, s := range edtest.P(e).Spreads {
			ids, st = append(ids, s.ID), append(st, s.LookStep)
		}
		return ids, st
	}
	if ids, st := steps(); len(ids) != 2 || st[0] != 2 || st[1] != 3 {
		t.Fatalf("до правки: %v %v", ids, st)
	}
	places := edtest.PlaceIDsOf(e)
	rev := e.View().Rev
	e.DeleteSelection("s1", nil, []string{"нет"}, nil) // холостая: неизвестное место
	if e.View().Rev != rev {
		t.Fatal("холостая правка сдвинула rev")
	}
	e.DeleteSelection("s1", []string{"a", "b"}, places, nil)
	if ids, st := steps(); len(ids) != 1 || ids[0] != "s2" || st[0] != 3 {
		t.Fatalf("после снятия мест и фото: листы %v, шаги %v", ids, st)
	}
	e.Undo()
	if ids, st := steps(); len(ids) != 2 || ids[0] != "s1" || st[0] != 2 || len(edtest.P(e).Spreads[0].Grid) != 2 {
		t.Fatalf("после отмены: листы %v, шаги %v", ids, st)
	}
}
