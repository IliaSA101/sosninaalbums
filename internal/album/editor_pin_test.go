package album

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167 T3: запись отпечатка при выгрузке без печати пути; «выгрузить как новый»; «Сохранить как»
// поверх файла под одним замком.

func TestNeedFileUIDIgnoresSeal(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := uidEditor(t, model.New(), path) // файл до 0.32: отпечатка на диске нет
	path, uid := e.Ident()
	if need, err := e.NeedFileUID(path, uid); !need || err != nil {
		t.Fatalf("до записи: %v %v", need, err)
	}
	got, err := e.SaveFileUID(path, uid)
	if err != nil || got != uid || fileUID(t, path) != uid {
		t.Fatalf("SaveFileUID: %q %v, в файле %q", got, err, fileUID(t, path))
	}
	if need, _ := e.NeedFileUID(path, uid); need {
		t.Error("отпечаток записан, а запись снова нужна")
	}
	if _, err := e.SaveFileUID(path, model.NewUID()); err == nil {
		t.Error("отпечаток другого проекта принят")
	}
}

func TestSaveFreshDropsRefs(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := uidEditor(t, model.New(), path)
	path, uid := e.Ident()
	if err := e.SetRef(path, uid, model.LinkRef(uid, "alb")); err != nil {
		t.Fatal(err)
	}
	nu, err := e.SaveFresh(path, uid)
	if err != nil || nu == uid || !model.GoodUID(nu) || fileUID(t, path) != nu {
		t.Fatalf("SaveFresh: %q %v", nu, err)
	}
	if refs := e.ShareRefs(); len(refs) != 0 {
		t.Errorf("привязки перешли к новому проекту: %v", refs)
	}
	if _, err := e.SaveFresh(path, uid); err == nil {
		t.Error("устаревший отпечаток принят")
	}
}

func TestSaveOverSameUIDKeeps(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, model.New(), a)
	a, uid := e.Ident()
	if err := e.SetRef(a, uid, model.LinkRef(uid, "alb")); err != nil {
		t.Fatal(err)
	}
	if fresh, err := e.SaveOver(a); fresh || err != nil { // тот же путь
		t.Fatalf("тот же путь: %v %v", fresh, err)
	}
	data, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(dir, "cp.salbum") // копия с тем же отпечатком
	if err := os.WriteFile(cp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if fresh, err := e.SaveOver(cp); fresh || err != nil {
		t.Fatalf("поверх файла с тем же отпечатком: %v %v", fresh, err)
	}
	if _, now := e.Ident(); now != uid || len(e.ShareRefs()) != 1 {
		t.Errorf("отпечаток %q, привязки %v", now, e.ShareRefs())
	}
	other := filepath.Join(dir, "other.salbum") // файла нет: другой проект
	if fresh, err := e.SaveOver(other); !fresh || err != nil {
		t.Fatalf("другой проект: %v %v", fresh, err)
	}
	if _, now := e.Ident(); now == uid || len(e.ShareRefs()) != 0 || fileUID(t, other) != now {
		t.Errorf("новый проект: %q, привязки %v", now, e.ShareRefs())
	}
}
