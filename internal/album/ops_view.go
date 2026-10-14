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
	v := view.BuildView(e.p, e.state(false))
	markVigs(e.p, &v) // имя шаблона и «Ручная» виньеток: план мест знает Editor (038)
	return v
}

// printView — вид печати для экспорта (Snapshot): пустые поля — пусто, заглушек нет (030).
// Договор: как у view.
func (e *Editor) printView() model.View { return view.BuildView(e.p, e.state(true)) }

func (e *Editor) state(forPrint bool) view.State {
	st := view.State{Path: e.path, Dirty: e.dirty, CanUndo: len(e.undo) > 0,
		CanRedo: len(e.redo) > 0, Rev: e.rev, MarksRev: e.marksRev, Print: forPrint}
	if e.p.Cover != nil && !forPrint {
		st.Guide = e.guideFor(e.p.Cover.Guide)
	}
	return st
}
