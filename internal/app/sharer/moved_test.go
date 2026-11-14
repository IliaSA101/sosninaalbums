package sharer

// 167: одна ссылка на проект. Запись по uid, «своя» только у файла своего пути (167-12): перенос,
// копия и другой компьютер дают экран moved и выбор «обновить / как новый».

import (
	"bytes"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// moveTo переносит текущий проект в файл name той же папки (тот же uid и привязки).
func (r *rig) moveTo(name string) {
	j := r.curJob()
	j.Path = filepath.Join(filepath.Dir(j.Path), name)
	j.Title = strings.TrimSuffix(name, ".salbum")
	r.setJob(j)

}

func (r *rig) shareErr(o Options) error {
	r.t.Helper()
	ok, err := r.svc.Share(o)
	if ok {
		r.t.Fatalf("Share(%+v) стартовал", o)
	}
	return err
}

// upload — первая выгрузка с паролем и введённым названием; возвращает id и uid.
func (r *rig) upload(title, password string) (id, uid string) {
	r.t.Helper()
	r.connect()
	r.must(Options{Days: 30, Title: title, Password: password})
	id, _ = r.rec()
	return id, r.curJob().UID
}

func TestShareMovedUpdateSameID(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	old := r.curJob().Path
	r.moveTo("b.salbum")
	v := r.state()
	if v.Link == nil || v.Link.State != "moved" || v.Link.Own || v.Link.URL == "" {
		t.Fatalf("после переноса: %+v", v.Link)
	}
	if !strings.Contains(v.Link.From, filepath.Base(old)) || !strings.Contains(v.Link.Here, "b.salbum") || !strings.HasPrefix(v.Link.Published, "Выгружено") {
		t.Errorf("строки показа: %+v", v.Link)
	}
	if err := r.shareErr(Options{}); err == nil { // TestShareMovedNoChoice
		t.Fatal("moved без выбора принят")
	}
	h := &headerLog{}
	r.setFault(h.fault)
	r.must(Options{Choice: "update"})
	h.onlyToken(t, r.srv.Token) // обновление ушло на подключённый сервис и только с его токеном
	if got, _ := r.rec(); got != id || r.srv.Fake.Albums() != 1 {
		t.Fatalf("id %q (был %q), альбомов %d", got, id, r.srv.Fake.Albums())
	}
	if rec, _ := r.svc.look(uid); !rec.Own(r.curJob().Path) {
		t.Errorf("запись не переехала на новый путь: %q", rec.Path)
	}
	// TestShareMovedRemembered: дальше без вопроса и без выбора.
	if v := r.state(); v.Link == nil || v.Link.State != "ok" || !v.Link.Own {
		t.Fatalf("после «Обновить»: %+v", v.Link)
	}
	r.must(Options{})
}

func TestShareMovedNoPasswordInLink(t *testing.T) {
	r := newRig(t)
	r.upload("Клиент А", "old-1234")
	r.moveTo("b.salbum")
	v := r.state()
	if v.Link == nil || v.Link.Password != "" || v.Link.Title != "" || v.Link.Own || v.Album.Title != "b" {
		t.Fatalf("секреты прежнего клиента в Link: %+v, название %q", v.Link, v.Album.Title)
	}
	h := &headerLog{}
	r.setFault(h.fault)
	if _, err := r.svc.Text(); err == nil { // TestShareMovedTextRefused
		t.Error("Text отдал текст чужой записи")
	}
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Error("Change прошёл")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("Revoke прошёл")
	}
	if h.n != 0 || len(r.asks) != 0 {
		t.Errorf("запросов %d, вопросов %d", h.n, len(r.asks))
	}
	if err := r.svc.Open(); err != nil { // Open можно
		t.Errorf("Open: %v", err)
	}
}

func TestShareMovedNewFreshUID(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("Клиент А", "old-1234")
	r.moveTo("b.salbum")
	if err := r.shareErr(Options{Choice: "new"}); err == nil { // срок обязателен у новой ссылки
		t.Fatal("новая ссылка без срока")
	}
	r.must(Options{Choice: "new", Days: 90})
	nu := r.curJob().UID
	if nu == uid || r.fresh != 1 || r.srv.Fake.Albums() != 2 {
		t.Fatalf("новый uid %v, fresh %d, альбомов %d", nu != uid, r.fresh, r.srv.Fake.Albums())
	}
	nrec, _ := r.svc.look(nu)
	if nrec.ID == id || nrec.Password != "" || nrec.Title == "Клиент А" || !nrec.Own(r.curJob().Path) {
		t.Errorf("запись новой ссылки (TestShareNewFromMovedNoOldSecrets): %+v", nrec)
	}
	if old, _ := r.svc.look(uid); old.ID != id || old.Password != "old-1234" {
		t.Errorf("прежняя запись тронута: %+v", old)
	}
}

func TestShareChoiceOnlyOnMoved(t *testing.T) {
	r := newRig(t)
	r.upload("", "")
	for _, c := range []string{"update", "new"} {
		if err := r.shareErr(Options{Choice: c, Days: 30}); err == nil {
			t.Errorf("choice %q принят на своей записи", c)
		}
	}
	if r.fresh != 0 || r.srv.Fake.Albums() != 1 {
		t.Errorf("fresh %d, альбомов %d", r.fresh, r.srv.Fake.Albums())
	}
	r.moveTo("b.salbum")
	if err := r.shareErr(Options{Choice: "что-то"}); err == nil {
		t.Error("негодный choice принят")
	}
}

func TestShareMovedUpdate404Stops(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	r.moveTo("b.salbum")
	var posts int
	r.setFault(func(q *http.Request) *albumstest.Fault {
		switch {
		case q.Method == http.MethodPut && !isPut(q):
			return &albumstest.Fault{Status: http.StatusNotFound}
		case q.Method == http.MethodPost:
			posts++
		}
		return nil
	})
	msg, _ := r.run(Options{Choice: "update"})
	if !strings.Contains(msg, movedGoneText) || posts != 0 {
		t.Fatalf("итог %q, Create %d", msg, posts)
	}
	rec, _ := r.svc.look(uid)
	if rec.ID != id || rec.Gone || rec.Own(r.curJob().Path) {
		t.Errorf("запись после остановки: %+v", rec)
	}
}

func TestShareChoiceAlbumChanged(t *testing.T) {
	r := newRig(t)
	r.upload("", "")
	r.moveTo("b.salbum")
	cur := r.curJob()
	cur.Questions = func() []string { // пока идут вопросы, открыли другой альбом
		o := cur
		o.UID = uidOf("другой")
		r.setJob(o)
		return []string{"вопрос"}
	}
	r.setJob(cur)
	err := r.shareErr(Options{Choice: "update"})
	if err == nil || !strings.Contains(err.Error(), changedText) {
		t.Fatalf("ошибка: %v", err)
	}
}

func TestShareExpiredOtherPathNoOldSecrets(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("Клиент А", "old-1234")
	rec, _ := r.svc.look(uid)
	rec.ExpiresAt = time.Now().Add(-time.Hour)
	if err := r.svc.put(uid, rec); err != nil {
		t.Fatal(err)
	}
	r.moveTo("b.salbum")
	v := r.state()
	if v.Link == nil || v.Link.State != "expired" || v.Link.Own || v.Link.Password != "" || v.Link.Title != "" {
		t.Fatalf("expired с другого пути: %+v", v.Link)
	}
	if err := r.shareErr(Options{Choice: "update", Days: 30}); err == nil {
		t.Error("choice принят вне moved")
	}
	r.must(Options{Days: 30})
	nu := r.curJob().UID
	got, _ := r.svc.look(nu)
	if nu == uid || got.ID == id || got.Password != "" || got.Title == "Клиент А" {
		t.Errorf("новая ссылка унесла секреты: uid новый %v, %+v", nu != uid, got)
	}
}

func TestShareTitleFollowsFile(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("", "")
	r.moveTo("Петров.salbum")
	r.must(Options{Choice: "update"})
	if rec, _ := r.svc.look(uid); rec.Title != "Петров" {
		t.Errorf("название не пошло за файлом: %q", rec.Title)
	}
}

func TestShareTitleTypedKept(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("Мария и Пётр", "")
	r.moveTo("Петров.salbum")
	r.must(Options{Choice: "update"})
	if rec, _ := r.svc.look(uid); rec.Title != "Мария и Пётр" {
		t.Errorf("введённое название затёрто: %q", rec.Title)
	}
}

func TestShareBindAfterCreate(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	if len(r.binds) != 1 || r.binds[0] != id {
		t.Fatalf("Bind: %v", r.binds)
	}
	if refs := r.curJob().Refs; len(refs) != 1 || refs[0] != model.LinkRef(uid, id) {
		t.Errorf("привязка: %v", refs)
	}
	r.must(Options{}) // уже есть в файле — второй раз не просим
	if len(r.binds) != 1 {
		t.Errorf("повторный Bind: %v", r.binds)
	}
}

// secondComputer — запись на этом компьютере стёрта, в файле остались привязки.
func (r *rig) secondComputer(uid string) {
	r.t.Helper()
	if err := r.svc.drop(uid); err != nil {
		r.t.Fatal(err)
	}
	r.svc.mu.Lock()
	r.svc.fnd = map[string]findState{}
	r.svc.mu.Unlock()
}

func TestShareFindFound(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "pass-1234")
	r.secondComputer(uid)
	if v := r.state(); v.Link != nil || v.Album.Find != "need" {
		t.Fatalf("до поиска: find %q, link %+v", v.Album.Find, v.Link)
	}
	v, err := r.svc.Find()
	if err != nil || v.Link == nil || v.Link.State != "moved" || v.Album.Find != "" {
		t.Fatalf("Find: %v, %+v", err, v)
	}
	if v.Link.From != "На другом компьютере или из другого файла" || v.Link.Password != "" {
		t.Errorf("строки: %+v", v.Link)
	}
	if rec, _ := r.svc.look(uid); rec.ID != id || !rec.PassUnknown || rec.Path != "" {
		t.Fatalf("найденная запись: %+v", rec)
	}
	// «Обновить»: тот же id, пароль не снят.
	h := &headerLog{}
	r.setFault(h.fault)
	r.must(Options{Choice: "update"})
	h.onlyToken(t, r.srv.Token)
	if got, _ := r.rec(); got != id || r.srv.Fake.Albums() != 1 {
		t.Fatalf("id %q, альбомов %d", got, r.srv.Fake.Albums())
	}
	if rec, _ := r.svc.look(uid); !rec.PassUnknown || rec.Password != "" || !rec.Own(r.curJob().Path) {
		t.Errorf("запись после обновления: %+v", rec)
	}
}

func TestShareFindLost(t *testing.T) {
	r := newRig(t)
	r.connect()
	j := r.curJob()
	j.Refs = []string{model.LinkRef(j.UID, "нет-такого-альбома")}
	r.setJob(j)
	v, err := r.svc.Find()
	if err != nil || v.Album.Find != "lost" || v.Link != nil {
		t.Fatalf("Find: %v, %q", err, v.Album.Find)
	}
	// «Выгрузить»: новая ссылка с новым uid.
	r.must(Options{Days: 30})
	if r.fresh != 1 || r.curJob().UID == j.UID || r.srv.Fake.Albums() != 1 {
		t.Errorf("fresh %d, альбомов %d", r.fresh, r.srv.Fake.Albums())
	}
}

func TestShareFindNetFail(t *testing.T) {
	r := newRig(t)
	r.connect()
	j := r.curJob()
	j.Refs = []string{model.LinkRef(j.UID, "x")}
	r.setJob(j)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodGet && q.URL.Path == "/v1/albums" {
			return &albumstest.Fault{Status: http.StatusInternalServerError}
		}
		return nil
	})
	v, err := r.svc.Find()
	if err != nil || v.Album.Find != "fail" || v.Album.FindErr == "" {
		t.Fatalf("Find: %v, %q %q", err, v.Album.Find, v.Album.FindErr)
	}
	// TestShareFindFailNewAllowed: выгрузка без поиска идёт, uid новый.
	r.setFault(nil)
	r.must(Options{Days: 30})
	if r.fresh != 1 || r.srv.Fake.Albums() != 1 {
		t.Errorf("fresh %d, альбомов %d", r.fresh, r.srv.Fake.Albums())
	}
}

func TestShareFindNeedsToken(t *testing.T) {
	r := newRig(t)
	j := r.curJob()
	j.Refs = []string{model.LinkRef(j.UID, "x")}
	r.setJob(j)
	if _, err := r.svc.Find(); err == nil {
		t.Error("поиск без подключения")
	}
	if v := r.state(); v.Album.Find != "" {
		t.Errorf("find без подключения: %q", v.Album.Find)
	}
}

func TestShareShareSearchesFirst(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("", "")
	r.secondComputer(uid)
	err := r.shareErr(Options{Days: 30}) // «Выгрузить» без окна поиска: ищет сам и не создаёт второй альбом
	if err == nil || !strings.Contains(err.Error(), "уже выгружен") || r.srv.Fake.Albums() != 1 {
		t.Fatalf("ошибка %v, альбомов %d", err, r.srv.Fake.Albums())
	}
}

func TestShareFindLogNoSecrets(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("Клиент А", "")
	r.secondComputer(uid)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(testLogOut) })
	if _, err := r.svc.Find(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, secret := range []string{id, uid, model.LinkRef(uid, id), "Клиент А", r.srv.URL, r.curJob().Path} {
		if strings.Contains(out, secret) {
			t.Errorf("в журнале поиска %q", secret)
		}
	}
}

var testLogOut = log.Writer()

func TestShareFindSwitchAlbum(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	r.secondComputer(uid)
	first := r.curJob()
	other := testJob(t, filepath.Join(filepath.Dir(first.Path), "z.salbum"), 2, 3)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodGet && q.URL.Path == "/v1/albums" {
			r.setJob(other) // пока идёт List, открыли другой альбом
		}
		return nil
	})
	if _, err := r.svc.Find(); err != nil {
		t.Fatal(err)
	}
	if rec, ok := r.svc.look(uid); !ok || rec.ID != id {
		t.Errorf("запись по uid снимка: %+v %v", rec, ok)
	}
	if v := r.state(); v.Link != nil || v.Album.Find != "" {
		t.Errorf("у текущего альбома find %q, link %+v", v.Album.Find, v.Link)
	}
}

func TestShareFindRefreshExpired(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("", "")
	rec, _ := r.svc.look(uid)
	rec.ExpiresAt = time.Now().Add(-time.Hour) // на этом компьютере срок вышел, на сервисе продлён
	if err := r.svc.put(uid, rec); err != nil {
		t.Fatal(err)
	}
	if v := r.state(); v.Link.State != "expired" || v.Album.Find != "need" {
		t.Fatalf("до поиска: %q find %q", v.Link.State, v.Album.Find)
	}
	v, err := r.svc.Find()
	if err != nil || v.Link == nil || v.Link.State == "expired" || v.Album.Find != "" {
		t.Fatalf("после поиска: %v %+v", err, v.Link)
	}
}

func TestShareGuardsUnlooked(t *testing.T) {
	r := newRig(t) // службу не подключали и записей нет: действия отказывают, а не паникуют
	if _, err := r.svc.Text(); err == nil {
		t.Error("Text без записи")
	}
	if _, err := r.svc.Change(Change{Days: 30}); err == nil {
		t.Error("Change без записи")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("Revoke без записи")
	}
	if err := r.svc.Open(); err == nil {
		t.Error("Open без записи")
	}
	if ref := r.svc.Opened(r.curJob().Path, r.curJob().UID); ref != "" {
		t.Errorf("Opened без записи: %q", ref)
	}
}
