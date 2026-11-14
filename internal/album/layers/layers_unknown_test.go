package layers_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
)

// Чужой разворот: выравнивание не находит перекрытий.
func TestUnknownSpreadNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	if e.AlignOverlap("нет", []string{"a", "b"}, "distH", "page") {
		t.Fatal("перекрытие на чужом развороте")
	}
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
}
