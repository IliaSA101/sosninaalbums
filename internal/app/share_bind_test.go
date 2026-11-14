package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167: привязка к выгруженному альбому ложится в файл ближайшим автосохранением (167-15), при
// открытии своим путём — из записи (167-24), «Новый альбом из шаблона…» привязок не наследует.

// fileRefs — shareRefs, лежащие в файле path (мимо редактора).
func fileRefs(t *testing.T, path string) []string {
	t.Helper()
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return p.ShareRefs()
}

func TestBindShareWrittenByAutosave(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	ref := model.LinkRef(uid, "alb-1")
	if err := a.bindShare(file, uid, "alb-1"); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, file); len(got) != 0 {
		t.Fatalf("привязка легла в файл мимо автосохранения: %v", got)
	}
	a.autosave()
	if got := fileRefs(t, file); !slices.Equal(got, []string{ref}) {
		t.Fatalf("после автосохранения: %v", got)
	}
	if v := a.editor().View(); v.Dirty {
		t.Error("привязка стала правкой (dirty)")
	}
	if err := a.bindShare(file, model.NewUID(), "alb-1"); err == nil {
		t.Error("привязка к чужому uid принята")
	}
	a.saveTimer.Stop()
}

func TestBindRespectsHoldSave(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	a.holdSave.Store(a.editor())
	if err := a.bindShare(file, uid, "alb-1"); err != nil {
		t.Fatal(err)
	}
	a.autosave()
	if got := fileRefs(t, file); len(got) != 0 {
		t.Fatalf("запись при запрете holdSave: %v", got)
	}
	if _, err := a.pinShare(file, uid, true); err == nil {
		t.Error("pinShare fresh писал при запрете holdSave")
	}
	a.saveTimer.Stop()
}

func TestBindAfterStop(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	a.stopped.Store(true)
	if err := a.bindShare(file, uid, "alb-1"); err != nil {
		t.Fatal(err)
	}
	a.autosave()
	if got := fileRefs(t, file); len(got) != 0 {
		t.Fatalf("запись после выхода: %v", got)
	}
	if _, err := a.pinShare(file, uid, true); err == nil {
		t.Error("pinShare писал после выхода")
	}
	a.saveTimer.Stop()
}

func TestOpenKeepsLinkRef(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	if _, err := a.pinShare(file, uid, false); err != nil {
		t.Fatal(err)
	}
	putProjectRecord(t, a, uid, file) // запись 0.37: привязки в файле нет
	a.ed.Store(mustEditor(t, model.New(), ""))
	if _, err := a.openPath(file); err != nil {
		t.Fatal(err)
	}
	want := model.LinkRef(uid, "LiZ9u4CAHMYsK8nxXrvchR")
	if got := a.editor().ShareRefs(); !slices.Equal(got, []string{want}) {
		t.Fatalf("привязка не взята из записи: %v", got)
	}
	if v := a.editor().View(); v.Dirty {
		t.Error("KeepRefs сделал правку")
	}
	if got := fileRefs(t, file); len(got) != 0 {
		t.Fatalf("привязка легла в файл при открытии, до записи: %v", got)
	}
	a.autosave() // ближайшая запись кладёт привязку в файл (167-24)
	if got := fileRefs(t, file); !slices.Equal(got, []string{want}) {
		t.Fatalf("после записи в файле: %v", got)
	}
}

func TestOpenKeepsLinkRefOwnPathOnly(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	if _, err := a.pinShare(file, uid, false); err != nil {
		t.Fatal(err)
	}
	putProjectRecord(t, a, uid, filepath.Join(testkit.TempDir(t), "Другой"+projectExt)) // запись другого пути
	a.ed.Store(mustEditor(t, model.New(), ""))
	if _, err := a.openPath(file); err != nil {
		t.Fatal(err)
	}
	if got := a.editor().ShareRefs(); len(got) != 0 {
		t.Fatalf("копия получила привязку к чужому альбому: %v", got)
	}
}

func TestNewFromTemplateFresh(t *testing.T) {
	a, file, _ := backupApp(t, 2)
	uid := mustUID(t, a)
	if err := a.bindShare(file, uid, "alb-1"); err != nil {
		t.Fatal(err)
	}
	a.autosave()
	a.saveTimer.Stop()
	if len(fileRefs(t, file)) != 1 {
		t.Fatal("оснастка: в шаблоне нет привязки")
	}
	// Открыт другой альбом; шаблон — файл с привязкой и ссылкой.
	a.ed.Store(mustEditor(t, model.New(), ""))
	before, _ := os.ReadFile(file)
	st, err := a.newFromTemplate(file)
	if err != nil {
		t.Fatal(err)
	}
	_, nu := a.editor().Ident()
	if !model.GoodUID(nu) || nu == uid || a.editor().Path() != "" || len(a.editor().ShareRefs()) != 0 {
		t.Fatalf("шаблон: uid %q (шаблона %q), путь %q, привязки %v", nu, uid, a.editor().Path(), a.editor().ShareRefs())
	}
	if len(st.View.Spreads) != 2 {
		t.Errorf("вёрстка шаблона не взята: разворотов %d", len(st.View.Spreads))
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Error("шаблон изменён: он читается только на чтение")
	}
}

func TestNewFromTemplateNewer(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	newer := newerAlbum(t, testkit.TempDir(t), "Новый", "99.0.0")
	cur := a.editor()
	st, err := a.newFromTemplate(newer)
	if err != nil || !st.Cancelled || st.Newer == nil {
		t.Fatalf("файл новее: %v %+v", err, st.Newer)
	}
	if a.editor() != cur {
		t.Error("текущий альбом заменён")
	}
}
