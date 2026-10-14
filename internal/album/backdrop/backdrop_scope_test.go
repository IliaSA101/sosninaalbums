package backdrop_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
)

// Неизвестная область (мусор из интерфейса) — холостая правка фона: ни шага, ни rev.
func TestBackdropUnknownScopeNoop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "spread", "bg")
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetBackdropFrame(s1, "середина", 2, 0.5, 0.5)
	e.FitBackdrop(s1, "середина")
	e.ClearBackdrop(s1, "середина")
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
	if edtest.BdOf(e, s1, "spread") == nil {
		t.Fatal("фон разворота пропал от чужой области")
	}
}
