package album

// Отмена и возврат: шаги-снимки, их предел по числу и по байтам, Undo и Redo.
// Сам шаг кладёт Editor.edit (editor.go).

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
)

// maxUndo — шагов отмены со снимком альбома; maxMarkUndo — шагов только отметок и подборок (решение
// владельца 2026-10-09): частые клики оценок не вытесняют правки альбома, а дельты лёгкие.
const (
	MaxUndo     = 200
	MaxMarkUndo = 2000
)

// maxUndoBytes — предел суммы снимков отмены (по длине JSON): альбом с текстом
// в 200 шагах иначе может занять гигабайты. var — чтобы тест мог уменьшить.
var maxUndoBytes = 256 << 20

// Snap — шаг отмены: снимок проекта и его размер в байтах JSON и/или правка «рядом с проектом»
// (дельта своих отметок, список подборок). p == nil — шаг только отметок и подборок (снимок
// альбома им не нужен).
type Snap struct {
	P     *model.Project
	N     int
	Marks []MarkChange
	Bins  *BinsSwap // nil — подборки шаг не менял
}

// BinsSwap — список подборок до и после шага целиком (083 §4.1): ≤ MaxBins строк, дельта не нужна.
// Срезы не правятся на месте: их делят Editor.bins и шаги отмены.
type BinsSwap struct{ before, After []model.Bin }

// MarkBytes — вес записи дельты отметок в undoBytes: замер с путём, полями и двумя Mark — 169–176 Б.
// BinBytes — вес строки списка подборок: ID ~10 Б, имя до 80 Б UTF-8, заголовки строк и среза.
const (
	MarkBytes = 176
	BinBytes  = 128
)

// Weight — вклад шага в undoBytes.
func (s Snap) Weight() int { return s.N + s.sideWeight() }

// sideWeight — вес правки «рядом с проектом» (отметки и подборки).
func (s Snap) sideWeight() int {
	n := MarkBytes * len(s.Marks)
	if s.Bins != nil {
		n += BinBytes * (len(s.Bins.before) + len(s.Bins.After))
	}
	return n
}

// side — у шага есть правка отметок или подборок.
func (s Snap) side() bool { return len(s.Marks) > 0 || s.Bins != nil }

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
		if !e.undo[i].side() {
			e.dropUndo(i)
			continue
		}
		// Снимок уходит, отметки и подборки остаются шагом «рядом»: иначе их цепочка порвалась бы.
		e.undoBytes -= e.undo[i].N
		e.undo[i].P, e.undo[i].N = nil, 0
		e.undoAlbum--
	}
	for len(e.undo)-e.undoAlbum > MaxMarkUndo {
		i := slices.IndexFunc(e.undo, func(s Snap) bool { return s.P == nil })
		// Шаги старше вытесненного теряют свои отметки и подборки: отменяются только снимки
		// альбома, а отметки и подборки — непрерывным хвостом истории (членство и список уходят
		// вместе — подборки без определения не остаётся, 083 §4.3).
		for j := range i {
			e.undoBytes -= e.undo[j].sideWeight()
			e.undo[j].Marks, e.undo[j].Bins = nil, nil
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

// sessionStart — проект до первой правки идущей цепочки merge (снимок её шага отмены): база
// правок «от начала жеста» (наклон, 057). Цепочка не идёт, ключ пуст или снимка нет — nil.
// Только чтение; зовётся из fn правки, под e.mu.
func (e *Editor) sessionStart(merge string) *model.Project {
	top := len(e.undo) - 1
	if merge == "" || merge != e.lastMerge || top < 0 {
		return nil
	}
	return e.undo[top].P
}

func (e *Editor) Undo() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.undoLocked()
}

// undoLocked — тело Undo под e.mu (UndoIfRev сверяет rev под тем же замком).
func (e *Editor) undoLocked() model.View {
	if e.shut("отмена") {
		return e.view()
	}
	if n := len(e.undo); n > 0 {
		s := e.undo[n-1]
		e.dropUndo(n - 1)
		e.redo = append(e.redo, e.swap(s, true))
		e.dirty = true
		e.rev++
		if s.P != nil {
			e.projRev++
		}
	}
	e.lastMerge = ""
	return e.view()
}

func (e *Editor) Redo() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("возврат") {
		return e.view()
	}
	if n := len(e.redo); n > 0 {
		s := e.redo[n-1]
		e.redo = e.redo[:n-1]
		e.pushUndo(e.swap(s, false))
		e.dirty = true
		e.rev++
		if s.P != nil {
			e.projRev++
		}
	}
	e.lastMerge = ""
	return e.view()
}

// swap ставит состояние шага s (back — отмена: отметки и подборки «до», иначе «после») и
// возвращает обратный шаг: снимок текущего проекта, если у s он есть, и ту же правку «рядом».
func (e *Editor) swap(s Snap, back bool) Snap {
	r := Snap{Marks: s.Marks, Bins: s.Bins}
	if s.P != nil {
		r.P, r.N = e.p, e.size
		e.p, e.size = s.P, s.N
	}
	if s.side() {
		e.setSide(s.Marks, s.Bins, back)
	}
	return r
}

// LibKeys — ключи своего из библиотеки (PhotoRef.Lib) в проекте и во всех снимках отмены и
// возврата: удаление набора держит их файлы, иначе Ctrl+Z/Ctrl+Y вернул бы фон без файла
// (ревью 067). Только чтение, без шага.
func (e *Editor) LibKeys() map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	keys := map[string]bool{}
	add := func(p *model.Project) {
		if p == nil { // шаг только отметок
			return
		}
		for _, r := range p.Photos {
			if r.Lib != "" {
				keys[r.Lib] = true
			}
		}
	}
	add(e.p)
	add(e.auto.base) // пересборка вернёт альбом до сборки, даже если его снимок ушёл из отмены
	for _, s := range e.undo {
		add(s.P)
	}
	for _, s := range e.redo {
		add(s.P)
	}
	return keys
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
	if e.shut("имена шаблонов") {
		return e.view(), false
	}
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
	if e.auto.base != nil { // снимок пересборки бывает и вне отмены (вытеснен); в отмене — уже переименован
		vigsheet.RenameOwn(e.auto.base, names)
	}
	return e.view(), changed
}
