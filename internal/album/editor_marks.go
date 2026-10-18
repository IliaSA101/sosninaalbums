package album

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Правки своих отметок (design 004 §2.3). Не через change: снимок проекта им не нужен,
// шаг отмены — дельта по затронутым фото в общем стеке. Контракт тот же: холостая — ни шага,
// ни rev, ни dirty; иначе шаг, rev++, dirty, redo = nil, цепочка слияния рвётся.

// MarkChange — отметка фото до и после правки; nil — записи нет.
type MarkChange struct {
	ID            string
	before, After *model.Mark
}

// marksSeq — общий для процесса счётчик: у нового проекта marksRev другой, и UI перечитает отметки.
var marksSeq atomic.Int64

func nextMarksRev() int { return int(marksSeq.Add(1)) }

// setMarks ставит в e.marks состояние дельты: back — «до» (отмена), иначе «после».
// ID в дельте не повторяются (diffMarks), порядок не важен.
func (e *Editor) setMarks(delta []MarkChange, back bool) {
	for _, c := range delta {
		m := c.After
		if back {
			m = c.before
		}
		if m == nil {
			delete(e.marks, c.ID)
		} else {
			e.marks[c.ID] = *m
		}
	}
	e.marksRev = nextMarksRev()
}

// diffMarks — дельта от текущих отметок к next (ID → отметка, nil — убрать); совпавшие пропускаются.
func (e *Editor) diffMarks(next map[string]*model.Mark) []MarkChange {
	var out []MarkChange
	for _, id := range slices.Sorted(maps.Keys(next)) {
		var before *model.Mark
		if m, ok := e.marks[id]; ok {
			before = &m
		}
		after := next[id]
		if after != nil && after.None() {
			after = nil
		}
		if before == nil && after == nil || before != nil && after != nil && model.SameMark(*before, *after) {
			continue
		}
		out = append(out, MarkChange{id, before, after})
	}
	return out
}

// commitMarks — шаг отмены только отметок. Под e.mu.
func (e *Editor) commitMarks(delta []MarkChange) model.View {
	if len(delta) == 0 {
		return e.view()
	}
	e.setMarks(delta, false)
	e.pushUndo(Snap{Marks: delta})
	e.lastMerge = ""
	e.redo = nil
	e.dirty = true
	e.rev++
	return e.view()
}

// SetMarks ставит свои отметки фото (ID, путь, отметки из файла). Холостая по полю: своего нет,
// а цель равна отметке из файла. Сверх MaxMarks записей — ошибка, отметки прежние.
func (e *Editor) SetMarks(photos []model.PhotoInfo, patch model.MarkPatch) (model.View, error) {
	photos = slices.DeleteFunc(model.DedupePhotos(photos), func(ph model.PhotoInfo) bool { return ph.ID == "" || ph.Path == "" })
	patch = model.CleanPatch(patch)
	e.mu.Lock()
	defer e.mu.Unlock()
	label := patch.Label
	if label != nil && patch.Toggle && *label != "" && len(photos) > 0 &&
		!slices.ContainsFunc(photos, func(ph model.PhotoInfo) bool { return model.Effective(e.marks[ph.ID], ph.Xmp).Label != *label }) {
		label = new(string) // у всех уже этот цвет — снять
	}
	next := make(map[string]*model.Mark, len(photos))
	for _, ph := range photos {
		m, ok := e.marks[ph.ID]
		if !ok {
			m = model.Mark{Path: ph.Path}
		}
		eff := model.Effective(m, ph.Xmp)
		switch {
		case patch.Rating != nil:
			model.SetField(&m.Rating, *patch.Rating, ph.Xmp.Rating)
		case patch.Step != 0:
			model.SetField(&m.Rating, max(0, min(5, eff.Rating+patch.Step)), ph.Xmp.Rating)
		}
		if label != nil {
			model.SetField(&m.Label, *label, ph.Xmp.Label)
		}
		if patch.Flag != nil {
			model.SetField(&m.Flag, *patch.Flag, ph.Xmp.Flag)
		}
		next[ph.ID] = &m
	}
	delta := e.diffMarks(next)
	added := 0
	for _, c := range delta {
		if c.before == nil {
			added++
		}
	}
	if len(e.marks)+added > model.MarksLimit() {
		return e.view(), fmt.Errorf("слишком много своих отметок (больше %d)", model.MarksLimit())
	}
	return e.commitMarks(delta), nil
}

// ResetMarks убирает свои отметки фото ids: снова действуют отметки из файлов.
func (e *Editor) ResetMarks(ids []string) model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	next := map[string]*model.Mark{}
	for _, id := range ids {
		if _, ok := e.marks[id]; ok {
			next[id] = nil
		}
	}
	return e.commitMarks(e.diffMarks(next))
}

// Marks — копия своих отметок (ID → заданные поля).
func (e *Editor) Marks() map[string]model.MarkView {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]model.MarkView, len(e.marks))
	for id, m := range e.marks {
		out[id] = m.View()
	}
	return out
}

// SnapshotMarks — копия проекта и своих отметок под одним замком (сбор в папку, 069): правка
// между двумя отдельными снимками развела бы альбом и отметки собранной копии.
func (e *Editor) SnapshotMarks() (*model.Project, map[string]model.Mark) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]model.Mark, len(e.marks))
	for id, m := range e.marks {
		out[id] = m.Clone()
	}
	return e.p.Clone(), out
}

// rating — действующая оценка фото (своя отметка поверх XMP из infos; фото не из ленты —
// только своя): звезда разворота в автосборке и «Разнообразить» (041). Зовётся и читает
// e.marks только под e.mu (внутри правки).
func (e *Editor) rating(infos []model.PhotoInfo) func(id string) int {
	xmp := make(map[string]model.XmpMarks, len(infos))
	for _, ph := range infos {
		xmp[ph.ID] = ph.Xmp
	}
	return func(id string) int { return model.Effective(e.marks[id], xmp[id]).Rating }
}

// relinkMarks — дельта переноса отметок по перепривязке: файл из files — точная пара,
// иначе первая папка из dirs, в которой лежит отметка, — замена префикса. ID — по новому пути.
// У нового пути уже есть своя отметка — старая остаётся на месте (не затираем ни одну).
func (e *Editor) relinkMarks(files, dirs []model.PathPair) []MarkChange {
	if len(files)+len(dirs) == 0 || len(e.marks) == 0 {
		return nil
	}
	byFile := make(map[string]string, len(files))
	for _, pr := range files {
		byFile[strings.ToLower(filepath.Clean(pr.Old))] = pr.New
	}
	next := map[string]*model.Mark{}
	for _, id := range slices.Sorted(maps.Keys(e.marks)) {
		m := e.marks[id]
		np, ok := byFile[strings.ToLower(filepath.Clean(m.Path))]
		for i := 0; !ok && i < len(dirs); i++ {
			np, ok = paths.ReplacePrefix(m.Path, dirs[i].Old, dirs[i].New)
		}
		if !ok || paths.SamePath(np, m.Path) {
			continue
		}
		nid := library.IDFor(np)
		if _, taken := e.marks[nid]; taken {
			continue
		}
		if _, taken := next[nid]; taken {
			continue
		}
		m.Path = np
		m = m.WithAlt("")
		next[id], next[nid] = nil, &m
	}
	return e.diffMarks(next)
}

// replaceMarks — дельта к отметкам marks целиком (Restore).
func (e *Editor) replaceMarks(marks map[string]model.Mark) []MarkChange {
	next := make(map[string]*model.Mark, len(e.marks)+len(marks))
	for id := range e.marks {
		next[id] = nil
	}
	for id, m := range marks {
		next[id] = &m
	}
	return e.diffMarks(next)
}
