package collect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Рамочки мест (079): своя рамочка собирается в «Свои фоны и стикеры», и при повторном сборе —
// тоже (079-27), а не в «Фото/Прочие» (ленту).

const frameKey = "my:fedcba9876543210"

// borderScene — сцена сбора плюс лист из пустых мест: два с одной своей рамочкой, одно со встроенной.
func borderScene(t *testing.T) *scene {
	t.Helper()
	sc := newScene(t)
	own := filepath.Join(sc.root, "Библиотека", "fedcba9876543210.png")
	put(t, own, "frame-png")
	edtest.LibFinder(t, map[string]string{libKey: filepath.Join(sc.root, "Библиотека", "0123456789abcdef.png"), frameKey: own})
	id := model.LibPhotoID(frameKey)
	sc.p.Photos[id] = model.PhotoRef{Lib: frameKey, Path: own, W: 300, H: 400, Size: 9}
	cut := [4]float64{0.1, 0.1, 0.1, 0.2}
	sc.p.Spreads = append(sc.p.Spreads, &model.Spread{ID: "0123456789a5", Photos: []string{}, Grid: []*model.Place{
		{ID: "0123456789b1", X: 10, Y: 10, W: 60, H: 80, Border: &model.Border{Photo: id, Cut: cut}},
		{ID: "0123456789b2", X: 100, Y: 10, W: 60, H: 80, Border: &model.Border{Photo: id, Cut: cut, Scale: 1.5}},
		{ID: "0123456789b3", X: 200, Y: 10, W: 60, H: 80, Border: &model.Border{Kind: "line", Width: 2}},
	}})
	if err := edtest.GridInvariants(sc.p); err != nil {
		t.Fatal(err)
	}
	return sc
}

// frameRefs — записи рамочек нового листа (последнего) после Load: ID обеих картинок и их путь.
func frameRefs(t *testing.T, p *model.Project) (string, model.PhotoRef) {
	t.Helper()
	if err := edtest.GridInvariants(p); err != nil {
		t.Fatal(err)
	}
	g := p.Spreads[len(p.Spreads)-1].Grid
	if len(g) != 3 || g[0].Border == nil || g[1].Border == nil || g[2].Border == nil || g[2].Border.Kind != "line" {
		t.Fatalf("места с рамочками: %+v", g)
	}
	if g[0].Border.Photo != g[1].Border.Photo || g[1].Border.Scale != 1.5 || g[0].Border.Cut != [4]float64{0.1, 0.1, 0.1, 0.2} {
		t.Fatalf("рамочки: %+v %+v", g[0].Border, g[1].Border)
	}
	return g[0].Border.Photo, p.Photos[g[0].Border.Photo]
}

// TestCollectBorder — сбор → Load → перенос папки → Load: своя рамочка — файл в «Свои фоны и
// стикеры», запись без lib с ID по пути; после переноса Rekey переводит рамочки обоих мест.
func TestCollectBorder(t *testing.T) {
	sc := borderScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, path := collectTo(t, sc, dir, false)
	want := filepath.Join(dir, DirOwn, "fedcba9876543210.png")
	if st, err := os.Stat(want); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("файл рамочки: %v", err)
	}
	if in := pl.Info(); in.Own != 2 {
		t.Fatalf("своих в сводке %d, ждали 2 (стикер и рамочка): %+v", in.Own, in)
	}
	salbum.SetLibFinder(nil) // другой компьютер: библиотеки нет
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	id, ref := frameRefs(t, lp)
	if ref.Lib != "" || ref.Path != want || id != library.IDFor(want) || ref.W != 300 {
		t.Fatalf("рамочка %s → %+v", id, ref)
	}
	moved := filepath.Join(sc.root, "Другой ПК", "Иванов")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	mp, err := salbum.Load(filepath.Join(moved, "Иванов.salbum"))
	if err != nil {
		t.Fatal(err)
	}
	mid, mref := frameRefs(t, mp)
	if nw := filepath.Join(moved, DirOwn, "fedcba9876543210.png"); mref.Path != nw || mid != library.IDFor(nw) {
		t.Fatalf("после переноса: %s → %+v", mid, mref)
	}
}

// TestCollectTwiceBorder — повторный сбор собранного альбома (079-27): записи рамочки и своего
// стикера без lib — в «Свои фоны и стикеры», в «Фото/Прочие» (ленту) не попадают.
func TestCollectTwiceBorder(t *testing.T) {
	sc := borderScene(t)
	_, path := collectTo(t, sc, filepath.Join(sc.root, "Флешка", "Иванов"), false)
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	again := &scene{root: sc.root, p: lp, marks: edtest.Loaded(lp)}
	dir := filepath.Join(sc.root, "Флешка2", "Иванов")
	pl, path2 := collectTo(t, again, dir, false)
	for _, name := range []string{"fedcba9876543210.png", "0123456789abcdef.png"} {
		if _, err := os.Stat(filepath.Join(dir, DirOwn, name)); err != nil {
			t.Errorf("%s не в «%s»: %v", name, DirOwn, err)
		}
		if _, err := os.Stat(filepath.Join(dir, DirPhotos, DirOther, name)); err == nil {
			t.Errorf("%s попал в ленту: %s/%s", name, DirPhotos, DirOther)
		}
	}
	if in := pl.Info(); in.Own != 2 {
		t.Fatalf("своих в сводке %d, ждали 2: %+v", in.Own, in)
	}
	rp, err := salbum.Load(path2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ref := frameRefs(t, rp); ref.Path != filepath.Join(dir, DirOwn, "fedcba9876543210.png") {
		t.Fatalf("рамочка после второго сбора: %+v", ref)
	}
}
