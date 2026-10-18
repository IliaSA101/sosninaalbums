package album

import (
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/inside"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Доступ тестов областей к живому состоянию Editor и к полям модели без экспорта (023):
// пакет inside виден только под internal/album, обёртки с типами — internal/edtest.
func init() {
	inside.P = func(e any) any { return e.(*Editor).p }
	inside.Undo = func(e any) any { return e.(*Editor).undo }
	inside.Redo = func(e any) any { return e.(*Editor).redo }
	inside.Marks = func(e any) any { return e.(*Editor).marks }
	inside.Mu = func(e any) *sync.Mutex { return &e.(*Editor).mu }
	inside.Rev = func(e any) int { return e.(*Editor).rev }
	inside.Dirty = func(e any) bool { return e.(*Editor).dirty }
	inside.Backup = func(e any) bool { return e.(*Editor).backup }
	inside.LastMerge = func(e any) string { return e.(*Editor).lastMerge }
	inside.UndoBytes = func(e any) int { return e.(*Editor).undoBytes }

	inside.SetUndo = func(e, v any) { e.(*Editor).undo = v.([]Snap) }
	inside.SetDirty = func(e any, v bool) { e.(*Editor).dirty = v }
	inside.SetLastMerge = func(e any, v string) { e.(*Editor).lastMerge = v }
	inside.SetUndoAlbum = func(e any, v int) { e.(*Editor).undoAlbum = v }
	inside.SetUndoBytes = func(e any, v int) { e.(*Editor).undoBytes = v }

	inside.Change = func(e, fn any) any { return e.(*Editor).change(fn.(func(*model.Project))) }
	inside.Try = func(e any, merge string, fn any) (any, error) {
		return e.(*Editor).try(merge, fn.(func(*model.Project) error))
	}
	inside.RelinkMarks = func(e, files, dirs any) any {
		return e.(*Editor).relinkMarks(files.([]model.PathPair), dirs.([]model.PathPair))
	}

	inside.From = func(p any) int { return p.(*model.Project).From() }
	inside.Loaded = func(p any) any { return p.(*model.Project).Loaded() }
	inside.Alt = func(m any) string { return m.(model.Mark).Alt() }

	inside.SlowOp = func() time.Duration { return slowOp }
	inside.SetSlowOp = func(d time.Duration) { slowOp = d }
	inside.SlowEdit = func() time.Duration { return slowEdit }
	inside.SetSlowEdit = func(d time.Duration) { slowEdit = d }
}
