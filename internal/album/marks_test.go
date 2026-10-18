package album_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// xp — фото ленты с отметкой из файла.
func xp(id string, x model.XmpMarks) model.PhotoInfo {
	ph := edtest.Photo(id)
	ph.Xmp = x
	return ph
}

func TestSetMarks(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v0 := e.View()
	v := edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(4)})
	if v.Rev != v0.Rev+1 || !v.Dirty || !v.CanUndo || edtest.UndoLen(e) != 1 || v.MarksRev == v0.MarksRev {
		t.Fatalf("rev %d→%d, dirty %v, undo %d, marksRev %d→%d", v0.Rev, v.Rev, v.Dirty, edtest.UndoLen(e), v0.MarksRev, v.MarksRev)
	}
	want := map[string][3]any{"a": {4, "-", -9}, "b": {4, "-", -9}}
	edtest.WantMarks(t, e, want)
	if edtest.Undo(e)[0].P != nil || edtest.UndoBytes(e) != 2*album.MarkBytes {
		t.Fatalf("шаг отметок со снимком проекта или с весом %d", edtest.UndoBytes(e))
	}
	u := e.Undo()
	edtest.WantMarks(t, e, map[string][3]any{})
	if u.MarksRev == v.MarksRev || u.Rev != v.Rev+1 || !u.CanRedo {
		t.Fatal("отмена отметок: marksRev/rev/redo")
	}
	r := e.Redo()
	edtest.WantMarks(t, e, want)
	if r.MarksRev == u.MarksRev || edtest.UndoBytes(e) != 2*album.MarkBytes {
		t.Fatal("возврат отметок")
	}
	// Новый проект — другой marksRev, даже без отметок.
	if a, b := edtest.Open(t, model.New(), ""), edtest.Open(t, model.New(), ""); a.View().MarksRev == b.View().MarksRev {
		t.Fatal("marksRev нового проекта совпал")
	}
}

func TestSetMarksNoop(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(3)})
	edtest.SetLastMerge(e, "text:s")
	for _, c := range []struct {
		ph []model.PhotoInfo
		pt model.MarkPatch
	}{
		{edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(3)}},                                    // повтор
		{edtest.Photos("a", "b"), model.MarkPatch{}},                                                   // пустой патч
		{[]model.PhotoInfo{xp("b", model.XmpMarks{Rating: 2})}, model.MarkPatch{Rating: edtest.Ip(2)}}, // как в файле, своей нет
		{[]model.PhotoInfo{xp("b", model.XmpMarks{Label: "red"})}, model.MarkPatch{Label: edtest.Sp("red")}},
		{edtest.Photos("b"), model.MarkPatch{Flag: edtest.Ip(0)}},
		{nil, model.MarkPatch{Rating: edtest.Ip(1)}},
	} {
		v := e.View()
		if got := edtest.SetMarks(t, e, c.ph, c.pt); got.Rev != v.Rev || got.MarksRev != v.MarksRev || edtest.UndoLen(e) != 1 {
			t.Fatalf("%+v: не холостая", c.pt)
		}
	}
	if edtest.LastMerge(e) != "text:s" {
		t.Fatal("холостая отметка порвала цепочку слияния")
	}
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(1)})
	if edtest.LastMerge(e) != "" || edtest.Redo(e) != nil {
		t.Fatal("отметка не порвала цепочку слияния")
	}
}

func TestSetMarksPerField(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(3)})
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Label: edtest.Sp("red")})
	edtest.WantMarks(t, e, map[string][3]any{"a": {3, "red", -9}})
	// 0 и "" — свои значения, а не «нет своей»: поверх файла 3★ и красной.
	edtest.SetMarks(t, e, []model.PhotoInfo{xp("b", model.XmpMarks{Rating: 3, Label: "red"})}, model.MarkPatch{Rating: edtest.Ip(0), Label: edtest.Sp("")})
	edtest.WantMarks(t, e, map[string][3]any{"a": {3, "red", -9}, "b": {0, "", -9}})
}

func TestSetMarksInvalid(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(7), Label: edtest.Sp("pink"), Flag: edtest.Ip(2), Step: 3})
	if v.Rev != 0 || edtest.MarksLen(e) != 0 {
		t.Fatal("недопустимый патч что-то поставил")
	}
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(-1), Label: edtest.Sp("other"), Flag: edtest.Ip(1)})
	edtest.WantMarks(t, e, map[string][3]any{"a": {-9, "-", 1}})
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(2), Step: 1}) // Step при Rating не действует
	edtest.WantMarks(t, e, map[string][3]any{"a": {2, "-", 1}})
}

// Недопустимый Rating отбрасывается, но Step при заданном Rating всё равно не действует.
func TestSetMarksInvalidRatingStep(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	x := xp("x", model.XmpMarks{Rating: 4})
	v := edtest.SetMarks(t, e, []model.PhotoInfo{x}, model.MarkPatch{Rating: edtest.Ip(7), Step: 1})
	if v.Rev != 0 || edtest.MarksLen(e) != 0 || edtest.UndoLen(e) != 0 {
		t.Fatal("Step сработал при недопустимом Rating")
	}
}

func TestSetMarksStep(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	x := xp("x", model.XmpMarks{Rating: 4})
	edtest.SetMarks(t, e, []model.PhotoInfo{x}, model.MarkPatch{Step: 1})
	edtest.WantMarks(t, e, map[string][3]any{"x": {5, "-", -9}})
	rev := e.View().Rev
	edtest.SetMarks(t, e, []model.PhotoInfo{x}, model.MarkPatch{Step: 1})
	edtest.SetMarks(t, e, edtest.Photos("z"), model.MarkPatch{Step: -1})
	if e.View().Rev != rev {
		t.Fatal("шаг за край 0–5 — не холостой")
	}
	edtest.SetMarks(t, e, edtest.Photos("y"), model.MarkPatch{Rating: edtest.Ip(2)})
	n := edtest.UndoLen(e)
	edtest.SetMarks(t, e, []model.PhotoInfo{x, edtest.Photo("y"), edtest.Photo("z")}, model.MarkPatch{Step: -1})
	edtest.WantMarks(t, e, map[string][3]any{"x": {4, "-", -9}, "y": {1, "-", -9}})
	if edtest.UndoLen(e) != n+1 {
		t.Fatal("смешанная выборка — не один шаг")
	}
}

func TestSetMarksToggle(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	a, b := xp("a", model.XmpMarks{Label: "red"}), edtest.Photo("b")
	edtest.SetMarks(t, e, []model.PhotoInfo{b}, model.MarkPatch{Label: edtest.Sp("red")})
	edtest.SetMarks(t, e, []model.PhotoInfo{a, b}, model.MarkPatch{Label: edtest.Sp("red"), Toggle: true}) // у всех красная — снять
	edtest.WantMarks(t, e, map[string][3]any{"a": {-9, "", -9}, "b": {-9, "", -9}})

	e = edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, []model.PhotoInfo{a, edtest.Photo("c")}, model.MarkPatch{Label: edtest.Sp("red"), Toggle: true}) // у части — поставить
	edtest.WantMarks(t, e, map[string][3]any{"c": {-9, "red", -9}})
}

func TestSetMarksLimit(t *testing.T) {
	old := model.MarksLimit()
	t.Cleanup(func() { model.SetMarksLimit(old) })
	model.SetMarksLimit(2)
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Flag: edtest.Ip(1)})
	v := e.View()
	if _, err := e.SetMarks(edtest.Photos("a", "c"), model.MarkPatch{Flag: edtest.Ip(-1)}); err == nil {
		t.Fatal("сверх предела — без ошибки")
	}
	if got := e.View(); got.Rev != v.Rev || got.MarksRev != v.MarksRev {
		t.Fatal("отказ поменял rev")
	}
	edtest.WantMarks(t, e, map[string][3]any{"a": {-9, "-", 1}, "b": {-9, "-", 1}})
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(5)}) // правка своих при пределе — можно
}

func TestResetMarks(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(1)})
	e.ResetMarks([]string{"a", "a", "нет"})
	edtest.WantMarks(t, e, map[string][3]any{"b": {1, "-", -9}})
	v := e.View()
	if e.ResetMarks([]string{"a"}).Rev != v.Rev {
		t.Fatal("сброс сброшенного — не холостой")
	}
	e.Undo()
	edtest.WantMarks(t, e, map[string][3]any{"a": {1, "-", -9}, "b": {1, "-", -9}})
}

// SnapshotMarks (069): копии проекта и отметок без общих указателей с редактором; без отметок —
// пустая карта, не nil; снимок не делает шага и не меняет rev.
func TestSnapshotMarks(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	if p, m := e.SnapshotMarks(); p == nil || m == nil || len(m) != 0 {
		t.Fatalf("пустой альбом: %v %v", p, m)
	}
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(2)})
	v := e.View()
	p, m := e.SnapshotMarks()
	if len(m) != 2 || *m["a"].Rating != 2 || *m["b"].Rating != 2 {
		t.Fatalf("снимок отметок: %v", m)
	}
	*m["a"].Rating = 5
	delete(m, "b")
	p.Settings.PageW = 1
	edtest.WantMarks(t, e, map[string][3]any{"a": {2, "-", -9}, "b": {2, "-", -9}})
	if edtest.P(e).Settings.PageW == 1 {
		t.Fatal("снимок проекта связан с редактором")
	}
	if nv := e.View(); nv.Rev != v.Rev || edtest.UndoLen(e) != 1 {
		t.Fatalf("снимок сделал шаг: rev %d→%d, undo %d", v.Rev, nv.Rev, edtest.UndoLen(e))
	}
}

// Общий стек: шаги альбома и отметок отменяются по порядку, каждый своё.
func TestMarksUndoMixed(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	edtest.SetUndo(e, nil)
	edtest.SetUndoBytes(e, 0)
	edtest.SetUndoAlbum(e, 0)
	type state struct {
		p     []byte
		marks map[string][3]any
	}
	at := func() state { return state{p: model.Marshal(edtest.P(e)), marks: edtest.MarkRows(e)} }
	states := []state{at()}
	s := edtest.P(e).Settings
	for i := range 2 {
		s.DPI += 10
		e.SetSettings(s)
		states = append(states, at())
		edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(i + 1)})
		states = append(states, at())
	}
	if edtest.UndoLen(e) != 4 {
		t.Fatalf("шагов %d", edtest.UndoLen(e))
	}
	same := func(i int) {
		t.Helper()
		got := at()
		if !bytes.Equal(got.p, states[i].p) || !reflect.DeepEqual(got.marks, states[i].marks) {
			t.Fatalf("состояние %d: %s %v, ждали %s %v", i, got.p, got.marks, states[i].p, states[i].marks)
		}
	}
	for i := 3; i >= 0; i-- {
		e.Undo()
		same(i)
	}
	for i := 1; i <= 4; i++ {
		e.Redo()
		same(i)
	}
	if edtest.UndoBytes(e) != edtest.Undo(e)[0].Weight()+edtest.Undo(e)[1].Weight()+edtest.Undo(e)[2].Weight()+edtest.Undo(e)[3].Weight() {
		t.Fatal("undoBytes разошёлся с шагами")
	}
}

// Отметки фото ленты, не только альбома: правки альбома их не трогают.
func TestMarksSurviveAlbumEdits(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, filepath.Join(testkit.TempDir(t), "Отбор"))
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	edtest.SetMarks(t, e, edtest.Photos("a", "b", "z"), model.MarkPatch{Rating: edtest.Ip(5), Flag: edtest.Ip(1)})
	want, n := edtest.MarkRows(e), edtest.UndoLen(e)
	e.RemovePhotos([]string{"a"})
	e.RemoveFolder(edtest.P(e).Folders[0])
	e.DeleteSpread(v.Spreads[0].ID)
	if _, err := e.AutoBuild(edtest.Photos("a", "b", "c"), -1, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.PruneRefs() })
	edtest.WantMarks(t, e, want)
	for edtest.UndoLen(e) > n {
		e.Undo()
	}
	edtest.WantMarks(t, e, want)
}

func TestMarksRelink(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	oldA := s.E.Refs()[s.IdA].Path
	lib := filepath.Join(s.OldFolder, "Отбор", "3.jpg") // фото ленты не из альбома
	other := filepath.Join(filepath.Dir(s.OldFolder), "Чужие", "4.jpg")
	edtest.SetMarks(t, s.E, []model.PhotoInfo{
		{ID: s.IdA, Path: oldA},
		{ID: library.IDFor(lib), Path: lib},
		{ID: library.IDFor(other), Path: other},
	}, model.MarkPatch{Rating: edtest.Ip(4)})
	before := edtest.MarkRows(s.E)
	n := edtest.UndoLen(s.E)
	s.E.Relink(map[string]string{s.IdA: s.NewA, s.IdB: s.NewB}, "")
	if edtest.UndoLen(s.E) != n+1 {
		t.Fatal("перепривязка с отметками — не один шаг")
	}
	newLib := filepath.Join(s.NewFd, "Отбор", "3.jpg")
	got := s.E.Marks()
	for _, path := range []string{s.NewA, newLib, other} {
		if m, ok := got[library.IDFor(path)]; !ok || m.Rating == nil || *m.Rating != 4 {
			t.Errorf("нет отметки у %s: %v", path, got)
		}
	}
	if m := edtest.Marks(s.E)[library.IDFor(s.NewA)]; m.Path != s.NewA {
		t.Errorf("путь отметки %q", m.Path)
	}
	if len(got) != 3 {
		t.Errorf("отметок %d, ждали 3", len(got))
	}
	s.E.Undo()
	if !reflect.DeepEqual(edtest.MarkRows(s.E), before) || edtest.Marks(s.E)[s.IdA].Path != oldA {
		t.Fatalf("отмена не вернула отметки: %v", edtest.MarkRows(s.E))
	}
	s.E.Redo()
	if _, ok := s.E.Marks()[library.IDFor(newLib)]; !ok {
		t.Fatal("возврат не перенёс отметки")
	}
}

// Restore ставит отметки копии вместе с альбомом одним шагом; совпало — холостая.
func TestRestoreMarks(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	a, b := filepath.Join(dir, "1.jpg"), filepath.Join(dir, "2.jpg")
	pa := model.PhotoInfo{ID: library.IDFor(a), Path: a, W: 300, H: 200}
	pb := model.PhotoInfo{ID: library.IDFor(b), Path: b, W: 300, H: 200}
	e := edtest.Open(t, model.New(), path)
	edtest.NewSpread(e, -1, []model.PhotoInfo{pa})
	edtest.SetMarks(t, e, []model.PhotoInfo{pa, pb}, model.MarkPatch{Rating: edtest.Ip(2)})
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	inCopy := edtest.MarkRows(e)
	edtest.SetMarks(t, e, []model.PhotoInfo{pa}, model.MarkPatch{Rating: edtest.Ip(5)})
	e.ResetMarks([]string{pb.ID})
	now := edtest.MarkRows(e)

	q, err := album.LoadBase(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	n := edtest.UndoLen(e)
	v, err := e.Restore(q)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(edtest.MarkRows(e), inCopy) || edtest.UndoLen(e) != n+1 || edtest.Undo(e)[n].P != nil {
		t.Fatalf("после Restore %v (ждали %v), шагов %d", edtest.MarkRows(e), inCopy, edtest.UndoLen(e)-n)
	}
	if again, _ := e.Restore(q); again.Rev != v.Rev {
		t.Fatal("повтор Restore — не холостой")
	}
	e.Undo()
	if !reflect.DeepEqual(edtest.MarkRows(e), now) {
		t.Fatalf("отмена Restore: %v, ждали %v", edtest.MarkRows(e), now)
	}
	// Копия без отметок: после замены их нет.
	e2 := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e2, []model.PhotoInfo{pa}, model.MarkPatch{Flag: edtest.Ip(1)})
	if _, err := e2.Restore(model.New()); err != nil || len(e2.Marks()) != 0 {
		t.Fatalf("Restore копии без отметок: %v %v", err, e2.Marks())
	}
}

// Вес шага на 200 фото — дельта, а не карта целиком.
func TestMarksUndoWeight(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ids := make([]string, 200)
	for i := range ids {
		ids[i] = "p" + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	edtest.SetMarks(t, e, edtest.Photos(ids...), model.MarkPatch{Rating: edtest.Ip(3)})
	if edtest.UndoBytes(e) != 200*album.MarkBytes || edtest.UndoBytes(e) >= 64<<10 {
		t.Fatalf("вес шага %d", edtest.UndoBytes(e))
	}
	allocs := testing.AllocsPerRun(20, func() {
		e.SetMarks(edtest.Photos("x"), model.MarkPatch{Rating: edtest.Ip(1)}) //nolint:errcheck // замер
		e.Undo()
	})
	if allocs > 200 {
		t.Fatalf("одна отметка с отменой — %v аллокаций", allocs)
	}
}

// Занятый путь: у нового пути уже есть своя отметка — обе остаются на месте; две отметки на
// один новый путь — переезжает одна.
func TestMarksRelinkTaken(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	oldA := s.E.Refs()[s.IdA].Path
	edtest.SetMarks(t, s.E, []model.PhotoInfo{{ID: s.IdA, Path: oldA}}, model.MarkPatch{Rating: edtest.Ip(4)})
	edtest.SetMarks(t, s.E, []model.PhotoInfo{edtest.Lib(s.NewA, model.XmpMarks{})}, model.MarkPatch{Rating: edtest.Ip(2)})
	s.E.Relink(map[string]string{s.IdA: s.NewA}, "")
	if m := edtest.Marks(s.E)[s.IdA]; m.Path != oldA || *m.Rating != 4 {
		t.Fatalf("старая отметка: %+v", m)
	}
	if m := edtest.Marks(s.E)[library.IDFor(s.NewA)]; m.Path != s.NewA || *m.Rating != 2 {
		t.Fatalf("отметка нового пути затёрта: %+v", m)
	}

	dir := testkit.TempDir(t)
	p1, p2, n := filepath.Join(dir, "1.jpg"), filepath.Join(dir, "2.jpg"), filepath.Join(dir, "н", "1.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, []model.PhotoInfo{edtest.Lib(p1, model.XmpMarks{}), edtest.Lib(p2, model.XmpMarks{})}, model.MarkPatch{Flag: edtest.Ip(1)})
	delta := edtest.RelinkMarks(e, []model.PathPair{{Old: p1, New: n}, {Old: p2, New: n}}, nil)
	moved, gone := 0, 0
	for _, c := range delta {
		switch {
		case c.After != nil && c.ID == library.IDFor(n):
			moved++
		case c.After == nil:
			gone++
		}
	}
	if len(delta) != 2 || moved != 1 || gone != 1 {
		t.Fatalf("дельта %+v", delta)
	}
}

// RelinkDrive переносит отметки альбома и ленты по смене буквы диска одним шагом.
func TestRelinkDriveMarks(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	tail := root[len(vol):]
	newFolder := filepath.Join(root, "Съёмка")
	edtest.PutFile(t, filepath.Join(newFolder, "1.jpg"), 10)
	oldFolder := from + ":" + filepath.Join(tail, "Съёмка")
	oldPath, oldLib := filepath.Join(oldFolder, "1.jpg"), filepath.Join(oldFolder, "2.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, oldFolder)
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: library.IDFor(oldPath), Path: oldPath, W: 3, H: 2, Size: 10}})
	edtest.SetMarks(t, e, []model.PhotoInfo{edtest.Lib(oldPath, model.XmpMarks{}), edtest.Lib(oldLib, model.XmpMarks{})}, model.MarkPatch{Rating: edtest.Ip(3)})
	before, n := edtest.MarkRows(e), edtest.UndoLen(e)
	e.RelinkDrive(from, to)
	if edtest.UndoLen(e) != n+1 {
		t.Fatal("смена диска с отметками — не один шаг")
	}
	want := map[string][3]any{
		library.IDFor(filepath.Join(newFolder, "1.jpg")): {3, "-", -9},
		library.IDFor(filepath.Join(newFolder, "2.jpg")): {3, "-", -9},
	}
	edtest.WantMarks(t, e, want)
	for _, m := range edtest.Marks(e) {
		if !strings.HasPrefix(m.Path, newFolder) {
			t.Fatalf("путь отметки %q", m.Path)
		}
	}
	e.Undo()
	if !reflect.DeepEqual(edtest.MarkRows(e), before) {
		t.Fatalf("отмена не вернула отметки: %v", edtest.MarkRows(e))
	}
}

// Отказная правка альбома (сверх MaxPhotos) отметки не трогает: ни отметок, ни MarksRev, ни шага.
func TestRefusedEditKeepsMarks(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(5)})
	want, n, v0 := edtest.MarkRows(e), edtest.UndoLen(e), e.View()
	many := make([]string, model.MaxPhotos+1)
	for i := range many {
		many[i] = "m" + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	if _, err := e.AddPhotos(v.Spreads[0].ID, edtest.Photos(many...), -1); err == nil {
		t.Fatal("правка сверх MaxPhotos прошла")
	}
	if v1 := e.View(); v1.Rev != v0.Rev || v1.MarksRev != v0.MarksRev || edtest.UndoLen(e) != n {
		t.Fatalf("rev %d→%d, marksRev %d→%d, шагов %d→%d", v0.Rev, v1.Rev, v0.MarksRev, v1.MarksRev, n, edtest.UndoLen(e))
	}
	edtest.WantMarks(t, e, want)
}
