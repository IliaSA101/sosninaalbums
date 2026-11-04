package album_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func pickBest(t *testing.T, e *album.Editor, best model.PhotoInfo, rest ...model.PhotoInfo) model.View {
	t.Helper()
	v, err := e.PickBest(best, rest)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestPickBestOneStep: лучшему «отобрано», трём остальным «отклонено», один шаг отмены; своя
// оценка у остальных не теряется; круг Save → Load держит отметки; Ctrl+Z возвращает всё, Ctrl+Y — снова.
func TestPickBestOneStep(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	e := edtest.Open(t, model.New(), path)
	ph := edtest.Photos("a", "b", "c", "d")
	edtest.SetMarks(t, e, ph[1:2], model.MarkPatch{Rating: edtest.Ip(3)})
	before := edtest.MarkRows(e)
	n, v0 := edtest.UndoLen(e), e.View()

	v := pickBest(t, e, ph[0], ph[1:]...)
	if v.Rev != v0.Rev+1 || v.MarksRev == v0.MarksRev || !v.Dirty || edtest.UndoLen(e) != n+1 || edtest.Undo(e)[n].P != nil {
		t.Fatalf("rev %d→%d, marksRev %d→%d, шагов %d→%d", v0.Rev, v.Rev, v0.MarksRev, v.MarksRev, n, edtest.UndoLen(e))
	}
	want := map[string][3]any{"a": {-9, "-", 1}, "b": {3, "-", -1}, "c": {-9, "-", -1}, "d": {-9, "-", -1}}
	edtest.WantMarks(t, e, want)

	// Save → Load: отметки доезжают.
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	// После Load ID другие (по пути): сверяем по имени файла.
	loaded := edtest.Open(t, q, path)
	byName := map[string][3]any{}
	for id, r := range edtest.MarkRows(loaded) {
		byName[filepath.Base(edtest.Marks(loaded)[id].Path)] = r
	}
	for id, r := range want {
		if byName[id+".jpg"] != r {
			t.Errorf("после Load %s: %v, ждали %v", id, byName[id+".jpg"], r)
		}
	}

	e.Undo()
	if !reflect.DeepEqual(edtest.MarkRows(e), before) || edtest.UndoLen(e) != n {
		t.Fatalf("Ctrl+Z: %v, ждали %v", edtest.MarkRows(e), before)
	}
	e.Redo()
	edtest.WantMarks(t, e, want)

	// Только лучший (остальных нет) и пара: «лучший» бывший «отклонённый» — флажок меняется.
	e2 := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e2, ph[:1], model.MarkPatch{Flag: edtest.Ip(-1)})
	pickBest(t, e2, ph[0])
	edtest.WantMarks(t, e2, map[string][3]any{"a": {-9, "-", 1}})
	pickBest(t, e2, ph[1], ph[0])
	edtest.WantMarks(t, e2, map[string][3]any{"a": {-9, "-", -1}, "b": {-9, "-", 1}})
}

// TestPickBestNoop: повтор и пустые входы — ни шага, ни rev, ни dirty; цепочка слияния цела.
func TestPickBestNoop(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ph := edtest.Photos("a", "b", "c")
	pickBest(t, e, ph[0], ph[1], ph[2])
	edtest.SetLastMerge(e, "text:s")
	v, n := e.View(), edtest.UndoLen(e)
	for name, f := range map[string]func() model.View{
		"повтор":        func() model.View { return pickBest(t, e, ph[0], ph[1], ph[2]) },
		"часть":         func() model.View { return pickBest(t, e, ph[0], ph[2]) },
		"пустой best":   func() model.View { return pickBest(t, e, model.PhotoInfo{}, ph[1]) },
		"best без пути": func() model.View { return pickBest(t, e, model.PhotoInfo{ID: "x"}, ph[1]) },
		"только best":   func() model.View { return pickBest(t, e, ph[0]) },
	} {
		if got := f(); got.Rev != v.Rev || got.MarksRev != v.MarksRev || edtest.UndoLen(e) != n {
			t.Errorf("%s: не холостая (rev %d→%d, шагов %d→%d)", name, v.Rev, got.Rev, n, edtest.UndoLen(e))
		}
	}
	if edtest.LastMerge(e) != "text:s" {
		t.Error("холостая порвала цепочку слияния")
	}
	// Закрытый редактор — прежний вид без ошибки.
	e.Close()
	if got := pickBest(t, e, ph[1], ph[0]); got.Rev != v.Rev {
		t.Error("закрытый редактор принял правку")
	}
}

// TestPickBestTooMany: больше трёх остальных — ошибка, отметки прежние; ровно 3 — можно;
// предел числа отметок — та же ошибка без правки.
func TestPickBestTooMany(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ph := edtest.Photos("a", "b", "c", "d", "e")
	edtest.SetMarks(t, e, ph[:1], model.MarkPatch{Rating: edtest.Ip(2)})
	v, before := e.View(), edtest.MarkRows(e)
	if _, err := e.PickBest(ph[0], ph[1:]); err == nil {
		t.Fatal("4 остальных — без ошибки")
	}
	if got := e.View(); got.Rev != v.Rev || got.MarksRev != v.MarksRev || !reflect.DeepEqual(edtest.MarkRows(e), before) {
		t.Fatal("отказ поменял отметки")
	}
	if _, err := e.PickBest(ph[0], ph[1:4]); err != nil {
		t.Fatalf("3 остальных: %v", err)
	}

	old := model.MarksLimit()
	t.Cleanup(func() { model.SetMarksLimit(old) })
	model.SetMarksLimit(2)
	e = edtest.Open(t, model.New(), "")
	v = e.View()
	if _, err := e.PickBest(ph[0], ph[1:3]); err == nil {
		t.Fatal("сверх предела отметок — без ошибки")
	}
	if got := e.View(); got.Rev != v.Rev || edtest.MarksLen(e) != 0 {
		t.Fatal("отказ по пределу поменял отметки")
	}
}

// TestPickBestXmp: поле, равное отметке из файла, своим не пишется; отличное — пишется.
func TestPickBestXmp(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	best := xp("a", model.XmpMarks{Flag: 1}) // в файле уже «отобрано»
	r1 := xp("b", model.XmpMarks{Flag: -1})  // в файле уже «отклонено»
	r2 := xp("c", model.XmpMarks{Rating: 4}) // флажка в файле нет
	pickBest(t, e, best, r1, r2)
	edtest.WantMarks(t, e, map[string][3]any{"c": {-9, "-", -1}})
	// Лучший, у которого в файле «отклонено», получает свой «отобрано».
	pickBest(t, e, r1, best)
	edtest.WantMarks(t, e, map[string][3]any{"b": {-9, "-", 1}, "c": {-9, "-", -1}, "a": {-9, "-", -1}})
}

// TestPickBestDedupe: best среди rest и повторы вычёркиваются (лучший не получает «отклонено»);
// после очистки остаётся не больше трёх — повторы ошибкой не считаются.
func TestPickBestDedupe(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ph := edtest.Photos("a", "b", "c", "d")
	rest := []model.PhotoInfo{ph[0], ph[1], ph[1], ph[2], ph[3], ph[2], ph[0]}
	if _, err := e.PickBest(ph[0], rest); err != nil {
		t.Fatal(err)
	}
	edtest.WantMarks(t, e, map[string][3]any{"a": {-9, "-", 1}, "b": {-9, "-", -1}, "c": {-9, "-", -1}, "d": {-9, "-", -1}})
	if edtest.UndoLen(e) != 1 {
		t.Fatalf("шагов %d, ждали 1", edtest.UndoLen(e))
	}
}
