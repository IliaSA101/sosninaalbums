package album_test

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Добавление отдельных фото — шаг отмены: rev растёт, dirty; Undo/Redo возвращают и снова ставят;
// холостое (всё уже есть) — ни шага, ни rev, ни dirty.
func TestAddLooseUndo(t *testing.T) {
	root := testkit.TempDir(t)
	a, b := filepath.Join(root, "a.jpg"), filepath.Join(root, "b.jpg")
	e := edtest.Open(t, model.New(), "")
	rev := edtest.Rev(e)
	v, r, err := e.AddLoose([]string{a, b}, 100)
	if err != nil || r.Added != 2 || v.Rev != rev+1 || !v.Dirty || !v.CanUndo || edtest.UndoLen(e) != 1 {
		t.Fatalf("добавление: %+v %v, rev %d → %d", r, err, rev, v.Rev)
	}
	edtest.SetDirty(e, false)
	if v, _, _ := e.AddLoose([]string{b, a}, 200); v.Rev != rev+1 || v.Dirty || edtest.UndoLen(e) != 1 {
		t.Fatalf("холостое: rev %d, dirty %v, шагов %d", v.Rev, v.Dirty, edtest.UndoLen(e))
	}
	if v := e.Undo(); len(v.Loose) != 0 || v.Rev != rev+2 || !v.CanRedo {
		t.Fatalf("Undo: %+v rev %d", v.Loose, v.Rev)
	}
	if v := e.Redo(); len(v.Loose) != 1 || v.Loose[0].ID != r.Batch || len(v.Loose[0].Files) != 2 {
		t.Fatalf("Redo: %+v", v.Loose)
	}
	v = e.RemoveLoose(r.Batch, a)
	if v = e.Undo(); len(v.Loose[0].Files) != 2 {
		t.Fatalf("Undo убирания: %+v", v.Loose)
	}
}

// После каждой операции над источниками (папки, отдельные, перепривязка, смена диска, Undo/Redo)
// инвариант держится: случайная последовательность на общем наборе путей — часть на диске, часть
// на «пропавшем диске Q» (его Relink и RelinkDrive переносят на настоящий).
func TestFoldSourcesAfterOps(t *testing.T) {
	root := testkit.TempDir(t)
	dirs := []string{root, filepath.Join(root, "a"), filepath.Join(root, "a", "b"), filepath.Join(root, "c"), filepath.Join(root, "A", "B", "d"), filepath.Join(root, "a", ".edits")}
	vol := filepath.VolumeName(root)
	from := "Q"
	if letter, _ := paths.DriveLetter(vol); letter == from {
		from = "P"
	}
	var files []string
	for _, d := range dirs {
		for i := range 3 {
			f := filepath.Join(d, fmt.Sprintf("%d.jpg", i))
			edtest.PutFile(t, f, 10)
			files = append(files, f)
		}
	}
	if vol != "" { // те же пути на «пропавшем» диске: источники там же, куда ведут Relink и RelinkDrive
		for _, d := range slices.Clone(dirs[1:]) {
			dirs = append(dirs, from+":"+d[len(vol):])
		}
		for _, f := range slices.Clone(files) {
			files = append(files, from+":"+f[len(vol):])
		}
	}
	fx := &foldOps{
		e: edtest.Open(t, model.New(), ""), rnd: rand.New(rand.NewPCG(1, 2)),
		files: files, dirs: dirs, from: from, vol: vol,
	}
	for _, f := range files[:4] { // фото альбома: их Relink переносит
		edtest.NewSpread(fx.e, -1, []model.PhotoInfo{{ID: library.IDFor(f), Path: f, W: 3, H: 2, Size: 10}})
	}
	for step := range 600 {
		what := fx.random(t, step)
		if err := edtest.SourcesInvariant(edtest.P(fx.e)); err != nil {
			t.Fatalf("шаг %d (%s): %v\nпапки %q", step, what, err, edtest.P(fx.e).Folders)
		}
	}
}

// foldOps — случайные операции над источниками для TestFoldSourcesAfterOps.
type foldOps struct {
	e           *album.Editor
	rnd         *rand.Rand
	files, dirs []string
	from, vol   string
}

func (f *foldOps) pick(list []string) string { return list[f.rnd.IntN(len(list))] }

// random выполняет одну случайную операцию и возвращает её название.
func (f *foldOps) random(t *testing.T, step int) string {
	t.Helper()
	ops := []struct {
		name string
		do   func(t *testing.T, step int)
	}{
		{"AddFolder", f.addFolder}, {"RemoveFolder", f.removeFolder},
		{"AddLoose", f.addLoose}, {"AddLoose", f.addLoose},
		{"RemoveLoose", f.removeLoose},
		{"Undo", func(*testing.T, int) { f.e.Undo() }},
		{"Redo", func(*testing.T, int) { f.e.Redo() }},
		{"Relink", f.relink}, {"RelinkDrive", f.relinkDrive},
	}
	op := ops[f.rnd.IntN(len(ops))]
	op.do(t, step)
	return op.name
}

func (f *foldOps) addFolder(t *testing.T, _ int) {
	t.Helper()
	if _, _, err := f.e.AddFolder(f.pick(f.dirs[1:])); err != nil {
		t.Fatal(err)
	}
}

func (f *foldOps) removeFolder(*testing.T, int) {
	if fs := f.e.View().Folders; len(fs) > 0 {
		f.e.RemoveFolder(f.pick(fs))
	}
}

func (f *foldOps) addLoose(t *testing.T, step int) {
	t.Helper()
	list := make([]string, 1+f.rnd.IntN(4))
	for i := range list {
		list[i] = f.pick(f.files)
	}
	if _, _, err := f.e.AddLoose(list, int64(step)); err != nil {
		t.Fatal(err)
	}
}

func (f *foldOps) removeLoose(*testing.T, int) {
	ls := f.e.View().Loose
	if len(ls) == 0 {
		return
	}
	a := ls[f.rnd.IntN(len(ls))]
	file := ""
	if f.rnd.IntN(2) == 0 {
		file = a.Files[f.rnd.IntN(len(a.Files))].Path
	}
	f.e.RemoveLoose(a.ID, file)
}

func (f *foldOps) relink(*testing.T, int) {
	moves := map[string]string{}
	for _, id := range slices.Sorted(maps.Keys(edtest.P(f.e).Photos)) {
		if f.rnd.IntN(2) == 0 {
			moves[id] = f.pick(f.files)
		}
	}
	addFolder := ""
	if f.rnd.IntN(2) == 0 {
		addFolder = f.pick(f.dirs)
	}
	f.e.Relink(moves, addFolder)
}

func (f *foldOps) relinkDrive(*testing.T, int) {
	// Без буквы диска (не Windows) тома нет: смена диска с пустой целью — пустая правка,
	// но сам вызов остаётся в смеси операций.
	to := ""
	if f.vol != "" {
		to = f.vol[:1]
	}
	f.e.RelinkDrive(f.from, to)
}

// Скрытая подпапка (A\.edits) сканер корня не читает: AddFolder при A в списке — отдельный корень,
// один шаг отмены, Undo убирает; обычная подпапка — холостая (ни шага, ни rev, ни dirty).
func TestAddFolderHiddenSeparate(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	hidden, plain := filepath.Join(a, ".edits"), filepath.Join(a, "Выбор")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, a)
	edtest.SetDirty(e, false)
	rev, n := edtest.Rev(e), edtest.UndoLen(e)
	if v, r, err := e.AddFolder(plain); err != nil || r.Within != a || r.Merged != 0 || v.Rev != rev || v.Dirty || edtest.UndoLen(e) != n {
		t.Fatalf("подпапка: %+v %v, rev %d → %d, шагов %d → %d", r, err, rev, v.Rev, n, edtest.UndoLen(e))
	}
	v, r, err := e.AddFolder(hidden)
	if err != nil || r.Within != "" || !slices.Equal(v.Folders, []string{a, hidden}) || v.Rev != rev+1 || !v.Dirty || edtest.UndoLen(e) != n+1 {
		t.Fatalf("скрытая: %+v %v, папки %q, rev %d, шагов %d", r, err, v.Folders, v.Rev, edtest.UndoLen(e))
	}
	if err := edtest.SourcesInvariant(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	if v := e.Undo(); !slices.Equal(v.Folders, []string{a}) {
		t.Fatalf("Undo: %q", v.Folders)
	}
	if v := e.Redo(); !slices.Equal(v.Folders, []string{a, hidden}) {
		t.Fatalf("Redo: %q", v.Folders)
	}
	// Корень выше обеих записей: скрытая (отдельный корень) остаётся, обычная — нет.
	if v, r, _ := e.AddFolder(root); r.Merged != 1 || !slices.Equal(v.Folders, []string{root, hidden}) {
		t.Fatalf("корень выше: %+v, папки %q", r, v.Folders)
	}
}

// Sources — копии: правка ответа не трогает проект; отдельные — по порядку добавлений.
func TestSourcesCopy(t *testing.T) {
	root := testkit.TempDir(t)
	dir, a, b := filepath.Join(root, "Март"), filepath.Join(root, "1.jpg"), filepath.Join(root, "2.jpg")
	e := edtest.Open(t, model.New(), "")
	if folders, files := e.Sources(); len(folders)+len(files) != 0 {
		t.Fatalf("пустой: %q %q", folders, files)
	}
	edtest.AddFolder(e, dir)
	_, _, _ = e.AddLoose([]string{b}, 1)
	_, _, _ = e.AddLoose([]string{a}, 2)
	folders, files := e.Sources()
	if !slices.Equal(folders, []string{dir}) || !slices.Equal(files, []string{b, a}) {
		t.Fatalf("источники: %q %q", folders, files)
	}
	folders[0], files[0] = "x", "y"
	if f2, l2 := e.Sources(); f2[0] != dir || l2[0] != b {
		t.Fatalf("ответ — не копия: %q %q", f2, l2)
	}
}

// NewEditor и Restore проверяют пути отдельных фото, как Load: UNC и сверх пределов — отказ.
func TestEditorChecksLoose(t *testing.T) {
	root := testkit.TempDir(t)
	p := model.New()
	p.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: `\\srv\share\1.jpg`}}}}
	if _, err := album.NewEditor(p, ""); err == nil {
		t.Fatal("UNC в отдельных прошёл NewEditor")
	}
	p.Loose[0].Files[0] = model.LooseFile{Path: filepath.Join(root, "1.jpg"), Alt: `\\srv\share\1.jpg`}
	if _, err := album.NewEditor(p, ""); err == nil {
		t.Fatal("UNC в запасном пути прошёл NewEditor")
	}
	p.Loose[0].Files[0].Alt = ""
	e := edtest.Open(t, model.New(), "")
	if _, err := e.Restore(p); err != nil {
		t.Fatalf("Restore годного: %v", err)
	}
	if v := e.View(); len(v.Loose) != 1 || v.Loose[0].Files[0].Path != filepath.Join(root, "1.jpg") {
		t.Fatalf("Restore: %+v", v.Loose)
	}
	q := model.New()
	for i := range model.MaxLooseAdds + 1 {
		q.Loose = append(q.Loose, model.LooseAdd{ID: fmt.Sprintf("%012x", i), Files: []model.LooseFile{{Path: filepath.Join(root, fmt.Sprint(i, ".jpg"))}}})
	}
	if _, err := e.Restore(q); err == nil {
		t.Fatal("сверх MaxLooseAdds прошло Restore")
	}
}
