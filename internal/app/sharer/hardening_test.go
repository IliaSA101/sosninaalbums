package sharer

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// reqLog пишет запросы PATCH и publish к альбому в порядке прихода: «PATCH <тело>» и «publish».
func reqLog(r *rig, patchFault, publishFault *albumstest.Fault) func() []string {
	var (
		mu  sync.Mutex
		log []string
	)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case q.Method == http.MethodPatch:
			log = append(log, "PATCH "+bodyOf(q))
			return patchFault
		case isPublish(q):
			log = append(log, "publish")
			return publishFault
		}
		return nil
	})
	return func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), log...) }
}

// changeScene — снимок, отличный от выгруженного: «Обновить» грузит страницы.
func changeScene(t *testing.T, r *rig) {
	t.Helper()
	sp := r.curJob().Spreads
	sp[1].Bg[0].B, sp[1].Bg[1].B = 250, 251
	r.setJob(jobOf(t, r.curJob().Path, sp))
}

// Новый пароль прежней ссылки ставится ДО публикации новой редакции; срок — после (аудит 29.10, п. 6).
func TestSharePasswordBeforePublish(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Password: "pass-1234", Days: 30})
	changeScene(t, r)
	get := reqLog(r, nil, nil)
	r.must(Options{Password: "new-5678", Days: 90})
	log := get()
	if len(log) != 3 || !strings.HasPrefix(log[0], "PATCH") || !strings.Contains(log[0], `"password":"new-5678"`) ||
		!strings.Contains(log[0], `"revoke_sessions":true`) || strings.Contains(log[0], "expires_at") ||
		log[1] != "publish" || !strings.Contains(log[2], "expires_at") || strings.Contains(log[2], "password") {
		t.Errorf("порядок запросов: %q, ждали PATCH пароля, publish, PATCH срока", log)
	}
	if rec, _ := r.svc.look(r.curJob().Path); rec.Password != "new-5678" {
		t.Errorf("пароль записи %q", rec.Password)
	}
}

// Упал PATCH пароля: публикации нет (редакция не открывается под старым паролем), запись прежняя.
func TestSharePasswordPatchFailsNoPublish(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Password: "pass-1234", Days: 30})
	changeScene(t, r)
	get := reqLog(r, &albumstest.Fault{Status: http.StatusBadRequest}, nil)
	if msg, stopped := r.run(Options{Password: "new-5678", Days: 90}); msg == "" || stopped {
		t.Fatalf("итог: %q stopped=%v, ждали ошибку", msg, stopped)
	}
	for _, l := range get() {
		if l == "publish" {
			t.Errorf("публикация после упавшего PATCH пароля: %q", get())
		}
	}
	if rec, _ := r.svc.look(r.curJob().Path); rec.Password != "pass-1234" {
		t.Errorf("пароль записи %q, ждали прежний", rec.Password)
	}
}

// Упала публикация после смены пароля: запись уже равна серверу (новый пароль), а не прежней.
func TestSharePublishFailsAfterPassword(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Password: "pass-1234", Days: 30})
	changeScene(t, r)
	reqLog(r, nil, &albumstest.Fault{Status: http.StatusBadRequest})
	if msg, _ := r.run(Options{Password: "new-5678", Days: 90}); msg == "" {
		t.Fatal("публикация прошла, ждали ошибку")
	}
	if rec, _ := r.svc.look(r.curJob().Path); rec.Password != "new-5678" {
		t.Errorf("пароль записи %q, ждали новый (он уже у сервиса)", rec.Password)
	}
}

// Пароль не менялся (обновление без окна): PATCH нет; новая ссылка — пароль идёт в Create.
func TestSharePasswordUnchangedNoPatch(t *testing.T) {
	r := newRig(t)
	r.connect()
	get := reqLog(r, nil, nil)
	r.must(Options{Password: "pass-1234", Days: 30}) // новая ссылка
	changeScene(t, r)
	r.must(Options{}) // та же ссылка, пароль и срок прежние
	if log := get(); len(log) != 2 || log[0] != "publish" || log[1] != "publish" {
		t.Errorf("запросы: %q, ждали два publish без PATCH", log)
	}
}

// Ошибка удаления временной папки попадает в журнал, без пути.
func TestRemoveDirLogsFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("запрет удаления через права каталога не работает")
	}
	parent := filepath.Join(testkit.TempDir(t), "parentdir")
	dir := filepath.Join(parent, "up-1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	buf := captureLog(t)
	removeDir(dir)
	if !strings.Contains(buf.String(), "временная папка не удалена") || strings.Contains(buf.String(), "parentdir") {
		t.Errorf("журнал: %q", buf.String())
	}
	buf.Reset()
	removeDir("") // пусто — ничего
	if buf.Len() != 0 {
		t.Errorf("пустой путь написал в журнал: %q", buf.String())
	}
}

// Пустая ссылка не годна (CheckURL("") её пропускает); обычная проходит, слишком длинная — нет.
func TestCheckLinkEmpty(t *testing.T) {
	r := newRig(t)
	if err := r.svc.checkLink(""); err == nil {
		t.Error("пустая ссылка принята")
	}
	if err := r.svc.checkLink("https://albums.example.ru/a/abc"); err != nil {
		t.Errorf("обычная ссылка: %v", err)
	}
	if err := r.svc.checkLink("https://albums.example.ru/" + strings.Repeat("a", 5000)); err == nil {
		t.Error("слишком длинная ссылка принята")
	}
}
