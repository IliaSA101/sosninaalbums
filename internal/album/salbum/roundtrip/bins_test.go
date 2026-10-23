package roundtrip_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Подборки (083) через Editor: открыть, сохранить, черновик, снимок для сбора.

// binsAlbum — файл v15 в dir: фото a, b; подборки «Обязательно», «Запас»; a — с оценкой в обеих,
// b — только в «Запасе».
func binsAlbum(t *testing.T, dir string) (string, model.Side) {
	t.Helper()
	a, b := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg")
	p := model.New()
	p.Photos[library.IDFor(a)] = model.PhotoRef{Path: a, W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{library.IDFor(a)}}}
	three := 3
	side := model.Side{
		Bins: []model.Bin{{ID: "b2", Name: "Обязательно"}, {ID: "b1", Name: "Запас"}},
		Marks: map[string]model.Mark{
			library.IDFor(a): {Path: a, Rating: &three, Bins: []string{"b1", "b2"}},
			library.IDFor(b): {Path: b, Bins: []string{"b1"}},
		},
	}
	path := filepath.Join(dir, "a.salbum")
	if err := salbum.SaveWith(p, path, side); err != nil {
		t.Fatal(err)
	}
	return path, side
}

// Editor не теряет подборки: Save, «Сохранить как», черновик (WriteCopy, WriteDraft) пишут их,
// Load возвращает те же; SnapshotSide — копия без общих данных с Editor.
func TestEditorKeepsBins(t *testing.T) {
	dir := testkit.TempDir(t)
	path, side := binsAlbum(t, dir)
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, p, path)
	other, draft, draft2 := filepath.Join(dir, "b.salbum"), filepath.Join(dir, "d.salbum"), filepath.Join(dir, "d2.salbum")
	for name, write := range map[string]func() error{
		path:   func() error { return e.Save("") },
		other:  func() error { return e.Save(other) },
		draft:  func() error { return e.WriteCopy(draft) },
		draft2: func() error { return e.WriteDraft(draft2) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
		q, err := salbum.Load(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := edtest.Loaded(q); !reflect.DeepEqual(got, side) {
			t.Fatalf("%s: после записи Editor\n%+v\nждали\n%+v", filepath.Base(name), got, side)
		}
	}
	_, snap := e.SnapshotSide()
	if !reflect.DeepEqual(snap, side) {
		t.Fatalf("снимок: %+v", snap)
	}
	snap.Bins[0].Name = "x"
	snap.Marks[library.IDFor(filepath.Join(dir, "a.jpg"))].Bins[0] = "x"
	if _, again := e.SnapshotSide(); !reflect.DeepEqual(again, side) {
		t.Fatalf("снимок связан с Editor: %+v", again)
	}
	// p (из Load) не связан с Editor: правка его стороны Editor не трогает.
	edtest.Loaded(p).Bins[0].Name = "y"
	if _, again := e.SnapshotSide(); again.Bins[0].Name != "Обязательно" {
		t.Fatalf("Editor делит подборки с p: %+v", again.Bins)
	}
}

// v14 (0.20) с отметками открывается как v15 без потерь: первая запись Editor делает .v14.bak с
// байтами исходника (вторая её не трогает), подборок нет, отметки те же; черновик v14 — так же;
// v16 — ErrNewer.
func TestLoadV14Bak(t *testing.T) {
	oldVersionOpens(t, 14)
	dir := testkit.TempDir(t)
	src, side := v14Source(t, dir)
	v14Opens(t, filepath.Join(dir, "album.salbum"), src, side, false)
	v14Opens(t, filepath.Join(dir, "draft.salbum"), src, side, true)
}

// v14Source — файл v14 с отметками (тот же альбом binsAlbum без подборок) и его отметки.
func v14Source(t *testing.T, dir string) ([]byte, model.Side) {
	t.Helper()
	path, side := binsAlbum(t, dir)
	side.Bins = nil
	for id, m := range side.Marks {
		if m = m.WithBins(nil); m.None() {
			delete(side.Marks, id)
		} else {
			side.Marks[id] = m
		}
	}
	p0, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := salbum.SaveWith(p0, path, side); err != nil {
		t.Fatal(err)
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := bytes.Replace(cur, fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), []byte(`"version": 14,`), 1)
	if bytes.Equal(src, cur) || bytes.Contains(src, []byte(`"bins"`)) || len(side.Marks) == 0 {
		t.Fatal("заготовка v14")
	}
	return src, side
}

// v14Opens — файл v14 src в file открывается с отметками side; две записи Editor (draft —
// черновиком WriteDraft, иначе Save) оставляют .v14.bak с байтами исходника и пишут v15.
func v14Opens(t *testing.T, file string, src []byte, side model.Side, draft bool) {
	t.Helper()
	if err := os.WriteFile(file, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if edtest.From(p) != 14 || !reflect.DeepEqual(edtest.Loaded(p), side) {
		t.Fatalf("%s: from %d, сторона %+v", file, edtest.From(p), edtest.Loaded(p))
	}
	e := edtest.Open(t, p, file)
	write := func() error { return e.Save("") }
	if draft {
		e = edtest.Open(t, p, "")
		write = func() error { return e.WriteDraft(file) }
	}
	for range 2 {
		if err := write(); err != nil {
			t.Fatal(err)
		}
		if bak, err := os.ReadFile(salbum.BackupName(file, 14, 1)); err != nil || !bytes.Equal(bak, src) {
			t.Fatalf("%s: копия для 0.20: %v, совпадает с исходником: %v", file, err, bytes.Equal(bak, src))
		}
	}
	q, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, file); f.Version != model.FormatVersion || edtest.From(q) != model.FormatVersion || !reflect.DeepEqual(edtest.Loaded(q), side) {
		t.Fatalf("%s: версия %d, отметки %+v", file, f.Version, edtest.Loaded(q))
	}
}
