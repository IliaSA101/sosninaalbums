package collect

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// «Собрать в папку» переносит подборки (083-12): список целиком, членство — с отметкой фото
// (новый путь у собранного, прежний у пропавшего); фото вне сбора теряют его вместе с отметкой.
// Со всей лентой — и у неиспользованного фото папки. Исходные side и проект не меняются.
func TestCollectBins(t *testing.T) {
	for _, all := range []bool{false, true} {
		sc, side, member := binScene(t)
		before := side.Clone()
		dir := filepath.Join(sc.root, "Сбор")
		pl := planCopy(t, sc, dir, all)
		q, out := Album(sc.p, side, dir, pl)
		if !slices.Equal(out.Bins, side.Bins) {
			t.Fatalf("подборки копии: %v", out.Bins)
		}
		out.Bins[0].Name = "x"
		if side.Bins[0].Name != "Обязательно" {
			t.Fatal("копия делит список подборок с исходником")
		}
		out.Bins[0].Name = "Обязательно"
		path := filepath.Join(dir, "Иванов.salbum")
		if err := Save(q, out, path, pl); err != nil {
			t.Fatal(err)
		}
		want := map[string][]string{
			filepath.Join(dir, DirPhotos, "JPG", "IMG_0001.jpg"): member[filepath.Join(sc.a, "IMG_0001.jpg")],
			sc.gone: member[sc.gone],
		}
		if all {
			want[filepath.Join(dir, DirPhotos, "JPG", "unused.jpg")] = member[sc.unused]
		}
		wantCollected(t, path, side.Bins, want)
		if !reflectEqualSide(side, before) || len(sc.marks) != len(before.Marks) {
			t.Fatal("сбор поменял исходные отметки или подборки")
		}
	}
}

// binScene — сцена newScene с подборками: у всех отметок «Обязательно», у неиспользованного и
// пропавшего фото — ещё «Запас»; member — путь источника → подборки.
func binScene(t *testing.T) (*scene, model.Side, map[string][]string) {
	t.Helper()
	sc := newScene(t)
	member := map[string][]string{}
	for id, m := range sc.marks {
		ids := []string{"b1"}
		if m.Path == sc.unused || m.Path == sc.gone {
			ids = []string{"b1", "b2"}
		}
		sc.marks[id] = m.WithBins(ids)
		member[m.Path] = ids
	}
	return sc, model.Side{Marks: sc.marks, Bins: []model.Bin{{ID: "b1", Name: "Обязательно"}, {ID: "b2", Name: "Запас"}}}, member
}

// planCopy — план сбора в dir (all — со всей лентой) и копирование файлов.
func planCopy(t *testing.T, sc *scene, dir string, all bool) Plan {
	t.Helper()
	var extra []string
	if all {
		extra = sc.extra
	}
	pl, err := Make(sc.p, extra, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatal(err)
	}
	return pl
}

// wantCollected — собранный альбом path: подборки bins, отметки ровно по want (путь → подборки).
func wantCollected(t *testing.T, path string, bins []model.Bin, want map[string][]string) {
	t.Helper()
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := edtest.Loaded(lp)
	if !slices.Equal(got.Bins, bins) || len(got.Marks) != len(want) {
		t.Fatalf("собранный: подборки %v, отметки %+v", got.Bins, got.Marks)
	}
	for _, m := range got.Marks {
		if w, ok := want[m.Path]; !ok || !slices.Equal(m.Bins, w) {
			t.Errorf("%s — подборки %v, ждали %v", m.Path, m.Bins, w)
		}
	}
}

func reflectEqualSide(a, b model.Side) bool {
	if !slices.Equal(a.Bins, b.Bins) || len(a.Marks) != len(b.Marks) {
		return false
	}
	for id, m := range a.Marks {
		if o, ok := b.Marks[id]; !ok || !model.SameMark(m, o) {
			return false
		}
	}
	return true
}
