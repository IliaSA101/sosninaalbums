package edtest

// Доступ к живому состоянию Editor и к полям модели без экспорта (R4, design 023 §3.2):
// e.p → P(e), len(e.undo) → UndoLen(e), e.dirty = v → SetDirty(e, v). Значения — из inside.

import (
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/inside"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func P(e *album.Editor) *model.Project  { return inside.P(e).(*model.Project) }
func Undo(e *album.Editor) []album.Snap { return inside.Undo(e).([]album.Snap) }
func Redo(e *album.Editor) []album.Snap { return inside.Redo(e).([]album.Snap) }
func Marks(e *album.Editor) map[string]model.Mark {
	return inside.Marks(e).(map[string]model.Mark)
}
func Mu(e *album.Editor) *sync.Mutex   { return inside.Mu(e) }
func Rev(e *album.Editor) int          { return inside.Rev(e) }
func Dirty(e *album.Editor) bool       { return inside.Dirty(e) }
func Backup(e *album.Editor) bool      { return inside.Backup(e) }
func LastMerge(e *album.Editor) string { return inside.LastMerge(e) }
func UndoBytes(e *album.Editor) int    { return inside.UndoBytes(e) }

func UndoLen(e *album.Editor) int  { return len(Undo(e)) }
func RedoLen(e *album.Editor) int  { return len(Redo(e)) }
func MarksLen(e *album.Editor) int { return len(Marks(e)) }

func SetUndo(e *album.Editor, v []album.Snap) { inside.SetUndo(e, v) }
func SetDirty(e *album.Editor, v bool)        { inside.SetDirty(e, v) }
func SetLastMerge(e *album.Editor, v string)  { inside.SetLastMerge(e, v) }
func SetUndoAlbum(e *album.Editor, v int)     { inside.SetUndoAlbum(e, v) }
func SetUndoBytes(e *album.Editor, v int)     { inside.SetUndoBytes(e, v) }

// Change — e.change(fn): правка без своей операции (тест готовит состояние через конвейер).
func Change(e *album.Editor, fn func(p *model.Project)) model.View {
	return inside.Change(e, fn).(model.View)
}

// Try — e.try(merge, fn): правка с ошибкой и откатом.
func Try(e *album.Editor, merge string, fn func(p *model.Project) error) (model.View, error) {
	v, err := inside.Try(e, merge, fn)
	return v.(model.View), err
}

func RelinkMarks(e *album.Editor, files, dirs []model.PathPair) []album.MarkChange {
	return inside.RelinkMarks(e, files, dirs).([]album.MarkChange)
}

// Поля модели без экспорта (JSON): версия исходного файла, отметки из файла, запасной путь.
func From(p *model.Project) int { return inside.From(p) }
func Loaded(p *model.Project) map[string]model.Mark {
	return inside.Loaded(p).(map[string]model.Mark)
}
func Alt(m model.Mark) string { return inside.Alt(m) }

// Порог медленной операции ядра (slowOp): чтение и подмена в тесте лога.
func SlowOp() time.Duration     { return inside.SlowOp() }
func SetSlowOp(d time.Duration) { inside.SetSlowOp(d) }

// Порог медленной правки ядра (slowEdit, 075-6).
func SlowEdit() time.Duration     { return inside.SlowEdit() }
func SetSlowEdit(d time.Duration) { inside.SetSlowEdit(d) }
