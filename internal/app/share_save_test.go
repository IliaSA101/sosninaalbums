package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Записи о ссылках лежат по uid проекта (167-4, 167-10): «Сохранить как» и сбор поверх файла их не
// стирают — другой проект на пути имеет другой uid.

// shareStore — projects.json папки данных a: тест читает и пишет его мимо службы.
func shareStore(a *App) *prefs.Projects {
	return prefs.NewProjects(a.cfgDir, prefs.ShareRules{
		ServiceURL: func(raw string) (string, error) { return share.ServiceURL(raw, true) },
		CheckLink:  printprof.CheckURL,
	})
}

// putProjectRecord кладёт запись о ссылке проекта uid, привязанную к файлу album.
func putProjectRecord(t *testing.T, a *App, uid, album string) {
	t.Helper()
	rec := prefs.ProjectRecord{Origin: "https://albums.example.org", ID: "LiZ9u4CAHMYsK8nxXrvchR", Title: "Альбом",
		URL: "https://albums.example.org/a/LiZ9u4CAHMYsK8nxXrvchR/", ExpiresAt: time.Now().Add(24 * time.Hour), Path: album}
	if err := shareStore(a).Put(uid, rec); err != nil {
		t.Fatal(err)
	}
}

func hasShareRecord(a *App, uid string) bool {
	_, ok := shareStore(a).Get(uid)
	return ok
}

func TestSaveAsKeepsShareRecord(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	other := filepath.Join(testkit.TempDir(t), "другой"+projectExt)
	if err := os.WriteFile(other, []byte(`{"spreads":[{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldUID := mustUID(t, a)
	putProjectRecord(t, a, oldUID, file)
	if _, err := a.saveAs(a.editor(), other); err != nil { // на путь other ложится другой проект
		t.Fatal(err)
	}
	if !hasShareRecord(a, oldUID) {
		t.Error("запись исходного проекта стёрта: она по uid и не относится к файлу other")
	}
	if _, uid := a.editor().Ident(); uid == oldUID {
		t.Error("другой проект получил прежний uid")
	}
	if hasLink(t, a) {
		t.Error("новый проект видит ссылку прежнего")
	}
}

func TestCollectKeepsShareRecord(t *testing.T) {
	e := newCollectEnv(t, 2)
	e.a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups")) // замена альбома требует копии
	if err := os.MkdirAll(e.book(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := salbum.Write(model.New(), e.albumFile(), "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	uid := mustUID(t, e.a)
	putProjectRecord(t, e.a, uid, e.file)
	if _, msg, _, _ := e.run(t, false); msg != "" {
		t.Fatalf("сбор: %q", msg)
	}
	if !hasShareRecord(e.a, uid) {
		t.Error("запись открытого альбома потеряна: сбор его не меняет")
	}
}
