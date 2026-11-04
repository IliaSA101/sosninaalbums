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

// Запись о ссылке пути не переходит к другому проекту, который лёг на этот путь (146, С1): App
// сообщает об этом службе при «Сохранить как» и при сборе поверх альбома.

// shareStore — файл записей о выгрузках папки данных a: тест читает и пишет его мимо службы.
func shareStore(a *App) *prefs.Shares {
	return prefs.NewShares(a.cfgDir, prefs.ShareRules{
		ServiceURL: func(raw string) (string, error) { return share.ServiceURL(raw, true) },
		CheckLink:  printprof.CheckURL,
	})
}

func putShareRecord(t *testing.T, a *App, album string) {
	t.Helper()
	rec := prefs.ShareRecord{Origin: "https://albums.example.org", ID: "LiZ9u4CAHMYsK8nxXrvchR", Title: "Альбом",
		URL: "https://albums.example.org/a/LiZ9u4CAHMYsK8nxXrvchR/", ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := shareStore(a).Put(album, rec); err != nil {
		t.Fatal(err)
	}
}

func hasShareRecord(a *App, album string) bool {
	_, ok := shareStore(a).Get(album)
	return ok
}

func TestSaveAsResetsShareRecord(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	other := filepath.Join(testkit.TempDir(t), "другой"+projectExt)
	if err := os.WriteFile(other, []byte(`{"spreads":[{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	putShareRecord(t, a, file)
	putShareRecord(t, a, other)
	if _, err := a.saveAs(a.editor(), other); err != nil { // на путь other ложится другой проект
		t.Fatal(err)
	}
	if hasShareRecord(a, other) {
		t.Error("запись прежнего файла пережила «Сохранить как» поверх него")
	}
	if !hasShareRecord(a, file) {
		t.Error("запись исходного файла потеряна: он не менялся")
	}
	// Тот же путь снова: проект тот же, запись нового проекта сохраняется.
	putShareRecord(t, a, other)
	if _, err := a.saveAs(a.editor(), other); err != nil {
		t.Fatal(err)
	}
	if !hasShareRecord(a, other) {
		t.Error("«Сохранить как» в тот же файл сбросило запись")
	}
}

func TestCollectResetsShareRecord(t *testing.T) {
	e := newCollectEnv(t, 2)
	e.a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups")) // замена альбома требует копии
	if err := os.MkdirAll(e.book(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := salbum.Write(model.New(), e.albumFile(), "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	putShareRecord(t, e.a, e.albumFile())
	putShareRecord(t, e.a, e.file)
	if _, msg, _, _ := e.run(t, false); msg != "" {
		t.Fatalf("сбор: %q", msg)
	}
	if hasShareRecord(e.a, e.albumFile()) {
		t.Error("запись прежнего альбома пережила сбор поверх него")
	}
	if !hasShareRecord(e.a, e.file) {
		t.Error("запись открытого альбома потеряна: сбор его не меняет")
	}
}
