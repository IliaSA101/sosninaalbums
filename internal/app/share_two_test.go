package app

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/sharer"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/secret"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167 T3b: сквозной тест «два компьютера» на уровне App. Служба выгрузки — настоящая, сервис —
// поддельный сервер (httptest); у каждого «компьютера» свой App, своя папка данных и кэш.

// shareComputer — App со своей папкой данных (projects.json пуст), памятью вместо хранилища
// ключей и службой выгрузки, которой разрешён loopback поддельного сервера. done — итоги share:done.
func shareComputer(t *testing.T) (a *App, done chan string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	a.secrets = secret.NewMemory()
	cache := testkit.TempDir(t)
	h := a.shareHost()
	h.AllowLoopback = true
	h.CacheDir = func() string { return cache }
	a.shr = sharer.New(h)
	done = make(chan string, 8)
	a.emit = func(name string, data ...any) {
		if name == "share:done" {
			b, _ := json.Marshal(data[0])
			done <- string(b)
		}
	}
	return a, done
}

// shareAndWait — «Выгрузить» и ожидание share:done (по условию, не по сроку); итог без ошибки.
func shareAndWait(t *testing.T, a *App, done chan string, o ShareOptions) {
	t.Helper()
	if ok, err := a.Share(o); err != nil || !ok {
		t.Fatalf("Share: %v %v", ok, err)
	}
	got, ok := testkit.WaitCh(t, done, testkit.Limit(30*time.Second), "share:done", nil)
	if !ok {
		t.Fatal("share:done не пришло")
	}
	if got != `{"err":"","stopped":false}` {
		t.Fatalf("итог выгрузки: %s", got)
	}
}

func TestShareTwoComputers(t *testing.T) {
	srv := albumstest.New(albumstest.Options{})
	t.Cleanup(srv.Close)

	// Компьютер 1: выгрузка, привязка ложится в файл автосохранением.
	a, doneA := shareComputer(t)
	file := filepath.Join(testkit.TempDir(t), "Ивановы"+projectExt)
	ed := mustEditor(t, model.New(), "")
	for range 2 {
		if _, err := ed.NewSpread(-1, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.Save(file); err != nil {
		t.Fatal(err)
	}
	a.ed.Store(ed)
	uid := mustUID(t, a)
	if _, err := a.ShareConnect(srv.URL, srv.Token); err != nil {
		t.Fatal(err)
	}
	shareAndWait(t, a, doneA, ShareOptions{Days: 30})
	a.autosave()
	a.saveTimer.Stop()
	v, err := a.ShareState()
	if err != nil || v.Link == nil || v.Link.State != "ok" || !v.Link.Own || srv.Fake.Albums() != 1 {
		t.Fatalf("после выгрузки: %v %+v, альбомов %d", err, v.Link, srv.Fake.Albums())
	}
	refs := fileRefsOf(t, file)
	if len(refs) != 1 || a.editor().View().Dirty {
		t.Fatalf("привязка в файле: %v, dirty %v", refs, a.editor().View().Dirty)
	}
	url := v.Link.URL

	// Компьютер 2: файл тот же, projects.json пуст — записи о ссылке нет.
	b, doneB := shareComputer(t)
	if _, err := b.openPath(file); err != nil {
		t.Fatal(err)
	}
	if got := mustUID(t, b); got != uid {
		t.Fatalf("отпечаток при открытии: %q, был %q", got, uid)
	}
	if _, err := b.ShareConnect(srv.URL, srv.Token); err != nil {
		t.Fatal(err)
	}
	vb, err := b.ShareState()
	if err != nil || vb.Link != nil || vb.Album.Find != "need" {
		t.Fatalf("до поиска: %v link %+v find %q", err, vb.Link, vb.Album.Find)
	}
	vb, err = b.ShareFind()
	if err != nil || vb.Link == nil || vb.Link.State != "moved" || vb.Link.Own || vb.Link.URL != url {
		t.Fatalf("ShareFind: %v %+v", err, vb.Link)
	}
	// «Обновить этот альбом»: тот же id на сервисе, второго альбома нет.
	shareAndWait(t, b, doneB, ShareOptions{Choice: "update"})
	vb, err = b.ShareState()
	if err != nil || vb.Link == nil || vb.Link.State != "ok" || !vb.Link.Own || vb.Link.URL != url || srv.Fake.Albums() != 1 {
		t.Fatalf("после обновления: %v %+v, альбомов %d", err, vb.Link, srv.Fake.Albums())
	}
	if got := fileRefsOf(t, file); !slices.Equal(got, refs) {
		t.Errorf("привязки в файле изменились: %v -> %v", refs, got)
	}
}
