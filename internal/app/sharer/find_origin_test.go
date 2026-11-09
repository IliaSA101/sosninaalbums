package sharer

// 167 T3b: поиск и записи чужого адреса. В файле проекта адресов нет вовсе: запросы уходят только
// на подключённый сервис и с его токеном; плохой список — «поиск не удался», а не паника и не тупик.

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// listProxy — сервер перед rig.srv: все запросы проходят насквозь, кроме GET /v1/albums, когда
// *body непуст или bad включён, — на него отвечает *body.
func listProxy(t *testing.T, r *rig, body *atomic.Pointer[string]) string {
	t.Helper()
	target, err := url.Parse(r.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	px := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if b := body.Load(); b != nil && q.Method == http.MethodGet && q.URL.Path == "/v1/albums" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(*b))
			return
		}
		rp.ServeHTTP(w, q)
	}))
	t.Cleanup(px.Close)
	return px.URL
}

func TestShareFindBadList(t *testing.T) {
	for name, body := range map[string]string{
		"битый":       "{не json",
		"больше 1 МБ": strings.Repeat(" ", 2<<20) + "[]",
		"пустой":      "",
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			var bad atomic.Pointer[string] // пока nil — прокси честный: Connect проверяет сервис списком
			if _, err := r.svc.Connect(listProxy(t, r, &bad), r.srv.Token); err != nil {
				t.Fatal(err)
			}
			bad.Store(&body)
			j := r.curJob()
			j.Refs = []string{model.LinkRef(j.UID, "x")}
			r.setJob(j)
			v, err := r.svc.Find()
			if err != nil || v.Album.Find != "fail" || v.Album.FindErr != "Сервис ответил неверно" {
				t.Fatalf("Find: %v, %q %q", err, v.Album.Find, v.Album.FindErr)
			}
			r.must(Options{Days: 30}) // «как новый» доступен: выгрузка без поиска, uid новый
			if r.fresh != 1 || r.srv.Fake.Albums() != 1 {
				t.Errorf("fresh %d, альбомов %d", r.fresh, r.srv.Fake.Albums())
			}
		})
	}
}

// foreignServer — сервер «адреса из записи»: считает запросы.
func foreignServer(t *testing.T) (u string, hits *atomic.Int32) {
	t.Helper()
	hits = new(atomic.Int32)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(s.Close)
	return s.URL, hits
}

// Запись проекта несёт чужой адрес: ни одного запроса туда — ни поиск, ни правка, ни отзыв.
func TestShareFindFileOriginNoRequest(t *testing.T) {
	r := newRig(t)
	r.connect()
	foreign, hits := foreignServer(t)
	j := r.curJob()
	j.Refs = []string{model.LinkRef(j.UID, "x")}
	r.setJob(j)
	rec := prefs.ProjectRecord{Origin: foreign, ID: "LiZ9u4CAHMYsK8nxXrvchR", Title: "А", URL: foreign + "/a/LiZ9u4CAHMYsK8nxXrvchR/",
		ExpiresAt: r.svc.now().AddDate(0, 0, 10), Path: j.Path}
	if err := r.svc.put(j.UID, rec); err != nil {
		t.Fatal(err)
	}
	if v := r.state(); v.Link == nil || v.Link.State != "other" {
		t.Fatalf("запись чужого адреса: %+v", v.Link)
	}
	if _, err := r.svc.Find(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Change(Change{Days: 30}); err == nil {
		t.Error("Change ушёл на чужой адрес")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("Revoke ушёл на чужой адрес")
	}
	// Без записи: поиск идёт только на подключённый адрес.
	if err := r.svc.drop(j.UID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Find(); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Errorf("на чужой адрес пришло запросов: %d", hits.Load())
	}
}

// Запись другого адреса (сервис переехал) не мешает «Выгрузить»: новая ссылка создаётся на
// подключённом сервисе (запись своя — uid тот же, id новый), чужой адрес не опрашивается.
func TestShareLinksOtherOrigin(t *testing.T) {
	r := newRig(t)
	r.connect()
	foreign, hits := foreignServer(t)
	j := r.curJob()
	rec := prefs.ProjectRecord{Origin: foreign, ID: "LiZ9u4CAHMYsK8nxXrvchR", Title: "Клиент А", Password: "old-1234",
		URL: foreign + "/a/LiZ9u4CAHMYsK8nxXrvchR/", ExpiresAt: r.svc.now().AddDate(0, 0, 10), Path: j.Path}
	if err := r.svc.put(j.UID, rec); err != nil {
		t.Fatal(err)
	}
	v := r.state()
	if v.Link == nil || v.Link.State != "other" || v.Link.Host == "" {
		t.Fatalf("состояние: %+v", v.Link)
	}
	r.must(Options{Days: 30})
	nu := r.curJob().UID
	got, _ := r.svc.look(nu)
	if nu != j.UID || got.ID == rec.ID || got.Origin != r.srv.URL || got.Password != "" || r.srv.Fake.Albums() != 1 || hits.Load() != 0 {
		t.Errorf("новая ссылка: uid прежний %v, %+v, альбомов %d, запросов на чужой %d", nu == j.UID, got, r.srv.Fake.Albums(), hits.Load())
	}
}

// Запись не своя (другой путь): Revoke и Change отказывают до сети, Delete и Patch не уходят.
func TestShareMovedRevokeChangeRefused(t *testing.T) {
	r := newRig(t)
	r.upload("Клиент А", "old-1234")
	r.moveTo("b.salbum")
	var writes atomic.Int32
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodDelete || q.Method == http.MethodPatch {
			writes.Add(1)
		}
		return nil
	})
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("Revoke с другого пути прошёл")
	}
	if _, err := r.svc.Change(Change{Days: 90, SetPassword: true, Password: "x"}); err == nil {
		t.Error("Change с другого пути прошёл")
	}
	if writes.Load() != 0 || len(r.asks) != 0 || r.srv.Fake.Albums() != 1 {
		t.Errorf("Delete/Patch %d, вопросов %d, альбомов %d", writes.Load(), len(r.asks), r.srv.Fake.Albums())
	}
}
