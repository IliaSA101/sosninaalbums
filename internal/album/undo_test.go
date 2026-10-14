package album

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// undoInvariants — счётчики стека отмены сходятся с шагами.
func undoInvariants(t *testing.T, e *Editor) {
	t.Helper()
	album, sum := 0, 0
	for _, s := range e.undo {
		if s.P != nil {
			album++
		}
		if s.P == nil && len(s.Marks) == 0 {
			t.Fatal("пустой шаг в отмене")
		}
		sum += s.Weight()
	}
	if album != e.undoAlbum || sum != e.undoBytes {
		t.Fatalf("снимков %d (счётчик %d), байт %d (счётчик %d)", album, e.undoAlbum, sum, e.undoBytes)
	}
}

// Шаги отметок не вытесняют правку альбома: 1 правка + 250 отметок, 251 Undo — разворот вернулся.
func TestUndoAlbumUnderMarks(t *testing.T) {
	e := newEditor(t, model.New(), "")
	v := newSpread(e, -1, photos("a", "b"))
	want := model.Marshal(e.p)
	e.DeleteSpread(v.Spreads[0].ID)
	for i := range 250 {
		setMarks(t, e, photos("a"), model.MarkPatch{Rating: ip(i%5 + 1)})
	}
	undoInvariants(t, e)
	for range 251 {
		e.Undo()
	}
	if got := model.Marshal(e.p); !bytes.Equal(got, want) || len(e.Marks()) != 0 {
		t.Fatalf("после отмены %s, отметок %d", got, len(e.Marks()))
	}
}

// 2001 шаг отметок: уходит самый старый шаг отметок, снимок альбома остаётся.
func TestUndoMarkStepsLimit(t *testing.T) {
	e := newEditor(t, model.New(), "")
	newSpread(e, -1, photos("a", "b"))
	for i := range MaxMarkUndo + 1 {
		setMarks(t, e, photos("a"), model.MarkPatch{Rating: ip(i%5 + 1)})
	}
	undoInvariants(t, e)
	if len(e.undo) != MaxMarkUndo+1 || e.undo[0].P == nil || e.undoAlbum != 1 {
		t.Fatalf("шагов %d, снимков %d", len(e.undo), e.undoAlbum)
	}
	for range MaxMarkUndo {
		e.Undo()
	}
	// Первый шаг отметок вытеснен: отмена останавливается на его результате.
	wantMarks(t, e, map[string][3]any{"a": {1, "-", -9}})
	e.Undo()
	if len(e.p.Spreads) != 0 || e.View().CanUndo {
		t.Fatal("снимок альбома не отменился")
	}
}

// Вытеснение по виду: снимок уходит, а дельта отметок того же шага остаётся шагом отметок;
// вытесненный шаг отметок забирает дельты из более старых шагов альбома.
func TestPushUndoKinds(t *testing.T) {
	d := func(id string) []MarkChange {
		return []MarkChange{{ID: id, After: &model.Mark{Path: id, Rating: ip(1)}}}
	}
	e := newEditor(t, model.New(), "")
	e.pushUndo(Snap{P: model.New(), N: 10, Marks: d("r")}) // перепривязка: снимок и отметки
	for range MaxUndo {
		e.pushUndo(Snap{P: model.New(), N: 10})
	}
	undoInvariants(t, e)
	if len(e.undo) != MaxUndo+1 || e.undo[0].P != nil || len(e.undo[0].Marks) != 1 {
		t.Fatalf("шагов %d, первый %+v", len(e.undo), e.undo[0])
	}

	e = newEditor(t, model.New(), "")
	e.pushUndo(Snap{P: model.New(), N: 10, Marks: d("r")})
	for i := range MaxMarkUndo + 1 {
		e.pushUndo(Snap{Marks: d(fmt.Sprint(i))})
	}
	undoInvariants(t, e)
	if len(e.undo) != MaxMarkUndo+1 || e.undo[0].P == nil || e.undo[0].Marks != nil || e.undo[1].Marks[0].ID != "1" {
		t.Fatalf("шагов %d, первый %+v", len(e.undo), e.undo[0])
	}
}

// Модель: случайная смесь правок альбома и отметок; Undo до конца — начало, Redo до конца — конец.
func TestUndoMixedModel(t *testing.T) {
	for seed := range uint64(5) {
		r := rand.New(rand.NewPCG(seed, 4))
		e := newEditor(t, model.New(), "")
		type state struct {
			p     []byte
			marks map[string][3]any
		}
		at := func() state { return state{model.Marshal(e.p), markRows(e)} }
		states := []state{at()}
		pool := []string{"a", "b", "c", "d", "e"}
		s := e.p.Settings
		for i := range 150 {
			rev := e.View().Rev
			id := pool[r.IntN(len(pool))]
			switch r.IntN(5) {
			case 0:
				s.DPI++
				e.SetSettings(s)
			case 1:
				newSpread(e, -1, photos(fmt.Sprint("n", i)))
			case 2:
				setMarks(t, e, photos(id, pool[r.IntN(len(pool))]), model.MarkPatch{Rating: ip(r.IntN(6)), Flag: ip(r.IntN(3) - 1)})
			case 3:
				e.ResetMarks([]string{id})
			case 4:
				if ps := e.p.Spreads; len(ps) > 0 {
					e.RemovePhotos(ps[r.IntN(len(ps))].Photos[:1])
				}
			}
			if e.View().Rev != rev {
				states = append(states, at())
			}
		}
		undoInvariants(t, e)
		if len(e.undo) != len(states)-1 {
			t.Fatalf("seed %d: шагов %d, состояний %d", seed, len(e.undo), len(states))
		}
		for e.View().CanUndo {
			e.Undo()
		}
		if got := at(); !bytes.Equal(got.p, states[0].p) || !reflect.DeepEqual(got.marks, states[0].marks) {
			t.Fatalf("seed %d: Undo до конца не дал начало", seed)
		}
		for e.View().CanRedo {
			e.Redo()
		}
		last := states[len(states)-1]
		if got := at(); !bytes.Equal(got.p, last.p) || !reflect.DeepEqual(got.marks, last.marks) {
			t.Fatalf("seed %d: Redo до конца не дал конец", seed)
		}
		undoInvariants(t, e)
	}
}

func TestCloneNaNNeverEmpties(t *testing.T) {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: "/a.jpg", Frame: &model.Frame{S: math.NaN()}}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("clone с NaN должен паниковать, а не отдавать пустой проект")
			}
		}()
		_ = p.Clone()
	}()
	e, _, _ := newTwo(t)
	v0, undo := e.View(), len(e.undo)
	e.change(func(p *model.Project) {
		r := p.Photos["a"]
		r.Frame = &model.Frame{S: math.Inf(1)}
		p.Photos["a"] = r
	})
	snap, v := e.Snapshot()
	if len(snap.Photos) != 3 || snap.Photos["a"].Frame != nil || v.Rev != v0.Rev || len(e.undo) != undo {
		t.Fatalf("правка с Inf должна откатиться: %+v rev %d/%d", snap.Photos["a"], v.Rev, v0.Rev)
	}
	// Проект жив: следующая правка проходит.
	if v := newSpread(e, -1, photos("d")); len(v.Spreads) != 3 {
		t.Fatal("после отката правки не работают")
	}
}

func TestUndoBytesLimit(t *testing.T) {
	old := maxUndoBytes
	t.Cleanup(func() { maxUndoBytes = old })
	e := newEditor(t, model.New(), "")
	for i := range 20 {
		if i == 10 {
			maxUndoBytes = 3 * len(model.Marshal(e.p)) // влезает два-три снимка
		}
		newSpread(e, -1, photos("p"+string(rune('a'+i))))
	}
	sum := 0
	for _, s := range e.undo {
		sum += s.N
		if s.N != len(model.Marshal(s.P)) {
			t.Fatal("размер снимка не совпал")
		}
	}
	if sum != e.undoBytes || sum > maxUndoBytes || len(e.undo) == 0 || len(e.undo) >= 19 {
		t.Fatalf("снимков %d, байт %d (учтено %d, предел %d)", len(e.undo), sum, e.undoBytes, maxUndoBytes)
	}
	n := len(e.undo)
	for range n {
		e.Undo()
	}
	if e.undoBytes != 0 || len(e.redo) != n {
		t.Fatalf("после отмен: байт %d, возвратов %d", e.undoBytes, len(e.redo))
	}
	for range n {
		e.Redo()
	}
	if len(e.View().Spreads) != 20 || e.undoBytes > maxUndoBytes {
		t.Fatalf("после возвратов: %d разворотов, %d байт", len(e.View().Spreads), e.undoBytes)
	}
}

// Паника внутри edit (снимок не собрался: NaN в проекте) не оставляет e.mu занятым и не трогает
// ни rev, ни стек отмены: следующий вызов отвечает, а не виснет.
func TestEditPanicReleasesLock(t *testing.T) {
	e := newEditor(t, model.New(), "")
	v := newSpread(e, -1, photos("a"))
	e.p.Photos["a"] = model.PhotoRef{Path: "/a.jpg", W: 300, H: 200, Frame: &model.Frame{S: math.NaN()}}
	rev, undo := e.rev, len(e.undo)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("паники не было")
			}
		}()
		_, _ = e.AddText(v.Spreads[0].ID, 0, "")
	}()
	done := make(chan int, 1)
	go func() { done <- e.View().Rev }()
	select {
	case got := <-done:
		if got != rev || e.rev != rev || len(e.undo) != undo {
			t.Fatalf("rev %d→%d, стек отмены %d→%d", rev, e.rev, undo, len(e.undo))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("e.mu остался занятым после паники в edit")
	}
}

// dropEditor — разворот-сетка s1: n мест 50×33 рядами по 10, первые full заняты f0…; «Авто»
// s2 с x1. Собран правкой и без шагов отмены: пути «/f0.jpg» NewEditor не пропустил бы.
func dropEditor(t *testing.T, n, full int) *Editor {
	e := newEditor(t, model.New(), "")
	e.change(func(p *model.Project) {
		s := &model.Spread{ID: "s1", Photos: []string{}}
		for i := range n {
			pl := &model.Place{ID: fmt.Sprintf("%012x", i+1), X: 5 + float64(i%10)*55, Y: 5 + float64(i/10)*45, W: 50, H: 33}
			if i < full {
				pl.Photo = fmt.Sprintf("f%d", i)
				s.Photos = append(s.Photos, pl.Photo)
				model.Remember(p, photos(pl.Photo))
			}
			s.Grid = append(s.Grid, pl)
		}
		model.Remember(p, photos("x1"))
		p.Spreads = []*model.Spread{s, {ID: "s2", Photos: []string{"x1"}}}
	})
	e.undo, e.undoAlbum, e.undoBytes = nil, 0, 0
	return e
}

// TestDropPhotosOneUndo — каждый исход броска 042 — один шаг отмены до байта (Undo, затем
// Redo); вопрос и отказ — без шага и без rev.
func TestDropPhotosOneUndo(t *testing.T) {
	vig := func(t *testing.T) *Editor {
		e := newEditor(t, NewVignetteSheet(204, 304), "")
		var ids []string
		for i := range 25 {
			ids = append(ids, fmt.Sprintf("v%d", i))
		}
		e.FillPlaces(photos(ids...), e.p.Spreads[0].ID)
		e.undo, e.undoAlbum, e.undoBytes = nil, 0, 0
		return e
	}
	for _, c := range []struct {
		name         string
		e            func(t *testing.T) *Editor
		place        string
		ph           []string
		rest         string
		step, failed bool
	}{
		{name: "места", e: func(t *testing.T) *Editor { return dropEditor(t, 4, 1) }, ph: []string{"n1", "n2"}, step: true},
		{name: "плитка", e: func(t *testing.T) *Editor { return dropEditor(t, 2, 2) }, ph: []string{"n1", "n2", "x1"}, step: true},
		{name: "вопрос", e: func(t *testing.T) *Editor { return dropEditor(t, 3, 1) }, ph: []string{"n1", "n2", "n3"}},
		{name: "вставить", e: func(t *testing.T) *Editor { return dropEditor(t, 3, 1) }, ph: []string{"n1", "n2", "n3"}, rest: "add", step: true},
		{name: "не добавлять", e: func(t *testing.T) *Editor { return dropEditor(t, 3, 1) }, ph: []string{"n1", "n2", "n3"}, rest: "skip", step: true},
		{name: "замена на месте", e: func(t *testing.T) *Editor { return dropEditor(t, 2, 2) }, place: "000000000001", ph: []string{"n1", "n2"}, rest: "add", step: true},
		{name: "обмен на месте", e: func(t *testing.T) *Editor { return dropEditor(t, 2, 2) }, place: "000000000002", ph: []string{"x1"}, step: true},
		{name: "«Авто»", e: func(t *testing.T) *Editor { return dropEditor(t, 2, 2) }, ph: []string{"n1"}, step: true},
		{name: "виньетка растёт", e: vig, ph: []string{"n1", "n2"}, step: true},
		{name: "64 места — отказ", e: func(t *testing.T) *Editor { return dropEditor(t, 62, 62) }, ph: []string{"n1", "n2", "n3"}, failed: true},
		{name: "холостой", e: func(t *testing.T) *Editor { return dropEditor(t, 2, 2) }, ph: []string{"f0"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := c.e(t)
			sid := e.p.Spreads[0].ID
			if c.name == "«Авто»" {
				sid = "s2"
			}
			before, rev := model.Marshal(e.p), e.rev
			_, out, err := e.DropPhotos(sid, c.place, photos(c.ph...), -1, c.rest)
			after := model.Marshal(e.p)
			undoInvariants(t, e)
			if (err != nil) != c.failed {
				t.Fatalf("ошибка %v", err)
			}
			if !c.step {
				if len(e.undo) != 0 || e.rev != rev || !bytes.Equal(after, before) {
					t.Fatalf("без шага: шагов %d, rev %d → %d, %+v", len(e.undo), rev, e.rev, out)
				}
				return
			}
			if len(e.undo) != 1 || bytes.Equal(after, before) {
				t.Fatalf("шагов %d, ждали 1; %+v", len(e.undo), out)
			}
			e.Undo()
			if !bytes.Equal(model.Marshal(e.p), before) {
				t.Fatal("Undo — не байт в байт")
			}
			e.Redo()
			if !bytes.Equal(model.Marshal(e.p), after) {
				t.Fatal("Redo — не байт в байт")
			}
		})
	}
}
