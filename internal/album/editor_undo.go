package album

// Отмена и возврат: шаги-снимки, их предел по числу и по байтам, Undo и Redo.
// Сам шаг кладёт Editor.edit (editor.go).

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
)

// maxUndo — шагов отмены со снимком альбома; maxMarkUndo — шагов только отметок (решение
// владельца 2026-10-09): частые клики оценок не вытесняют правки альбома, а дельты лёгкие.
const (
	MaxUndo     = 200
	MaxMarkUndo = 2000
)

// maxUndoBytes — предел суммы снимков отмены (по длине JSON): альбом с текстом
// в 200 шагах иначе может занять гигабайты. var — чтобы тест мог уменьшить.
var maxUndoBytes = 256 << 20

// Snap — шаг отмены: снимок проекта и его размер в байтах JSON и/или дельта своих отметок.
// p == nil — шаг только отметок (снимок альбома им не нужен).
type Snap struct {
	P     *model.Project
	N     int
	Marks []MarkChange
}

// MarkBytes — вес записи дельты отметок в undoBytes: замер с путём, полями и двумя Mark — 169–176 Б.
const MarkBytes = 176

// Weight — вклад шага в undoBytes.
func (s Snap) Weight() int { return s.N + MarkBytes*len(s.Marks) }

// pushUndo кладёт шаг в отмену. Сверх maxUndo снимков альбома уходит самый старый снимок,
// сверх maxMarkUndo шагов только отметок — самый старый такой шаг, сверх maxUndoBytes — самый
// старый шаг вообще (последний остаётся всегда). Порядок стека не меняется.
func (e *Editor) pushUndo(s Snap) {
	e.undo = append(e.undo, s)
	e.undoBytes += s.Weight()
	if s.P != nil {
		e.undoAlbum++
	}
	for e.undoAlbum > MaxUndo {
		i := slices.IndexFunc(e.undo, func(s Snap) bool { return s.P != nil })
		if len(e.undo[i].Marks) == 0 {
			e.dropUndo(i)
			continue
		}
		// Снимок уходит, дельта отметок остаётся шагом отметок: иначе цепочка отметок порвалась бы.
		e.undoBytes -= e.undo[i].N
		e.undo[i].P, e.undo[i].N = nil, 0
		e.undoAlbum--
	}
	for len(e.undo)-e.undoAlbum > MaxMarkUndo {
		i := slices.IndexFunc(e.undo, func(s Snap) bool { return s.P == nil })
		// Шаги старше вытесненного теряют свои дельты отметок: отменяются только снимки альбома,
		// а отметки — непрерывным хвостом истории.
		for j := range i {
			e.undoBytes -= MarkBytes * len(e.undo[j].Marks)
			e.undo[j].Marks = nil
		}
		e.dropUndo(i)
	}
	for len(e.undo) > 1 && e.undoBytes > maxUndoBytes {
		e.dropUndo(0)
	}
}

// dropUndo убирает шаг i из отмены со счётчиками.
func (e *Editor) dropUndo(i int) {
	e.undoBytes -= e.undo[i].Weight()
	if e.undo[i].P != nil {
		e.undoAlbum--
	}
	e.undo = slices.Delete(e.undo, i, i+1)
}

func (e *Editor) Undo() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n := len(e.undo); n > 0 {
		s := e.undo[n-1]
		e.dropUndo(n - 1)
		e.redo = append(e.redo, e.swap(s, true))
		e.dirty = true
		e.rev++
	}
	e.lastMerge = ""
	return e.view()
}

func (e *Editor) Redo() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n := len(e.redo); n > 0 {
		s := e.redo[n-1]
		e.redo = e.redo[:n-1]
		e.pushUndo(e.swap(s, false))
		e.dirty = true
		e.rev++
	}
	e.lastMerge = ""
	return e.view()
}

// swap ставит состояние шага s (back — отмена: отметки «до», иначе «после») и возвращает
// обратный шаг: снимок текущего проекта, если у s он есть, и ту же дельту отметок.
func (e *Editor) swap(s Snap, back bool) Snap {
	r := Snap{Marks: s.Marks}
	if s.P != nil {
		r.P, r.N = e.p, e.size
		e.p, e.size = s.P, s.N
	}
	if len(s.Marks) > 0 {
		e.setMarks(s.Marks, back)
	}
	return r
}

// SyncOwnNames — имена своих шаблонов виньетки на листах (Vig.Own.Name) по файлу шаблонов tpls,
// в проекте и во всех снимках отмены и возврата, без шага отмены. Имя шаблона — не правка
// альбома (переименование отмены не имеет); иначе Ctrl+Z после «Применить» возвращал листу
// старое имя, и оно всплывало, когда шаблон удаляли (QA 038). changed — проект изменился
// (rev, dirty): вызывающий рассылает вид.
func (e *Editor) SyncOwnNames(tpls []model.Template) (v model.View, changed bool) {
	names := vigsheet.OwnNames(tpls)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(names) == 0 {
		return e.view(), false
	}
	if n, grow := vigsheet.RenameOwn(e.p, names); n > 0 {
		e.size += grow
		e.dirty = true
		e.rev++
		changed = true
	}
	for i := range e.undo {
		if e.undo[i].P != nil {
			_, grow := vigsheet.RenameOwn(e.undo[i].P, names)
			e.undo[i].N += grow
			e.undoBytes += grow
		}
	}
	for i := range e.redo {
		if e.redo[i].P != nil {
			_, grow := vigsheet.RenameOwn(e.redo[i].P, names)
			e.redo[i].N += grow
		}
	}
	return e.view(), changed
}
