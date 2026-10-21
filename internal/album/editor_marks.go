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

// setSide ставит в e.marks состояние дельты, а если bins != nil — и список подборок: back — «до»
// (отмена), иначе «после». ID в дельте не повторяются (diffMarks), порядок не важен.
func (e *Editor) setSide(delta []MarkChange, bins *BinsSwap, back bool) {
	if bins != nil {
		e.bins = bins.After
		if back {
			e.bins = bins.before
		}
	}
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

// commitSide — шаг отмены только отметок и подборок (bins nil — список прежний); projRev не
// трогается. Ничего нет — холостая. Под e.mu.
func (e *Editor) commitSide(delta []MarkChange, bins *BinsSwap) model.View {
	if len(delta) == 0 && bins == nil {
		return e.view()
	}
	e.setSide(delta, bins, false)
	e.pushUndo(Snap{Marks: delta, Bins: bins})
	e.lastMerge = ""
	e.redo = nil
	e.dirty = true
	e.rev++
	return e.view()
}

// SetMarks ставит свои отметки фото (ID, путь, отметки из файла). Холостая по полю: своего нет,
// а цель равна отметке из файла. Сверх MaxMarks записей — ошибка, отметки прежние.
func (e *Editor) SetMarks(photos []model.PhotoInfo, patch model.MarkPatch) (model.View, error) {
	photos = feedPhotos(photos)
	patch = model.CleanPatch(patch)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("отметки") {
		return e.view(), nil
	}
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
	if err := e.marksFit(delta); err != nil {
		return e.view(), err
	}
	return e.commitSide(delta, nil), nil
}

// feedPhotos — фото ленты для правки отметок: без повторов, пустых ID и путей.
func feedPhotos(photos []model.PhotoInfo) []model.PhotoInfo {
	return slices.DeleteFunc(model.DedupePhotos(photos), func(ph model.PhotoInfo) bool { return ph.ID == "" || ph.Path == "" })
}

// marksFit — дельта не выводит число записей отметок за MarksLimit: иначе ошибка, шага нет.
func (e *Editor) marksFit(delta []MarkChange) error {
	added := 0
	for _, c := range delta {
		if c.before == nil {
			added++
		}
	}
	if len(e.marks)+added > model.MarksLimit() {
		return fmt.Errorf("слишком много своих отметок (больше %d)", model.MarksLimit())
	}
	return nil
}

// ResetMarks убирает свои отметки фото ids: снова действуют отметки из файлов. Членство в
// подборках остаётся (083-23): в XMP его нет; запись без подборок удаляется целиком.
func (e *Editor) ResetMarks(ids []string) model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("отметки") {
		return e.view()
	}
	next := map[string]*model.Mark{}
	for _, id := range ids {
		if m, ok := e.marks[id]; ok {
			m.Rating, m.Label, m.Flag = nil, nil, nil
			next[id] = &m
		}
	}
	return e.commitSide(e.diffMarks(next), nil)
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

// SnapshotSide — копия проекта, своих отметок и подборок под одним замком (сбор в папку, 069):
// правка между двумя отдельными снимками развела бы альбом и отметки собранной копии.
func (e *Editor) SnapshotSide() (*model.Project, model.Side) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.p.Clone(), e.side().Clone()
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

// replaceBins — смена списка подборок на bins целиком (Restore); совпал — nil.
func (e *Editor) replaceBins(bins []model.Bin) *BinsSwap {
	if slices.Equal(e.bins, bins) {
		return nil
	}
	return &BinsSwap{before: e.bins, After: bins}
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
