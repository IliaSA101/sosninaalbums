package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Привязки к выгруженным альбомам при «Сохранить как» (167-1, 167-25): другой проект — новый
// отпечаток без привязок; поверх файла с тем же отпечатком — тот же проект, отпечаток и привязки
// сохраняются; поверх файла с другим отпечатком — новый проект.
func TestSaveAsOverSameUIDKeeps(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	ed := a.editor()
	uid, err := a.pinShare(file, mustUID(t, a), false)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.LinkRef(uid, "alb-1")
	if err := ed.SetRef(file, uid, ref); err != nil {
		t.Fatal(err)
	}
	if err := ed.Save(""); err != nil {
		t.Fatal(err)
	}
	want := []string{ref}
	dir := testkit.TempDir(t)

	// Копия в Проводнике (тот же отпечаток) → «Сохранить как» поверх копии: тот же проект.
	cp := filepath.Join(dir, "Копия"+projectExt)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveAs(ed, cp); err != nil {
		t.Fatal(err)
	}
	if _, got := ed.Ident(); got != uid {
		t.Fatalf("поверх файла с тем же отпечатком отпечаток сменился: %q -> %q", uid, got)
	}
	if got := fileRefsOf(t, cp); !reflect.DeepEqual(got, want) {
		t.Fatalf("привязки в записанном файле %v, ждали %v", got, want)
	}

	// Файл с другим отпечатком: новый проект, привязок нет.
	foreign := filepath.Join(dir, "Чужой"+projectExt)
	other := model.New()
	other.UID = model.NewUID()
	if err := salbum.Save(other, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveAs(ed, foreign); err != nil {
		t.Fatal(err)
	}
	if _, got := ed.Ident(); got == uid || got == other.UID || !model.GoodUID(got) {
		t.Fatalf("поверх чужого файла отпечаток %q (прежний %q, чужой %q)", got, uid, other.UID)
	}
	if got := fileRefsOf(t, foreign); len(got) != 0 {
		t.Fatalf("привязки новому проекту: %v", got)
	}
	if got := fileRefsOf(t, cp); !reflect.DeepEqual(got, want) {
		t.Fatalf("прежний файл потерял привязки: %v", got)
	}

	// Новый файл (нет цели): новый проект.
	if _, err := a.saveAs(ed, filepath.Join(dir, "Новый"+projectExt)); err != nil {
		t.Fatal(err)
	}
	if got := ed.ShareRefs(); len(got) != 0 {
		t.Errorf("привязки у нового проекта: %v", got)
	}
}

func mustUID(t *testing.T, a *App) string {
	t.Helper()
	_, uid := a.editor().Ident()
	if !model.GoodUID(uid) {
		t.Fatalf("отпечаток %q", uid)
	}
	return uid
}

func fileRefsOf(t *testing.T, path string) []string {
	t.Helper()
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return p.ShareRefs()
}
