package album

import (
	"errors"
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Подборки фото (083, design §4): список — e.bins, членство — Mark.Bins своих отметок. Правка —
// шаг «рядом с проектом» (commitSide), как у отметок: без снимка альбома и projRev, поэтому
// «Пересобрать» остаётся живым (083-3). Определение и членство меняются одним шагом.

// Тексты уходят в интерфейс как есть (тост, строка под полем) — поэтому с заглавной.
var (
	errBinGone  = errors.New("Подборки больше нет")                  //nolint:staticcheck // текст уходит пользователю как есть
	errBinTaken = errors.New("Такая подборка уже есть")              //nolint:staticcheck // текст уходит пользователю как есть
	errBinsFull = fmt.Errorf("Подборок не больше %d", model.MaxBins) //nolint:staticcheck // текст уходит пользователю как есть
)

// NewBin — новая подборка в конце списка с фото ленты photos, один шаг. Пустое имя — «Подборка N».
// Ошибка без шага: подборок уже MaxBins, имя занято (без учёта регистра), сверх предела отметок.
func (e *Editor) NewBin(name string, photos []model.PhotoInfo) (model.View, error) {
	photos = feedPhotos(photos)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("подборки") {
		return e.view(), nil
	}
	if len(e.bins) >= model.MaxBins {
		return e.view(), errBinsFull
	}
	name = model.CleanBinName(name)
	switch {
	case name == "":
		name = model.FreeBinName(e.bins)
	case model.BinNameTaken(e.bins, name, ""):
		return e.view(), errBinTaken
	}
	id := model.NewBinID(e.binIDsInUse())
	delta := e.binDelta(id, photos, true)
	if err := e.marksFit(delta); err != nil {
		return e.view(), err
	}
	after := append(slices.Clip(e.bins), model.Bin{ID: id, Name: name})
	return e.commitSide(delta, &BinsSwap{before: e.bins, After: after}), nil
}

// RenameBin — новое имя подборки id. Пустое после очистки или то же — холостая; своё имя в другом
// регистре — можно. Ошибка: подборки нет, имя занято другой.
func (e *Editor) RenameBin(id, name string) (model.View, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("подборки") {
		return e.view(), nil
	}
	i := e.binAt(id)
	if i < 0 {
		return e.view(), errBinGone
	}
	name = model.CleanBinName(name)
	if name == "" || name == e.bins[i].Name {
		return e.view(), nil
	}
	if model.BinNameTaken(e.bins, name, id) {
		return e.view(), errBinTaken
	}
	after := slices.Clone(e.bins)
	after[i].Name = name
	return e.commitSide(nil, &BinsSwap{before: e.bins, After: after}), nil
}

// DeleteBin — подборка id и её членство у всех фото (и вне ленты) одним шагом; фото остаются в
// ленте и альбоме. Неизвестный ID — холостая.
func (e *Editor) DeleteBin(id string) model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("подборки") {
		return e.view()
	}
	i := e.binAt(id)
	if i < 0 {
		return e.view()
	}
	next := map[string]*model.Mark{}
	for pid, m := range e.marks {
		if slices.Contains(m.Bins, id) {
			m = m.WithBins(slices.DeleteFunc(slices.Clone(m.Bins), func(b string) bool { return b == id }))
			next[pid] = &m
		}
	}
	after := slices.Delete(slices.Clone(e.bins), i, i+1)
	return e.commitSide(e.diffMarks(next), &BinsSwap{before: e.bins, After: after})
}

// SetBin — фото ленты photos в подборку id (in) или из неё; уже так — холостая, без шага и rev.
// Добавить или убрать решает интерфейс по тому, что видит пользователь (083-20). Ошибка без шага:
// подборки нет, сверх предела отметок.
func (e *Editor) SetBin(id string, photos []model.PhotoInfo, in bool) (model.View, error) {
	photos = feedPhotos(photos)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("подборки") {
		return e.view(), nil
	}
	if e.binAt(id) < 0 {
		return e.view(), errBinGone
	}
	delta := e.binDelta(id, photos, in)
	if err := e.marksFit(delta); err != nil {
		return e.view(), err
	}
	return e.commitSide(delta, nil), nil
}

// binAt — номер подборки id в e.bins; нет — −1.
func (e *Editor) binAt(id string) int {
	return slices.IndexFunc(e.bins, func(b model.Bin) bool { return b.ID == id })
}

// binIDsInUse — занят ли ID: в списке или в членстве отметок (запись, правленная руками, или
// копия для Restore не отдаст своё членство новой подборке, 083-21).
func (e *Editor) binIDsInUse() func(string) bool {
	used := model.BinSet(e.bins)
	for _, m := range e.marks {
		for _, b := range m.Bins {
			used[b] = true
		}
	}
	return func(id string) bool { return used[id] }
}

// binDelta — дельта отметок: фото photos в подборке id (in) или без неё. Отметка без подборок и
// своих полей уходит (diffMarks), новая заводится с путём фото ленты.
func (e *Editor) binDelta(id string, photos []model.PhotoInfo, in bool) []MarkChange {
	next := make(map[string]*model.Mark, len(photos))
	for _, ph := range photos {
		m, ok := e.marks[ph.ID]
		if slices.Contains(m.Bins, id) == in || !ok && !in {
			continue
		}
		if !ok {
			m = model.Mark{Path: ph.Path}
		}
		if in {
			m = m.WithBins(append(slices.Clip(m.Bins), id))
		} else {
			m = m.WithBins(slices.DeleteFunc(slices.Clone(m.Bins), func(b string) bool { return b == id }))
		}
		next[ph.ID] = &m
	}
	return e.diffMarks(next)
}
