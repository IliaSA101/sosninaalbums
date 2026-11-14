package album

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// MarkUnsaved ставит dirty без правки: ни шага отмены, ни rev; повтор безвреден.
func TestMarkUnsaved(t *testing.T) {
	e := newEditor(t, model.New(), "")
	rev := e.rev
	e.MarkUnsaved()
	e.MarkUnsaved()
	if !e.dirty || e.rev != rev || len(e.undo) != 0 || len(e.redo) != 0 {
		t.Fatalf("dirty %v, rev %d→%d, шагов %d", e.dirty, rev, e.rev, len(e.undo))
	}
}
