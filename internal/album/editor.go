package album

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Editor — открытый проект: все правки идут через него, каждая
// правка запоминает прошлое состояние для отмены.
type Editor struct {
	mu        sync.Mutex
	p         *model.Project
	size      int // длина JSON e.p; известна после первой правки (до неё undo пуст)
	path      string
	dirty     bool
	undo      []Snap
	undoBytes int
	undoAlbum int // шагов undo со снимком альбома (p != nil); остальные — только отметки
	redo      []Snap
	rev       int // растёт при каждой реальной правке, отмене и возврате
	// lastMerge — ключ слияния последней правки (layout:<разворот>, text:<сессия>):
	// подряд идущие правки с тем же ключом — один шаг отмены.
	lastMerge string
	// origin, from, backup: файл старой версии перед первой записью в тот же
	// путь копируется в <путь>.v<from>.bak.
	origin string
	from   int
	backup bool
	// marks — свои отметки (ID фото → отметка), не nil; marksRev — из общего счётчика процесса.
	marks    map[string]model.Mark
	marksRev int
}

// NewEditor открывает проект p (из salbum.Load или model.New). Editor держит свою копию, p
// остаётся у вызывающего как был; отметки из файла (p.loaded) дальше живут в Editor.
// Проект проверяется заново (own): собранный не через Load не протащит в альбом чужой путь.
// Проверяются только пути (salbum.Check) и NaN (Marshal); остальные инварианты (лимиты,
// зажимы, цвета) — только Load/Decode, единственный источник внешних проектов.
func NewEditor(p *model.Project, path string) (*Editor, error) {
	q, marks, err := own(p)
	if err != nil {
		return nil, err
	}
	return &Editor{p: q, path: path, origin: path, from: q.From(),
		backup: path != "" && q.From() != 0 && q.From() < model.FormatVersion,
		marks:  marks, marksRev: nextMarksRev()}, nil
}

// own — своя копия внешнего проекта (NewEditor, Restore) и его отметки из файла. Пути — по
// правилам Load (salbum.Check): иначе UNC или относительный путь ушёл бы в rescan и экспорт.
// Копия идёт через JSON: from и loaded в него не попадают и переносятся явно; NaN/Inf —
// ошибка, а не паника Clone.
func own(p *model.Project) (*model.Project, map[string]model.Mark, error) {
	if err := salbum.Check(p); err != nil {
		return nil, nil, fmt.Errorf("проект не прошёл проверку: %w", err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, nil, fmt.Errorf("проект не прошёл проверку: %w", err)
	}
	q := model.Unmarshal(data)
	q.SetFrom(p.From())
	// Глубокая копия: указатели Rating/Label/Flag иначе общие с p.
	marks := make(map[string]model.Mark, len(p.Loaded()))
	for id, m := range p.Loaded() {
		marks[id] = m.Clone()
	}
	return q, marks, nil
}

// change выполняет правку с сохранением состояния для отмены.
// Правка, ничего не изменившая в проекте, в отмену не попадает.
func (e *Editor) change(fn func(p *model.Project)) model.View { return e.apply("", fn) }

// apply — тело change. merge непуст у правок, которые сливаются в один шаг
// отмены: если прошлая правка была с тем же ключом, новый снимок не делается.
func (e *Editor) apply(merge string, fn func(p *model.Project)) model.View {
	v, _ := e.try(merge, func(p *model.Project) error { fn(p); return nil })
	return v
}

// try — apply, где fn может отказать: проект откатывается к состоянию до fn,
// в отмену ничего не попадает, возвращается текущий View и ошибка. Лимиты
// проверяются внутри fn, под тем же замком, что и правка.
func (e *Editor) try(merge string, fn func(p *model.Project) error) (model.View, error) {
	return e.edit(merge, false, fn, nil)
}

// edit — тело try. whole — fn заменяет проект целиком (Restore): поправки относительно
// прежнего состояния (normalize, keepLayouts, rescale*) к нему не относятся — они исказили бы
// восстановленное (тексты пересчитались бы под прежний формат страницы).
// marks (может быть nil) — дельта своих отметок той же правки (перепривязка, Restore): зовётся
// после удачной правки проекта и идёт в тот же шаг отмены; слияние с прошлым шагом она рвёт.
func (e *Editor) edit(merge string, whole bool, fn func(p *model.Project) error, marks func() []MarkChange) (model.View, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	data := model.Marshal(e.p) // паника, а не пустой снимок: e.p остаётся как был
	e.size = len(data)
	before := model.Unmarshal(data)
	err := fn(e.p)
	if err == nil {
		err = e.p.SyncGrid(false) // страховка инвариантов сетки; без пустого места — отказ
	}
	if err != nil {
		e.p = before
		return e.view(), err
	}
	e.p.PruneRefs()
	if !whole {
		e.p.Normalize(before)
		e.p.KeepLayouts(before)
		// Смена формата страницы: тексты, места и фоны остаются на своих местах страницы.
		design.Rescale(e.p, before)
		grid.Rescale(e.p, before)
		backdrop.Rescale(e.p, before)
	}
	e.p.FitTexts()
	e.p.SyncOrder()
	if err := e.p.CheckTotals(before); err != nil {
		e.p = before
		return e.view(), err
	}
	after, err := json.Marshal(e.p)
	if err != nil {
		// В проект попало недопустимое число (NaN/Inf): правка откатывается.
		e.p = before
		return e.view(), fmt.Errorf("правка не применена: %w", err)
	}
	var delta []MarkChange
	if marks != nil {
		delta = marks()
	}
	same := bytes.Equal(data, after)
	if same && len(delta) == 0 {
		return e.view(), nil
	}
	if len(delta) > 0 {
		merge = ""
		e.setMarks(delta, false)
	}
	top := len(e.undo) - 1
	merged := merge != "" && merge == e.lastMerge && top >= 0
	switch {
	case same:
		e.pushUndo(Snap{Marks: delta}) // проект не изменился: снимок альбома не нужен
	case !merged:
		e.pushUndo(Snap{P: before, N: len(data), Marks: delta})
	case e.undo[top].P != nil && e.undo[top].N == len(after) && bytes.Equal(model.Marshal(e.undo[top].P), after):
		// Слитая цепочка вернула проект к началу (блок добавили и стёрли):
		// пустой шаг отмены не нужен.
		e.dropUndo(top)
		merge = ""
	}
	e.size = len(after)
	e.lastMerge = merge
	e.redo = nil
	e.dirty = true
	e.rev++
	return e.view(), nil
}

// Save сохраняет проект; пустой path — в текущий файл.
func (e *Editor) Save(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if path == "" {
		path = e.path
	}
	// Первая запись поверх файла старой версии: сначала копия исходника.
	// «Сохранить как» в другой путь копию не делает — исходник и так цел.
	first := e.backup && paths.SamePath(path, e.origin)
	if first {
		if err := salbum.BackupOld(path, e.from); err != nil {
			return fmt.Errorf("копия файла старой версии: %w", err)
		}
	}
	if err := salbum.SaveWith(e.p, path, e.marks); err != nil {
		return err
	}
	if first {
		e.backup = false
	}
	e.path = path
	e.dirty = false
	return nil
}

func (e *Editor) Path() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

// read — чтение проекта под замком: операции областей получают p, а не e.mu.
func (e *Editor) read(fn func(p *model.Project)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e.p)
}

func (e *Editor) Folders() (dirs []string) {
	e.read(func(p *model.Project) { dirs = slices.Clone(p.Folders) })
	return dirs
}

// Snapshot — копия проекта для экспорта в фоне.
func (e *Editor) Snapshot() (*model.Project, model.View) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.p.Clone(), e.view()
}

// WriteCopy пишет проект в файл, не делая его текущим (черновик). Пути —
// абсолютные: черновик лежит в папке данных, а не рядом с фото.
func (e *Editor) WriteCopy(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return salbum.Write(e.p, path, "", e.marks)
}

// Ref — сохранённые в проекте сведения о фото; копия без общих указателей (PhotoRef.Copy).
func (e *Editor) Ref(id string) (r model.PhotoRef, ok bool) {
	e.read(func(p *model.Project) {
		r, ok = p.Photos[id]
		r = r.Copy()
	})
	return r, ok
}

func (e *Editor) View() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	t0 := time.Now()
	v := e.view()
	slow(t0, "View: %d разворотов", len(v.Spreads))
	return v
}

// Медленные операции в log.txt (спека 020).

// slowOp — порог «медленной» операции: дольше него в log.txt уходит строка с временем.
// Переменная, а не const, только ради теста.
var slowOp = 300 * time.Millisecond

// slow пишет в лог строку, если с t0 прошло не меньше slowOp. В строке только счётчики
// и время — ни путей, ни имён файлов (design 020 §4.3).
func slow(t0 time.Time, format string, args ...any) {
	if d := time.Since(t0); d >= slowOp {
		log.Printf(format+" за %s", append(args, d.Round(time.Millisecond))...)
	}
}
