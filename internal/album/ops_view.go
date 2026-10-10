package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
)

// Обёртка Editor над сборкой вида (internal/album/view). Вызывается под e.mu: View, edit,
// Undo/Redo и правки отметок берут замок раньше. Всё остальное чтение вида идёт через
// публичный Editor.View (editor.go).

// view — вид проекта с состоянием редактора (путь, dirty, глубина отмены, rev).
// Договор: e.mu уже взят вызывающим; проект только читается.
func (e *Editor) view() model.View {
	return view.BuildView(e.p, view.State{Path: e.path, Dirty: e.dirty, CanUndo: len(e.undo) > 0,
		CanRedo: len(e.redo) > 0, Rev: e.rev, MarksRev: e.marksRev})
}
