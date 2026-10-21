package album_test

// Подборки фото (083, design §4): правки Editor, отмена, «Пересобрать», Restore, перепривязка.

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// binNames — имена подборок вида по порядку.
func binNames(v model.View) []string {
	out := []string{}
	for _, b := range v.Bins {
		out = append(out, b.Name)
	}
	return out
}

// binRows — членство по имени подборки: имя → фото по возрастанию. ID без подборки — «?ID»
// (осиротевшее членство, его быть не должно).
func binRows(e *album.Editor) map[string][]string {
	name := map[string]string{}
	out := map[string][]string{}
	for _, b := range e.View().Bins {
		name[b.ID] = b.Name
		out[b.Name] = []string{}
	}
	for pid, m := range e.Marks() {
		for _, id := range m.Bins {
			n, ok := name[id]
			if !ok {
				n = "?" + id
			}
			out[n] = append(out[n], pid)
		}
	}
	for _, ids := range out {
		slices.Sort(ids)
	}
	return out
}

func wantBins(t *testing.T, e *album.Editor, names []string, rows map[string][]string) {
	t.Helper()
	if got := binNames(e.View()); !slices.Equal(got, names) {
		t.Fatalf("подборки %q, ждали %q", got, names)
	}
	if got := binRows(e); !reflect.DeepEqual(got, rows) {
		t.Fatalf("членство %v, ждали %v", got, rows)
	}
}

// newBin — новая подборка; ошибка — провал. Возвращает её ID (последний в списке).
func newBin(t *testing.T, e *album.Editor, name string, ph []model.PhotoInfo) string {
	t.Helper()
	v, err := e.NewBin(name, ph)
	if err != nil {
		t.Fatal(err)
	}
	return v.Bins[len(v.Bins)-1].ID
}

func setBin(t *testing.T, e *album.Editor, id string, ph []model.PhotoInfo, in bool) model.View {
	t.Helper()
	v, err := e.SetBin(id, ph, in)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// wantIdle — правка op холостая: ни шага, ни rev, ни marksRev; ошибка — с текстом errText.
func wantIdle(t *testing.T, e *album.Editor, what, errText string, op func() error) {
	t.Helper()
	v, n := e.View(), edtest.UndoLen(e)
	err := op()
	if errText == "" && err != nil || errText != "" && (err == nil || err.Error() != errText) {
		t.Fatalf("%s: ошибка %v, ждали %q", what, err, errText)
	}
	if got := e.View(); got.Rev != v.Rev || got.MarksRev != v.MarksRev || edtest.UndoLen(e) != n || !reflect.DeepEqual(got.Bins, v.Bins) {
		t.Fatalf("%s: не холостая (rev %d→%d, шагов %d→%d)", what, v.Rev, got.Rev, n, edtest.UndoLen(e))
	}
}

func TestNewBin(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v0 := e.View()
	if v0.Bins == nil || len(v0.Bins) != 0 {
		t.Fatalf("без подборок bins %#v, ждали пустой список", v0.Bins)
	}
	v, err := e.NewBin("  Обязательно\t", edtest.Photos("a", "b", "a"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Rev != v0.Rev+1 || v.MarksRev == v0.MarksRev || !v.Dirty || edtest.UndoLen(e) != 1 || edtest.Undo(e)[0].P != nil {
		t.Fatalf("новая подборка: rev %d→%d, шагов %d", v0.Rev, v.Rev, edtest.UndoLen(e))
	}
	if w := edtest.UndoBytes(e); w != 2*album.MarkBytes+album.BinBytes {
		t.Fatalf("вес шага %d", w)
	}
	a := v.Bins[0].ID
	if !model.OkBinID(a) {
		t.Fatalf("ID %q", a)
	}
	wantBins(t, e, []string{"Обязательно"}, map[string][]string{"Обязательно": {"a", "b"}})
	// Пустое имя — первое свободное, без фото — только список.
	newBin(t, e, " ", nil)
	newBin(t, e, "", nil)
	wantBins(t, e, []string{"Обязательно", "Подборка 1", "Подборка 2"},
		map[string][]string{"Обязательно": {"a", "b"}, "Подборка 1": {}, "Подборка 2": {}})
	wantIdle(t, e, "повтор имени", "Такая подборка уже есть", func() error {
		_, err := e.NewBin("ОБЯЗАТЕЛЬНО", edtest.Photos("c"))
		return err
	})
	if len(e.Marks()) != 2 {
		t.Fatal("отказ завёл отметку")
	}
	// Отмена и возврат: подборка и членство уходят и приходят одним шагом, ID прежний.
	e.Undo()
	e.Undo()
	e.Undo()
	wantBins(t, e, []string{}, map[string][]string{})
	if len(e.Marks()) != 0 {
		t.Fatalf("после отмены отметки %v", e.Marks())
	}
	if r := e.Redo(); r.Bins[0].ID != a {
		t.Fatalf("возврат: ID %q, был %q", r.Bins[0].ID, a)
	}
	wantBins(t, e, []string{"Обязательно"}, map[string][]string{"Обязательно": {"a", "b"}})
}

// 51-я подборка и сверх предела отметок — ошибка без шага.
func TestNewBinLimits(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	for range model.MaxBins {
		newBin(t, e, "", nil)
	}
	if v := e.View(); len(v.Bins) != model.MaxBins || v.Bins[model.MaxBins-1].Name != fmt.Sprint("Подборка ", model.MaxBins) {
		t.Fatalf("подборок %d", len(v.Bins))
	}
	wantIdle(t, e, "51-я", fmt.Sprint("Подборок не больше ", model.MaxBins), func() error {
		_, err := e.NewBin("Ещё", edtest.Photos("a"))
		return err
	})
	old := model.MarksLimit()
	t.Cleanup(func() { model.SetMarksLimit(old) })
	model.SetMarksLimit(2)
	e = edtest.Open(t, model.New(), "")
	wantIdle(t, e, "сверх отметок", "слишком много своих отметок (больше 2)", func() error {
		_, err := e.NewBin("A", edtest.Photos("a", "b", "c"))
		return err
	})
	newBin(t, e, "A", edtest.Photos("a", "b")) // ровно предел — можно
}

func TestSetBin(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	a := newBin(t, e, "A", nil)
	b := newBin(t, e, "B", nil)
	v0 := e.View()
	v := setBin(t, e, a, edtest.Photos("p1", "p2", "p3", "p1"), true)
	if v.Rev != v0.Rev+1 || v.MarksRev == v0.MarksRev || edtest.UndoLen(e) != 3 {
		t.Fatalf("SetBin: rev %d→%d, шагов %d", v0.Rev, v.Rev, edtest.UndoLen(e))
	}
	setBin(t, e, b, edtest.Photos("p2", "p4"), true)
	wantBins(t, e, []string{"A", "B"}, map[string][]string{"A": {"p1", "p2", "p3"}, "B": {"p2", "p4"}})
	if got, want := e.Marks()["p2"].Bins, []string{min(a, b), max(a, b)}; !slices.Equal(got, want) {
		t.Fatalf("p2 в подборках %q, ждали %q (по возрастанию)", got, want)
	}
	edtest.SetLastMerge(e, "text:s")
	for what, op := range map[string]func() error{
		"уже там":     func() error { _, err := e.SetBin(a, edtest.Photos("p1", "p3"), true); return err },
		"уже нет":     func() error { _, err := e.SetBin(a, edtest.Photos("p4", "p5"), false); return err },
		"пусто":       func() error { _, err := e.SetBin(a, nil, true); return err },
		"пустой путь": func() error { _, err := e.SetBin(a, []model.PhotoInfo{{ID: "p9"}}, true); return err },
	} {
		wantIdle(t, e, what, "", op)
	}
	if edtest.LastMerge(e) != "text:s" {
		t.Fatal("холостая подборка порвала цепочку слияния")
	}
	wantIdle(t, e, "нет подборки", "Подборки больше нет", func() error {
		_, err := e.SetBin("b00000000", edtest.Photos("p1"), true)
		return err
	})
	// Убрать: запись без подборок и своих полей уходит, звезда остаётся.
	edtest.SetMarks(t, e, edtest.Photos("p3"), model.MarkPatch{Rating: edtest.Ip(4)})
	setBin(t, e, a, edtest.Photos("p1", "p3"), false)
	if edtest.LastMerge(e) != "" {
		t.Fatal("подборка не порвала цепочку слияния")
	}
	wantBins(t, e, []string{"A", "B"}, map[string][]string{"A": {"p2"}, "B": {"p2", "p4"}})
	edtest.WantMarks(t, e, map[string][3]any{"p2": {-9, "-", -9}, "p3": {4, "-", -9}, "p4": {-9, "-", -9}})
	e.Undo()
	wantBins(t, e, []string{"A", "B"}, map[string][]string{"A": {"p1", "p2", "p3"}, "B": {"p2", "p4"}})
	// Предел отметок: новое фото — ошибка, фото со своей записью — можно.
	old := model.MarksLimit()
	t.Cleanup(func() { model.SetMarksLimit(old) })
	model.SetMarksLimit(len(e.Marks()))
	wantIdle(t, e, "сверх отметок", fmt.Sprintf("слишком много своих отметок (больше %d)", len(e.Marks())), func() error {
		_, err := e.SetBin(b, edtest.Photos("p1", "new"), true)
		return err
	})
	setBin(t, e, b, edtest.Photos("p1"), true)
}

func TestDeleteBin(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	a := newBin(t, e, "A", edtest.Photos("p1", "p2", "p3"))
	newBin(t, e, "B", edtest.Photos("p2"))
	c := newBin(t, e, "C", nil)
	edtest.SetMarks(t, e, edtest.Photos("p3"), model.MarkPatch{Flag: edtest.Ip(1)})
	before := binRows(e)
	n := edtest.UndoLen(e)
	v := e.DeleteBin(a)
	if edtest.UndoLen(e) != n+1 || edtest.Undo(e)[n].P != nil {
		t.Fatal("удаление — не один шаг без снимка альбома")
	}
	if slices.ContainsFunc(v.Bins, func(b model.Bin) bool { return b.ID == a }) {
		t.Fatal("подборка осталась в виде")
	}
	wantBins(t, e, []string{"B", "C"}, map[string][]string{"B": {"p2"}, "C": {}})
	edtest.WantMarks(t, e, map[string][3]any{"p2": {-9, "-", -9}, "p3": {-9, "-", 1}})
	wantIdle(t, e, "повтор", "", func() error { e.DeleteBin(a); return nil })
	wantIdle(t, e, "неизвестная", "", func() error { e.DeleteBin(""); return nil })
	e.Undo()
	if got := e.View().Bins; got[0].ID != a || !reflect.DeepEqual(binRows(e), before) {
		t.Fatalf("отмена: %v, ждали %v на первом месте", binRows(e), a)
	}
	e.DeleteBin(c) // последняя — список без хвоста
	wantBins(t, e, []string{"A", "B"}, map[string][]string{"A": {"p1", "p2", "p3"}, "B": {"p2"}})
}

func TestRenameBin(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	a := newBin(t, e, "Обязательно", edtest.Photos("p1"))
	newBin(t, e, "Родители", nil)
	n := edtest.UndoLen(e)
	v0 := e.View()
	v, err := e.RenameBin(a, "  Запас ")
	if err != nil || v.Rev != v0.Rev+1 || edtest.UndoLen(e) != n+1 || v.Bins[0] != (model.Bin{ID: a, Name: "Запас"}) {
		t.Fatalf("переименование: %v, %+v", err, v.Bins)
	}
	if s := edtest.Undo(e)[n]; s.P != nil || len(s.Marks) != 0 || s.Weight() != 4*album.BinBytes {
		t.Fatalf("шаг переименования: снимок %v, отметок %d, вес %d", s.P != nil, len(s.Marks), s.Weight())
	}
	rename := func(id, name string) func() error {
		return func() error { _, err := e.RenameBin(id, name); return err }
	}
	wantIdle(t, e, "то же имя", "", rename(a, "Запас"))
	wantIdle(t, e, "пустое", "", rename(a, " \t "))
	wantIdle(t, e, "чужое имя", "Такая подборка уже есть", rename(a, "РОДИТЕЛИ"))
	wantIdle(t, e, "нет подборки", "Подборки больше нет", rename("b00000000", "X"))
	if _, err := e.RenameBin(a, "запас"); err != nil { // своё в другом регистре — можно
		t.Fatal(err)
	}
	long := string(slices.Repeat([]rune("Я"), model.MaxBinName+5))
	if v, err := e.RenameBin(a, long); err != nil || len([]rune(v.Bins[0].Name)) != model.MaxBinName {
		t.Fatalf("длинное имя: %v %q", err, v.Bins[0].Name)
	}
	e.Undo()
	e.Undo()
	e.Undo()
	wantBins(t, e, []string{"Обязательно", "Родители"}, map[string][]string{"Обязательно": {"p1"}, "Родители": {}})
}

// «Как в Lightroom» снимает свои звёзды, метки и флажки, а членство в подборках оставляет (083-23).
func TestResetMarksKeepsBins(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	a := newBin(t, e, "A", edtest.Photos("p1", "p2"))
	edtest.SetMarks(t, e, edtest.Photos("p1", "p3"), model.MarkPatch{Rating: edtest.Ip(5), Flag: edtest.Ip(1)})
	n := edtest.UndoLen(e)
	e.ResetMarks([]string{"p1", "p2", "p3"})
	if edtest.UndoLen(e) != n+1 {
		t.Fatal("сброс — не один шаг")
	}
	edtest.WantMarks(t, e, map[string][3]any{"p1": {-9, "-", -9}, "p2": {-9, "-", -9}})
	wantBins(t, e, []string{"A"}, map[string][]string{"A": {"p1", "p2"}})
	wantIdle(t, e, "повтор сброса", "", func() error { e.ResetMarks([]string{"p1", "p2"}); return nil })
	e.Undo()
	edtest.WantMarks(t, e, map[string][3]any{"p1": {5, "-", 1}, "p2": {-9, "-", -9}, "p3": {5, "-", 1}})
	if m := e.Marks()["p1"]; !slices.Equal(m.Bins, []string{a}) {
		t.Fatalf("p1 после отмены: %v", m.Bins)
	}
}

// Правка подборок, её отмена и возврат не гасят «Пересобрать» (083 сц. 9).
func TestBinOpsKeepAgain(t *testing.T) {
	e, ph, _ := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	alive := func(step string, v model.View) {
		t.Helper()
		if !v.Again {
			t.Fatalf("%s: «Пересобрать» погасло", step)
		}
	}
	v, err := e.NewBin("Обязательно", ph[:5])
	if err != nil {
		t.Fatal(err)
	}
	alive("новая", v)
	id := v.Bins[0].ID
	alive("добавить", setBin(t, e, id, ph[5:8], true))
	alive("убрать", setBin(t, e, id, ph[:2], false))
	v, err = e.RenameBin(id, "Запас")
	if err != nil {
		t.Fatal(err)
	}
	alive("имя", v)
	id2 := newBin(t, e, "", nil)
	alive("ещё одна", e.View())
	alive("удалить", e.DeleteBin(id2))
	for i := range 6 {
		alive(fmt.Sprint("отмена ", i), e.Undo())
	}
	for i := range 6 {
		alive(fmt.Sprint("возврат ", i), e.Redo())
	}
	autoV(t, e, ph, -1, 2, true)
}

// binState — всё, что отменяется: альбом, отметки и подборки.
type binState struct {
	Spreads int
	Names   []string
	Rows    map[string][]string
	Marks   map[string][3]any
}

func binStateOf(e *album.Editor) binState {
	v := e.View()
	return binState{len(v.Spreads), binNames(v), binRows(e), edtest.MarkRows(e)}
}

// Отмена вперемешку: отметки, подборки и правки альбома — по порядку, туда и обратно.
func TestBinsUndoMix(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	var a, b string
	steps := []func(){
		func() { edtest.SetMarks(t, e, edtest.Photos("p1"), model.MarkPatch{Rating: edtest.Ip(2)}) },
		func() { a = newBin(t, e, "A", edtest.Photos("p1", "p2")) },
		func() { edtest.NewSpread(e, -1, edtest.Photos("p3")) },
		func() { setBin(t, e, a, edtest.Photos("p3"), true) },
		func() { b = newBin(t, e, "B", edtest.Photos("p2")) },
		func() { edtest.SetMarks(t, e, edtest.Photos("p2"), model.MarkPatch{Flag: edtest.Ip(-1)}) },
		func() { _, _ = e.RenameBin(a, "AA") },
		func() { e.DeleteSpread(e.View().Spreads[0].ID) },
		func() { e.DeleteBin(b) },
		func() { setBin(t, e, a, edtest.Photos("p1"), false) },
	}
	states := []binState{binStateOf(e)}
	for i, step := range steps {
		n := edtest.UndoLen(e)
		step()
		if edtest.UndoLen(e) != n+1 {
			t.Fatalf("шаг %d: шагов отмены %d→%d", i, n, edtest.UndoLen(e))
		}
		states = append(states, binStateOf(e))
	}
	for i := len(steps) - 1; i >= 0; i-- {
		e.Undo()
		if got := binStateOf(e); !reflect.DeepEqual(got, states[i]) {
			t.Fatalf("отмена до %d: %+v, ждали %+v", i, got, states[i])
		}
	}
	for i := 1; i <= len(steps); i++ {
		e.Redo()
		if got := binStateOf(e); !reflect.DeepEqual(got, states[i]) {
			t.Fatalf("возврат до %d: %+v, ждали %+v", i, got, states[i])
		}
	}
}

// Круг Save → Load → Editor: подборки, их порядок, ID и членство; фото только в подборке не теряется.
func TestBinsSaveLoad(t *testing.T) {
	ph := []model.PhotoInfo{}
	for _, n := range []string{"p1", "p2", "s1"} { // ID — по пути, как после Load
		p := edtest.Abs(n + ".jpg")
		ph = append(ph, model.PhotoInfo{ID: library.IDFor(p), Path: p, W: 300, H: 200})
	}
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[2:])
	a := newBin(t, e, "Обязательно", nil)
	setBin(t, e, a, ph, true)
	newBin(t, e, "Запас", ph[1:2])
	newBin(t, e, "Пустая", nil)
	edtest.SetMarks(t, e, ph[:1], model.MarkPatch{Rating: edtest.Ip(3)})
	q := edtest.SaveLoad(t, e)
	e2 := edtest.Open(t, q, "")
	if got, want := e2.View().Bins, e.View().Bins; !reflect.DeepEqual(got, want) {
		t.Fatalf("после открытия %+v, ждали %+v", got, want)
	}
	if got, want := binStateOf(e2), binStateOf(e); !reflect.DeepEqual(got.Rows, want.Rows) || !reflect.DeepEqual(got.Marks, want.Marks) {
		t.Fatalf("после открытия %+v, ждали %+v", got, want)
	}
	if e2.View().CanUndo {
		t.Fatal("открытие сделало шаг")
	}
}

func TestRestoreBins(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	ph := []model.PhotoInfo{}
	for _, n := range []string{"1", "2", "3"} {
		f := filepath.Join(dir, n+".jpg")
		ph = append(ph, model.PhotoInfo{ID: library.IDFor(f), Path: f, W: 300, H: 200})
	}
	e := edtest.Open(t, model.New(), path)
	edtest.NewSpread(e, -1, ph[:1])
	a := newBin(t, e, "A", ph[:2])
	newBin(t, e, "B", ph[2:])
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	inCopy, copyBins := binStateOf(e), e.View().Bins
	_, _ = e.RenameBin(a, "AA")
	e.DeleteBin(e.View().Bins[1].ID)
	newBin(t, e, "C", ph[1:])
	now := binStateOf(e)

	q, err := album.LoadBase(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	n := edtest.UndoLen(e)
	v, err := e.Restore(q)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(binStateOf(e), inCopy) || !reflect.DeepEqual(v.Bins, copyBins) {
		t.Fatalf("после Restore %+v, ждали %+v", binStateOf(e), inCopy)
	}
	if edtest.UndoLen(e) != n+1 || edtest.Undo(e)[n].P != nil || edtest.Undo(e)[n].Bins == nil {
		t.Fatal("Restore подборок — не один шаг рядом с альбомом")
	}
	wantIdle(t, e, "повтор Restore", "", func() error { _, err := e.Restore(q); return err })
	e.Undo()
	if !reflect.DeepEqual(binStateOf(e), now) {
		t.Fatalf("отмена Restore: %+v, ждали %+v", binStateOf(e), now)
	}
	// Только подборки разошлись (альбом и отметки те же) — тоже шаг.
	e.Redo()
	_, _ = e.RenameBin(a, "AA")
	n = edtest.UndoLen(e)
	if _, err := e.Restore(q); err != nil || edtest.UndoLen(e) != n+1 || !reflect.DeepEqual(e.View().Bins, copyBins) {
		t.Fatalf("Restore одного имени: %v, %+v", err, e.View().Bins)
	}
	// Копия без подборок: после замены их нет.
	if _, err := e.Restore(model.New()); err != nil || len(e.View().Bins) != 0 || len(binRows(e)) != 0 {
		t.Fatalf("Restore копии без подборок: %v %v", err, binRows(e))
	}
}

func TestRelinkKeepsBins(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	oldA := s.E.Refs()[s.IdA].Path
	a := newBin(t, s.E, "A", []model.PhotoInfo{{ID: s.IdA, Path: oldA}})
	before := binRows(s.E)
	s.E.Relink(map[string]string{s.IdA: s.NewA, s.IdB: s.NewB}, "")
	if m, ok := s.E.Marks()[library.IDFor(s.NewA)]; !ok || !slices.Equal(m.Bins, []string{a}) {
		t.Fatalf("членство не переехало: %v", s.E.Marks())
	}
	if _, ok := s.E.Marks()[s.IdA]; ok {
		t.Fatal("старая запись осталась")
	}
	s.E.Undo()
	if !reflect.DeepEqual(binRows(s.E), before) {
		t.Fatalf("отмена перепривязки: %v", binRows(s.E))
	}
}

// bestOf — самое короткое из n замеров op; reset между замерами не в счёт.
func bestOf(n int, op, reset func()) time.Duration {
	best := time.Duration(-1)
	for range n {
		t0 := time.Now()
		op()
		if d := time.Since(t0); best < 0 || d < best {
			best = d
		}
		reset()
	}
	return best
}

// 3200 фото, 10 подборок: SetBin на всех, DeleteBin, NewBin с 3200 фото — в пределах 30 мс (083 сц. 14).
func TestBinsFast(t *testing.T) {
	ids := make([]string, 3200)
	for i := range ids {
		ids[i] = fmt.Sprintf("p%04d", i)
	}
	ph := edtest.Photos(ids...)
	e := edtest.Open(t, model.New(), "")
	bins := make([]string, 10)
	for i := range bins {
		bins[i] = newBin(t, e, "", ph[i*320:(i+1)*320])
	}
	limit := testkit.Limit(30 * time.Millisecond)
	for what, op := range map[string]func(){
		"SetBin":    func() { setBin(t, e, bins[0], ph, true) },
		"DeleteBin": func() { e.DeleteBin(bins[1]) },
		"NewBin":    func() { newBin(t, e, "Все", ph) },
	} {
		if d := bestOf(3, op, func() { e.Undo() }); d > limit {
			t.Errorf("%s на 3200 фото: %v > %v", what, d, limit)
		}
	}
	if len(e.Marks()) != 3200 || len(e.View().Bins) != 10 {
		t.Fatalf("после замеров отметок %d, подборок %d", len(e.Marks()), len(e.View().Bins))
	}
}
