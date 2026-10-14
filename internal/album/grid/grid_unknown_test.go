package grid_test

import (
	"bytes"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Поворот места на чужом развороте — холостая правка: ни шага, ни rev.
func TestSetPlaceAngleUnknownSpread(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetPlaceAngle("нет", "a", 15)
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
}

// GrowVignette не виньетке и при k ≤ 0 ничего не меняет и не зовёт план: из Drop таких
// вызовов нет, это край самой операции.
func TestGrowVignetteNoop(t *testing.T) {
	plan := func(*model.Project, *model.Spread, model.Vignette) grid.VigPlan {
		t.Fatal("план не нужен: правки нет")
		return grid.VigPlan{}
	}
	e, sid := vigSheet(t, 25)
	plain := &model.Spread{ID: "plain", Photos: []string{"a1"}}
	p := edtest.P(e)
	before := model.Marshal(p)
	for _, k := range []int{3, 0, -2} {
		if err := grid.GrowVignette(p, plain, k, plan); err != nil {
			t.Fatalf("не виньетка, k=%d: %v", k, err)
		}
	}
	_, s := p.Find(sid)
	for _, k := range []int{0, -2} {
		if err := grid.GrowVignette(p, s, k, plan); err != nil {
			t.Fatalf("виньетка, k=%d: %v", k, err)
		}
	}
	if !bytes.Equal(model.Marshal(p), before) || len(plain.Grid) != 0 || plain.Vig != nil {
		t.Fatal("холостой вызов изменил проект")
	}
}
