package collect

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// collectRefs — сбор с привязками refs (как App: SnapshotSideRefs → Album → Save) и Load записанного.
func collectRefs(t *testing.T, sc *scene, refs []string) *model.Project {
	t.Helper()
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, err := Make(sc.p, nil, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatal(err)
	}
	q, side := Album(sc.p, model.Side{Marks: sc.marks}, dir, pl, refs)
	path := filepath.Join(dir, "Иванов.salbum")
	if err := Save(q, side, path, pl); err != nil {
		t.Fatal(err)
	}
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return lp
}

// Сбор сохраняет отпечаток (167-9) и после Load привязки на месте, в прежнем порядке; исходный
// проект сбор не трогает.
func TestCollectKeepsLinks(t *testing.T) {
	sc := newScene(t)
	uid := sc.p.UID
	refs := []string{model.LinkRef(uid, "alb-1"), model.LinkRef(uid, "alb-2")}
	lp := collectRefs(t, sc, refs)
	if lp.UID != uid {
		t.Errorf("отпечаток копии %q, исходного %q", lp.UID, uid)
	}
	if got := lp.ShareRefs(); !reflect.DeepEqual(got, refs) {
		t.Errorf("привязки собранной копии %v, ждали %v", got, refs)
	}
	if got := sc.p.ShareRefs(); len(got) != 0 {
		t.Errorf("сбор изменил привязки исходного проекта: %v", got)
	}
}

// Отпечаток сохраняется и без привязок; негодные привязки в файл не попадают.
func TestCollectKeepsUID(t *testing.T) {
	sc := newScene(t)
	lp := collectRefs(t, sc, nil)
	if lp.UID != sc.p.UID || !model.GoodUID(lp.UID) {
		t.Errorf("отпечаток копии %q, исходного %q", lp.UID, sc.p.UID)
	}
	if got := lp.ShareRefs(); len(got) != 0 {
		t.Errorf("привязки без источника: %v", got)
	}
	lp = collectRefs(t, newScene(t), []string{"не-привязка", strings.Repeat("A", 32)})
	if got := lp.ShareRefs(); len(got) != 0 {
		t.Errorf("негодные привязки легли в файл: %v", got)
	}
}
